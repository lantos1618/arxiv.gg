# Security and performance review — 2026-09-14

## Scope

Reviewed the Go HTTP cache, rate limiting, browser/REST/MCP search, public embedding status, frontend streams and analytics, citation/category database paths, and Go runtime advisories. Findings were reproduced using synthetic local test data. Production checks were read-only: low-volume sequential HTTP requests, database activity, and PostgreSQL query plans/results. No load test or inspection of real users' private page content was performed.

## Findings and fixes

| Priority | Finding | Resolution |
| --- | --- | --- |
| High | `X-API-Key` authentication did not bypass the shared response cache. A cold authenticated page could expose the user's navigation email to the next anonymous visitor. | Added API-key bypass, private policies on authenticated query/stream pages, and refusal to retain private/no-store/no-cache, cookie-setting, or unsupported Vary responses. Cookie-setting responses override public cache headers. Regression reproduced an email leak using a synthetic account. |
| High | The configured Go 1.25.10 runtime had ten reachable standard-library advisories in the current vulnerability database. | Upgraded minimum/toolchain/build to supported Go 1.26.8, pinned the official builder digest, and reran govulncheck: no vulnerabilities found. This is static reachability evidence, not evidence that an exploit occurred. |
| Medium | MCP tool errors and semantic fallback notices could return raw database/inference errors. | Public errors now use safe messages; validation and retry guidance remain available. Detailed diagnostics are logged internally. |
| Medium | Successful public embedding-status responses exposed worker error text and lease details. | Public readiness serialization uses an allowlist of progress fields; stored worker diagnostics are unchanged. |
| Medium | Search queries could reach database and inference work at HTTP/MCP body limits. | A shared 2,048-byte query limit applies before browser, REST, MCP, and query-embedding work. |
| Medium | Rate counters reset after inactivity rather than after the configured window, eventually blocking steady users below the intended rate. | Track the start of each fixed window separately from last activity. |
| Performance | Citation graph and sidebar did one metadata query and one count query per related paper on cold caches. | Batch at most 500 IDs per query, reuse metadata/count caches, and retain the citation concurrency limit. |
| Performance | Category freshness checks scanned the full paper catalog for every uncached call, including concurrent calls. | Cache category snapshots for one minute and combine concurrent refreshes. Local metadata updates invalidate the cache; external updates may take up to a minute to appear, in addition to existing HTTP cache lifetimes. |
| Performance | Homepage background tabs held recent-paper streams; reconnects repeatedly rendered the initial snapshot and live rows accumulated. | Stop streams when hidden/offline/leaving, keep one stream/retry timer, back off retries to 30 seconds, cap live rows at 50, and batch initial math rendering. |
| Measurement | Analytics waited for full page load plus five seconds, excluding short visits. | Shared asynchronous tag initializes during page parsing. Existing signed-in/sensitive-page exclusions remain. |

## Measured evidence

Local SQLite regression fixture with 201 related papers:

- Citation graph: 405+ SQL queries before, 6 after.
- Citation sidebar: 403 queries before, 4 after; repeated warm sidebar uses zero SQL.
- Twenty simultaneous category snapshot requests: 20 scans before, 1 after.

Production PostgreSQL `EXPLAIN`, without executing full scans, showed the category freshness check scanning approximately 3.15 million estimated rows. The batched citation count plan uses the existing `idx_citations_to_id` index. Batched metadata/count SQL was also executed successfully inside a read-only transaction with a three-second statement timeout.

Before-change origin HTTP sample (three sequential requests per route, no load test): homepage 2.1/0.6/0.5 ms; paper page 202.5/0.8/0.6 ms; categories 5,434.8/3.1/0.8 ms; Quick search 403.4/333.8/274.7 ms. These combine a cold or partially warm first request and subsequent cache hits; they are not browser timings or a production speedup guarantee.

## Validation

- Full `go test ./... -count=1` and `go test -race ./... -count=1`.
- `go vet ./...` and `git diff --check`.
- Regression tests for synthetic API-key cache leakage, private responses, query bounds, backend error/worker diagnostic redaction, rate windows, citation query counts, and category refresh concurrency.
- Node-executed tests of the actual analytics and recent-paper stream JavaScript. Node was available; the checks did not skip.
- Official `govulncheck ./...`: ten reachable standard-library advisories before upgrade, none after.
- Production PostgreSQL read-only query-plan and query-result checks. No schema changes are part of this release.

## Remaining limits and follow-up

- QuickSearch's broad ILIKE fallback can still perform a parallel scan of the paper table. Redesigning that fallback/index strategy needs a separate relevance and production query-budget evaluation.
- Category cache misses still perform the existing freshness check; this change reduces frequency and duplicate work rather than making a cold scan indexed.
- Synchronous D3 loading and source-status polling on paper pages remain potential frontend optimizations; no browser timing was measured here.
- The cache has limited HTTP revalidation support (for example, explicit max-age=0); no current handler using that combination was identified.
- This review did not perform an OS-image or fully resolved Python/remote GPU-worker dependency audit, a live OAuth flow, or a browser-to-Google receipt test. It does not establish that every attack path is safe.
- GA counts still exclude signed-in users, sensitive pages, blocked scripts, and some automated requests. Use eligible pageviews and geography for advertising estimates.

## Sources

- [Official Go releases](https://go.dev/dl/?mode=json)
- [Go template advisory GO-2026-6091](https://pkg.go.dev/vuln/GO-2026-6091)
- [Go URL advisory GO-2026-6218](https://pkg.go.dev/vuln/GO-2026-6218)

Deployment verification is recorded alongside the release and backups in `/home/ubuntu/arxiv-ops/review-20260914/`.
