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
