-- Rebuild the low-cardinality category btree without posting-list
-- deduplication. During OAI metadata expansion, the old index hit btree
-- posting-list split errors on large category runs.
--
-- Run outside a transaction because REINDEX CONCURRENTLY requires it.

ALTER INDEX IF EXISTS idx_papers_categories
    SET (deduplicate_items = off);

REINDEX INDEX CONCURRENTLY idx_papers_categories;
