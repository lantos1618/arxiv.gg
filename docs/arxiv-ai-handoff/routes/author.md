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
