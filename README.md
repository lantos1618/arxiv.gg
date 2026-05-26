# arxiv.gg

arxiv.gg is a fast, searchable arXiv discovery app with semantic search, citation graphs, author pages, category pages, exports, and an SEO-friendly public paper index.

[Live site](https://arxiv.gg) · [CLI docs](docs/CLI.md) · [API docs](docs/API.md) · [Deployment runbook](docs/DEPLOYMENT_RUNBOOK_2026-05-15.md) · [SEO report](docs/SEO_INDEXING_REPORT_2026-05-16.md)

## What It Does

- Browses paper pages at canonical `/abs/{id}` URLs.
- Searches papers by keyword, PDF text, category, author, and semantic similarity.
- Shows paper metadata, abstracts, PDFs, citation relationships, and export formats.
- Builds author and category discovery pages for long-tail research browsing.
- Serves a REST API for papers, search, embeddings, citations, stats, and exports.
- Publishes full-corpus sitemap shards and IndexNow submissions for faster search indexing.

## Current Production Shape

- Go web app and CLI.
- PostgreSQL with pgvector in production.
- SQLite fallback for local/offline use.
- Python embedding service using sentence-transformers.
- Docker Compose deployment with app and database health checks.
- Cloudflare tunnel in front of `https://arxiv.gg`.
- Public sitemap index covering 600k+ paper URLs.

## Quick Start

Build the CLI:

```bash
go build -o bin/arxiv ./cmd/arxiv
```

Run a small local cache with SQLite:

```bash
export ARXIV_CACHE="$HOME/.cache/arxiv"
./bin/arxiv sync -set cs -from 2024-01-01
./bin/arxiv serve -port 8080
```

Open `http://localhost:8080`.

Fetch a specific paper:

```bash
./bin/arxiv fetch 1706.03762
./bin/arxiv fetch -pdf 1706.03762
```

Search locally:

```bash
./bin/arxiv search "attention mechanism"
./bin/arxiv search -category cs.CL "language model"
```

## Web App

The web interface includes:

- Fast keyword search with live results.
- Semantic search when embeddings are available.
- Paper pages with abstracts, authors, categories, PDFs, source links, and export actions.
- Citation graph visualization.
- Author profiles with publication history and collaborators.
- Category pages with canonical URLs, meta descriptions, and structured data.
- `/health` for container and uptime monitoring.

## REST API

All API routes live under `/api/v1`.

Examples:

```bash
curl https://arxiv.gg/api/v1/stats
curl https://arxiv.gg/api/v1/papers/1706.03762
curl "https://arxiv.gg/api/v1/search?q=transformer&limit=10"
curl "https://arxiv.gg/api/v1/search/semantic?q=attention+mechanisms&limit=10"
curl https://arxiv.gg/api/v1/papers/1706.03762/export/bibtex
```

See [docs/API.md](docs/API.md) for endpoint details.

## Semantic Search

Embeddings are generated from paper title and abstract text.

Local batch generation:

```bash
pip3 install -r tools/requirements.txt
python3 tools/generate_embeddings.py "$ARXIV_CACHE" --limit 1000
```

CLI generation:

```bash
./bin/arxiv reindex --embeddings --limit 1000
./bin/arxiv fetch --with-embedding 2301.00001
```

Production uses PostgreSQL and pgvector; local development can fall back to SQLite.

## SEO And Indexing

arxiv.gg is built for crawlable research pages:

- Self-referencing canonical URLs.
- Paper `ScholarlyArticle` JSON-LD.
- Author `ProfilePage` JSON-LD.
- Category `CollectionPage` and `ItemList` JSON-LD.
- Full sitemap index at `https://arxiv.gg/sitemap.xml`.
- Paper sitemap shards at `/sitemaps/papers-N.xml`.
- Public IndexNow key file.

Submit changed URLs to IndexNow:

```bash
python3 tools/submit_indexnow.py --url https://arxiv.gg/category/cs.LG
python3 tools/submit_indexnow.py --file changed-urls.txt
python3 tools/submit_indexnow.py --sitemap https://arxiv.gg/sitemap.xml --limit 1000
```

See [docs/SEO_INDEXING_REPORT_2026-05-16.md](docs/SEO_INDEXING_REPORT_2026-05-16.md) for the current indexing plan.

## Production Deploy

Production is managed with Docker Compose. The important rule: preserve the existing Postgres volume because it contains the paper cache and embeddings.

App-only redeploy:

```bash
docker compose build arxiv
docker compose up -d --no-deps arxiv
curl -fsS http://127.0.0.1/health
```

Do not recreate the database volume during routine app deploys.

Required environment:

```bash
DATABASE_URL=postgres://...
ADMIN_TOKEN=...
ADMIN_EMAILS=you@example.com
POSTGRES_PASSWORD=...
TRUST_PROXY_HEADERS=true
INDEXNOW_KEY=...
```

See [docs/DEPLOYMENT_RUNBOOK_2026-05-15.md](docs/DEPLOYMENT_RUNBOOK_2026-05-15.md) for the full safe-deploy flow.

## Repository Map

```text
cmd/arxiv/              Web server, CLI entrypoint, templates, API handlers
tools/                  Embedding service, embedding jobs, IndexNow submitter
docs/                   API, deployment, SEO, performance, and infra reports
cache.go                Cache/database initialization
search.go               Keyword/category/author search
search_embeddings.go    Semantic search
citations.go            Citation graph and reference lookup
export_sitemap.go       Sitemap index and sitemap shard generation
docker-compose.yml      Production compose services
```

## Development

Run tests:

```bash
go test ./...
python3 -m py_compile tools/submit_indexnow.py
```

Format Go changes:

```bash
gofmt -w .
```

Check production health:

```bash
curl -fsS https://arxiv.gg/health
curl -fsS https://arxiv.gg/sitemap.xml
```

## License

MIT. See [LICENSE](LICENSE).
