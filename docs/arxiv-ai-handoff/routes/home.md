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
