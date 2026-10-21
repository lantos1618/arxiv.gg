package main

import "testing"

func TestSearchHistoryRestoresResultsAndCancelsOldStreams(t *testing.T) {
	runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const page = createPage(`+searchAvailabilityScript(t, "search", true)+`, '/search?q=graph&mode=quick&category=cs.AI&limit=7');
const input = page.element('search-input'), results = page.element('paper-results');
input.value = 'graph';
results.innerHTML = 'Original graph results';
const beta = page.start('semantic', 'attention & retrieval');
const params = stream => new URL(stream.url, 'https://arxiv.gg').searchParams;
assert.equal(params(beta).get('q'), 'attention & retrieval');
assert.equal(params(beta).get('mode'), 'semantic');
assert.equal(params(beta).get('category'), 'cs.AI');
assert.equal(params(beta).get('limit'), '7');
message(beta, {type:'result', paper:{ID:'1706.03762', Title:'Attention result'}});
assert.match(results.innerHTML, /Attention result/);
page.moveHistory(-1);
const alpha = page.streams.at(-1);
assert.equal(beta.closed, true, 'Back closes the search that was displayed');
assert.equal(input.value, 'graph');
assert.equal(page.element('search-mode').value, 'quick');
assert.equal(params(alpha).get('q'), 'graph');
assert.equal(params(alpha).get('category'), 'cs.AI');
assert.equal(params(alpha).get('limit'), '7');
assert.equal(results.innerHTML, '', 'old-query results are cleared before restored results arrive');
message(beta, {type:'result', paper:{ID:'1706.03763', Title:'Stale attention result'}});
beta.onerror();
assert.equal(results.innerHTML, '', 'queued callbacks from the old stream are ignored');
assert.equal(page.element('search-status').textContent, 'Searching...');
message(alpha, {type:'result', paper:{ID:'1706.03764', Title:'Graph result'}});
message(alpha, {type:'complete', mode:'quick', count:1});
assert.match(results.innerHTML, /Graph result/);
assert.match(page.element('search-status').textContent, /1 quick results for "graph"/);
page.moveHistory(1);
const forward = page.streams.at(-1);
assert.equal(input.value, 'attention & retrieval');
assert.equal(page.element('search-mode').value, 'semantic');
assert.equal(params(forward).get('mode'), 'semantic');
assert.equal(page.historyEntries.length, 2, 'Back and Forward never push another entry');
message(forward, {type:'fallback', retry:{recommended:true, afterSeconds:75}});
message(forward, {type:'complete', mode:'quick', count:0});
assert.equal(page.timers.size, 1);
page.moveHistory(-1);
assert.equal(page.timers.size, 0, 'Back cancels queued semantic retry');
page.moveHistory(1);
const pending = page.streams.at(-1);
page.windowListeners.pagehide();
assert.equal(pending.closed, true, 'leaving the page closes its stream');
const count = page.streams.length;
page.windowListeners.pageshow({persisted:true});
assert.equal(page.streams.length, count + 1, 'bfcache restoration restarts the current search');
assert.equal(params(page.streams.at(-1)).get('q'), 'attention & retrieval');
`)
}

func TestSearchHistoryRestoresLegacyModesAndEmptyLocation(t *testing.T) {
	runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const source = `+searchAvailabilityScript(t, "search", true)+`;
for (const [raw, mode] of [['search-mode=search', 'semantic'], ['search-mode=full-paper', 'deep'], ['search-mode=keyword', 'quick'], ['search-mode=normal', 'quick'], ['mode=%20&search-mode=semantic', 'semantic']]) {
  const page = createPage(source, '/search?q=legacy&' + raw);
  page.start('quick', 'other');
  page.moveHistory(-1);
  const params = new URL(page.streams.at(-1).url, 'https://arxiv.gg').searchParams;
  assert.equal(params.get('mode'), mode);
  assert.equal(page.element('search-mode').value, mode);
  assert.equal(page.element('search-input').value, 'legacy');
}
const page = createPage(source, '/search');
page.start('quick', 'other');
page.element('paper-results').innerHTML = 'Other result';
page.moveHistory(-1);
assert.equal(page.element('search-input').value, '');
assert.equal(page.element('paper-results').innerHTML, '');
assert.equal(page.element('search-status').textContent, '');
assert.equal(page.streams.length, 1, 'empty history locations never issue invalid requests');
`)
}

func TestSearchButtonsAndFormValidateBlankQueries(t *testing.T) {
	for _, pageName := range []string{"index", "search"} {
		t.Run(pageName, func(t *testing.T) {
			runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const source = `+searchAvailabilityScript(t, pageName, false)+`;
for (const trigger of ['quick', 'semantic', 'deep', 'submit', 'invalid']) {
 for (const value of ['', '   ']) {
  const page = createPage(source), input = page.element('search-input');
  const prior = page.start('quick', 'previous');
  input.value = value;
  if (trigger === 'submit') page.element('search-form').listeners.submit({preventDefault(){}});
  else if (trigger === 'invalid') input.listeners.invalid({preventDefault(){}});
  else page.element('btn-' + trigger).listeners.click();
  assert.equal(page.context.document.activeElement, input, trigger + ' focuses the blank input');
  assert.equal(page.streams.length, 1, trigger + ' must not issue an empty search');
  assert.equal(prior.closed, true, trigger + ' cancels outstanding work');
  assert.equal(page.context.window.location.pathname, '/search', trigger + ' must not send signed-out readers to login');
  const visible = page.element('search-status').textContent + page.element('search-results').innerHTML;
  assert.match(visible, /Enter a paper ID, author, keyword, or research idea/);
 }
}
`)
		})
	}
}

func TestSearchIdentifierShortcutsAndAutocomplete(t *testing.T) {
	for _, pageName := range []string{"index", "search"} {
		t.Run(pageName, func(t *testing.T) {
			runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const source = `+searchAvailabilityScript(t, pageName, false)+`;
const cases = [
 ['1706.03762', '1706.03762'], [' 1706.03762v7 ', '1706.03762'],
 ['https://arxiv.org/abs/1706.03762v7', '1706.03762'],
 ['https://arxiv.org/pdf/1706.03762v7.pdf?download=1#page=2', '1706.03762'],
 ['astro-ph/0510857v1', 'astro-ph/0510857'],
 ['math.GT/0309136', 'math.GT/0309136'], ['math.GT/0309136v2', 'math.GT/0309136'],
 ['https://export.arxiv.org/pdf/math.GT/0309136v2.pdf', 'math.GT/0309136'],
];
for (const [query, id] of cases) {
 const page = createPage(source);
 page.start('quick', query);
 assert.equal(page.context.window.location.pathname, '/abs/' + id);
 assert.equal(page.streams.length, 0, query + ' avoids metadata search');
 assert.equal(page.historyEntries.length, 1, 'paper jumps must not insert a redirecting search entry');
 if (page.element('search-input').listeners.keydown) {
   const suggestions = createPage(source);
   suggestions.type(query); suggestions.debounce();
   assert.equal(suggestions.requests.length, 0, query + ' offers a paper link without a query request');
   assert.match(suggestions.element('search-dropdown').innerHTML, /Go to paper/);
   assert.equal(suggestions.element('search-dropdown').links[0].href, '/abs/' + id);
 }
}
for (const query of ['1706.03762v7 extra', 'https://evil.example/abs/1706.03762', 'math.GT/0309136?bad=1', 'unknown/0309136']) {
 const page = createPage(source);
 page.start('quick', query);
 assert.equal(page.streams.length, 1, 'unrecognized input remains a normal search');
}
`)
		})
	}
}

func TestSearchDeepLoginRetainsFiltersWithoutHistoryLoop(t *testing.T) {
	runFrontendReviewJavaScript(t, searchAvailabilityDOM+`
const page = createPage(`+searchAvailabilityScript(t, "search", false)+`, '/search?q=original&category=cs.AI&limit=7');
page.start('deep', 'new idea');
const login = new URL(page.context.window.location.href, 'https://arxiv.gg');
const next = new URL(login.searchParams.get('next'), 'https://arxiv.gg');
assert.equal(next.searchParams.get('q'), 'new idea');
assert.equal(next.searchParams.get('mode'), 'deep');
assert.equal(next.searchParams.get('category'), 'cs.AI');
assert.equal(next.searchParams.get('limit'), '7');
assert.equal(page.historyEntries.length, 1, 'Back from sign-in must not restore a redirecting Deep search');
`)
}
