# Application implementation — 2026-09-19

Current release disposition: **deployed and checked**. See [RELEASE-2026-09-19.md](RELEASE-2026-09-19.md). The pre-release account below is retained as dated implementation history.

## Scope and release state

The seven requested priorities are implemented in `/home/ubuntu/arxiv`, based on commit `e7f0026adabfd01af027b608eae96b08db5538ac`, with uncommitted working-tree changes. This entry records the application changes and local evidence available when written. **This batch has not been deployed.** No production restart or application of the reviewed-date SQL is claimed here. Initial combined normal/race tests, vet and build pass. The final settled-tree recheck and any verified release evidence will be appended separately.

[The September 17 assessment](IMPLEMENTATION.md) remains the record of the original AXG-001 patch and its checks. Original September 17 observations, generated revision-2.0 route maps and route briefs, and the hosted review are retained. Editing these handoff files does not publish the application or report.

The requested priority numbers differ from stable AXG IDs:

| Requested priority | Finding | Result |
|---|---|---|
| 1. Reliable known-paper search | AXG-001 | Existing exact-title/ID ranking and autocomplete cancellation fix retained and rechecked. |
| 2. Clear search availability | AXG-002 | Visible, accessible explanations distinguish configuration, authentication, queue progress, fallback and runtime failure. |
| 3. Trustworthy paper metadata | AXG-003 | Explicit date/citation labels, conservative date warnings, and a guarded repair for three source-checked IDs. |
| 4. Honest browsing order | AXG-006 | Homepage and category lists describe actual fetch ordering and their different coverage. |
| 5. Readable subjects | AXG-007 | Full names and discipline navigation cover the 176 observed category codes, including legacy codes. |
| 6. Usable related-paper maps | AXG-004 | Keyboard-accessible companion links expose full titles; simpler labels distinguish semantic similarity from citations. |
| 7. Outcome-focused homepage | AXG-005 | “Find a paper. Follow its connections.” with primary Quick Search, a real paper example and secondary coverage details. |

Related AXG-008 improvements qualify login feature claims and sparse readership signals, while preserving the existing contextual paper invitation and return paths. Real Google sign-in and reading-trail persistence are not verified by this batch's fixtures.

## Implementation and evidence

### AXG-001: retain reliable Quick Search

The September 17 implementation remains intact: normalized exact-title and ID precedence, deterministic ranking, removal of random sampling from unfiltered Quick Search, consistent count snapshots and bounded database work. Autocomplete still cancels timers and requests and rejects stale callbacks when dismissed or submitted.

The existing PostgreSQL search regressions were rerun during this batch. Homepage JavaScript regressions and controlled Chromium checks continue to cover Enter, Quick Search, dismissal of pending suggestions and direct paper navigation. These checks are not a production relevance or broad-query load benchmark. Category-filtered callers and the separate keyword Search path retain their earlier scope limits.

### AXG-002: distinguish availability states

The homepage and search page now put Quick Search in the primary search bar. Semantic and Deep are secondary controls with visible explanations referenced through `aria-describedby`; unconfigured controls use disabled semantics and distinct styling. Configuration allows a search attempt and does not claim a healthy or currently running worker. Deep Search separately states the sign-in requirement and variable full-paper coverage. Login no longer implies that signing in enables an unconfigured Deep Search.

Stream handling preserves explicit queue states: queued Deep completion does not become “0 results”; Semantic fallback explanations survive result rendering and completion; automatic Semantic retries respect the server's 75-second delay and stop when the query changes. Interrupted streams show recovery text. Server status messages are inserted as text, and queue state comes from the response contract rather than keywords in a message. Deep login return URLs preserve punctuation in the query.

The search field has a visible label. Homepage suggestions use a named, nonmodal popup with ordinary links: Down Arrow moves focus to a link, arrows move between links, and Escape returns focus to the input. Live announcements communicate suggestions and search status. This preserves native link navigation.

Local evidence: `cmd/arxiv/search_availability_test.go` exercises unconfigured, asynchronous-configured and synchronous-configured rendering, both signed-in and signed-out states, login copy, runtime queue/fallback/error cases, query-preserving redirects and keyboard suggestion behavior. Existing AXG-001 tests remain passing. Chromium fixtures at 1280×900 and 390×844 pass all three configuration/authentication cases, keyboard paper navigation, visible controls, zero document overflow and no JavaScript errors.

### AXG-003: scope date repair and citation meaning

Paper pages display “First submitted,” “Latest revision,” and “citations in our index.” Suspicious or missing dates show a verification notice and link to official arXiv history. `Paper.DatesNeedReview` checks missing first dates, reversed chronology and large disagreement with the identifier's announcement month. It is a plausibility heuristic, not verification of every record; an identifier cannot establish an exact submission day. The HTML parser no longer substitutes a revision date when the first-submission field is absent.

The guarded SQL file `deploy/sql/2026-09-19-reviewed-paper-dates.sql` contains only the following source-checked corrections:

| Paper | First submission | Latest revision | Official history |
|---|---|---|---|
| 1706.03762 | 2017-06-12 | 2023-08-02 | [Attention Is All You Need](https://arxiv.org/abs/1706.03762) |
| 1312.6114 | 2013-12-20 | 2022-12-10 | [Auto-Encoding Variational Bayes](https://arxiv.org/abs/1312.6114) |
| 1810.04805 | 2018-10-11 | 2019-05-24 | [BERT](https://arxiv.org/abs/1810.04805) |

Each update requires both previously observed date values to match, so a newer metadata refresh is not overwritten. It preserves `fetched_at`, sets bounded statement/lock timeouts, and is listed in the deployment SQL manifest. The file's presence is not evidence that production data has been repaired. Broader stored-date provenance and corpus-wide repair remain outside this bounded correction.

Focused coverage is in `paper_dates_test.go`, `paper_dates_postgres_test.go` and `cmd/arxiv/paper_accessibility_test.go`: missing-source dates, plausibility boundaries, guarded updates and preservation of newer/unrelated rows. Final combined check results will be appended by the release integrator.

### AXG-006 and AXG-007: explain browsing and subjects

The homepage heading is “Recently fetched papers.” Its visible explanation describes an initial maximum of 50 cached records ordered by `fetched_at DESC NULLS LAST, id DESC` on PostgreSQL, followed by incoming fetch events prepended in arrival order. The lite stream does not supply fetch dates; no per-row date is invented.

Category detail labels the list “Recently fetched papers,” shows available fetch timestamps, and identifies missing dates. It states the maximum of 200 papers with downloaded source. Directory counts include all indexed metadata, including papers without downloaded source; the interface does not imply these differently scoped totals form a simple “200 of total” pagination count. Existing backend ordering and inclusion rules are retained.

The subject directory pairs names with codes and groups them under eight disciplines with keyboard-accessible jump links. All 176 codes observed in the indexed directory have definitions, including explicitly marked legacy categories. Legacy codes retain their own counts and routes rather than being merged into their replacements. Unknown future codes remain visible through the fallback. Sources are the [official category taxonomy](https://arxiv.org/category_taxonomy) and [arXiv's taxonomy definitions](https://github.com/arXiv/arxiv-base/blob/develop/arxiv/taxonomy/definitions.py).

Focused taxonomy, grouping, escaping, count preservation and fetch-date rendering checks are in `cmd/arxiv/category_taxonomy_test.go`. Browser fixtures verify directory discipline links and keyboard focus into subject links. Homepage lifecycle and approved-copy regressions cover the final ordering explanation.

### AXG-004: make connections accessible

The paper's semantic-neighbor list exposes full titles and ordinary paper links, with keyboard selection synchronized to the map. The interface explicitly distinguishes semantic similarity from citation relationships. Selected and hovered papers have clearer labels; dense default labeling is reduced. The existing projected-coordinate scaling is retained rather than adding an unverified extra fit operation. The companion list remains useful when the interactive renderer is unavailable.

Citation references and cited-by lists keep their directional meaning, and counts state the index scope. Sparse “Readers Also Viewed” evidence is visibly qualified as reading overlap, not scientific relevance. Template and behavior checks are in `cmd/arxiv/paper_accessibility_test.go`; [related-map browser evidence](evidence/2026-09-19/related-map.json) and [paper layout evidence](evidence/2026-09-19/paper-layout.json) are archived. The latter verifies two full-title companion rows and keyboard selection at 320px when D3 and MathJax are unavailable. No researcher task-timing or map usefulness study was conducted.

### AXG-005: lead with the outcome

The homepage now says “Find a paper. Follow its connections.” It presents a visible search label and primary Quick Search action, then secondary modes with honest availability explanations. The factual example links to `/abs/1706.03762`; it does not promise that a related-work map is always ready. Index coverage remains accessible in a native expandable details element below the browsing list.

This is an editorial decision supported by the implemented interface, not a measured conversion or usability improvement. The browser checks establish layout and interaction behavior, not visitor comprehension.

### Shared accessibility and API documentation

Shared pages have a skip link, one main landmark, visible keyboard focus, labels that remain visible on narrow screens and tooltip behavior that supports focus, Escape dismissal and viewport boundaries. The API documentation's setup controls support tab keyboard navigation and accessible panel labels. Clipboard success and failure are reported, with focus preserved.

The API prompt styling was corrected after inspecting the actual CSS shown in the screenshot. Computed contrast ratios in Chromium fixtures are 16.12:1 for prompt text and 12.48:1 for selected prompt text. Additional sampled ratios were 11.95:1 for the header, 6.87:1 for parameters, 5.48:1 for a badge and 5.24:1 for GET. These are measurements of specific foreground/background combinations, not a blanket WCAG-conformance claim.

Controlled Chromium checks at 1280×900 and 320×900 cover seven shared-layout pages: API documentation, homepage, search, login, feedback, subject directory and a populated category. Each has one main landmark, a working first-tab skip link, no document overflow and no JavaScript errors. API keyboard controls, copy feedback, tooltip dismissal and category anchor navigation pass. Dedicated tests are `TestPublicPagesHaveSingleMainAndSkipTarget` and `TestAPIAgentSetupKeyboardAndCopyFeedback`.

## Changed application files

- Search/home/login: `cmd/arxiv/templates/index.html`, `search.html`, `login.html`; `cmd/arxiv/auth_handlers.go`; `cmd/arxiv/search_availability_test.go`, `seo_test.go`. Existing AXG-001 search/test files remain part of the working tree.
- Paper metadata and exploration: `paper_dates.go`, `paper_dates_test.go`, `paper_dates_postgres_test.go`, `data_fetch.go`, `cmd/arxiv/templates/paper.html`, `cmd/arxiv/paper_accessibility_test.go`; guarded SQL and `deploy/sql/manifest.txt`.
- Browsing: `cmd/arxiv/category_taxonomy.go`, `category_taxonomy_test.go`, `serve.go`, `templates/categories.html`, `templates/category.html`.
- Shared accessibility and API documentation: `cmd/arxiv/templates/head.html`, `foot.html`, `api.html`, `feedback.html`, and `cmd/arxiv/accessibility_test.go`.

## Evidence boundaries and remaining work

- New AXG-002 through AXG-007 changes are recorded as `implemented`; scoped local checks are recorded above. AXG-001 retains its prior `verified` local status. AXG-008 is `in_progress` because related copy is improved but a real signed-in persistence check remains unperformed.
- Initial combined `go test ./...`, `go test -race ./...`, `go vet ./...` and application build pass. Nine PostgreSQL search regressions plus the guarded-date repair pass in normal and race runs. Focused homepage/search tests and `git diff --check` pass after the last homepage ordering-copy adjustment. The release integrator will append final settled-tree results and a real commit reference.
- Archived browser results: [public templates](evidence/2026-09-19/public-templates.json), [search availability](evidence/2026-09-19/search-availability.json), [related map](evidence/2026-09-19/related-map.json), and [paper layout](evidence/2026-09-19/paper-layout.json). Local screenshot sources include `/tmp/arxiv-next-homepage-{390,1280}.png`, `/tmp/arxiv-accessibility-api-{1280,320}.png` and `/tmp/arxiv-accessibility-paper-320.png`.
- Browser fixtures intercept requests and do not establish production backend ranking, worker health, semantic readiness or a successful deployment. No screen-reader user study, axe scan or complete WCAG audit is claimed.
- The three official date histories were checked; the rest of the corpus is not certified or repaired by inference. Guarded production SQL application and post-release endpoint/date verification remain separate release actions.
- The original report site and generated route briefs are unchanged. Current pointers are `START-HERE.md`, `llm.md`, `findings.json` and the append-only `REVISIONS.md`.

## Final integrated checks and release evidence

To be appended by the release integrator after results are known. No deployment is implied by this section.

### Final pre-release checks

The settled application tree passes `go test ./...`, `go test -race ./...`, `go vet ./...`, the application build and `git diff --check`. Nine PostgreSQL search regressions plus the guarded-date migration test pass normally and with the race detector on the isolated disposable PostgreSQL database. [Check record](evidence/2026-09-19/checks.json).

The real D3 browser checks cover projected and fallback maps at 1280px and 320px, full titles, deduplication, keyboard selection, selection/focus preservation on resize, reduced-motion settling and retained usable results after refresh failure. A blocked-D3 case verifies that the companion list remains usable. [Map evidence](evidence/2026-09-19/related-map.json). Independent review found no remaining must-fix issue after repairing resize selection/focus.

Deployment evidence, including the exact image/commit and post-release checks, belongs in `RELEASE-2026-09-19.md` once the release actually occurs.
