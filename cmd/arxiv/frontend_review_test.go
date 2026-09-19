package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/lantos1618/arxiv.gg"
)

func runFrontendReviewJavaScript(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required to execute frontend lifecycle checks")
	}
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("frontend behavior check failed: %v\n%s", err, output)
	}
}

func TestPublicPageAnalyticsStartWithoutWaiting(t *testing.T) {
	for _, name := range []string{"head", "paper"} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			(&server{googleAnalyticsID: "G-TEST123"}).renderTemplate(recorder, httptest.NewRequest(http.MethodGet, "/abs/2501.00001", nil), name, map[string]any{
				"Title": "Test paper", "Paper": &arxiv.Paper{ID: "2501.00001", Title: "Test paper"},
			})
			if recorder.Code != http.StatusOK {
				t.Fatalf("render %s: %s", name, recorder.Body.String())
			}
			var analyticsScript string
			for _, match := range regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllStringSubmatch(recorder.Body.String(), -1) {
				if strings.Contains(match[1], "googletagmanager.com") {
					if analyticsScript != "" {
						t.Fatal("page renders multiple analytics loaders")
					}
					analyticsScript = match[1]
				}
			}
			if analyticsScript == "" {
				t.Fatal("public page does not render analytics")
			}
			encoded, _ := json.Marshal(analyticsScript)
			runFrontendReviewJavaScript(t, `
const assert = require('node:assert/strict');
const vm = require('node:vm');
const appended = [];
const context = {
  window: {},
  document: {
    readyState: 'loading',
    createElement: type => { assert.equal(type, 'script'); return {}; },
    head: { appendChild: script => appended.push(script) },
  },
};
vm.createContext(context);
const source = `+string(encoded)+`;
vm.runInContext(source, context);
assert.equal(appended.length, 1, 'starts while document is loading');
assert.equal(appended[0].async, true, 'does not block document parsing');
assert.equal(appended[0].src, 'https://www.googletagmanager.com/gtag/js?id=G-TEST123');
assert.equal(context.window.dataLayer.length, 2);
assert.equal(context.window.dataLayer[0][0], 'js');
assert.equal(context.window.dataLayer[1][0], 'config');
assert.equal(context.window.dataLayer[1][1], 'G-TEST123');
vm.runInContext(source, context);
assert.equal(appended.length, 1, 'initialization is idempotent');
assert.equal(context.window.dataLayer.length, 2, 'does not double-count configuration');
`)
		})
	}
}

func TestRecentPapersStreamLifecycle(t *testing.T) {
	body, err := templateFS.ReadFile("templates/index.html")
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the actual stream controller independently of the search UI.
	start := strings.Index(string(body), "\tlet recentEventSource = null;")
	end := strings.Index(string(body), "\n\tloadRecentPapers();")
	if start < 0 || end < start {
		t.Fatal("recent-paper stream controller not found")
	}
	source, _ := json.Marshal(string(body[start : end+len("\n\tloadRecentPapers();")]))
	runFrontendReviewJavaScript(t, `
const assert = require('node:assert/strict');
const vm = require('node:vm');
const listeners = {};
const timers = new Map();
const streams = [];
let timerID = 0;
let mathCalls = 0;
let rowCount = 0;
let html = '<p class="loading-status">Loading recent papers...</p>';
const recentList = {
  children: { get length() { return rowCount; } },
  get innerHTML() { return html; },
  set innerHTML(value) { html = value; rowCount = (value.match(/class="paper"/g) || []).length; },
  querySelector: () => rowCount ? {} : null,
  insertAdjacentHTML: (_, value) => { html = value + html; rowCount++; },
  get firstElementChild() { return {}; },
  get lastElementChild() { return { remove: () => rowCount-- }; },
};
const context = {
  recentList,
  renderRecentPaper: paper => '<div class="paper">' + paper.ID + '</div>',
  escapeHtml: value => value,
  document: { hidden: false, addEventListener: (event, fn) => listeners[event] = fn },
  navigator: { onLine: true },
  window: {
    addEventListener: (event, fn) => listeners[event] = fn,
    typesetMath: () => mathCalls++,
  },
  setTimeout: (fn, delay) => { timers.set(++timerID, { fn, delay }); return timerID; },
  clearTimeout: id => timers.delete(id),
  EventSource: class {
    constructor(url) { this.url = url; this.closed = false; streams.push(this); }
    close() { this.closed = true; }
  },
};
vm.createContext(context);
vm.runInContext(`+string(source)+`, context);
const message = (stream, data) => stream.onmessage({ data: JSON.stringify(data) });
const nextTimer = () => {
  assert.equal(timers.size, 1, 'only one retry timer exists');
  const [id, timer] = timers.entries().next().value;
  timers.delete(id);
  timer.fn();
  return timer.delay;
};
assert.equal(streams.length, 1);
listeners.pageshow();
listeners.online();
assert.equal(streams.length, 1, 'repeated lifecycle events do not create extra streams');
message(streams[0], { type: 'start' });
for (let i = 0; i < 50; i++) message(streams[0], { type: 'paper', paper: { ID: i } });
assert.equal(mathCalls, 0, 'batch initial math rendering');
message(streams[0], { type: 'complete' });
assert.equal(rowCount, 50);
assert.equal(mathCalls, 1);
for (let i = 50; i < 125; i++) message(streams[0], { type: 'new', paper: { ID: i } });
assert.equal(rowCount, 50, 'live updates keep at most fifty rows');
context.document.hidden = true;
listeners.visibilitychange();
assert.equal(streams[0].closed, true, 'hidden pages release server stream');
assert.equal(timers.size, 0);
streams[0].onerror();
assert.equal(timers.size, 0, 'stale stream cannot restart after pause');
context.document.hidden = false;
listeners.visibilitychange();
assert.equal(streams.length, 2, 'visible page reconnects once');
const previousHTML = html;
message(streams[1], { type: 'start' });
message(streams[1], { type: 'paper', paper: { ID: 999 } });
assert.equal(html, previousHTML, 'reconnect preserves existing rows until complete');
streams[1].onerror();
assert.equal(streams[1].closed, true);
for (const expected of [1000, 2000, 4000, 8000, 16000, 30000, 30000]) {
  assert.equal(nextTimer(), expected, 'reconnect uses bounded exponential backoff');
  streams.at(-1).onerror();
}
context.navigator.onLine = false;
listeners.offline();
assert.equal(timers.size, 0, 'offline cancels retries');
const offlineCount = streams.length;
listeners.visibilitychange();
assert.equal(streams.length, offlineCount, 'offline pages do not reconnect');
context.navigator.onLine = true;
listeners.online();
assert.equal(streams.length, offlineCount + 1);
message(streams.at(-1), { type: 'paper', paper: { ID: 1000 } });
message(streams.at(-1), { type: 'complete' });
assert.equal(rowCount, 1, 'new snapshot replaces previous snapshot');
message(streams.at(-1), { type: 'timeout' });
assert.equal(nextTimer(), 1000, 'successful snapshot resets retry backoff');
listeners.pagehide();
assert.equal(streams.at(-1).closed, true, 'navigation releases stream');
listeners.online();
assert.equal(streams.at(-1).closed, true, 'pagehide remains paused');
const beforeRestore = streams.length;
listeners.pageshow();
assert.equal(streams.length, beforeRestore + 1, 'back-forward cache restoration resumes');
`)
}

func TestHomepageSearchDismissesPendingSuggestions(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&server{}).renderTemplate(recorder, httptest.NewRequest(http.MethodGet, "/", nil), "index", map[string]any{
		"Stats": &arxiv.CacheStats{},
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("render homepage: %s", recorder.Body.String())
	}
	var searchScript string
	for _, match := range regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllStringSubmatch(recorder.Body.String(), -1) {
		if strings.Contains(match[1], "function doDropdownSearch(") {
			searchScript = match[1]
		}
	}
	if searchScript == "" {
		t.Fatal("homepage search script not found")
	}
	encoded, _ := json.Marshal(searchScript)
	runFrontendReviewJavaScript(t, `
const assert = require('node:assert/strict');
const vm = require('node:vm');
const source = `+string(encoded)+`;
function createPage() {
  const elements = new Map();
  const timers = new Map();
  const requests = [];
  const streams = [];
  let timerID = 0;
  function element(id) {
    if (!elements.has(id)) {
      const classes = new Set();
      elements.set(id, {
        value: '', innerHTML: '', textContent: '', style: {}, listeners: {},
        classList: {
          add: name => classes.add(name), remove: name => classes.delete(name),
          contains: name => classes.has(name),
        },
        addEventListener(event, fn) { this.listeners[event] = fn; },
        querySelectorAll: () => [],
        insertAdjacentHTML(_, html) { this.innerHTML += html; },
      });
    }
    return elements.get(id);
  }
  const context = {
    document: {
      hidden: true,
      getElementById: element,
      querySelector: element,
      addEventListener() {},
      createElement: () => ({ set textContent(value) { this.innerHTML = value; } }),
    },
    navigator: { onLine: true },
    window: { addEventListener() {}, location: {} },
    AbortController, URLSearchParams,
    setTimeout: fn => { timers.set(++timerID, fn); return timerID; },
    clearTimeout: id => timers.delete(id),
    fetch: (url, options) => new Promise((resolve, reject) => requests.push({ url, options, resolve, reject })),
    EventSource: class {
      constructor(url) { this.url = url; streams.push(this); }
      close() {}
    },
  };
  vm.runInNewContext(source, context);
  return {
    element, timers, requests, streams, context,
    type(query) { element('search-input').value = query; element('search-input').listeners.input(); },
    debounce() {
      assert.equal(timers.size, 1);
      const [id, fn] = timers.entries().next().value;
      timers.delete(id);
      fn();
    },
    submit(button) {
      if (button) element(button).listeners.click();
      else element('search-form').listeners.submit({ preventDefault() {} });
    },
  };
}
const reply = (request, id, title) => request.resolve({ json: () => Promise.resolve({
  data: { papers: [{ id, title, authors: 'A. Researcher' }] },
}) });
const flush = () => new Promise(resolve => setImmediate(resolve));
(async () => {
  for (const button of [null, 'btn-quick']) {
    const page = createPage();
    page.type('attention is all you need');
    page.submit(button);
    assert.equal(page.timers.size, 0, 'submitting cancels the queued autocomplete debounce');
    assert.equal(page.requests.length, 0, 'a submitted query does not start a pending autocomplete request');
    assert.equal(page.streams.length, 1, 'submitted results still load');
    assert.equal(page.element('search-dropdown').classList.contains('open'), false);
  }

  for (const outcome of ['success', 'error']) {
    const page = createPage();
    page.type('attention is all you need');
    page.debounce();
    assert.equal(page.element('search-dropdown').classList.contains('open'), true);
    const request = page.requests[0];
    page.submit();
    assert.equal(request.options.signal.aborted, true, 'submission aborts the in-flight autocomplete');
    const hiddenHTML = page.element('search-dropdown').innerHTML;
    // A callback already queued before abort must still be ignored.
    if (outcome === 'success') reply(request, '9999.00001', 'Stale suggestion');
    else request.reject(new Error('late failure'));
    await flush();
    assert.equal(page.element('search-dropdown').innerHTML, hiddenHTML, 'late responses cannot replace dismissed suggestions');
    assert.equal(page.element('search-dropdown').classList.contains('open'), false);
    page.streams[0].onmessage({ data: JSON.stringify({ type: 'result', paper: {
      id: '1706.03762', title: 'Attention Is All You Need', authors: 'A. Researcher',
    } }) });
    assert.match(page.element('paper-results').innerHTML, /Attention Is All You Need/);
  }

  const page = createPage();
  page.type('old query');
  page.debounce();
  page.type('1706.03762');
  page.debounce();
  const directLink = page.element('search-dropdown').innerHTML;
  assert.match(directLink, /href="\/abs\/1706\.03762"/);
  reply(page.requests[0], '9999.00001', 'Stale suggestion');
  await flush();
  assert.equal(page.element('search-dropdown').innerHTML, directLink, 'late keyword responses cannot replace an ID suggestion');
  page.submit();
  assert.equal(page.context.window.location.href, '/abs/1706.03762', 'ID submission still opens the paper');

  page.type('fresh query');
  page.debounce();
  reply(page.requests[1], '2501.00001', 'Fresh suggestion');
  await flush();
  assert.equal(page.element('search-dropdown').classList.contains('open'), true, 'editing resumes autocomplete after dismissal');
  assert.match(page.element('search-dropdown').innerHTML, /Fresh suggestion/);

  page.type('fresh query edited');
  page.type('fresh query');
  assert.equal(page.timers.size, 1, 'editing then reverting within the debounce window still refreshes suggestions');
  page.debounce();
  assert.equal(page.requests.length, 3, 'only the final reverted query is requested');
  reply(page.requests[2], '2501.00001', 'Fresh suggestion');
  await flush();
  assert.equal(page.element('search-dropdown').classList.contains('open'), true, 'reverted query suggestions reopen');
  assert.match(page.element('search-dropdown').innerHTML, /Fresh suggestion/);
})().catch(error => { console.error(error); process.exitCode = 1; });
`)
}
