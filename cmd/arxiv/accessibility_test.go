package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	arxiv "github.com/lantos1618/arxiv.gg"
)

func accessibilityPage(t *testing.T, name string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	(&server{}).renderTemplate(recorder, httptest.NewRequest(http.MethodGet, "/", nil), name, map[string]any{
		"Title": "Accessibility check", "Stats": &arxiv.CacheStats{},
		"BaseURL": "https://example.test", "MCPURL": "https://example.test/mcp",
		"Papers": []arxiv.Paper{}, "Categories": []any{}, "FeedbackPosts": []any{},
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("render %s: %s", name, recorder.Body.String())
	}
	return recorder.Body.String()
}

func TestPublicPagesHaveSingleMainAndSkipTarget(t *testing.T) {
	for _, name := range []string{"api", "index", "search", "login", "feedback", "categories", "category"} {
		t.Run(name, func(t *testing.T) {
			body := accessibilityPage(t, name)
			if strings.Count(body, "<main ") != 1 || strings.Count(body, "</main>") != 1 {
				t.Fatal("page must have exactly one main landmark")
			}
			for _, required := range []string{
				`href="#main-content">Skip to main content</a>`,
				`<main id="main-content" tabindex="-1">`,
				`<nav class="nav" aria-label="Main navigation">`,
			} {
				if !strings.Contains(body, required) {
					t.Errorf("missing keyboard-navigation markup %s", required)
				}
			}
		})
	}
}

func TestAPIAgentSetupKeyboardAndCopyFeedback(t *testing.T) {
	body := accessibilityPage(t, "api")
	for _, required := range []string{
		`id="agent-tab-panel" role="tabpanel" aria-labelledby="agent-tab-codex" tabindex="0"`,
		`id="agent-copy-status" role="status" aria-live="polite" aria-atomic="true"`,
	} {
		if !strings.Contains(body, required) {
			t.Errorf("missing accessible agent setup markup %s", required)
		}
	}
	for _, tab := range []string{"codex", "claude", "mcp"} {
		if !strings.Contains(body, `id="agent-tab-`+tab+`" aria-controls="agent-tab-panel"`) {
			t.Errorf("%s tab must identify its panel", tab)
		}
	}
	if strings.Contains(body, "<h3>") {
		t.Fatal("API sections should follow the h1 with h2 headings")
	}
	var source string
	for _, match := range regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllStringSubmatch(body, -1) {
		if strings.Contains(match[1], "function promptFor(") {
			source = match[1]
		}
	}
	if source == "" {
		t.Fatal("agent setup controller missing")
	}
	encoded, _ := json.Marshal(source)
	runFrontendReviewJavaScript(t, `
const assert = require('node:assert/strict');
const vm = require('node:vm');
const nodes = new Map();
let clipboardFails = false;
let fallbackWorks = false;
let copiedText = '';
let removed = 0;
function node(id) {
  if (!nodes.has(id)) nodes.set(id, {
    id, dataset: {}, attributes: {}, listeners: {}, value: '', style: {},
    setAttribute(name, value) { this.attributes[name] = value; },
    addEventListener(event, callback) { this.listeners[event] = callback; },
    focus() { context.document.activeElement = this; },
    select() {}, remove() { removed++; },
  });
  return nodes.get(id);
}
const tabs = ['codex', 'claude', 'mcp'].map(name => {
  const tab = node('agent-tab-' + name); tab.dataset.agentTab = name; return tab;
});
const panel = node('agent-setup');
panel.dataset = { baseUrl: 'https://example.test', mcpUrl: 'https://example.test/mcp' };
panel.querySelectorAll = () => tabs;
panel.querySelector = selector => node(selector);
const context = {
  window: { location: { origin: 'https://example.test' } },
  document: {
    activeElement: null,
    getElementById: node,
    createElement: () => node('fallback-textarea'),
    body: { appendChild() {} },
    execCommand: () => fallbackWorks,
  },
  navigator: { clipboard: { writeText: async text => {
    if (clipboardFails) throw new Error('clipboard denied');
    copiedText = text;
  } } },
};
vm.runInNewContext(`+string(encoded)+`, context);
function checkActive(index) {
  for (let i = 0; i < tabs.length; i++) {
    assert.equal(tabs[i].attributes['aria-selected'], String(i === index));
    assert.equal(tabs[i].tabIndex, i === index ? 0 : -1);
  }
  assert.equal(node('agent-tab-panel').attributes['aria-labelledby'], tabs[index].id);
}
function key(index, key, expected) {
  let prevented = false;
  tabs[index].listeners.keydown({key, preventDefault() { prevented = true; }});
  assert.equal(prevented, true);
  checkActive(expected);
  assert.equal(context.document.activeElement, tabs[expected]);
}
(async () => {
  checkActive(0);
  key(0, 'ArrowRight', 1);
  assert.match(node('agent-prompt').textContent, /MCP connector/);
  key(1, 'End', 2);
  key(2, 'ArrowRight', 0);
  key(0, 'ArrowLeft', 2);
  key(2, 'Home', 0);
  tabs[1].listeners.click();
  checkActive(1);
  const copy = node('[data-agent-copy="prompt"]');
  copy.focus();
  await copy.listeners.click();
  assert.match(copiedText, /MCP connector/);
  assert.equal(node('agent-copy-status').textContent, 'Prompt copied.');
  assert.equal(node('agent-copy-status').dataset.error, 'false');
  clipboardFails = true;
  await copy.listeners.click();
  assert.match(node('agent-copy-status').textContent, /Copy failed/);
  assert.equal(node('agent-copy-status').dataset.error, 'true');
  assert.equal(context.document.activeElement, copy, 'clipboard fallback preserves keyboard focus');
  assert.equal(removed, 1, 'failed fallback removes temporary input');
  fallbackWorks = true;
  await copy.listeners.click();
  assert.equal(node('agent-copy-status').textContent, 'Prompt copied.');
  assert.equal(removed, 2);
})().catch(error => { console.error(error); process.exitCode = 1; });
`)
}
