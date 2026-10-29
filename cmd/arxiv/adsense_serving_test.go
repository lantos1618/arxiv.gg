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
				cmp := "https://fundingchoicesmessages.google.com/i/pub-9693175657690620?ers=1"
				if strings.Count(body, cmp) != 1 || strings.Index(body, cmp) > strings.Index(body, "https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js") {
					t.Fatal("consent must bootstrap independently before the ad loader")
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
		if strings.Contains(w.Body.String(), "adsbygoogle") || w.Header().Get("Content-Security-Policy") != "" {
			t.Fatalf("ads enabled on %s", path)
		}
		if got := strings.Contains(w.Body.String(), "googletagmanager.com"); got != thirdPartyScriptsAllowed(httptest.NewRequest("GET", path, nil).URL.Path) {
			t.Fatalf("wrong analytics visibility on %s", path)
		}
	}
}

func TestAdSenseConsentCallbacks(t *testing.T) {
	w := httptest.NewRecorder()
	adSenseTestServer().renderTemplate(w, httptest.NewRequest("GET", "/privacy", nil), "privacy", adSenseTestData())
	var analytics, controls, ads string
	for _, match := range regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`).FindAllStringSubmatch(w.Body.String(), -1) {
		if strings.Contains(match[1], "pauseAdRequests = permitted") {
			ads = match[1]
		}
		if strings.Contains(match[1], "window.__arxivAnalyticsLoaded") {
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
  ConsentModePurposeStatusEnum: {GRANTED: 1, NOT_APPLICABLE: 3},
  showRevocationMessage: () => revoked++,
 }
}, document: {getElementById: () => button, createElement: () => ({}), head: {appendChild: s => appended.push(s)}}};
vm.createContext(context);
vm.runInContext(`+string(d)+`, context);
assert.equal(context.window.adsbygoogle.pauseAdRequests, 1, 'requests paused before CMP');
const adsReady = context.window.googlefc.callbackQueue.shift().CONSENT_MODE_DATA_READY;
for (const statuses of [{GRANTED: 1, NOT_APPLICABLE: 3}, {CONSENT_MODE_PURPOSE_STATUS_GRANTED: 1, CONSENT_MODE_PURPOSE_STATUS_NOT_APPLICABLE: 3}]) {
 context.window.googlefc.ConsentModePurposeStatusEnum = statuses;
 for (const denied of [undefined, 0, 2, 4, 99]) { state = denied; adsReady(); assert.equal(context.window.adsbygoogle.pauseAdRequests, 1); }
 for (const permitted of [1, 3]) { state = permitted; adsReady(); assert.equal(context.window.adsbygoogle.pauseAdRequests, 0); }
}
state = 2; adsReady(); assert.equal(context.window.adsbygoogle.pauseAdRequests, 1, 'refusal pauses requests');
state = 0;
vm.runInContext(`+string(a)+`, context);
assert.equal(appended.length, 1, 'cookieless analytics starts even if the CMP never becomes ready');
for (const purpose of ['analytics_storage', 'ad_storage', 'ad_user_data', 'ad_personalization']) {
 assert.equal(context.window.dataLayer[0][2][purpose], 'denied', 'storage denied before any measurement');
}
assert.equal(context.window.dataLayer[3][0], 'config');
assert.equal(context.window.dataLayer[3][2].allow_google_signals, false);
assert.equal(context.window.dataLayer[3][2].allow_ad_personalization_signals, false);
vm.runInContext(`+string(a)+`, context);
assert.equal(appended.length, 1, 'consent updates and rerenders never duplicate the loader');
vm.runInContext(`+string(c)+`, context);
context.window.googlefc.callbackQueue.shift().CONSENT_API_READY();
tcf(null, false); assert.equal(button.hidden, true);
tcf({gdprApplies: false}, true); assert.equal(button.hidden, true);
tcf({gdprApplies: true, eventStatus: 'tcloaded'}, true); assert.equal(button.hidden, false);
context.window.adsbygoogle.pauseAdRequests = 0;
button.click();
assert.equal(context.window.adsbygoogle.pauseAdRequests, 1, 'reopening pauses new requests');
assert.equal(context.window['ga-disable-G-TEST123'], undefined);
assert.equal(context.window.dataLayer.at(-1)[2].ad_storage, 'denied');
assert.equal(context.window.dataLayer.at(-1)[2].analytics_storage, undefined, 'advertising controls do not override the separate analytics choice');
context.window.googlefc.callbackQueue.shift().CONSENT_API_READY();
assert.equal(revoked, 1);
tcf({gdprApplies: true, eventStatus: 'useractioncomplete'}, true);
assert.equal(reloads, 1, 'new page applies changed consent from a clean state');
`)
}

func TestAnalyticsCookieChoice(t *testing.T) {
	w := httptest.NewRecorder()
	adSenseTestServer().renderTemplate(w, httptest.NewRequest("GET", "/privacy", nil), "privacy", adSenseTestData())
	var loader, controls string
	for _, match := range regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`).FindAllStringSubmatch(w.Body.String(), -1) {
		if strings.Contains(match[1], "window.__arxivAnalyticsLoaded") {
			loader = match[1]
		}
		if strings.Contains(match[1], "const key = 'arxiv.analytics-consent.v1'") {
			controls = match[1]
		}
	}
	if loader == "" || controls == "" {
		t.Fatal("missing analytics consent controls")
	}
	l, _ := json.Marshal(loader)
	c, _ := json.Marshal(controls)
	runFrontendReviewJavaScript(t, `
const assert = require('node:assert/strict');
const vm = require('node:vm');
const preferences = new Map();
const key = 'arxiv.analytics-consent.v1';
function page(blocked = false) {
 const elements = {};
 for (const id of ['analytics-consent-banner', 'analytics-privacy-settings', 'analytics-consent-allow', 'analytics-consent-deny']) {
  elements[id] = {hidden: true, attributes: {}, setAttribute(name, value) {this.attributes[name] = value;}, addEventListener(name, cb) {this[name] = cb;}, focus() {this.focused = true;}};
 }
 const expired = [], appended = [];
 let reloads = 0;
 const document = {getElementById: id => elements[id], createElement: () => ({}), head: {appendChild: script => appended.push(script)}};
 Object.defineProperty(document, 'cookie', {get: () => '_ga=old; _ga_TEST=old; session=keep', set: value => expired.push(value)});
 const context = {document, window: {location: {hostname: 'arxiv.gg', reload: () => reloads++}, localStorage: {
  getItem: key => {if (blocked) throw Error('blocked'); return preferences.get(key) || null;},
  setItem: (key, value) => {if (blocked) throw Error('blocked'); preferences.set(key, value);}
 }}};
 vm.createContext(context);
 vm.runInContext(`+string(l)+`, context);
 vm.runInContext(`+string(c)+`, context);
 return {context, elements, expired, appended, reloads: () => reloads};
}
let current = page();
assert.equal(current.appended.length, 1);
assert.equal(current.context.window.dataLayer[0][2].analytics_storage, 'denied');
assert.equal(current.elements['analytics-consent-banner'].hidden, false);
current.elements['analytics-consent-allow'].click();
assert.equal(preferences.get(key), 'granted');
assert.equal(current.context.window.dataLayer.at(-1)[2].analytics_storage, 'granted');
assert.equal(current.reloads(), 0, 'grant applies without unloading the current page');
assert.equal(current.elements['analytics-consent-banner'].hidden, true);
assert.equal(current.elements['analytics-privacy-settings'].attributes['aria-expanded'], 'false');
current.elements['analytics-privacy-settings'].click();
current.elements['analytics-consent-allow'].click();
assert.equal(current.reloads(), 0, 'repeated grants do not navigate');
assert.equal(current.appended.length, 1, 'grant does not load a second tag');
const grantedUpdate = current.context.window.dataLayer.at(-1);
current.context.window.gtag('event', 'qa_after_grant');
assert.equal(grantedUpdate[0], 'consent');
assert.equal(grantedUpdate[1], 'update');
assert.equal(grantedUpdate[2].analytics_storage, 'granted');
assert.equal(current.context.window.dataLayer.at(-1)[1], 'qa_after_grant', 'subsequent events use the existing page queue');
current = page();
assert.equal(current.context.window.dataLayer[0][2].analytics_storage, 'granted');
for (const purpose of ['ad_storage', 'ad_user_data', 'ad_personalization']) assert.equal(current.context.window.dataLayer[0][2][purpose], 'denied');
assert.equal(current.elements['analytics-consent-banner'].hidden, true);
current.elements['analytics-privacy-settings'].click();
assert.equal(current.elements['analytics-consent-banner'].hidden, false);
assert.equal(current.elements['analytics-consent-allow'].focused, true);
current.elements['analytics-consent-deny'].click();
assert.equal(preferences.get(key), 'denied');
assert.equal(current.context.window.dataLayer.at(-1)[2].analytics_storage, 'denied');
assert.equal(current.reloads(), 0, 'refusal applies without unloading the current page');
assert.equal(current.expired.length, 6);
assert.ok(current.expired.every(value => /^_ga(?:_|=)/.test(value) && value.includes('Max-Age=0')));
assert.ok(current.expired.every(value => !value.includes('session=')));
current.elements['analytics-privacy-settings'].click();
current.elements['analytics-consent-deny'].click();
assert.equal(current.reloads(), 0, 'repeated refusals do not navigate');
assert.equal(current.appended.length, 1);
const deniedUpdate = current.context.window.dataLayer.at(-1);
current.context.window.gtag('event', 'qa_after_refusal');
assert.equal(deniedUpdate[2].analytics_storage, 'denied');
assert.equal(current.context.window.dataLayer.at(-1)[1], 'qa_after_refusal');
for (const command of current.context.window.dataLayer) {
 if (command[0] === 'consent' && command[1] === 'update') {
  assert.equal(Object.keys(command[2]).join(','), 'analytics_storage', 'analytics choices never change advertising consent');
 }
}
current = page();
assert.equal(current.context.window.dataLayer[0][2].analytics_storage, 'denied');
assert.equal(current.elements['analytics-consent-banner'].hidden, true, 'refusal persists across navigation');
current = page(true);
assert.equal(current.context.window.dataLayer[0][2].analytics_storage, 'denied');
assert.equal(current.elements['analytics-consent-banner'].hidden, false);
current.elements['analytics-consent-allow'].click();
assert.equal(current.context.window.dataLayer.at(-1)[2].analytics_storage, 'granted', 'grant applies in place even when persistence fails');
assert.equal(preferences.get(key), 'denied', 'failed storage does not invent a saved grant');
assert.equal(current.reloads(), 0);
current.context.window.gtag('event', 'qa_after_storage_failure');
assert.equal(current.context.window.dataLayer.at(-1)[1], 'qa_after_storage_failure');
current = page(true);
assert.equal(current.context.window.dataLayer[0][2].analytics_storage, 'denied', 'unreadable storage fails closed on navigation');
current.elements['analytics-consent-deny'].click();
assert.equal(current.reloads(), 0, 'blocked storage remains usable without a reload loop');
assert.equal(current.context.window.dataLayer.at(-1)[2].analytics_storage, 'denied');
`)
}
