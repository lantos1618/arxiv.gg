# Catalog count correction — 2026-09-14

The homepage showed 3,101,899 cached papers divided by a configured reference of 3,045,638 official papers as of 2026-05-16. The resulting 101.8% compared different catalog dates and did not measure coverage. The reference was a fixed configuration value, not a live arXiv count.

Production read-only SQL, with a five-second statement timeout, confirmed:

- 3,101,899 rows in `papers`, keyed uniquely by `id`.
- 81,669 papers with `created >= '2026-05-17'`; newest paper date 2026-09-11.
- Zero version-suffixed IDs, URL/prefix IDs, case/whitespace anomalies, or unexpected modern/legacy ID formats.
- Zero missing titles or abstracts.

These checks found no identifier-related inflation. They do not establish that the cache contains every official paper: a valid coverage measurement would need a matching reference corpus and deduplicated membership comparison.

A separate browser issue incremented the total on every recent-paper `new` event. Fetching an already-cached paper also emits that event. Reconnects, dropped events, and imports do not reconcile the total, so event counting cannot maintain a catalog count.

The correction displays the server's local count snapshot and removes the official percentage and browser increments. The API retains its historical reference fields for compatibility; the deprecated `OfficialArxivCoveragePercent` string is empty to represent an unavailable measurement. Counts refresh in the server background; an open page keeps its snapshot until reload.

The Qwen count represents distinct paper IDs with stored abstract vectors for the configured Qwen model and dimension. It does not audit source-hash freshness or orphaned vectors; no Qwen integrity scan or data mutation was part of this correction.

Validation: homepage regression using the reported counts, stats API compatibility regression, and the existing Node-executed recent-paper lifecycle checks. Full `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, and `git diff --check` passed. No database schema changes are required.
