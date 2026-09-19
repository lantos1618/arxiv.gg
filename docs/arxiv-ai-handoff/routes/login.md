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
