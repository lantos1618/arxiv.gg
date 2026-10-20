package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	arxiv "github.com/lantos1618/arxiv.gg"
)

func adSenseTestServer() *server {
	return &server{googleAdSenseClientID: "ca-pub-9693175657690620", googleAdSenseAdSlot: "1234567890", googleAdSenseAdsEnabled: true, googleAnalyticsID: "G-TEST123"}
}

func adSenseTestData() map[string]any {
	return map[string]any{"Title": "Test", "Stats": &arxiv.CacheStats{}, "Paper": &arxiv.Paper{ID: "2501.00001", Title: "Test paper", Abstract: "Research abstract"}}
}

func TestAdSenseServingGates(t *testing.T) {
	for _, flags := range [][2]string{{"", ""}, {"true", ""}, {"", "true"}, {"true", "false"}, {"false", "true"}, {"TRUE", "true"}, {"true", "true"}} {
		t.Setenv("GOOGLE_ADSENSE_ADS_ENABLED", flags[0])
		t.Setenv("GOOGLE_ADSENSE_CONSENT_READY", flags[1])
		if got, want := configuredAdSenseAdsEnabled(), flags[0] == "true" && flags[1] == "true"; got != want {
			t.Fatalf("flags=%v got=%v", flags, got)
		}
	}
	for _, slot := range []string{"", "123\ninjected", "<script>", "-1", "123456789012345678901"} {
		t.Setenv("GOOGLE_ADSENSE_AD_SLOT", slot)
		if configuredAdSenseAdSlot() != "" {
			t.Fatalf("accepted slot %q", slot)
		}
	}
	for _, change := range []func(*server){
		func(s *server) { s.googleAdSenseAdsEnabled = false },
		func(s *server) { s.googleAdSenseClientID = "invalid" },
		func(s *server) { s.googleAdSenseAdSlot = "" },
		func(s *server) { s.localMode = true },
	} {
		s := adSenseTestServer()
		change(s)
		w := httptest.NewRecorder()
		s.renderTemplate(w, httptest.NewRequest("GET", "/", nil), "index", adSenseTestData())
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if strings.Contains(w.Body.String(), "adsbygoogle") || strings.Contains(w.Body.String(), "ad-placement") {
			t.Fatal("ad code leaked through disabled configuration")
		}
	}
}

func TestAdSenseNonceMatchesEveryScriptAndBypassesCache(t *testing.T) {
	for _, page := range []struct {
		path, name string
		placements int
	}{{"/", "index", 1}, {"/abs/2501.00001", "paper", 1}, {"/privacy", "privacy", 0}} {
		t.Run(page.name, func(t *testing.T) {
			cm := newCacheMiddleware(time.Minute)
			s := adSenseTestServer()
			handler := securityHeadersMiddleware(cm.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.renderTemplate(w, r, page.name, adSenseTestData()) })))
			previousNonce := ""
			for i := 0; i < 2; i++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", page.path, nil)
				req.Header.Set("If-None-Match", `"old-cache-entry"`)
				handler.ServeHTTP(w, req)
				if w.Code != 200 {
					t.Fatal(w.Body.String())
				}
				if len(w.Header().Values("Content-Security-Policy")) != 1 {
					t.Fatal("multiple CSP policies block the ad integration")
				}
				csp := w.Header().Get("Content-Security-Policy")
				match := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(csp)
				if len(match) != 2 {
					t.Fatalf("missing CSP nonce: %s", csp)
				}
				nonce := match[1]
				if len(nonce) < 32 || nonce == previousNonce {
					t.Fatal("nonce missing entropy or reused")
				}
				previousNonce = nonce
				if w.Header().Get("Cache-Control") != "private, no-store, no-transform" || len(cm.cache) != 0 {
					t.Fatal("nonce-bearing document was cacheable")
				}
				for _, required := range []string{"'strict-dynamic'", "object-src 'none'", "base-uri 'none'", "frame-ancestors 'none'"} {
					if !strings.Contains(csp, required) {
						t.Errorf("CSP missing %s", required)
					}
				}
				body := w.Body.String()
				for _, script := range regexp.MustCompile(`<script\b[^>]*>`).FindAllString(body, -1) {
					if !strings.Contains(script, `nonce="`+nonce+`"`) {
						t.Errorf("script not covered by CSP: %s", script)
					}
				}
				if strings.Contains(body, "ZgotmplZ") {
					t.Fatal("template rejected attribute")
				}
				if strings.Count(body, "pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-9693175657690620") != 1 {
					t.Fatal("missing or duplicate loader")
				}
				if strings.Count(body, `data-ad-slot="1234567890"`) != page.placements {
					t.Fatal("wrong placement count")
				}
			}
		})
	}
}

func TestAdSenseExcludesOtherPages(t *testing.T) {
	s := adSenseTestServer()
	for _, path := range []string{"/login", "/account", "/admin", "/auth/google/callback", "/api/v1/", "/search?q=private", "/feedback", "/categories", "/author/test", "/abs/2501.00001"} {
		w := httptest.NewRecorder()
		s.renderTemplate(w, httptest.NewRequest("GET", path, nil), "head", adSenseTestData())
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if strings.Contains(w.Body.String(), "adsbygoogle") || strings.Contains(w.Body.String(), "googletagmanager.com") || w.Header().Get("Content-Security-Policy") != "" {
			t.Fatalf("ads enabled on %s", path)
		}
	}
}

func TestAdSenseConsentCallbacks(t *testing.T) {
	w := httptest.NewRecorder()
	adSenseTestServer().renderTemplate(w, httptest.NewRequest("GET", "/privacy", nil), "privacy", adSenseTestData())
	var analytics, controls, ads string
	for _, match := range regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`).FindAllStringSubmatch(w.Body.String(), -1) {
		if strings.Contains(match[1], "const permitted") {
			ads = match[1]
		}
		if strings.Contains(match[1], "function startAnalytics") {
			analytics = match[1]
		}
		if strings.Contains(match[1], "showRevocationMessage") {
			controls = match[1]
		}
	}
	if analytics == "" || controls == "" || ads == "" {
		t.Fatal("missing consent integration")
	}
	d, _ := json.Marshal(ads)
	a, _ := json.Marshal(analytics)
	c, _ := json.Marshal(controls)
	runFrontendReviewJavaScript(t, `
const assert = require('node:assert/strict');
const vm = require('node:vm');
const appended = [];
const button = {hidden: true, addEventListener: (_, cb) => { button.click = cb; }};
let state = 0, revoked = 0, reloads = 0, tcf;
const context = {window: {
 location: {reload: () => reloads++},
 __tcfapi: (cmd, version, cb) => { assert.equal(cmd, 'addEventListener'); assert.equal(version, 2); tcf = cb; },
 googlefc: {
  callbackQueue: [],
  getGoogleConsentModeValues: () => ({analyticsStoragePurposeConsentStatus: state, adStoragePurposeConsentStatus: state}),
  ConsentModePurposeStatusEnum: {CONSENT_MODE_PURPOSE_STATUS_GRANTED: 1, CONSENT_MODE_PURPOSE_STATUS_NOT_APPLICABLE: 3},
  showRevocationMessage: () => revoked++,
 }
}, document: {getElementById: () => button, createElement: () => ({}), head: {appendChild: s => appended.push(s)}}};
vm.createContext(context);
vm.runInContext(`+string(d)+`, context);
assert.equal(context.window.adsbygoogle.pauseAdRequests, 1, 'requests paused before CMP');
const adsReady = context.window.googlefc.callbackQueue.shift().CONSENT_MODE_DATA_READY;
for (const denied of [undefined, 0, 2, 4, 99]) { state = denied; adsReady(); assert.equal(context.window.adsbygoogle.pauseAdRequests, 1); }
for (const permitted of [1, 3]) { state = permitted; adsReady(); assert.equal(context.window.adsbygoogle.pauseAdRequests, 0); }
state = 2; adsReady(); assert.equal(context.window.adsbygoogle.pauseAdRequests, 1, 'refusal pauses requests');
state = 0;
vm.runInContext(`+string(a)+`, context);
assert.equal(appended.length, 0, 'no analytics before CMP');
const ready = context.window.googlefc.callbackQueue.shift().CONSENT_MODE_DATA_READY;
for (const denied of [undefined, 0, 2, 4, 99]) { state = denied; ready(); assert.equal(appended.length, 0, 'unknown/denied/unconfigured consent blocks analytics'); }
state = 1; ready(); ready();
assert.equal(appended.length, 1, 'granted consent starts analytics exactly once');
assert.equal(context.window.dataLayer[0][0], 'consent');
assert.equal(context.window.dataLayer[0][2].ad_storage, 'denied');
vm.runInContext(`+string(c)+`, context);
context.window.googlefc.callbackQueue.shift().CONSENT_API_READY();
tcf(null, false); assert.equal(button.hidden, true);
tcf({gdprApplies: false}, true); assert.equal(button.hidden, true);
tcf({gdprApplies: true, eventStatus: 'tcloaded'}, true); assert.equal(button.hidden, false);
context.window.adsbygoogle.pauseAdRequests = 0;
button.click();
assert.equal(context.window.adsbygoogle.pauseAdRequests, 1, 'reopening pauses new requests');
assert.equal(context.window['ga-disable-G-TEST123'], true);
context.window.googlefc.callbackQueue.shift().CONSENT_API_READY();
assert.equal(revoked, 1);
tcf({gdprApplies: true, eventStatus: 'useractioncomplete'}, true);
assert.equal(reloads, 1, 'new page applies changed consent from a clean state');
`)
}
