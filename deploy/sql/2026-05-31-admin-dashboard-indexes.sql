-- Speed up admin dashboard counts for rare paper states.
-- Run with psql directly; these are concurrent indexes for production.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_papers_pdf_text_present
    ON papers(id)
    WHERE pdf_text IS NOT NULL AND length(pdf_text) > 0;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_papers_missing_abstract_text
    ON papers(id)
    WHERE title IS NULL OR title = '' OR abstract IS NULL OR abstract = '';
