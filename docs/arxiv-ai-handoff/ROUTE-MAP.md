# arXiv.gg — Route map and agent implementation handoff

Revision 2.0 · 2026-09-17

## Scope

This maps known product routes and documented interfaces, not every route in the repository. Page-to-API associations below are candidates to investigate and reuse, not verified internal network wiring. All API endpoints are documented but untested in this review. The application repository and backend causes have not been inspected. No arxiv.gg implementation changes have been made.

## Start with this prompt

Read this route map, llm.md, findings.json, and the repository’s own instructions. Locate the actual arxiv.gg checkout, not the report website. Start with route home and AXG-001. Reproduce the current behavior, trace existing page/API code, implement the smallest correction, and verify the acceptance criteria. Reuse existing REST/MCP capabilities where suitable. Keep dated observations intact and append current evidence, changed files, checks, and status to findings.json and REVISIONS.md. Do not invent code paths or endpoint schemas. Follow the user’s deployment scope.

## Journey

- Discover: homepage → search results → paper; category directory → category papers → paper.
- Read and explore: paper → related paper; paper → author → paper.
- Return: paper → sign-in → account/research trail → paper. Account URL and signed-in behavior remain unverified.
- Agent access: API documentation → MCP or REST → paper/search/graph interfaces. These describe agent workflows, not confirmed UI request chains.
- Feedback: navigation link observed; form not reviewed.

## Implementation sequence

1. AXG-001: homepage/search exact-title relevance.
2. AXG-002: search availability and fallback states.
3. AXG-003: paper date meaning and citation scope.
4. AXG-004 + AXG-006: readable maps and honest ordering.
5. AXG-005: homepage promise after its features work.
6. AXG-007 + AXG-008: subject navigation and contextual return flow.

## API facts to preserve

The docs describe title/abstract FTS5 search separately from downloaded-PDF search. Semantic search may return Quick results with fallback=true. Similar-paper requests require readiness. Paper-profile generation exposes hasEmbedding, mapReady, and generator and may queue work. Citation references and cited-by endpoints have opposite directions. Verify all of these contracts in current responses/code before changing callers. Public MCP tools require no login; account and Deep Search flows require a session cookie or API key per the setup text. Do not assume every REST endpoint has identical auth rules.

POST fetch and embedding routes can change cache state or start work; do not use them as passive health checks. Legacy bulk generation is documented as retired. Do not include account keys or cookies in reports or generated prompts.

## Route index

| Route | Evidence | Issues |
|---|---|---|
| `/` | Inspected | AXG-001, AXG-002, AXG-005, AXG-006 |
| `/search + inline homepage results` | Partially inspected | AXG-001, AXG-002 |
| `/categories` | Inspected | AXG-007 |
| `/category/{code}` | Inspected example | AXG-006, AXG-007 |
| `/abs/{id}` | Inspected example | AXG-003, AXG-004, AXG-008 |
| `/author/{name}` | Link observed only | Investigate only |
| `/login` | Copy inspected; auth untested | AXG-002, AXG-008 |
| `Account destination: URL not verified` | Documented flow only | Investigate only |
| `/feedback` | Link observed only | Investigate only |
| `/api/v1/` | Documentation inspected | Investigate only |
| `/mcp` | Documented; not connected | Investigate only |

# Route: Homepage

Path: `/`
Evidence: Inspected
Purpose: Find a paper or start exploring.
Implementation status: proposed
Source: https://arxiv.gg/

## Locate and investigate

Inspect the homepage search controller, shared search components, availability state, and recent-paper query.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Keep one obvious available search action.
- Move coverage detail beneath the primary task.
- Show accurate availability and freshness labels.

## Existing interfaces to investigate

- `GET /api/v1/search/quick` — IDs, titles, authors, keywords, categories. q required; limit default 10. [Documented; endpoint not tested; read]
- `GET /api/v1/search` — FTS5 title and abstract search. q required; category; limit default 20. Do not confuse with whole-PDF search. [Documented; endpoint not tested; read]
- `GET /api/v1/search/semantic` — Abstract vector similarity search. q required; limit default 20. Documentation says Quick fallback returns fallback=true when abstract Search is unavailable. [Documented; endpoint not tested; read]
- `GET /api/v1/search/stream` — Stream Quick, Search, or Deep results using SSE. q required; mode=quick|search|deep (default search); category; limit default 100; Accept: text/event-stream. [Documented; endpoint not tested; read]
- `GET /api/v1/stats` — Cache statistics. Total papers, downloads, queue status; not a service-availability contract unless verified. [Documented; endpoint not tested; read]

## Linked implementation tasks

### AXG-001: Find the known paper (P0)

Status: proposed

Observed 2026-09-17: Quick Search for “attention is all you need” put other titles first. Autocomplete began with “Why Any-Order Autoregressive Models…” while the submitted list began with “Attention Needs to Focus…”. Direct lookup of 1706.03762 opened the intended paper.

Proposal: Prioritize normalized exact-title matches and IDs before broad keyword matches. Use consistent ranking in autocomplete and submitted results. Close suggestions when a search is submitted.

Reproduce:

1. Open the homepage signed out.
2. Enter attention is all you need; inspect suggestions, then submit Quick Search.
3. Record the first results and whether suggestions cover submitted results.
4. Repeat using 1706.03762; compare the destination.

Acceptance criteria:

- [ ] The case-insensitive exact title ranks first when cached.
- [ ] The same query produces compatible top results in suggestions and the full list.
- [ ] Submitting a query removes the overlay and exposes results.

Validation: Ask a researcher to find this paper by title, then by ID. Record success and time; this single observed query is not a relevance benchmark.

Rubin lens: Make the simplest intention work without negotiation: I know the title; take me to the paper.

Draper lens: A famous exact-title query is a credibility test. The first result must earn the promise of discovery.

### AXG-002: Make availability unambiguous (P0)

Status: proposed

Observed 2026-09-17: Semantic and Deep controls were disabled in the accessibility tree, but still appeared as prominent blue and dark buttons. Their unavailable explanation lived in help text. The paper-level related-work map loaded separately.

Proposal: Show a visible status next to unavailable modes. Keep Quick Search as the clear available action. Distinguish signed-out, temporarily unavailable, and paper-ready states instead of conflating them.

Reproduce:

1. Open the homepage signed out.
2. Compare the visual treatment, accessible disabled state, and visible explanation of Quick, Semantic, and Deep.
3. Open /abs/1706.03762 and wait for the related-work map to finish loading.
4. Check each service independently; do not infer a shared backend failure.

Acceptance criteria:

- [ ] Unavailable controls have visible text, accessible explanation, and distinct styling.
- [ ] Status accurately describes each capability independently.
- [ ] A working fallback is offered without erasing the query.

Validation: Show the page for five seconds and ask which search mode can be used now. Separately verify signed-in and outage states.

Rubin lens: One available action should feel unmistakable. Remove the need to decode the system.

Draper lens: Do not invite someone into a promise the current session cannot fulfill. Trust is part of the positioning.

### AXG-005: Lead with the research outcome (P1)

Status: proposed

Observed 2026-09-17: The homepage leads with “Search cached arXiv papers…” and prominent counts for cached papers, prepared abstracts, and semantic preparation percentage. The paper page contains the more distinctive exploration experience.

Proposal: Lead with finding a paper and following its connections. Keep coverage information available below the main task. Offer a real example paper as an entry into the map, with honest availability status.

Reproduce:

1. Read the homepage before interacting.
2. Identify the primary action and compare emphasis on user outcomes versus indexing status.
3. Follow a real paper into its related-work experience and identify what the homepage could truthfully demonstrate.

Acceptance criteria:

- [ ] The first screen communicates the user outcome and a clear search action.
- [ ] Internal indexing detail is secondary but still accessible.
- [ ] Any showcased map or feature works in the current session.

Validation: Compare current copy with each proposed profile direction. Ask new visitors what the product helps them do before measuring clicks.

Rubin lens: Let the work speak. Keep the search field, a clear action, and one meaningful place to begin.

Draper lens: Own the idea of the next useful paper. Show a concrete discovery before asking the reader to care about the machinery.

### AXG-006: Say what “recent” really means (P1)

Status: proposed

Observed 2026-09-17: “Recent Papers” on the homepage began with identifiers from 2016 and 2015. The categories page promised latest cached papers; the machine-learning list began with Playing Atari… (1312.5602) and showed “200 papers” despite a much larger index count on the category directory.

Proposal: Name the actual ordering. Use “Recently added to our index” only if that is the real sort, or sort by submission date for “Latest papers.” Clarify “Showing 200 of…” and expose the active sort.

Reproduce:

1. Compare the homepage Recent Papers list with /categories and /category/cs.LG.
2. Record the visible sort, dates, list limits, and total counts.
3. Trace the actual ordering query before renaming or changing the sort.

Acceptance criteria:

- [ ] List heading and visible sort describe the actual ordering field.
- [ ] Publication or revision dates are visible where relevant.
- [ ] Result caps and total coverage are distinguished.

Validation: Check the ordering field and query behind each list; the observed interface alone does not establish the backend sort.

Rubin lens: A familiar word should not conceal an unfamiliar definition.

Draper lens: Freshness is a promise. Make it exact enough to believe.


## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: Search results

Path: `/search + inline homepage results`
Evidence: Partially inspected
Purpose: Turn a query into a relevant paper.
Implementation status: proposed
Source: https://arxiv.gg/search

## Locate and investigate

Trace suggestions, submitted results, result ranking, SSE events, and fallback presentation. The /search URL was linked; its separate page was not inspected.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Rank cached exact-title matches ahead of broad matches.
- Align suggestions and submitted results.
- Preserve the query and disclose fallback results.

## Existing interfaces to investigate

- `GET /api/v1/search/quick` — IDs, titles, authors, keywords, categories. q required; limit default 10. [Documented; endpoint not tested; read]
- `GET /api/v1/search` — FTS5 title and abstract search. q required; category; limit default 20. Do not confuse with whole-PDF search. [Documented; endpoint not tested; read]
- `GET /api/v1/search/semantic` — Abstract vector similarity search. q required; limit default 20. Documentation says Quick fallback returns fallback=true when abstract Search is unavailable. [Documented; endpoint not tested; read]
- `GET /api/v1/search/stream` — Stream Quick, Search, or Deep results using SSE. q required; mode=quick|search|deep (default search); category; limit default 100; Accept: text/event-stream. [Documented; endpoint not tested; read]
- `GET /api/v1/search/pdf` — Search downloaded PDF content. q required; limit default 50; fuzzy=true. Account/Deep Search flows require authentication per MCP setup text. Verify endpoint-level requirements. [Documented; endpoint not tested; read]

## Linked implementation tasks

### AXG-001: Find the known paper (P0)

Status: proposed

Observed 2026-09-17: Quick Search for “attention is all you need” put other titles first. Autocomplete began with “Why Any-Order Autoregressive Models…” while the submitted list began with “Attention Needs to Focus…”. Direct lookup of 1706.03762 opened the intended paper.

Proposal: Prioritize normalized exact-title matches and IDs before broad keyword matches. Use consistent ranking in autocomplete and submitted results. Close suggestions when a search is submitted.

Reproduce:

1. Open the homepage signed out.
2. Enter attention is all you need; inspect suggestions, then submit Quick Search.
3. Record the first results and whether suggestions cover submitted results.
4. Repeat using 1706.03762; compare the destination.

Acceptance criteria:

- [ ] The case-insensitive exact title ranks first when cached.
- [ ] The same query produces compatible top results in suggestions and the full list.
- [ ] Submitting a query removes the overlay and exposes results.

Validation: Ask a researcher to find this paper by title, then by ID. Record success and time; this single observed query is not a relevance benchmark.

Rubin lens: Make the simplest intention work without negotiation: I know the title; take me to the paper.

Draper lens: A famous exact-title query is a credibility test. The first result must earn the promise of discovery.

### AXG-002: Make availability unambiguous (P0)

Status: proposed

Observed 2026-09-17: Semantic and Deep controls were disabled in the accessibility tree, but still appeared as prominent blue and dark buttons. Their unavailable explanation lived in help text. The paper-level related-work map loaded separately.

Proposal: Show a visible status next to unavailable modes. Keep Quick Search as the clear available action. Distinguish signed-out, temporarily unavailable, and paper-ready states instead of conflating them.

Reproduce:

1. Open the homepage signed out.
2. Compare the visual treatment, accessible disabled state, and visible explanation of Quick, Semantic, and Deep.
3. Open /abs/1706.03762 and wait for the related-work map to finish loading.
4. Check each service independently; do not infer a shared backend failure.

Acceptance criteria:

- [ ] Unavailable controls have visible text, accessible explanation, and distinct styling.
- [ ] Status accurately describes each capability independently.
- [ ] A working fallback is offered without erasing the query.

Validation: Show the page for five seconds and ask which search mode can be used now. Separately verify signed-in and outage states.

Rubin lens: One available action should feel unmistakable. Remove the need to decode the system.

Draper lens: Do not invite someone into a promise the current session cannot fulfill. Trust is part of the positioning.


## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: Subject directory

Path: `/categories`
Evidence: Inspected
Purpose: Choose a field using understandable labels.
Implementation status: proposed
Source: https://arxiv.gg/categories

## Locate and investigate

Inspect taxonomy/name mapping and category directory rendering.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Show subject name beside its code.
- Explain what the count measures.

## Existing interfaces to investigate

- `GET /api/v1/categories` — Category list and paper counts. Validate count scope against displayed list limits. [Documented; endpoint not tested; read]

## Linked implementation tasks

### AXG-007: Translate the subject codes (P2)

Status: proposed

Observed 2026-09-17: The directory showed 176 categories as codes and counts, such as cs.LG and hep-ph. The category detail revealed “machine learning,” but the directory made users interpret the code first.

Proposal: Pair each code with its full subject name. Group by broad discipline, and make the distinction between a directory count and the displayed result count clear.

Reproduce:

1. Open /categories and inspect how names, codes, disciplines, and counts are presented.
2. Open cs.LG and compare the human-readable name and displayed count.

Acceptance criteria:

- [ ] Every category shows its plain-language name and code.
- [ ] Discipline grouping is clear and keyboard-navigable.
- [ ] Counts use consistent scope labels.

Validation: Ask someone outside computer science to find machine learning and someone outside physics to find the relevant physics subject.

Rubin lens: Use language the curious reader already understands. Keep the code as helpful metadata.

Draper lens: Welcome the adjacent-field researcher, not only someone who already knows the catalog.


## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: Subject papers

Path: `/category/{code}`
Evidence: Inspected example
Purpose: Browse papers in a field.
Implementation status: proposed
Source: https://arxiv.gg/category/cs.LG

## Locate and investigate

Inspect the category query and pagination/limit contract. Mapping category pages to these API endpoints is a reuse candidate, not confirmed internal wiring.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- State the real sort order and visible result cap.
- Keep category names consistent across views.

## Existing interfaces to investigate

- `GET /api/v1/categories` — Category list and paper counts. Validate count scope against displayed list limits. [Documented; endpoint not tested; read]
- `GET /api/v1/search` — FTS5 title and abstract search. q required; category; limit default 20. Do not confuse with whole-PDF search. [Documented; endpoint not tested; read]

## Linked implementation tasks

### AXG-006: Say what “recent” really means (P1)

Status: proposed

Observed 2026-09-17: “Recent Papers” on the homepage began with identifiers from 2016 and 2015. The categories page promised latest cached papers; the machine-learning list began with Playing Atari… (1312.5602) and showed “200 papers” despite a much larger index count on the category directory.

Proposal: Name the actual ordering. Use “Recently added to our index” only if that is the real sort, or sort by submission date for “Latest papers.” Clarify “Showing 200 of…” and expose the active sort.

Reproduce:

1. Compare the homepage Recent Papers list with /categories and /category/cs.LG.
2. Record the visible sort, dates, list limits, and total counts.
3. Trace the actual ordering query before renaming or changing the sort.

Acceptance criteria:

- [ ] List heading and visible sort describe the actual ordering field.
- [ ] Publication or revision dates are visible where relevant.
- [ ] Result caps and total coverage are distinguished.

Validation: Check the ordering field and query behind each list; the observed interface alone does not establish the backend sort.

Rubin lens: A familiar word should not conceal an unfamiliar definition.

Draper lens: Freshness is a promise. Make it exact enough to believe.

### AXG-007: Translate the subject codes (P2)

Status: proposed

Observed 2026-09-17: The directory showed 176 categories as codes and counts, such as cs.LG and hep-ph. The category detail revealed “machine learning,” but the directory made users interpret the code first.

Proposal: Pair each code with its full subject name. Group by broad discipline, and make the distinction between a directory count and the displayed result count clear.

Reproduce:

1. Open /categories and inspect how names, codes, disciplines, and counts are presented.
2. Open cs.LG and compare the human-readable name and displayed count.

Acceptance criteria:

- [ ] Every category shows its plain-language name and code.
- [ ] Discipline grouping is clear and keyboard-navigable.
- [ ] Counts use consistent scope labels.

Validation: Ask someone outside computer science to find machine learning and someone outside physics to find the relevant physics subject.

Rubin lens: Use language the curious reader already understands. Keep the code as helpful metadata.

Draper lens: Welcome the adjacent-field researcher, not only someone who already knows the catalog.


## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: Paper + maps

Path: `/abs/{id}`
Evidence: Inspected example
Purpose: Read, inspect provenance, and follow related work.
Implementation status: proposed
Source: https://arxiv.gg/abs/1706.03762

## Locate and investigate

Inspect metadata ingestion/rendering, graph layout, similarity readiness, source links, and research-trail invitation.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Correct date meaning and citation-count scope.
- Make map labels, fit, and keyboard alternatives usable.
- Offer a contextual way to keep this paper.

## Existing interfaces to investigate

- `GET /api/v1/papers/{id}` — Metadata, citation count, references. id is an arXiv identifier. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/citations` — References cited by this paper. Direction: outgoing references. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/cited-by` — Papers that cite this paper. limit defaults to 50. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/graph` — Citation graph nodes and edges. Response schema and rendering contract need verification. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/similar` — Related papers and semantic map data. limit default 80, maximum 160; requires paper readiness. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/export/{format}` — Export metadata. Formats: bibtex, ris, json. [Documented; endpoint not tested; read]
- `POST /api/v1/papers/{id}/embeddings` — Generate a paper profile. Response includes hasEmbedding, mapReady, generator; may queue work. [Documented; endpoint not tested; mutation]
- `POST /api/v1/papers/{id}/fetch` — Fetch and cache an arXiv paper. pdf=true, source=true (default), embedding=true are documented options. [Documented; endpoint not tested; mutation]

## Linked implementation tasks

### AXG-003: Make the facts trustworthy (P0)

Status: proposed

Observed 2026-09-17: The paper page displayed 2023-08-02 with help text calling it the original submission date. arXiv lists first submission on 12 June 2017 and revision on 2 August 2023. “Cited by 3461” explained its cached scope only in help text.

Proposal: Separate “First submitted” from “Latest revision.” Make citation coverage visible: “3,461 citations in our index” at the observed snapshot, rather than implying a universal total.

Reproduce:

1. Open /abs/1706.03762 and inspect the visible date, its help text, and citation count.
2. Compare against https://arxiv.org/abs/1706.03762 submission history.
3. Inspect the ingestion fields and renderer before deciding whether the defect is data mapping or labeling.

Acceptance criteria:

- [ ] First submission and revision date match the source fields.
- [ ] Visible date labels remain correct without hovering.
- [ ] Cached citation totals explicitly state their scope.

Validation: Compare several multi-version records with arXiv submission histories. The verified defect here concerns one paper.

Rubin lens: Metadata should quietly orient the reader. A date needs a precise meaning, not more explanation on hover.

Draper lens: Specific, honest limits are stronger proof than a large unexplained number.

### AXG-004: Turn the map into a reading decision (P1)

Status: proposed

Observed 2026-09-17: The related-work map loaded 39 papers across 3 clusters. In the desktop view, points occupied a small part of the canvas and multiple shortened labels overlapped. The citation graph loaded, with most nodes unlabeled in the default view.

Proposal: Fit the map to its content, label the selected and hovered paper clearly, and provide a synchronized list with full titles and connection type. Preserve the distinction between citations and semantic similarity.

Reproduce:

1. Open /abs/1706.03762 on desktop and allow both graphs to load.
2. Inspect initial fit, overlapping labels, selection behavior, and access to full titles.
3. Check keyboard and narrow-screen behavior during implementation; these were not established in the initial audit.

Acceptance criteria:

- [ ] Default view uses the available canvas without overlapping essential labels.
- [ ] Every paper can be reached in a keyboard-accessible companion list.
- [ ] Selection exposes the full title and explains citation versus similarity.

Validation: Ask users to find one related paper and explain why it is connected. Test keyboard access and a narrow viewport during implementation.

Rubin lens: The visualization should reveal a relationship, not ask the reader to untangle a picture.

Draper lens: This is the proof of the product. Make the moment of finding a useful neighbor visible and comprehensible.

### AXG-008: Invite the return at the right moment (P2)

Status: proposed

Observed 2026-09-17: The paper page offers a free account to retain a research trail. The sign-in page has clear private-trail messaging, but also promotes full-text search while the homepage reported Deep Search unavailable. “Readers Also Viewed” included several entries based on one reader and one view.

Proposal: Connect sign-in to keeping the paper or trail just explored. Make availability claims consistent. Avoid presenting sparse reading overlap as strong relevance evidence; label it clearly or use a more reliable fallback.

Reproduce:

1. Inspect the paper-page account invitation and /login without creating an account.
2. Compare promised search capabilities with current availability.
3. Inspect Readers Also Viewed evidence strength.
4. Use an authorized test account for return-path and trail persistence checks; the initial audit did not test them.

Acceptance criteria:

- [ ] The invitation refers to the paper or trail in context.
- [ ] Return destination is preserved through sign-in.
- [ ] Feature claims and sparse recommendation signals are visibly qualified.

Validation: Verify return navigation and trail persistence in a signed-in test session; neither was tested in this public review.

Rubin lens: Do not interrupt discovery before it has value. Let keeping the trail be the natural next action.

Draper lens: The emotional benefit is continuity: your useful discovery will still be here when you return.


## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: Author papers

Path: `/author/{name}`
Evidence: Link observed only
Purpose: Continue from a paper to its author.
Implementation status: investigate_only
Source: https://arxiv.gg/author/Ashish%20Vaswani

## Locate and investigate

Author links were observed on the paper page. Page behavior and exact matching semantics remain untested; there is no established defect here.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Verify author identity handling, encoded names, and result relevance before filing a new issue.

## Existing interfaces to investigate

- `GET /api/v1/search/quick` — IDs, titles, authors, keywords, categories. q required; limit default 10. [Documented; endpoint not tested; read]
- `GET /api/v1/search` — FTS5 title and abstract search. q required; category; limit default 20. Do not confuse with whole-PDF search. [Documented; endpoint not tested; read]

## Linked implementation tasks

No defect established for this route. Investigate first; add a new stable issue ID only with evidence.

## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: Sign-in / account entry

Path: `/login`
Evidence: Copy inspected; auth untested
Purpose: Keep a private research trail and access account features.
Implementation status: proposed
Source: https://arxiv.gg/login

## Locate and investigate

Inspect login return-path handling and capability checks using an authorized test account. Never embed API keys in handoff documents.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Align feature claims with real availability.
- Preserve the intended return destination.

## Existing interfaces to investigate

- `GET /api/v1/search/pdf` — Search downloaded PDF content. q required; limit default 50; fuzzy=true. Account/Deep Search flows require authentication per MCP setup text. Verify endpoint-level requirements. [Documented; endpoint not tested; read]
- `GET /api/v1/search/stream` — Stream Quick, Search, or Deep results using SSE. q required; mode=quick|search|deep (default search); category; limit default 100; Accept: text/event-stream. [Documented; endpoint not tested; read]

## Linked implementation tasks

### AXG-002: Make availability unambiguous (P0)

Status: proposed

Observed 2026-09-17: Semantic and Deep controls were disabled in the accessibility tree, but still appeared as prominent blue and dark buttons. Their unavailable explanation lived in help text. The paper-level related-work map loaded separately.

Proposal: Show a visible status next to unavailable modes. Keep Quick Search as the clear available action. Distinguish signed-out, temporarily unavailable, and paper-ready states instead of conflating them.

Reproduce:

1. Open the homepage signed out.
2. Compare the visual treatment, accessible disabled state, and visible explanation of Quick, Semantic, and Deep.
3. Open /abs/1706.03762 and wait for the related-work map to finish loading.
4. Check each service independently; do not infer a shared backend failure.

Acceptance criteria:

- [ ] Unavailable controls have visible text, accessible explanation, and distinct styling.
- [ ] Status accurately describes each capability independently.
- [ ] A working fallback is offered without erasing the query.

Validation: Show the page for five seconds and ask which search mode can be used now. Separately verify signed-in and outage states.

Rubin lens: One available action should feel unmistakable. Remove the need to decode the system.

Draper lens: Do not invite someone into a promise the current session cannot fulfill. Trust is part of the positioning.

### AXG-008: Invite the return at the right moment (P2)

Status: proposed

Observed 2026-09-17: The paper page offers a free account to retain a research trail. The sign-in page has clear private-trail messaging, but also promotes full-text search while the homepage reported Deep Search unavailable. “Readers Also Viewed” included several entries based on one reader and one view.

Proposal: Connect sign-in to keeping the paper or trail just explored. Make availability claims consistent. Avoid presenting sparse reading overlap as strong relevance evidence; label it clearly or use a more reliable fallback.

Reproduce:

1. Inspect the paper-page account invitation and /login without creating an account.
2. Compare promised search capabilities with current availability.
3. Inspect Readers Also Viewed evidence strength.
4. Use an authorized test account for return-path and trail persistence checks; the initial audit did not test them.

Acceptance criteria:

- [ ] The invitation refers to the paper or trail in context.
- [ ] Return destination is preserved through sign-in.
- [ ] Feature claims and sparse recommendation signals are visibly qualified.

Validation: Verify return navigation and trail persistence in a signed-in test session; neither was tested in this public review.

Rubin lens: Do not interrupt discovery before it has value. Let keeping the trail be the natural next action.

Draper lens: The emotional benefit is continuity: your useful discovery will still be here when you return.


## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: Account / research trail

Path: `Account destination: URL not verified`
Evidence: Documented flow only
Purpose: Return to useful papers and obtain an account API key.
Implementation status: investigate_only

## Locate and investigate

Do not invent an account URL. API docs mention the Account UI and API keys; no account session was inspected.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Discover the actual account route and persistence behavior in the application repository.

## Existing interfaces to investigate

No REST dependency established. Discover the current implementation; do not invent an endpoint.

## Linked implementation tasks

No defect established for this route. Investigate first; add a new stable issue ID only with evidence.

## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: Feedback

Path: `/feedback`
Evidence: Link observed only
Purpose: Send product feedback.
Implementation status: investigate_only
Source: https://arxiv.gg/feedback

## Locate and investigate

A navigation link was observed. No feedback was submitted and no defect was established.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Inspect the form and submission behavior before suggesting fixes.

## Existing interfaces to investigate

No REST dependency established. Discover the current implementation; do not invent an endpoint.

## Linked implementation tasks

No defect established for this route. Investigate first; add a new stable issue ID only with evidence.

## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: API documentation

Path: `/api/v1/`
Evidence: Documentation inspected
Purpose: Give developers and agents the existing integration contract.
Implementation status: investigate_only
Source: https://arxiv.gg/api/v1/

## Locate and investigate

Inspect docs/API.md and route definitions in the application checkout. The API page links to https://github.com/lantos1618/arxiv.gg/blob/main/docs/API.md; repository contents were not inspected in this review.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Reuse documented interfaces before inventing replacements.
- Check fallback flags, readiness states, auth boundaries, and error responses in current code.

## Existing interfaces to investigate

- `GET /api/v1/papers/{id}` — Metadata, citation count, references. id is an arXiv identifier. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/citations` — References cited by this paper. Direction: outgoing references. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/cited-by` — Papers that cite this paper. limit defaults to 50. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/graph` — Citation graph nodes and edges. Response schema and rendering contract need verification. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/similar` — Related papers and semantic map data. limit default 80, maximum 160; requires paper readiness. [Documented; endpoint not tested; read]
- `POST /api/v1/papers/{id}/fetch` — Fetch and cache an arXiv paper. pdf=true, source=true (default), embedding=true are documented options. [Documented; endpoint not tested; mutation]
- `GET /api/v1/papers/{id}/export/{format}` — Export metadata. Formats: bibtex, ris, json. [Documented; endpoint not tested; read]
- `GET /api/v1/search` — FTS5 title and abstract search. q required; category; limit default 20. Do not confuse with whole-PDF search. [Documented; endpoint not tested; read]
- `GET /api/v1/search/quick` — IDs, titles, authors, keywords, categories. q required; limit default 10. [Documented; endpoint not tested; read]
- `GET /api/v1/search/stream` — Stream Quick, Search, or Deep results using SSE. q required; mode=quick|search|deep (default search); category; limit default 100; Accept: text/event-stream. [Documented; endpoint not tested; read]
- `GET /api/v1/search/semantic` — Abstract vector similarity search. q required; limit default 20. Documentation says Quick fallback returns fallback=true when abstract Search is unavailable. [Documented; endpoint not tested; read]
- `GET /api/v1/search/pdf` — Search downloaded PDF content. q required; limit default 50; fuzzy=true. Account/Deep Search flows require authentication per MCP setup text. Verify endpoint-level requirements. [Documented; endpoint not tested; read]
- `POST /api/v1/papers/{id}/embeddings` — Generate a paper profile. Response includes hasEmbedding, mapReady, generator; may queue work. [Documented; endpoint not tested; mutation]
- `POST /api/v1/embeddings/generate` — Legacy bulk-generation endpoint. Documentation marks bulk generation retired. Do not build new workflows on this route. [Documented; endpoint not tested; retired]
- `GET /api/v1/categories` — Category list and paper counts. Validate count scope against displayed list limits. [Documented; endpoint not tested; read]
- `GET /api/v1/stats` — Cache statistics. Total papers, downloads, queue status; not a service-availability contract unless verified. [Documented; endpoint not tested; read]

## Linked implementation tasks

No defect established for this route. Investigate first; add a new stable issue ID only with evidence.

## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

---
# Route: MCP integration

Path: `/mcp`
Evidence: Documented; not connected
Purpose: Expose paper research tools to an external agent.
Implementation status: investigate_only
Source: https://arxiv.gg/mcp

## Locate and investigate

Documented tools: arxiv_api_overview, arxiv_account, arxiv_search, arxiv_get_paper, arxiv_related_papers. Public tools need no login; account and Deep Search need a session cookie or API key according to the setup text.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Verify the tool schemas and connection in the receiving agent environment.
- Use public tools first; only use account credentials in the appropriate authenticated context.

## Existing interfaces to investigate

No REST dependency established. Discover the current implementation; do not invent an endpoint.

Documented MCP tools: arxiv_api_overview, arxiv_account, arxiv_search, arxiv_get_paper, arxiv_related_papers.

## Linked implementation tasks

No defect established for this route. Investigate first; add a new stable issue ID only with evidence.

## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.

## Complete documented API inventory

- `GET /api/v1/papers/{id}` — Metadata, citation count, references. id is an arXiv identifier. [read; untested]
- `GET /api/v1/papers/{id}/citations` — References cited by this paper. Direction: outgoing references. [read; untested]
- `GET /api/v1/papers/{id}/cited-by` — Papers that cite this paper. limit defaults to 50. [read; untested]
- `GET /api/v1/papers/{id}/graph` — Citation graph nodes and edges. Response schema and rendering contract need verification. [read; untested]
- `GET /api/v1/papers/{id}/similar` — Related papers and semantic map data. limit default 80, maximum 160; requires paper readiness. [read; untested]
- `POST /api/v1/papers/{id}/fetch` — Fetch and cache an arXiv paper. pdf=true, source=true (default), embedding=true are documented options. [mutation; untested]
- `GET /api/v1/papers/{id}/export/{format}` — Export metadata. Formats: bibtex, ris, json. [read; untested]
- `GET /api/v1/search` — FTS5 title and abstract search. q required; category; limit default 20. Do not confuse with whole-PDF search. [read; untested]
- `GET /api/v1/search/quick` — IDs, titles, authors, keywords, categories. q required; limit default 10. [read; untested]
- `GET /api/v1/search/stream` — Stream Quick, Search, or Deep results using SSE. q required; mode=quick|search|deep (default search); category; limit default 100; Accept: text/event-stream. [read; untested]
- `GET /api/v1/search/semantic` — Abstract vector similarity search. q required; limit default 20. Documentation says Quick fallback returns fallback=true when abstract Search is unavailable. [read; untested]
- `GET /api/v1/search/pdf` — Search downloaded PDF content. q required; limit default 50; fuzzy=true. Account/Deep Search flows require authentication per MCP setup text. Verify endpoint-level requirements. [read; untested]
- `POST /api/v1/papers/{id}/embeddings` — Generate a paper profile. Response includes hasEmbedding, mapReady, generator; may queue work. [mutation; untested]
- `POST /api/v1/embeddings/generate` — Legacy bulk-generation endpoint. Documentation marks bulk generation retired. Do not build new workflows on this route. [retired; untested]
- `GET /api/v1/categories` — Category list and paper counts. Validate count scope against displayed list limits. [read; untested]
- `GET /api/v1/stats` — Cache statistics. Total papers, downloads, queue status; not a service-availability contract unless verified. [read; untested]

## Source documents

- https://arxiv.gg/api/v1/
- https://arxiv.gg/
- https://arxiv.gg/abs/1706.03762
- https://arxiv.gg/categories
- https://arxiv.gg/category/cs.LG
- https://arxiv.gg/login
- https://github.com/lantos1618/arxiv.gg/blob/main/docs/API.md (linked by API page; repository contents not inspected)
