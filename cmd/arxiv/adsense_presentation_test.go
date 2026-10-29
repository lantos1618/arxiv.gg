package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestAdSensePresentationLifecycle(t *testing.T) {
	var body bytes.Buffer
	if err := templates.ExecuteTemplate(&body, "adsense_placement", map[string]any{"AdSenseAdSlot": "test", "GoogleAdSenseClientID": "simulation"}); err != nil {
		t.Fatal(err)
	}
	html := body.String()
	if !strings.Contains(html, `class="ad-placement-label" hidden`) || strings.Contains(html, `<aside`) {
		t.Fatal("pending placement must not expose an advertisement landmark/label")
	}
	match := regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`).FindStringSubmatch(html)
	if len(match) != 2 {
		t.Fatal("missing presentation controller")
	}
	source, _ := json.Marshal(match[1])
	runFrontendReviewJavaScript(t, `
const assert = require('node:assert/strict');
const vm = require('node:vm');
const source = `+string(source)+`;
const mutations = [], sizes = [], listeners = {}, placements = [];
let requests = 0, fail = false;
const context = {getComputedStyle: slot => ({visibility: slot.visibility || 'visible', opacity: slot.opacity || '1'}), window: {adsbygoogle: {push() { requests++; if (fail) throw Error('blocked'); }},
 addEventListener(name, fn) { (listeners[name] ||= []).push(fn); }}, document: {},
 MutationObserver: class { constructor(fn) { this.fn = fn; mutations.push(this); } observe(slot) { this.slot = slot; this.active = true; } disconnect() { this.active = false; } },
 ResizeObserver: class { constructor(fn) { this.fn = fn; sizes.push(this); } observe(slot) { this.slot = slot; this.active = true; } disconnect() { this.active = false; } }};
vm.createContext(context);
function create() {
 const classes = new Set(), attrs = new Map(), slotAttrs = new Map();
 const slot = {width: 800, height: 0, getAttribute: key => slotAttrs.get(key),
 getBoundingClientRect() { return {width: this.width, height: classes.has('ad-placement-error') ? 0 : this.height}; }};
 const label = {hidden: true};
 const placement = {slot, label, classes, attrs, slotAttrs,
 querySelector: selector => selector === '.adsbygoogle' ? slot : label,
 classList: {toggle(name, on) { if (on) classes.add(name); else classes.delete(name); }},
 setAttribute: (key, value) => attrs.set(key, value), removeAttribute: key => attrs.delete(key)};
 placements.push(placement);
 context.document.currentScript = {previousElementSibling: placement};
 vm.runInContext(source, context);
 return placement;
}
function status(p, value, height) { p.slotAttrs.set('data-ad-status', value); p.slot.height = height; mutations.find(m => m.slot === p.slot).fn(); }
function shown(p, expected) { assert.equal(!p.label.hidden, expected); assert.equal(p.classes.has('ad-placement-filled'), expected); assert.equal(p.attrs.get('aria-label'), expected ? 'Advertisement' : undefined); assert.equal(p.attrs.get('role'), expected ? 'complementary' : undefined); }
const first = create(); shown(first, false); assert.equal(first.classes.size, 0, 'pending is not force hidden');
first.slot.height = 250; mutations[0].fn(); shown(first, false); // iframe/height without provider fill is insufficient
status(first, 'unfilled', 0); shown(first, false);
status(first, 'filled', 250); shown(first, true); // delayed fill after unfilled
mutations[0].fn(); shown(first, true); assert.equal(requests, 1);
first.slot.height = 0; sizes[0].fn(); shown(first, false);
first.slot.height = 250; sizes[0].fn(); shown(first, true);
first.slot.width = 0; sizes[0].fn(); shown(first, false);
first.slot.width = 800; sizes[0].fn(); shown(first, true);
first.slot.visibility = 'hidden'; mutations[0].fn(); shown(first, false);
first.slot.visibility = 'visible'; mutations[0].fn(); shown(first, true);
first.slot.opacity = '0'; mutations[0].fn(); shown(first, false);
first.slot.opacity = '1'; mutations[0].fn(); shown(first, true);
const second = create(); shown(second, false); shown(first, true);
status(second, 'filled', 300); shown(second, true);
status(first, 'unfilled', 0); shown(first, false); shown(second, true);
status(second, 'unfill-optimized', 200); shown(second, false);
status(second, 'unknown', 200); shown(second, false);
fail = true; const failed = create(); shown(failed, false); assert.ok(failed.classes.has('ad-placement-error'));
status(failed, 'filled', 250); shown(failed, true); assert.ok(!failed.classes.has('ad-placement-error'), 'late fill recovers');
context.document.currentScript = {previousElementSibling: first}; vm.runInContext(source, context);
assert.equal(requests, 3, 'same placement never queues twice');
listeners.pagehide.forEach(fn => fn()); assert.ok(mutations.every(m => !m.active)); assert.ok(sizes.every(m => !m.active));
listeners.pageshow.forEach(fn => fn()); assert.ok(mutations.every(m => m.active)); shown(failed, true); assert.equal(requests, 3, 'BFCache does not queue again');
// Older browsers still react to provider status without ResizeObserver.
delete context.ResizeObserver; fail = false; const fallback = create(); status(fallback, 'filled', 250); shown(fallback, true);
`)
	var styles bytes.Buffer
	if err := templates.ExecuteTemplate(&styles, "adsense_styles", nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(styles.String(), "min-height") {
		t.Fatal("pending ads must not reserve artificial height")
	}
	if !strings.Contains(styles.String(), `[data-ad-status="unfilled"]`) {
		t.Fatal("missing provider-supported unfilled collapse")
	}
}

// Isolated browser fixture: actual presentation templates with synthetic status
// controls, no Google library, publisher ID, consent changes, or ad requests.
func serveAdPresentationFixture(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'none'; frame-src 'none'")
	w.Write([]byte(`<!doctype html><title>Ad presentation simulation</title><style>body{font:16px system-ui;max-width:850px;margin:40px auto;padding:0 16px}button{margin:4px;padding:8px}.simulation-filled{height:250px;background:#e5edf7;text-align:center;line-height:250px;text-decoration:none}</style><h1>Ad presentation simulation</h1><p>No Google scripts or ad requests. Each slot is independent.</p><script>window.adsbygoogle=[];</script>`))
	templates.ExecuteTemplate(w, "adsense_styles", nil)
	for _, id := range []string{"A", "B", "C (request error)"} {
		if strings.HasPrefix(id, "C") {
			w.Write([]byte(`<script>window.adsbygoogle={push(){throw Error('simulated request failure');}};</script>`))
		}
		w.Write([]byte(`<section><h2>Slot ` + id + `</h2><div class="controls">`))
		for _, state := range []string{"pending", "filled", "unfilled", "zero-size", "blocked", "unfill-optimized"} {
			w.Write([]byte(`<button type="button" data-state="` + state + `">` + id + `: ` + state + `</button>`))
		}
		w.Write([]byte(`</div><p>Content before placement.</p>`))
		templates.ExecuteTemplate(w, "adsense_placement", map[string]any{"AdSenseAdSlot": "simulation-" + id, "GoogleAdSenseClientID": "simulation"})
		w.Write([]byte(`<p>Content after placement.</p></section>`))
	}
	w.Write([]byte(`<script>document.querySelectorAll('button[data-state]').forEach(button => button.addEventListener('click', () => {
 const slot = button.closest('section').querySelector('.adsbygoogle');
 const state = button.dataset.state;
 slot.classList.toggle('simulation-filled', state === 'filled' || state === 'blocked');
 slot.style.display = state === 'blocked' ? 'none' : 'block';
 slot.textContent = state === 'filled' ? 'Simulated creative — no paid impression' : '';
 if (state === 'pending') slot.removeAttribute('data-ad-status');
 else slot.setAttribute('data-ad-status', state === 'zero-size' || state === 'blocked' ? 'filled' : state);
}));</script>`))
}

func TestAdSensePresentationFixtureCannotRequestAds(t *testing.T) {
	w := httptest.NewRecorder()
	serveAdPresentationFixture(w, httptest.NewRequest("GET", "/ad-presentation", nil))
	body := w.Body.String()
	if strings.Contains(body, "ca-pub-") || strings.Contains(body, "<script src=") || strings.Contains(body, "https://") {
		t.Fatal("simulation must not include a real publisher or remote resource")
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "connect-src 'none'; frame-src 'none'") {
		t.Fatal("simulation must block outbound collection and ad frames")
	}
}
