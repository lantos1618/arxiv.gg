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
