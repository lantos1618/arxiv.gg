-- Quick Search's no-full-text-match fallback searches all four fields with OR.
-- Every branch needs a usable index: one unindexed branch forces a scan of the
-- entire papers table. Keep the title expression identical to search.go so
-- whitespace-normalized and stopword-only titles remain searchable.
-- Run after the deployment backup, outside an explicit transaction.
SET lock_timeout = '5s';
SET statement_timeout = '30min';

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_papers_id_trgm
    ON papers USING gin (id gin_trgm_ops) WITH (fastupdate = off);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_papers_title_normalized_trgm
    ON papers USING gin ((regexp_replace(title, '[[:space:]]+', ' ', 'g')) gin_trgm_ops) WITH (fastupdate = off);

-- Failed concurrent builds can leave an invalid index. Do not record this
-- migration as applied until both indexes are valid; inspect/rebuild any invalid
-- index before retrying. Existing author/category trigram indexes remain required.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_index
        WHERE indexrelid IN (
            'idx_papers_id_trgm'::regclass,
            'idx_papers_title_normalized_trgm'::regclass
        ) AND NOT indisvalid
    ) THEN
        RAISE EXCEPTION 'Quick Search index is invalid; rebuild it concurrently before retrying';
    END IF;
END
$$;

ANALYZE papers;
RESET lock_timeout;
RESET statement_timeout;
