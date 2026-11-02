# Testing

## Go Suite

The current tree contains root-package tests for auth/API keys, fetch and sync behavior, exports/sitemaps, feedback, Qwen queues, semantic search, and cross-cutting review regressions. `cmd/arxiv` tests admin auth, API error handling, Qwen HTTP behavior, feedback handlers, MCP, security regressions, and SEO. `cmd/migrate` tests migration failures and data handling.

List the authoritative current inventory instead of relying on a copied count:

```bash
find . -name '*_test.go' -not -path './.git/*' -print | sort
go test -list . ./...
```

## Validation Commands

```bash
# Focused package/test
go test ./... -run TestName

# Full unit/HTTP suite
go test ./...

# Concurrency checks
go test -race ./...

# Static analysis and patch hygiene
go vet ./...
git diff --check

# Python and shell syntax used by workers
python3 -m compileall -q tools
bash -n start.sh tools/*.sh deploy/systemd/*
```

Most Go database tests explicitly clear `DATABASE_URL` and use temporary SQLite databases. That is useful for deterministic unit tests but does not establish production PostgreSQL correctness.

## Browser UI Regressions

The search and graph scripts execute the rendered templates in Chromium with controlled API responses. `TestUIBrowserFixtureServer` serves public fixture data on loopback and never opens a database. It is skipped in normal Go test runs. These checks cover browser interactions; they do not validate production search queries or live graph data.

Install Playwright and its Chromium browser before running the scripts. If those dependencies are stored elsewhere, set `PLAYWRIGHT_MODULE` to the module path and `CHROMIUM_EXECUTABLE_PATH` to the browser executable. For offline graph checks, set `D3_SCRIPT_PATH` to a local D3 v7 script; otherwise the graph harness loads D3 from its configured CDN.

Run from the repository root in a shell; the fixture process runs in the background:

```bash
ui_fixture_dir="$(mktemp -d)"
go test -c -o "$ui_fixture_dir/arxiv-ui-fixture.test" ./cmd/arxiv
ARXIV_UI_TEST_ADDR=127.0.0.1:18081 "$ui_fixture_dir/arxiv-ui-fixture.test" \
  -test.run '^TestUIBrowserFixtureServer$' -test.v -test.timeout=10m \
  >"$ui_fixture_dir/server.log" 2>&1 &
ui_fixture_pid=$!
trap 'kill "$ui_fixture_pid" 2>/dev/null; wait "$ui_fixture_pid" 2>/dev/null; rm -rf "$ui_fixture_dir"' EXIT
export ARXIV_TEST_URL=http://127.0.0.1:18081
curl --retry 30 --retry-connrefused --retry-delay 1 --fail --silent "$ARXIV_TEST_URL/" >/dev/null
node tools/test_search_ui.cjs
node tools/test_graph_ui.cjs
```

Both scripts require a loopback `ARXIV_TEST_URL`. The search suite defaults to port 18081; the graph suite defaults to 8080, so the shared setting above points both at the fixture. Search checks cover history restoration, cancellation, query validation, filters, and identifier shortcuts. Graph checks cover resizing/fullscreen behavior, stale responses, and visible error recovery. Each script prints its results and exits nonzero on failure. Set `ARXIV_TEST_OUTPUT_DIR` when running the search suite to save its JSON report and screenshots; no artifacts are written by default. Rebuild and restart the fixture after template changes because templates are embedded in the test binary.

## Account API Key Checks

The account page shows profile and API keys together, with reading activity in an expandable section. Multiple named keys stay masked in a compact table; each has its own inline Rotate and Revoke confirmation. Create API key adds a separate key without changing existing ones. New keys are shown once with reveal and copy controls. Recoverable key errors remain on the account page. Existing profile/API-key anchors remain valid, the reading-history anchor opens its section, and normal confirmation forms remain available without JavaScript.

The same loopback browser fixture supports `node tools/test_account_ui.cjs`. It uses example account data and fake keys, checks layouts from 320px to 1280px, key/header copying, secret reveal, create/rotate/revoke inline confirmations with the selected key ID, confirmation/cancel/Escape, reading deep links, duplicate submission prevention, and the JavaScript-free fallback. Login is checked in normal, error, and unavailable states at the same widths. Use the browser environment variables above; `ARXIV_TEST_OUTPUT_DIR` saves screenshots of all four account states.

The same fixture supports `node tools/test_admin_ui.cjs` for the four admin screens, active navigation, expandable details, and responsive tables.

Run the PostgreSQL rotation regression against a disposable database:

```bash
ARXIV_TEST_POSTGRES_DSN='postgres://arxivqa@127.0.0.1:55432/arxivqa?sslmode=disable' \
  go test -race . -run '^TestAPIKeyPostgres' -count=1
```

The PostgreSQL API-key tests check independent named keys, ownership boundaries, stale rotation requests, revocation and replacement, authentication with the new key, and rollback when replacement insertion fails. Fixtures use temporary tables and do not rotate a real account key.

## Quick Search PostgreSQL Checks

Run the focused database regressions against an explicitly disposable PostgreSQL instance. The harness uses temporary tables, installs `pg_trgm` if necessary, and avoids application startup migrations:

```bash
ARXIV_TEST_POSTGRES_DSN='postgres://arxivqa@127.0.0.1:55432/arxivqa?sslmode=disable' \
  go test -race . -run '^TestQuickSearchPostgres' -count=1
```

Quick Search keeps exact ranking and counts. Each operation has a 30-second budget, including time waiting for one of two process-wide search slots. Parent deadlines and disconnected clients cancel work sooner. Cold searches on production disk can take substantially longer than warm repeats, so record first-query latency separately; do not claim cold-query performance from repeated warm measurements. SSE reports progress during longer searches.

## Required Integration Coverage

Before deploying changes in these areas, add focused validation:

- **PostgreSQL/pgvector:** fresh schema, upgrade from a production-like copy, required extensions/indexes, lock timeout/failure behavior, and `deploy/sql/2026-07-11-qwen-vector-not-null.sql` preconditions.
- **Qwen:** service health, query fallback markers, abstract and chunk hash freshness, non-NULL dimensions, leased claim/heartbeat/complete/fail fencing, ambiguous completion retry, and multiple workers.
- **HTTP/proxy:** real reverse proxy, trusted/untrusted forwarding headers, rate limits, request/body limits, SSE disconnects, SIGTERM shutdown, and public error redaction.
- **Accounts/security:** Google OAuth callback, session cookies, API-key rotation, same-origin mutation checks, admin allowlists/tokens, and worker tokens.
- **Deployment:** clean immutable image, schema backup, PostgreSQL health identity, preserved volume, exact-image rollback, and schema compatibility after rollback.

## Qwen Smoke Test

```bash
QWEN_EMBEDDING_SERVICE_URL=http://127.0.0.1:8010 \
python3 tools/qwen_pipeline_check.py \
  --scope both --window-minutes 15 --min-recent 1

curl --get 'http://127.0.0.1:8080/api/v1/search/semantic' \
  --data-urlencode 'q=causal representation learning'
```

Assert the response either identifies Qwen semantic mode or explicitly sets `fallback:true`; never accept an unmarked mode change.

## Adding Tests

Prefer table-driven tests for validation boundaries and regression tests for every fixed failure mode. Use `t.TempDir`, `t.Setenv`, request contexts, and deterministic fixtures. Do not depend on live arXiv, OAuth, or GPU services in the default unit suite; put those checks in explicit integration workflows.

## Research API integration

Run against a disposable PostgreSQL database with pgvector. The HTTP integration test initializes the schema and creates fixture papers and account keys; never point it at production.

```bash
ARXIV_TEST_POSTGRES_DSN='postgres://arxivqa@127.0.0.1:55432/arxivqa?sslmode=disable' \
  go test -race . -run '^TestResearchPostgres' -count=1
ARXIV_RESEARCH_TEST_POSTGRES_DSN='postgres://arxivqa@127.0.0.1:55432/arxivqa?sslmode=disable' \
  go test -race ./cmd/arxiv -run '^TestResearchHTTPPostgres$' -count=1
```

These checks cover exact-name ranking, category/date filters before limiting, similarity cutoffs, fresh full-text snippets, account-key authentication, metadata requests, queued-query polling, and batch ordering. The search browser suite also checks that initial search pages start the results stream without waiting for database work.
