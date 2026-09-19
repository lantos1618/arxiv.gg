# Handoff assessment and AXG-001 implementation — 2026-09-17

> Historical September 17 record. The current application disposition is [IMPLEMENTATION-2026-09-19.md](IMPLEMENTATION-2026-09-19.md); statements below describe the earlier batch and are retained as dated evidence.

Assessment: **8/10 as an evidence-led UX audit; 6/10 as an implementation-ready specification.** The dated observations, stable IDs, acceptance criteria and distinction between report and application are useful. Root causes, query costs, data provenance and some existing features were not established. The attached instructions were evaluated as proposals; the real application and repository guidance determined the change.

## Scope and current state

Application: `/home/ubuntu/arxiv`, repository `https://github.com/lantos1618/arxiv.gg`, base commit `e7f0026adabfd01af027b608eae96b08db5538ac`.

AXG-001 is implemented and locally verified. Changes remain uncommitted. No deployment, production application restart, production schema change or stored-data repair was performed. The public site still runs the original build. Other AXG issues are not implemented by this change.

The original ZIP is retained as supplied in the attachment. ROUTE-MAP.md, route-map.json and routes/ in this copy are archived revision-2.0 generated briefs, not current implementation status. They were not regenerated because the separate report project/generator is not part of this checkout. Current status is in findings.json, llm.md and REVISIONS.md. The hosted report was not edited or published.

## Reproduction and cause

A signed-out public Quick API request for `attention is all you need` with limit 5 returned unrelated first results, including `1805.10973` (GLAC Net), and reported 89 matches. Different earlier audit results are consistent with the code's random sampling, not evidence that the audit used the wrong query.

`search.go` enabled `gin_fuzzy_search_limit=5000` for multiword searches. PostgreSQL chooses a random subset when this setting is nonzero, before result ranking; count and result queries sampled independently. There was no exact-title preference or final tie-breaker. See [PostgreSQL's GIN documentation](https://www.postgresql.org/docs/16/gin-tips.html). This is stronger evidence than the handoff's ranking hypothesis alone.

The homepage already called hideDropdown on submit, but that function did not clear the pending 300 ms timer or invalidate in-flight callbacks. A JavaScript regression test failed on the original code because the pending debounce survived submission.

## Change

- Normalize query case for exact comparison and collapse query whitespace; PostgreSQL title matching also collapses stored whitespace.
- Disable random GIN sampling only for Quick Search. Rank exact normalized titles before full-text relevance, date, then stable ID; exact IDs precede longer prefix matches.
- Return text-match counts with `count(*) OVER()` in the same query snapshot. Preserve author lookup, with exact-title precedence and stable author ordering. Handle stopword-only titles through the existing fallback.
- Bound the entire PostgreSQL Quick operation with a five-second context and its full-text/fallback statements with a five-second statement timeout. Propagate errors/cancellation instead of starting a more expensive broad fallback after failure.
- Cancel autocomplete debounce and active request, and invalidate queued success/error callbacks when dismissed, submitted, edited, or replaced by an ID suggestion.
- Keep SQLite's legacy path covered for exact-case/input-whitespace queries and stable ordering. Production validation uses PostgreSQL, not a SQLite inference.

Application/test files: `search.go`, `search_quick_test.go`, `search_quick_postgres_test.go`, `cmd/arxiv/templates/index.html`, `cmd/arxiv/frontend_review_test.go`.

## Acceptance evidence

1. **Exact title first:** isolated PostgreSQL tests verify case changes, input and stored whitespace, a more strongly ranked partial title, author-name collisions, stopword fallback and exact ID precedence. A read-only query using the new ordering on production metadata returned `1706.03762` first, followed by `1912.00349` and `2103.03404`.
2. **Compatible suggestions/list ranking:** repeated PostgreSQL searches with limits 1, 8 and 100 preserve prefixes, including equal-rank/date ties and a 12,001-paper GIN fixture. Both unfiltered Quick API and Quick stream use Cache.QuickSearch. A baseline overlay with original search.go fails the ranking regressions; with direct GIN postings, its sampled count is 8,352 instead of the fixture’s 12,001 matches.
3. **Overlay dismissed on submit:** actual homepage JavaScript tests cover form submit, Quick click, pending timer, late success/error despite abort, exposed submitted results, direct ID navigation, resumed suggestions and editing then reverting to the previous query during debounce. Chromium checks at 1280×900 and 390×844 pass Enter, Quick click, visible results and keyboard ID selection, with no JavaScript errors. Browser responses were controlled fixtures; they do not independently establish backend ranking.

All full-suite commands passed, including nine PostgreSQL regressions enabled for both normal and race runs. Commands: `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -o /tmp/arxiv-axg001-checked ./cmd/arxiv`, `git diff --check`. PostgreSQL regression command: `ARXIV_TEST_POSTGRES_DSN=<disposable database> go test . -run TestQuickSearchPostgres -count=1 -v`. Fixtures use temporary tables on a dedicated retained connection and do not invoke application startup or migrations.

Read-only PostgreSQL query-cost check: first cold attempt exceeded a three-second cap; an intermediate run took 1.884 seconds; the final query including the window count took 185.137 ms over 7,735 matches on a warmed run. These are measurements for one query, not a latency guarantee or load benchmark. All production database checks used explicit READ ONLY transactions and statement timeouts.

## Limits and follow-up decisions

- No production acceptance claim: deploy and check the actual endpoints before considering the public experience fixed. Very broad/cold searches may exhaust the five-second budget; this deliberately reports unavailability rather than silently returning random samples. A normalized-title index/search-plan improvement is a separate performance task.
- This patch concerns unfiltered Quick Search. The separate keyword Search path, including category-filtered Quick callers, still has its prior sampling policy. Missing/stale full-text vectors are not repaired here.
- No researcher usability/timing study, signed-in session or relevance benchmark was conducted.
- Browser inspection exposed existing 390px tooltip overflow: original and patched scripts both produce document width 480px, and disabling existing tooltip pseudoelements restores 390px. Recorded in evidence/layout.json; no unrelated CSS redesign was included.
- AXG-002: visible state improvements are justified, but current configuration flags do not establish worker health. Distinguish configuration, authentication, queue state and runtime errors.
- AXG-003: the public API currently reports Created=2023-08-02 and Updated=2023-08-03 for 1706.03762; [official arXiv history](https://arxiv.org/abs/1706.03762) gives first submission 2017-06-12 and revision 2023-08-02. The current HTML parser maps upstream fields correctly. Investigate stored-data provenance before any repair; relabeling alone would leave false dates. Citation scope is a separate presentation correction.
- AXG-004: an accessible semantic-neighbor list is justified. The existing graph already scales projected coordinates; inspect settled geometry before simply adding another fit operation.
- AXG-005: homepage copy is an editorial proposal, to revisit after working retrieval and map examples can support it.
- AXG-006: homepage/category ordering uses fetched_at, while category detail additionally filters src_downloaded=true. Directory totals count broader metadata; a simple “200 of total” label would conflate scopes.
- AXG-007: category display-name mapping is incomplete; finish authoritative names before claiming every code is translated.
- AXG-008: contextual “Keep this paper” and sanitized OAuth next-path return already exist. Preserve and verify those flows; qualify unavailable Deep Search claims and sparse readership signals separately.
- The handoff labels some GET search routes as read operations, but semantic search can enqueue query embeddings. HTTP method alone does not prove a probe is passive; no semantic queue probes were used here.

Next useful batch: visible availability and citation-scope labels; then investigate date provenance and accurately describe list ordering/coverage. Keep data repair and larger design work separately reviewable.
