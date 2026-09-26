package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/lantos1618/arxiv.gg"
)

func renderSearchAvailabilityPage(t *testing.T, s *server, page string, signedIn bool) string {
	t.Helper()
	data := map[string]any{
		"Stats": &arxiv.CacheStats{}, "Papers": []arxiv.Paper{}, "GoogleEnabled": true,
		"Next": "/abs/1706.03762", "CurrentUser": nil,
	}
	if signedIn {
		data["CurrentUser"] = &arxiv.User{}
	}
	recorder := httptest.NewRecorder()
	s.renderTemplate(recorder, httptest.NewRequest(http.MethodGet, "/", nil), page, data)
	if recorder.Code != http.StatusOK {
		t.Fatalf("render %s: %s", page, recorder.Body.String())
	}
	return recorder.Body.String()
}

func TestSearchModeConfigurationAndAuthenticationCopy(t *testing.T) {
	for _, config := range []struct {
		name       string
		server     *server
		configured bool
	}{
		{"unconfigured", &server{}, false},
		{"async_configured", &server{qwenAsyncWorkerEnabled: true}, true},
		// Rendering must not probe this service or infer that it is healthy.
		{"sync_configured", &server{qwenEmbeddingServiceURL: "http://127.0.0.1:1"}, true},
	} {
		for _, page := range []string{"index", "search"} {
			for _, signedIn := range []bool{false, true} {
				name := config.name + "/" + page + "/signed_out"
				if signedIn {
					name = config.name + "/" + page + "/signed_in"
				}
				t.Run(name, func(t *testing.T) {
					html := renderSearchAvailabilityPage(t, config.server, page, signedIn)
					for _, mode := range []string{"quick", "semantic", "deep"} {
						tag := regexp.MustCompile(`<button\b[^>]*id="btn-` + mode + `"[^>]*>`).FindString(html)
						if tag == "" || !strings.Contains(tag, `aria-describedby="`+mode+`-help"`) {
							t.Fatalf("%s control lacks its visible explanation: %q", mode, tag)
						}
						wantDisabled := mode != "quick" && !config.configured
						if strings.Contains(tag, " disabled") != wantDisabled {
							t.Fatalf("%s disabled state does not match configuration: %s", mode, tag)
						}
					}
					if !strings.Contains(html, `<label class="search-label" for="search-input">`) {
						t.Fatal("search input has no visible associated label")
					}
					if config.configured {
						if !strings.Contains(html, "Requests may queue") {
							t.Fatal("configured search must qualify runtime availability")
						}
						if strings.Contains(html, "Sign in required to search prepared full-paper text.") == signedIn {
							t.Fatal("Deep Search sign-in explanation does not match authentication")
						}
					} else if !strings.Contains(html, "Not enabled on this site. Signing in does not enable Deep Search.") {
						t.Fatal("disabled Deep Search must not imply sign-in will enable it")
					}
					if strings.Contains(html, "worker healthy") || strings.Contains(html, "worker online") {
						t.Fatal("configuration must not be reported as worker health")
					}
				})
			}
		}
		t.Run(config.name+"/login", func(t *testing.T) {
			html := renderSearchAvailabilityPage(t, config.server, "login", false)
			if strings.Contains(html, "Deep Search is not enabled on this site.") == config.configured {
				t.Fatal("login Deep Search copy does not match configuration")
			}
			if !strings.Contains(html, `/auth/google/start?next=%2Fabs%2F1706.03762`) {
				t.Fatal("login must preserve the requested paper return path")
			}
		})
	}
}

func searchAvailabilityScript(t *testing.T, page string, signedIn bool) string {
	t.Helper()
	html := renderSearchAvailabilityPage(t, &server{qwenAsyncWorkerEnabled: true}, page, signedIn)
	for _, match := range regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllStringSubmatch(html, -1) {
		if strings.Contains(match[1], "const maxSemanticRetries") {
			encoded, err := json.Marshal(match[1])
			if err != nil {
				t.Fatal(err)
			}
			return string(encoded)
		}
	}
	t.Fatal("search script missing")
	return ""
}

// Run the shipped page scripts against controlled stream events. No real search,
// query-embedding jobs, application startup, or external service is involved.
const searchAvailabilityDOM = `
const assert = require('node:assert/strict');
const vm = require('node:vm');
function createPage(source) {
  const elements = new Map(), streams = [], timers = new Map(), requests = [];
  let timerID = 0;
  const context = {
    document: { hidden: true, activeElement: null, addEventListener() {},
      getElementById: element, querySelector: element,
      createElement: () => ({ set textContent(value) { this.innerHTML = String(value).replace(/</g, '&lt;'); } }),
    },
    navigator: { onLine: true },
    window: { addEventListener() {}, location: {} },
    history: { pushState() {} }, AbortController, URLSearchParams,
    setTimeout: (fn, delay) => { timers.set(++timerID, { fn, delay }); return timerID; },
    clearTimeout: id => timers.delete(id),
    fetch: (url, options) => new Promise((resolve, reject) => requests.push({url, options, resolve, reject})),
    EventSource: class {
      constructor(url) { this.url = url; streams.push(this); }
      close() { this.closed = true; }
    },
  };
  function makeElement(id) {
    const classes = new Set();
    return {
      id, value: '', style: {}, listeners: {}, textContent: '', links: [], html: '',
      classList: { add: name => classes.add(name), remove: name => classes.delete(name),
        contains: name => classes.has(name), toggle: (name, value) => value ? classes.add(name) : classes.delete(name) },
      get innerHTML() { return this.html; },
      set innerHTML(value) {
        this.html = value;
        this.links = [...value.matchAll(/<a class="dropdown-item" href="([^"]+)"/g)].map((match, index) => {
          const link = makeElement(id + '-' + index); link.href = match[1]; return link;
        });
      },
      focus() { context.document.activeElement = this; },
      click() { context.window.location.href = this.href; },
      addEventListener(event, fn) { this.listeners[event] = fn; },
      querySelectorAll() { return this.links; },
      insertAdjacentHTML(_, value) { this.innerHTML += value; },
    };
  }
  function element(id) {
    if (!elements.has(id)) elements.set(id, makeElement(id));
    return elements.get(id);
  }
  vm.runInNewContext(source, context);
  return {
    element, streams, timers, requests, context,
    start(mode, query = 'attention') {
      element('search-input').value = query;
      element('btn-' + mode).listeners.click();
      return streams.at(-1);
    },
    type(query) { element('search-input').value = query; element('search-input').listeners.input(); },
    debounce() { const [id, timer] = timers.entries().next().value; timers.delete(id); timer.fn(); },
    key(target, key) { target.listeners.keydown({ key, preventDefault() {} }); },
  };
}
const message = (stream, data) => stream.onmessage({ data: JSON.stringify(data) });
const flush = () => new Promise(resolve => setImmediate(resolve));
`

func TestSearchRuntimeAvailabilityMessages(t *testing.T) {
	for _, page := range []string{"index", "search"} {
		t.Run(page, func(t *testing.T) {
			source := searchAvailabilityScript(t, page, true)
			runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const source = `+source+`;
{
  const page = createPage(source), stream = page.start('deep');
  message(stream, { type: 'status', queued: true, retryAfterSeconds: 75, message: 'Deep Search is being prepared.' });
  message(stream, { type: 'complete', mode: 'deep', queued: true, count: 0, retryAfterSeconds: 75 });
  assert.match(page.element('search-status').textContent, /Deep Search preparation is queued.*75 seconds/);
  assert.doesNotMatch(page.element('search-status').textContent, /0 .*results/);
  assert.equal(page.element('search-status').style.display, '');
  assert.equal(page.timers.size, 1, 'Deep Search automatically checks queued work');
  page.debounce();
  assert.equal(new URL(page.streams.at(-1).url, 'https://arxiv.gg').searchParams.get('mode'), 'deep');
}
for (const retry of [false, true]) {
  const page = createPage(source), stream = page.start('semantic');
  const notice = retry ? 'Semantic results are being prepared; showing Quick results now.' : 'Semantic search is not ready; showing Quick results.';
  message(stream, { type: 'fallback', message: notice, retry: { recommended: retry, afterSeconds: retry ? 75 : 0 } });
  message(stream, { type: 'result', paper: { ID: '1706.03762', Title: 'Attention', Authors: 'A. Researcher' } });
  assert.match(page.element('search-status').textContent, /showing Quick results/);
  message(stream, { type: 'complete', mode: 'quick', count: 1 });
  if (retry) {
    assert.equal(page.timers.size, 1);
    assert.equal([...page.timers.values()][0].delay, 75000, 'honor server retry delay');
    assert.match(page.element('search-status').textContent, /75 seconds/);
    page.type('a different idea');
    const before = page.streams.length;
    assert.equal([...page.timers.values()].some(timer => timer.delay === 75000), false, 'editing cancels semantic retry');
    assert.equal(page.streams.length, before);
  } else {
    assert.equal(page.element('search-status').textContent, notice, 'completion preserves non-retrying fallback explanation');
    assert.equal(page.timers.size, 0);
  }
}
{
  const page = createPage(source), stream = page.start('deep');
  message(stream, { type: 'status', message: '<b>Untrusted message</b>' });
  assert.equal(page.element('terminal-status').textContent, '<b>Untrusted message</b>', 'status is text, not HTML');
  stream.onerror();
  assert.equal(page.element('search-status').style.display, '');
  assert.match(page.element('search-status').textContent, /interrupted.*Quick Search/);
}
`)
		})
	}
}

func TestSearchDeepLoginPreservesQuery(t *testing.T) {
	for _, page := range []string{"index", "search"} {
		t.Run(page, func(t *testing.T) {
			runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const page = createPage(`+searchAvailabilityScript(t, page, false)+`);
const query = 'attention & retrieval #1?';
page.start('deep', query);
const login = new URL(page.context.window.location.href, 'https://arxiv.gg');
const next = new URL(login.searchParams.get('next'), 'https://arxiv.gg');
assert.equal(login.pathname, '/login');
assert.equal(next.pathname, '/search');
assert.equal(next.searchParams.get('q'), query);
assert.equal(next.searchParams.get('mode'), 'deep');
assert.equal(page.streams.length, 0, 'signed-out Deep Search must go to sign-in before issuing a request');
`)
		})
	}
}

func TestHomepageSuggestionsKeyboardAndFailure(t *testing.T) {
	runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const source = `+searchAvailabilityScript(t, "index", false)+`;
(async () => {
  const page = createPage(source), input = page.element('search-input'), dropdown = page.element('search-dropdown');
  page.type('attention'); page.debounce();
  assert.equal(input.ariaExpanded, 'true');
  page.requests[0].resolve({ ok: true, json: async () => ({ success: true, data: { papers: [
    { id: '1706.03762', title: 'Attention' }, { id: '2501.00001', title: 'Another paper' }
  ] } }) });
  await flush();
  assert.match(page.element('suggestion-status').textContent, /2 suggestions/);
  page.key(input, 'ArrowDown');
  assert.equal(page.context.document.activeElement.href, '/abs/1706.03762');
  page.key(dropdown, 'ArrowDown');
  assert.equal(page.context.document.activeElement.href, '/abs/2501.00001');
  page.key(dropdown, 'ArrowUp');
  assert.equal(page.context.document.activeElement.href, '/abs/1706.03762');
  page.key(dropdown, 'ArrowUp');
  assert.equal(page.context.document.activeElement, input);
  page.key(input, 'ArrowDown'); page.key(dropdown, 'Escape');
  assert.equal(page.context.document.activeElement, input);
  assert.equal(input.ariaExpanded, 'false');
  assert.equal(dropdown.classList.contains('open'), false);
  page.type('another query'); page.debounce();
  page.requests[1].resolve({ ok: false, json: async () => ({ success: false }) });
  await flush();
  assert.match(dropdown.innerHTML, /Suggestions are unavailable.*Quick Search/);
  assert.doesNotMatch(dropdown.innerHTML, /No results/);
})().catch(error => { console.error(error); process.exitCode = 1; });
`)
}

func TestQueuedSearchSurvivesFallbackFailure(t *testing.T) {
	for _, name := range []string{"index", "search"} {
		t.Run(name, func(t *testing.T) {
			runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const source = `+searchAvailabilityScript(t, name, true)+`;
for (const mode of ['semantic', 'deep']) {
 for (const failure of ['server', 'transport']) {
  const page = createPage(source), stream = page.start(mode, 'a new research idea');
  if (mode === 'semantic') {
   message(stream, {type:'fallback', message:'Preparing embedding', retry:{recommended:true, afterSeconds:75}});
  } else {
   message(stream, {type:'status', queued:true, retryAfterSeconds:75});
  }
  if (failure === 'server') message(stream, {type:'error', error:'Search is temporarily unavailable.'});
  else stream.onerror();
  assert.equal(page.timers.size, 1, 'pending work survives fallback/transport failure');
  assert.match(page.element('search-status').textContent, /queued.*automatically/);
  assert.equal(page.element('search-status').classList.contains('error'), false);
  stream.onerror();
  assert.equal(page.timers.size, 1, 'closed stream cannot duplicate retry');
  page.debounce();
  const resumed = page.streams.at(-1);
  assert.equal(new URL(resumed.url, 'https://arxiv.gg').searchParams.get('mode'), mode);
  message(resumed, {type:'result', paper:{ID:'1706.03762', Title:'Recovered paper', Authors:'A. Researcher'}});
  message(resumed, {type:'complete', mode, count:1});
  assert.match(page.element('paper-results').innerHTML, /Recovered paper/);
  assert.match(page.element('search-status').textContent, /1 .*results/);
  assert.equal(page.timers.size, 0, 'completed search stops polling');
 }
 const page = createPage(source), stream = page.start(mode);
 message(stream, {type:'status', queued:true, retryAfterSeconds:75});
 message(stream, {type:'complete', queued:true, retryAfterSeconds:75});
 page.type('changed query');
 assert.equal([...page.timers.values()].some(timer => timer.delay === 75000), false, 'editing cancels queued retry');
}
{
 const page = createPage(source), stream = page.start('quick');
 message(stream, {type:'error', error:'Search is temporarily unavailable.'});
 assert.equal(page.timers.size, 0, 'ordinary errors must not create retry loops');
}
{
 const page = createPage(source);
 let stream = page.start('semantic');
 for (let attempt=0; attempt<9; attempt++) {
  message(stream, {type:'fallback', retry:{recommended:true, afterSeconds:75}});
  message(stream, {type:'error', error:'Quick timeout'});
  if (attempt < 8) { assert.equal(page.timers.size, 1); page.debounce(); stream=page.streams.at(-1); }
 }
 assert.equal(page.timers.size, 0, 'polling remains bounded');
 assert.match(page.element('search-status').textContent, /still queued.*later/);
}
`)
		})
	}
}
