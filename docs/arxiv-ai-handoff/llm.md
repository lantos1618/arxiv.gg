# arXiv.gg — AI implementation brief

Revision: 2.2 · Updated: 2026-09-19 · Original audit: 2026-09-17

## Current implementation disposition

The requested September 19 priorities 1–7 are implemented in the application checkout, with scoped local checks. See [IMPLEMENTATION-2026-09-19.md](IMPLEMENTATION-2026-09-19.md) for current evidence and limits, and [IMPLEMENTATION.md](IMPLEMENTATION.md) for historical AXG-001 evidence. AXG-001 retains its locally verified status; AXG-002 through AXG-007 are implemented. AXG-008 has related improvements but remains in progress pending real sign-in/trail validation. No production deployment or production date repair is claimed in these records. The generated route briefs retain the original audit context.

## Route-based handoff

Start with [START-HERE.md](START-HERE.md), then [ROUTE-MAP.md](ROUTE-MAP.md). The route map connects known pages to documented APIs, existing AXG tasks, and per-route briefs under routes/. API-to-page associations are candidates, not verified internal wiring. The API documentation was inspected after the original browser audit; endpoints were not executed.

## Start here

This document turns a browser review into bounded implementation tasks. It is a handoff, not proof that the current product still has these problems. The arxiv.gg application repository is not included in this package; the adjacent report site is a separate product. Locate the real application checkout and its instructions before implementing fixes. Do not mistake editing the report for fixing arxiv.gg.

[Interactive review](https://arxiv-fieldnotes-rubin-draper.lantos.chatgpt.site)

Companion files: [findings.json](findings.json) contains structured issue records; [AGENTS.md](AGENTS.md) defines the working protocol; [REVISIONS.md](REVISIONS.md) records changes and evidence.

## Product intent

Help a researcher find a paper, judge its relevance, and follow a useful connection. Preserve direct ID lookup, source/PDF access, the lightweight reading experience, citation graphs, and related-work discovery.

- **Rubin lens:** clarity, restraint, fewer choices, readable papers, progressive disclosure.
- **Draper lens:** a truthful memorable promise, visible proof, and a reason to return.
- Resolve tension by prioritizing scientific accuracy, accessibility, and working behavior, then clarity, then persuasion. These are inspired lenses, not quotations or endorsements.

## Evidence and limits

The initial signed-out desktop review covered the homepage, one exact-title query, direct ID lookup, one paper detail, loaded citation and related-work graphs, categories, cs.LG, and sign-in copy. The observed build was e7f0026adabf, displayed as 2026-09-17 15:16 UTC.

Semantic and Deep search were disabled on the homepage during that visit; the paper-level related-work map loaded. Do not claim all semantic functionality was broken. Signed-in behavior, mobile behavior, backend causes, and conversion impact were not tested. Counts and feature availability are historical snapshots. Treemap weights are editorial allocation, not traffic, severity probabilities, or measured impact. P0 means first implementation batch in this review, not a production incident classification.

## Suggested execution order

1. AXG-001–003: exact-title retrieval, availability states, metadata meaning.
2. AXG-004 and AXG-006: readable connections and truthful list ordering.
3. AXG-005: outcome-focused homepage, after its claims can be demonstrated.
4. AXG-007–008: accessible subject navigation and contextual research-trail invitation.

Start with AXG-001 alone unless the user requests a larger batch. Keep changes reviewable. Do not add dependencies or redesign unrelated routes merely to satisfy a creative lens.

## Implementation loop

1. Read repository instructions and inspect existing architecture, tests, and user changes. Fill in actual code locations; do not invent paths from this report.
2. Reproduce the selected issue on the current revision. Record date, environment, query, and outcome. Mark already-fixed or cannot-reproduce issues honestly.
3. Identify the cause. The proposed fix is a hypothesis until code and current behavior support it.
4. Implement the smallest coherent correction, preserving existing features and contracts.
5. Run focused checks that establish the acceptance criteria. For retrieval/data defects, add meaningful regression coverage; for presentation, inspect the relevant desktop, narrow-screen, and keyboard states. Do not treat visual plausibility as functional verification.
6. Update the issue status and revision log with changed files, tests, remaining uncertainty, and commit when available. Keep report copy consistent when revising the report too.
7. Summarize the before/after behavior and evidence. Follow the user's deployment scope and repository rules; this brief itself does not authorize production deployment or destructive data changes.

## Task records

The original observations and proposed acceptance criteria below are preserved. Status and dated implementation notes reflect subsequent application work; unchecked historical criteria are not blanket claims of current failure or completed production verification. Read the current implementation entry before repeating a task.

### AXG-001 — Find the known paper

- Priority: P0
- Status: verified (local acceptance checks; not deployed)
- Effort: Medium effort (estimate)
- Report key: `search`
- Sources: [https://arxiv.gg/](https://arxiv.gg/)
- Code locations: `search.go`, `cmd/arxiv/templates/index.html`; regressions in `search_quick_test.go`, `search_quick_postgres_test.go`, `cmd/arxiv/frontend_review_test.go`.

**Implementation update — 2026-09-19:** Existing exact-title/ID ranking and cancellation fixes preserved. Nine PostgreSQL search regressions pass in normal and race runs; actual-JavaScript and Chromium fixture checks still pass. No deployment or general relevance/load guarantee. See [the current implementation record](IMPLEMENTATION-2026-09-19.md).

**Observed on 2026-09-17:** Quick Search for “attention is all you need” put other titles first. Autocomplete began with “Why Any-Order Autoregressive Models…” while the submitted list began with “Attention Needs to Focus…”. Direct lookup of 1706.03762 opened the intended paper.

**Reproduce / investigate:**

1. Open the homepage signed out.
2. Enter attention is all you need; inspect suggestions, then submit Quick Search.
3. Record the first results and whether suggestions cover submitted results.
4. Repeat using 1706.03762; compare the destination.

**Proposed improvement:** Prioritize normalized exact-title matches and IDs before broad keyword matches. Use consistent ranking in autocomplete and submitted results. Close suggestions when a search is submitted.

**Acceptance criteria:**

- [ ] The case-insensitive exact title ranks first when cached.
- [ ] The same query produces compatible top results in suggestions and the full list.
- [ ] Submitting a query removes the overlay and exposes results.

**Validation plan:** Ask a researcher to find this paper by title, then by ID. Record success and time; this single observed query is not a relevance benchmark.

**Rubin:** Make the simplest intention work without negotiation: I know the title; take me to the paper.

**Draper:** A famous exact-title query is a credibility test. The first result must earn the promise of discovery.

### AXG-002 — Make availability unambiguous

- Priority: P0
- Status: implemented (local checks recorded; not deployed)
- Effort: Small–medium effort (estimate)
- Report key: `availability`
- Sources: [https://arxiv.gg/](https://arxiv.gg/)
- Code locations: `cmd/arxiv/templates/index.html`, `cmd/arxiv/templates/search.html`, `cmd/arxiv/templates/login.html`, `cmd/arxiv/templates/head.html`, `cmd/arxiv/search_availability_test.go`.

**Implementation update — 2026-09-19:** Visible accessible mode explanations distinguish configuration, sign-in requirement and runtime queues/errors. Conditional render and JavaScript tests pass; Chromium fixtures cover configured/unconfigured and signed-in/out interfaces. Deep queue state remains queued, Semantic fallback explanations persist, server retry delay is honored, query survives login. Configuration is not worker-health evidence. See [the current implementation record](IMPLEMENTATION-2026-09-19.md).

**Observed on 2026-09-17:** Semantic and Deep controls were disabled in the accessibility tree, but still appeared as prominent blue and dark buttons. Their unavailable explanation lived in help text. The paper-level related-work map loaded separately.

**Reproduce / investigate:**

1. Open the homepage signed out.
2. Compare the visual treatment, accessible disabled state, and visible explanation of Quick, Semantic, and Deep.
3. Open /abs/1706.03762 and wait for the related-work map to finish loading.
4. Check each service independently; do not infer a shared backend failure.

**Proposed improvement:** Show a visible status next to unavailable modes. Keep Quick Search as the clear available action. Distinguish signed-out, temporarily unavailable, and paper-ready states instead of conflating them.

**Acceptance criteria:**

- [ ] Unavailable controls have visible text, accessible explanation, and distinct styling.
- [ ] Status accurately describes each capability independently.
- [ ] A working fallback is offered without erasing the query.

**Validation plan:** Show the page for five seconds and ask which search mode can be used now. Separately verify signed-in and outage states.

**Rubin:** One available action should feel unmistakable. Remove the need to decode the system.

**Draper:** Do not invite someone into a promise the current session cannot fulfill. Trust is part of the positioning.

### AXG-003 — Make the facts trustworthy

- Priority: P0
- Status: implemented (local checks recorded; not deployed)
- Effort: Small–medium effort (estimate)
- Report key: `metadata`
- Sources: [https://arxiv.org/abs/1706.03762](https://arxiv.org/abs/1706.03762) · [https://arxiv.gg/abs/1706.03762](https://arxiv.gg/abs/1706.03762)
- Code locations: `paper_dates.go`, `data_fetch.go`, `paper_dates_test.go`, `paper_dates_postgres_test.go`, `cmd/arxiv/templates/paper.html`, `cmd/arxiv/paper_accessibility_test.go`, `deploy/sql/2026-09-19-reviewed-paper-dates.sql`, `deploy/sql/manifest.txt`.

**Implementation update — 2026-09-19:** Implemented explicit first-submitted/latest-revision and index-scoped citation labels, conservative DatesNeedReview warnings, missing-source parser correction, and SQL guarded by old date values for only 1706.03762, 1312.6114 and 1810.04805. Official histories checked for those IDs. Guarded repair passes disposable PostgreSQL tests in normal/race runs. SQL not claimed applied to production; broader dates remain uncertified. See [the current implementation record](IMPLEMENTATION-2026-09-19.md).

**Observed on 2026-09-17:** The paper page displayed 2023-08-02 with help text calling it the original submission date. arXiv lists first submission on 12 June 2017 and revision on 2 August 2023. “Cited by 3461” explained its cached scope only in help text.

**Reproduce / investigate:**

1. Open /abs/1706.03762 and inspect the visible date, its help text, and citation count.
2. Compare against https://arxiv.org/abs/1706.03762 submission history.
3. Inspect the ingestion fields and renderer before deciding whether the defect is data mapping or labeling.

**Proposed improvement:** Separate “First submitted” from “Latest revision.” Make citation coverage visible: “3,461 citations in our index” at the observed snapshot, rather than implying a universal total.

**Acceptance criteria:**

- [ ] First submission and revision date match the source fields.
- [ ] Visible date labels remain correct without hovering.
- [ ] Cached citation totals explicitly state their scope.

**Validation plan:** Compare several multi-version records with arXiv submission histories. The verified defect here concerns one paper.

**Rubin:** Metadata should quietly orient the reader. A date needs a precise meaning, not more explanation on hover.

**Draper:** Specific, honest limits are stronger proof than a large unexplained number.

### AXG-004 — Turn the map into a reading decision

- Priority: P1
- Status: implemented (local checks recorded; not deployed)
- Effort: Medium effort (estimate)
- Report key: `map`
- Sources: [https://arxiv.gg/abs/1706.03762](https://arxiv.gg/abs/1706.03762)
- Code locations: `cmd/arxiv/templates/paper.html`, `cmd/arxiv/paper_accessibility_test.go`.

**Implementation update — 2026-09-19:** Implemented keyboard-accessible companion links with full titles, synchronized map selection, simpler labels and explicit semantic-versus-citation meaning; existing projected-coordinate scaling retained. Related-map and no-renderer narrow-layout fixture evidence is archived. No researcher usability study or general graph-layout guarantee. See [the current implementation record](IMPLEMENTATION-2026-09-19.md).

**Observed on 2026-09-17:** The related-work map loaded 39 papers across 3 clusters. In the desktop view, points occupied a small part of the canvas and multiple shortened labels overlapped. The citation graph loaded, with most nodes unlabeled in the default view.

**Reproduce / investigate:**

1. Open /abs/1706.03762 on desktop and allow both graphs to load.
2. Inspect initial fit, overlapping labels, selection behavior, and access to full titles.
3. Check keyboard and narrow-screen behavior during implementation; these were not established in the initial audit.

**Proposed improvement:** Fit the map to its content, label the selected and hovered paper clearly, and provide a synchronized list with full titles and connection type. Preserve the distinction between citations and semantic similarity.

**Acceptance criteria:**

- [ ] Default view uses the available canvas without overlapping essential labels.
- [ ] Every paper can be reached in a keyboard-accessible companion list.
- [ ] Selection exposes the full title and explains citation versus similarity.

**Validation plan:** Ask users to find one related paper and explain why it is connected. Test keyboard access and a narrow viewport during implementation.

**Rubin:** The visualization should reveal a relationship, not ask the reader to untangle a picture.

**Draper:** This is the proof of the product. Make the moment of finding a useful neighbor visible and comprehensible.

### AXG-005 — Lead with the research outcome

- Priority: P1
- Status: implemented (local checks recorded; not deployed)
- Effort: Small effort (estimate)
- Report key: `promise`
- Sources: [https://arxiv.gg/](https://arxiv.gg/)
- Code locations: `cmd/arxiv/templates/index.html`, `cmd/arxiv/templates/head.html`, `cmd/arxiv/search_availability_test.go`.

**Implementation update — 2026-09-19:** Implemented outcome-focused heading, primary Quick Search, factual Attention Is All You Need paper link and native expandable index coverage. Narrow/desktop fixtures pass. No promise that example maps are always ready and no measured conversion improvement. See [the current implementation record](IMPLEMENTATION-2026-09-19.md).

**Observed on 2026-09-17:** The homepage leads with “Search cached arXiv papers…” and prominent counts for cached papers, prepared abstracts, and semantic preparation percentage. The paper page contains the more distinctive exploration experience.

**Reproduce / investigate:**

1. Read the homepage before interacting.
2. Identify the primary action and compare emphasis on user outcomes versus indexing status.
3. Follow a real paper into its related-work experience and identify what the homepage could truthfully demonstrate.

**Proposed improvement:** Lead with finding a paper and following its connections. Keep coverage information available below the main task. Offer a real example paper as an entry into the map, with honest availability status.

**Acceptance criteria:**

- [ ] The first screen communicates the user outcome and a clear search action.
- [ ] Internal indexing detail is secondary but still accessible.
- [ ] Any showcased map or feature works in the current session.

**Validation plan:** Compare current copy with each proposed profile direction. Ask new visitors what the product helps them do before measuring clicks.

**Rubin:** Let the work speak. Keep the search field, a clear action, and one meaningful place to begin.

**Draper:** Own the idea of the next useful paper. Show a concrete discovery before asking the reader to care about the machinery.

### AXG-006 — Say what “recent” really means

- Priority: P1
- Status: implemented (local checks recorded; not deployed)
- Effort: Small–medium effort (estimate)
- Report key: `freshness`
- Sources: [https://arxiv.gg/category/cs.LG](https://arxiv.gg/category/cs.LG)
- Code locations: `search.go`, `cmd/arxiv/templates/index.html`, `cmd/arxiv/templates/categories.html`, `cmd/arxiv/templates/category.html`, `cmd/arxiv/category_taxonomy_test.go`.

**Implementation update — 2026-09-19:** Homepage identifies initial fetched-at/ID ordering, missing-date placement, 50-record cap and live insertion order. Category detail shows fetch dates, 200-record cap and downloaded-source restriction; directory totals explicitly count broader metadata. Focused render and lifecycle checks pass. No submission-date sort introduced. See [the current implementation record](IMPLEMENTATION-2026-09-19.md).

**Observed on 2026-09-17:** “Recent Papers” on the homepage began with identifiers from 2016 and 2015. The categories page promised latest cached papers; the machine-learning list began with Playing Atari… (1312.5602) and showed “200 papers” despite a much larger index count on the category directory.

**Reproduce / investigate:**

1. Compare the homepage Recent Papers list with /categories and /category/cs.LG.
2. Record the visible sort, dates, list limits, and total counts.
3. Trace the actual ordering query before renaming or changing the sort.

**Proposed improvement:** Name the actual ordering. Use “Recently added to our index” only if that is the real sort, or sort by submission date for “Latest papers.” Clarify “Showing 200 of…” and expose the active sort.

**Acceptance criteria:**

- [ ] List heading and visible sort describe the actual ordering field.
- [ ] Publication or revision dates are visible where relevant.
- [ ] Result caps and total coverage are distinguished.

**Validation plan:** Check the ordering field and query behind each list; the observed interface alone does not establish the backend sort.

**Rubin:** A familiar word should not conceal an unfamiliar definition.

**Draper:** Freshness is a promise. Make it exact enough to believe.

### AXG-007 — Translate the subject codes

- Priority: P2
- Status: implemented (local checks recorded; not deployed)
- Effort: Small effort (estimate)
- Report key: `categories`
- Sources: [https://arxiv.gg/categories](https://arxiv.gg/categories)
- Code locations: `cmd/arxiv/category_taxonomy.go`, `cmd/arxiv/category_taxonomy_test.go`, `cmd/arxiv/serve.go`, `cmd/arxiv/templates/categories.html`, `cmd/arxiv/templates/category.html`.

**Implementation update — 2026-09-19:** Implemented full names for all 176 observed category codes, eight discipline groups and keyboard jump links. Legacy codes retain distinct routes/counts and visible legacy labels; unknown future codes retain a fallback. Sources are official arXiv taxonomy/definitions. Focused tests and controlled browser navigation pass. See [the current implementation record](IMPLEMENTATION-2026-09-19.md).

**Observed on 2026-09-17:** The directory showed 176 categories as codes and counts, such as cs.LG and hep-ph. The category detail revealed “machine learning,” but the directory made users interpret the code first.

**Reproduce / investigate:**

1. Open /categories and inspect how names, codes, disciplines, and counts are presented.
2. Open cs.LG and compare the human-readable name and displayed count.

**Proposed improvement:** Pair each code with its full subject name. Group by broad discipline, and make the distinction between a directory count and the displayed result count clear.

**Acceptance criteria:**

- [ ] Every category shows its plain-language name and code.
- [ ] Discipline grouping is clear and keyboard-navigable.
- [ ] Counts use consistent scope labels.

**Validation plan:** Ask someone outside computer science to find machine learning and someone outside physics to find the relevant physics subject.

**Rubin:** Use language the curious reader already understands. Keep the code as helpful metadata.

**Draper:** Welcome the adjacent-field researcher, not only someone who already knows the catalog.

### AXG-008 — Invite the return at the right moment

- Priority: P2
- Status: in_progress (related improvements; real sign-in/trail validation remains)
- Effort: Medium effort (estimate)
- Report key: `return`
- Sources: [https://arxiv.gg/login](https://arxiv.gg/login)
- Code locations: `cmd/arxiv/templates/login.html`, `cmd/arxiv/templates/paper.html`, `cmd/arxiv/templates/index.html`, `cmd/arxiv/templates/search.html`, `cmd/arxiv/auth_handlers.go`, `cmd/arxiv/search_availability_test.go`.

**Implementation update — 2026-09-19:** Related batch qualifies Deep availability and sparse reading overlap, preserves contextual paper invitation, and regression-tests punctuated Deep queries through login return URLs. Existing sanitized OAuth next paths retained. Real Google sign-in and reading-trail persistence were not exercised; full acceptance remains in progress. See [the current implementation record](IMPLEMENTATION-2026-09-19.md).

**Observed on 2026-09-17:** The paper page offers a free account to retain a research trail. The sign-in page has clear private-trail messaging, but also promotes full-text search while the homepage reported Deep Search unavailable. “Readers Also Viewed” included several entries based on one reader and one view.

**Reproduce / investigate:**

1. Inspect the paper-page account invitation and /login without creating an account.
2. Compare promised search capabilities with current availability.
3. Inspect Readers Also Viewed evidence strength.
4. Use an authorized test account for return-path and trail persistence checks; the initial audit did not test them.

**Proposed improvement:** Connect sign-in to keeping the paper or trail just explored. Make availability claims consistent. Avoid presenting sparse reading overlap as strong relevance evidence; label it clearly or use a more reliable fallback.

**Acceptance criteria:**

- [ ] The invitation refers to the paper or trail in context.
- [ ] Return destination is preserved through sign-in.
- [ ] Feature claims and sparse recommendation signals are visibly qualified.

**Validation plan:** Verify return navigation and trail persistence in a signed-in test session; neither was tested in this public review.

**Rubin:** Do not interrupt discovery before it has value. Let keeping the trail be the natural next action.

**Draper:** The emotional benefit is continuity: your useful discovery will still be here when you return.

## Candidate copy, subject to working capabilities

Rubin direction: **Start with a paper. Follow an idea.** Supporting copy: Search by title, author, or arXiv ID. Explore the work around it. Action: Find papers.

Draper direction: **Your next insight has a reference.** Supporting copy: Find a paper. Explore its connections. Build your next idea. Action: Explore a paper.

These are proposals, not validated winners. Never advertise a capability that is unavailable in the current state. Keep indexing coverage accessible without letting internal preparation terminology dominate the main task.

## Prompt for a coding agent

> Read AGENTS.md and llm.md in this handoff, then inspect the actual arxiv.gg repository and its instructions. Implement AXG-001 first. Reproduce it against the current code, identify the cause, make the smallest coherent fix, and verify every applicable acceptance criterion. Preserve unrelated work. Update findings.json and REVISIONS.md with actual evidence, code locations, status, and commit when available. Do not claim the issue is fixed from a report edit alone. If the application checkout is unavailable, report that specific blocker. Return a concise before/after summary and remaining risks. Follow the current user’s deployment scope.

## Prompt for revising the review

> Revisit the current arxiv.gg experience and revise this review using fresh evidence. Preserve stable AXG issue IDs and the dated original observation. Append current verification evidence; close resolved findings without silently deleting their history. Add new IDs for new findings. Keep the Markdown, findings.json, treemap report, and revision log consistent where applicable. Separate facts, design judgments, and untested hypotheses. Apply the Rubin and Draper lenses without impersonation or invented claims.

## Completion definition

An implementation issue is verified only when current checks demonstrate its applicable acceptance criteria. Publication is a separate state: record the deployment URL and version only after confirmed success. A review revision does not change application implementation status. If acceptance criteria change, record why; do not weaken them simply to make the issue appear complete.

## API and MCP integration context — added in revision 2.0

Source: https://arxiv.gg/api/v1/ (documentation inspected 2026-09-17). Complete route and endpoint inventory: ROUTE-MAP.md and route-map.json. Reuse existing search, metadata, citations, similar-paper, export, categories, and stats interfaces where appropriate after checking their actual contract.

- Semantic search is documented to fall back to Quick results with fallback=true. A fallback response is not proof that semantic retrieval succeeded.
- Similar-paper requests require readiness. Profile generation documents hasEmbedding, mapReady, and generator; queued work must be handled separately from ready data.
- FTS5 title/abstract search, abstract-vector search, and downloaded-PDF search are distinct. Match product copy and states to the actual mode.
- Citation references and cited-by are opposite relationships. Keep graph semantics explicit.
- Public MCP tools require no login; account/Deep Search flows require a session cookie or API key per the setup text. Endpoint-specific rules still need verification.
- POST fetch/profile-generation requests can mutate state or queue work. Do not run them as passive availability probes. The bulk-generation route is documented as retired.
- No MCP connection was configured or invoked in this review. Never put account keys or cookies into this handoff or copied prompts.

Documented MCP tools: arxiv_api_overview, arxiv_account, arxiv_search, arxiv_get_paper, arxiv_related_papers.
