# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build
make build              # Build binary to bin/arxiv
go build ./cmd/arxiv    # Quick local build (no output binary)

# Test
make test               # Run all tests (120s timeout)
make test-verbose       # Run tests with output
go test ./... -run TestName  # Run specific test

# Code quality
make fmt                # Format code
make vet                # Run go vet
make check              # Run fmt + vet + test

# Docker
make docker             # Build Docker image (arxiv-cache)
docker stop arxiv-container && docker rm arxiv-container && docker run -d --name arxiv-container -p 80:80 -v /data/arxiv:/data/arxiv arxiv-cache:latest
```

## Architecture Overview

This is an offline arXiv paper cache manager with CLI, REST API, and web UI. Requires Go 1.24+ and Python 3 (for semantic search embeddings).

### Package Structure

Single flat package (`package arxiv`) at root with 20+ files:

- **cache.go, cache_models.go, cache_lru.go, cache_paper.go** - Core cache manager, data models (Paper, Citation, Embedding, DownloadQueueItem), LRU memory cache
- **search.go, search_embeddings.go, search_fts.go, search_pdf.go** - FTS5 full-text search, semantic vector search, PDF text search
- **data_fetch.go, data_download.go, data_sync.go, data_oai.go** - arXiv API fetching, PDF/TeX downloads, OAI-PMH bulk sync
- **citations.go, citations_refs.go** - Citation graph queries, TeX reference extraction
- **export.go, export_sitemap.go** - BibTeX/RIS/JSON export, sitemap generation

### CLI Application (cmd/arxiv/)

- **main.go** - CLI entry point with 8 commands (fetch, sync, stats, search, get, ls, reindex, serve)
- **serve.go** - HTTP server with HTML templates, web UI
- **api.go** - REST API handlers at /api/v1/
- **middleware.go** - HTTP middleware (rate limiting, caching)
- **templates/** - 11 embedded HTML templates

### Key Patterns

1. **Context-aware**: All cache methods accept `context.Context`
2. **GORM + raw SQL**: GORM for regular tables, raw SQL for FTS5 virtual tables
3. **Embedded templates**: HTML compiled into binary via `//go:embed`
4. **Pure Go SQLite**: Uses `github.com/glebarez/sqlite` (no CGO required)

### Database

SQLite with WAL mode. Main tables: `papers`, `citations`, `embeddings`, `sync_state`, `download_queue`. FTS5 virtual table `papers_fts` for full-text search.

### Web Server

```bash
arxiv serve -port 8080           # Default gateway mode (redirects to arxiv.org)
arxiv serve -port 8080 -local    # Local mode (serves PDFs/source locally)
```

API routes include SSE streaming search (`/api/v1/search/stream`) and semantic search (`/api/v1/search/semantic`). Admin routes exist at `/admin/embeddings`.

### Template Data

When modifying templates, check how data is passed in serve.go. Common pattern:
```go
data := map[string]any{
    "Title":        paper.Title,
    "Paper":        paper,
    "HasEmbedding": s.cache.HasEmbedding(ctx, id),
    "LocalMode":    s.localMode,
}
templates.ExecuteTemplate(w, "paper", data)
```

Template functions available: `truncate`, `parseAuthors`, `parseCategories`, `arxivIDToDate`, `mul`.

## Development Workflow

1. Build locally first: `go build ./cmd/arxiv` - catches errors before Docker build
2. Test templates by running server locally before deploying
3. For production: rebuild Docker image and restart container
