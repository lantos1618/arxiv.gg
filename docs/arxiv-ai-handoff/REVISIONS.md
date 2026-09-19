# Revisions and implementation evidence

## 1.0 — 2026-09-17 — AI handoff created

- Converted the signed-out desktop audit into eight stable records: AXG-001 through AXG-008.
- Added reproduction procedures, acceptance criteria, two review lenses, status rules, and bounded implementation/review prompts.
- All application issues remain `proposed`; no arxiv.gg application code was changed.
- Original report: https://arxiv-fieldnotes-rubin-draper.lantos.chatgpt.site
- The hosted report remains the original published version; these companion files have not been added to that deployment.
- Package validation: record counts, stable IDs, statuses, Markdown task coverage, and local companion links checked during creation. This is document validation, not application acceptance testing.

## Template for the next entry

Copy this structure into a new dated entry; retain prior entries.

- Revision / date:
- Issue IDs:
- Change type: review revision / application implementation / deployment
- Application checkout and revision:
- Current reproduction: environment, actions, observed result
- Original finding still valid: yes / no / partial, with explanation
- Actual cause, if established:
- Changed files:
- Acceptance criteria checked and evidence for each:
- Tests or browser checks run, with results:
- Remaining uncertainty or blocked criteria:
- Status before → after:
- Commit / PR, if available:
- Deployment status and URL, only if applicable and verified:
- Companion files/report sections updated:

## 1.1 — 2026-09-17 — Report usability review

- Reviewed and corrected the report UI: treemap proportions, keyboard focus, phone detail navigation, mobile legend, and ambiguous example actions.
- Added report links to this package and stable AXG identifiers to report details.
- Original application issue records remain unchanged and proposed; this is a report revision only.
- Detailed report QA is recorded in the companion report-site-review.md deliverable.

## 2.0 — 2026-09-17 — Route explorer and portable agent tasks

- Added 11 known route/flow entries, linked to the existing eight AXG issues.
- Added all 16 REST endpoints documented on the inspected API page and the five documented MCP tools. Endpoints remain untested.
- Added per-route task briefs, START-HERE.md, ROUTE-MAP.md, and route-map.json.
- Distinguished inspected pages, observed links, documented flows, and unknown account destination.
- Added generated route explorer at /routes/ with deep links, acceptance criteria, copyable tasks, and downloads.
- Existing arxiv.gg implementation issues remain proposed. No application code or account state was changed.

## 2.1 — 2026-09-17 — Critical assessment and AXG-001 implementation

- Change type: application implementation and local handoff evidence; no deployment.
- Application checkout: /home/ubuntu/arxiv, base e7f0026adabfd01af027b608eae96b08db5538ac; uncommitted working-tree changes, no new commit or PR claimed.
- Current reproduction: signed-out public Quick API returned unrelated first papers for attention is all you need. PostgreSQL code enabled random GIN sampling before ranking; the original-code fixture count was 8,352 of 12,001 matches. Original finding remains valid.
- Root causes: random candidate sampling, missing exact-title priority/stable ties, and pending autocomplete debounce/callbacks surviving dismissal.
- Changed application files: search.go, cmd/arxiv/templates/index.html. Regression files: search_quick_test.go, search_quick_postgres_test.go, cmd/arxiv/frontend_review_test.go.
- Acceptance evidence: exact title and IDs first; consistent prefixes at limits 1/8/100 including case/whitespace/tie/author/stopword fixtures; real homepage JavaScript dismisses pending and stale suggestions. Chromium desktop/narrow fixtures pass Enter, Quick, visible results and keyboard ID navigation. Final review's edit-and-revert regression was reproduced and corrected.
- Checks: full go test ./... and go test -race ./... passed with nine isolated PostgreSQL tests enabled; go vet ./..., application build and git diff --check passed. HTTP/frontend package rechecked after final autocomplete adjustment. No application startup against production.
- Current-data SQL: read-only query ranks 1706.03762 first; final warm query including accurate count took 185.137 ms for 7,735 matches. A cold attempt exceeded 3 seconds. Quick searches now have a 5-second operation budget. No universal latency claim.
- Status: AXG-001 proposed -> confirmed -> implemented -> verified (local acceptance evidence). AXG-002 through AXG-008 remain proposed and unimplemented in this batch.
- Remaining limits: no deployment/public post-deploy verification, researcher usability study, broad-query load benchmark, data repair or category-filtered keyword search redesign. Existing narrow-screen tooltip overflow documented independently against original and patched scripts.
- Companion files: findings.json and llm.md updated; IMPLEMENTATION.md contains assessment and rejected/deferred assumptions; evidence/browser.json and evidence/layout.json retain browser results. Original generated route maps/briefs remain archived revision-2.0 snapshots with their status clarified in START-HERE.md. Hosted report unchanged.

## 2.2 — 2026-09-19 — Requested priorities 1–7 implemented locally

- Change type: application implementation and handoff evidence; no production deployment claimed.
- Application checkout: `/home/ubuntu/arxiv`, base `e7f0026adabfd01af027b608eae96b08db5538ac`; working-tree changes, no new commit or PR recorded at entry time.
- Scope: retained AXG-001; implemented availability (AXG-002), metadata meaning and bounded date repair (AXG-003), fetch ordering/coverage (AXG-006), the 176 observed subject codes (AXG-007), accessible related-paper exploration (AXG-004), and outcome-focused homepage (AXG-005). Related AXG-008 copy and return-path improvements preserve existing flows.
- Established causes/limits: configuration is not worker health; queued completion and fallback messages were being overwritten; category/detail counts have different inclusion scopes; cached dates require provenance checks; current coordinate scaling already exists; original category names omitted current and legacy codes.
- Metadata repair: official histories checked for 1706.03762, 1312.6114 and 1810.04805 only. SQL requires the observed old date pair and preserves fetched_at. Broader DatesNeedReview is a warning heuristic, not corpus certification. No production SQL application claimed.
- Checks known at entry time: scoped rendering/JavaScript regressions, PostgreSQL search and guarded-repair checks, and controlled Chromium desktop/narrow checks pass. Root integration reports normal/race suites, vet and build passing before the last copy review; final settled-tree results and release verification will be appended separately.
- Browser evidence: seven shared-layout pages have zero overflow at 320px, one main landmark and working keyboard skip navigation; API setup tabs/copy feedback and tooltips pass. Measured API prompt contrast is 16.12:1, selected text 12.48:1. Separate homepage configuration/authentication fixtures and related-map/paper-layout checks are recorded under `evidence/2026-09-19/`. No full WCAG certification, screen-reader study or live worker-health claim.
- Status: AXG-001 remains verified locally; AXG-002–007 proposed -> implemented with scoped local evidence; AXG-008 proposed -> in_progress for related improvements with real sign-in/trail persistence unverified.
- Details and changed files: [IMPLEMENTATION-2026-09-19.md](IMPLEMENTATION-2026-09-19.md). Final full checks, real commit, any SQL application, deployment and endpoint acceptance must be recorded with their actual outcomes.
- Companion files: findings.json, llm.md and START-HERE.md current pointers updated; the September 17 implementation record is explicitly historical. Original observations, prior revision entries, generated route briefs/maps and hosted report are preserved.
