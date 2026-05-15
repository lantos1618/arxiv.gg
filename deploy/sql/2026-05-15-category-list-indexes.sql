CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_papers_categories_trgm
ON papers USING gin (categories gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_papers_src_recent
ON papers (fetched_at DESC NULLS LAST, id DESC)
WHERE src_downloaded = true;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_papers_embedding_pending_recent
ON papers (fetched_at DESC NULLS LAST, id DESC)
WHERE title != '' AND abstract != '';

ANALYZE papers;
