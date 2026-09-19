# Route: API documentation

Path: `/api/v1/`
Evidence: Documentation inspected
Purpose: Give developers and agents the existing integration contract.
Implementation status: investigate_only
Source: https://arxiv.gg/api/v1/

## Locate and investigate

Inspect docs/API.md and route definitions in the application checkout. The API page links to https://github.com/lantos1618/arxiv.gg/blob/main/docs/API.md; repository contents were not inspected in this review.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Reuse documented interfaces before inventing replacements.
- Check fallback flags, readiness states, auth boundaries, and error responses in current code.

## Existing interfaces to investigate

- `GET /api/v1/papers/{id}` — Metadata, citation count, references. id is an arXiv identifier. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/citations` — References cited by this paper. Direction: outgoing references. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/cited-by` — Papers that cite this paper. limit defaults to 50. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/graph` — Citation graph nodes and edges. Response schema and rendering contract need verification. [Documented; endpoint not tested; read]
- `GET /api/v1/papers/{id}/similar` — Related papers and semantic map data. limit default 80, maximum 160; requires paper readiness. [Documented; endpoint not tested; read]
- `POST /api/v1/papers/{id}/fetch` — Fetch and cache an arXiv paper. pdf=true, source=true (default), embedding=true are documented options. [Documented; endpoint not tested; mutation]
- `GET /api/v1/papers/{id}/export/{format}` — Export metadata. Formats: bibtex, ris, json. [Documented; endpoint not tested; read]
- `GET /api/v1/search` — FTS5 title and abstract search. q required; category; limit default 20. Do not confuse with whole-PDF search. [Documented; endpoint not tested; read]
- `GET /api/v1/search/quick` — IDs, titles, authors, keywords, categories. q required; limit default 10. [Documented; endpoint not tested; read]
- `GET /api/v1/search/stream` — Stream Quick, Search, or Deep results using SSE. q required; mode=quick|search|deep (default search); category; limit default 100; Accept: text/event-stream. [Documented; endpoint not tested; read]
- `GET /api/v1/search/semantic` — Abstract vector similarity search. q required; limit default 20. Documentation says Quick fallback returns fallback=true when abstract Search is unavailable. [Documented; endpoint not tested; read]
- `GET /api/v1/search/pdf` — Search downloaded PDF content. q required; limit default 50; fuzzy=true. Account/Deep Search flows require authentication per MCP setup text. Verify endpoint-level requirements. [Documented; endpoint not tested; read]
- `POST /api/v1/papers/{id}/embeddings` — Generate a paper profile. Response includes hasEmbedding, mapReady, generator; may queue work. [Documented; endpoint not tested; mutation]
- `POST /api/v1/embeddings/generate` — Legacy bulk-generation endpoint. Documentation marks bulk generation retired. Do not build new workflows on this route. [Documented; endpoint not tested; retired]
- `GET /api/v1/categories` — Category list and paper counts. Validate count scope against displayed list limits. [Documented; endpoint not tested; read]
- `GET /api/v1/stats` — Cache statistics. Total papers, downloads, queue status; not a service-availability contract unless verified. [Documented; endpoint not tested; read]

## Linked implementation tasks

No defect established for this route. Investigate first; add a new stable issue ID only with evidence.

## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.
