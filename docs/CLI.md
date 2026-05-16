# arxiv CLI

The `arxiv` command is the local toolkit behind arxiv.gg. It can fetch papers, sync arXiv metadata, search the cache, generate embeddings, rebuild indexes, and run the web server locally.

## Usage

```bash
arxiv <command> [options]
```

## Commands

```text
fetch      Fetch and download specific papers
sync       Sync paper metadata from arXiv OAI-PMH
stats      Show cache statistics
search     Search cached papers
get        Get a specific paper's info
ls         List cached papers
reindex    Rebuild search index, citations, and embeddings
serve      Start the web server
```

## Environment

```text
ARXIV_CACHE    Cache directory, default ~/.cache/arxiv
DATABASE_URL   Optional PostgreSQL connection string; SQLite is used when unset
```

## Fetching Papers

```bash
arxiv fetch 2301.00001
arxiv fetch -pdf 2301.00001
arxiv fetch -all 2301.00001
arxiv fetch --with-embedding 2301.00001
```

## Syncing Metadata

```bash
arxiv sync
arxiv sync -set cs
arxiv sync -from 2024-01-01
```

## Searching

```bash
arxiv search "transformer attention"
arxiv search -category cs.CL "language model"
arxiv search -limit 50 "neural network"
```

## Web Server

```bash
arxiv serve
arxiv serve -port 3000
```

The web server exposes the browser UI and the `/api/v1` REST API.

## Embeddings

```bash
arxiv reindex --embeddings
arxiv reindex --embeddings --limit 1000
python3 tools/generate_embeddings.py "$ARXIV_CACHE" --limit 1000
```

Embeddings enable semantic search over paper titles and abstracts.
