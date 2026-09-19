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
