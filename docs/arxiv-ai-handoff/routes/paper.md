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
