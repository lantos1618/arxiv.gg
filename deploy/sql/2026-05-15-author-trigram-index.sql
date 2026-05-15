-- Quick production mitigation for expensive author substring searches.
-- Run manually after backup verification. CREATE INDEX CONCURRENTLY must not be
-- wrapped in an explicit transaction.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_papers_authors_trgm
    ON papers USING gin (authors gin_trgm_ops);

ANALYZE papers;
