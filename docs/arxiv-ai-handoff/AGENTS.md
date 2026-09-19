# arXiv.gg review handoff

## Scope

This directory contains review evidence and an implementation brief, not the arxiv.gg application. Read `START-HERE.md`, `ROUTE-MAP.md`, and `llm.md` before using the tasks. The neighboring `arxiv-review` directory is the report website, not the application under review.

When transferring this package into an application checkout, preserve its existing AGENTS.md and merge only relevant guidance. Repository and user instructions take precedence over this review. This file does not grant deployment, account, external communication, or data-deletion authorization.

## Working protocol

- Treat dated observations as historical evidence; verify current behavior before implementation.
- Preserve stable AXG issue IDs. Never renumber them to match sorting or delete resolved history.
- Keep original observations intact. Append new verification evidence with date, environment, outcome, and source.
- Identify actual application code locations before proposing file-specific edits.
- Keep facts, root-cause hypotheses, design judgments, and estimates distinct.
- Use the Rubin lens for clarity and restraint, and the Draper lens for truthful positioning. Accuracy and accessibility take precedence.
- Work on a bounded issue or explicitly requested batch. Preserve unrelated user changes.
- Verify applicable acceptance criteria. Use focused regression checks for logic/data behavior and browser checks for relevant UI changes; avoid tests that merely repeat implementation.
- Record changed files, checks, remaining gaps, and a real commit reference when available. Never invent test results or commits.
- Distinguish changes to this report from changes to the actual application.

## Files and revisions

- `llm.md`: readable context, procedures, acceptance criteria, and prompts.
- `findings.json`: structured issue records for automation and tracking.
- `REVISIONS.md`: append-only revision and implementation evidence log.

Synchronize changed issue meaning across the Markdown and JSON. If modifying the report website as part of the same request, update its corresponding report key too. Do not silently change implementation status when only editing prose.

Allowed statuses: `proposed`, `confirmed`, `in_progress`, `implemented`, `verified`, `cannot_reproduce`, `deferred`.

`implemented` means code changed; `verified` requires recorded acceptance evidence. `cannot_reproduce` requires a current attempt and result, not absence of investigation. Deployment is recorded separately from implementation status.

## Route-based work

Use route-map.json to associate known routes with issue IDs and documented interfaces; use routes/*.md for bounded agent tasks. These are not verified internal request chains. Preserve evidence labels and unknown routes, including the unverified Account URL. Test the current contracts before changing callers. Keep the original observation intact when adding new API evidence.

For revisions to the report itself, data/route-map.json and dist/handoff/findings.json are the route generator inputs in the report checkout. Run scripts/build-route-map.py to regenerate the explorer, route briefs, route-map Markdown/JSON and ZIP. Do not hand-edit generated route briefs. The original treemap still has its own issue copy and needs synchronization when issue meaning changes.
