-- HNSW index for Qwen abstract-vector search.
-- Run manually after backup verification. CREATE INDEX CONCURRENTLY must not be
-- wrapped in an explicit transaction.
--
-- Production note from 2026-05-29:
-- Docker's default shared-memory size was too small for a high
-- maintenance_work_mem build, so production used 32MB successfully. If the
-- Postgres container is recreated with a larger shm_size, raise
-- maintenance_work_mem before running this to speed up the build.

SET maintenance_work_mem = '32MB';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_embeddings_v2_qwen_abstract_vector_hnsw
    ON embeddings_v2 USING hnsw (vector vector_cosine_ops)
    WITH (m = 16, ef_construction = 64)
    WHERE scope = 'abstract'
      AND model = 'Qwen/Qwen3-Embedding-8B'
      AND dim = 1024
      AND vector IS NOT NULL;

ANALYZE embeddings_v2;
