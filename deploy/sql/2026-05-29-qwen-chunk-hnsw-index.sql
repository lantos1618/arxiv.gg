-- HNSW index for Qwen full-paper chunk-vector search.
-- Run manually after backup verification. CREATE INDEX CONCURRENTLY must not be
-- wrapped in an explicit transaction.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chunk_embeddings_v2_qwen_vector_hnsw
    ON chunk_embeddings_v2 USING hnsw (vector vector_cosine_ops)
    WITH (m = 16, ef_construction = 64)
    WHERE model = 'Qwen/Qwen3-Embedding-8B'
      AND dim = 1024
      AND vector IS NOT NULL;

ANALYZE chunk_embeddings_v2;
