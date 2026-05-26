#!/usr/bin/env python3
"""
Generate Qwen v2 embeddings for full-paper chunks.

Expected flow:
  1. Run chunk_full_papers.py on the DB host to fill paper_chunks.
  2. Run qwen_embedding_service.py on the GPU host.
  3. Run this script on the DB host, pointing --service-url at the GPU service.
"""

import argparse
import hashlib
import json
import os
import sys
import time
import urllib.error
import urllib.request
from urllib.parse import urlparse

import psycopg2


DEFAULT_MODEL = "Qwen/Qwen3-Embedding-8B"
DEFAULT_DIM = 1024


def db_connect():
    db_url = os.environ.get("DATABASE_URL", "")
    if not db_url.startswith("postgres"):
        raise SystemExit("DATABASE_URL must be a PostgreSQL URL")
    parsed = urlparse(db_url)
    return psycopg2.connect(
        host=parsed.hostname,
        port=parsed.port or 5432,
        user=parsed.username,
        password=parsed.password,
        dbname=parsed.path.lstrip("/"),
    )


def ensure_schema(conn):
    with conn.cursor() as cur:
        cur.execute("CREATE EXTENSION IF NOT EXISTS vector")
        cur.execute(
            """
            CREATE TABLE IF NOT EXISTS chunk_embeddings_v2 (
                chunk_id text NOT NULL,
                model text NOT NULL,
                dim integer NOT NULL,
                source_hash text,
                text_chars integer DEFAULT 0,
                token_estimate integer DEFAULT 0,
                created timestamptz DEFAULT now(),
                updated timestamptz DEFAULT now(),
                vector vector(1024),
                PRIMARY KEY (chunk_id, model, dim)
            )
            """
        )
        cur.execute(
            "CREATE INDEX IF NOT EXISTS idx_chunk_embeddings_v2_lookup "
            "ON chunk_embeddings_v2(model, dim, chunk_id)"
        )
    conn.commit()


def fetch_chunks(conn, model, dim, scope, limit, order):
    order_sql = "c.created DESC, c.paper_id, c.chunk_index"
    if order == "oldest":
        order_sql = "c.created ASC, c.paper_id, c.chunk_index"
    elif order == "random":
        order_sql = "random()"
    query = f"""
        SELECT c.id, c.text, c.text_hash, c.text_chars, c.token_estimate
        FROM paper_chunks c
        WHERE c.scope = %s
          AND COALESCE(c.text, '') <> ''
          AND NOT EXISTS (
              SELECT 1
              FROM chunk_embeddings_v2 e
              WHERE e.chunk_id = c.id
                AND e.model = %s
                AND e.dim = %s
          )
        ORDER BY {order_sql}
        LIMIT %s
    """
    with conn.cursor() as cur:
        cur.execute(query, (scope, model, dim, limit))
        return cur.fetchall()


def vector_literal(embedding):
    return "[" + ",".join(str(float(value)) for value in embedding) + "]"


def text_hash(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def encode_remote(service_url, texts, timeout):
    payload = json.dumps({"texts": texts}).encode("utf-8")
    req = urllib.request.Request(
        service_url.rstrip("/") + "/embed/batch",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read()
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", "replace")
        raise RuntimeError(f"embedding service http {err.code}: {detail}") from err
    data = json.loads(body)
    embeddings = data.get("embeddings", [])
    if len(embeddings) != len(texts):
        raise RuntimeError(f"embedding service returned {len(embeddings)} vectors for {len(texts)} texts")
    return embeddings


def store_batch(conn, rows, embeddings, model, dim):
    payload = []
    for (chunk_id, text, stored_hash, text_chars, token_estimate), embedding in zip(rows, embeddings):
        source_hash = stored_hash or text_hash(text)
        payload.append(
            (
                chunk_id,
                model,
                dim,
                source_hash,
                text_chars or len(text),
                token_estimate or max(1, len(text) // 4),
                vector_literal(embedding),
            )
        )

    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO chunk_embeddings_v2
                (chunk_id, model, dim, source_hash, text_chars,
                 token_estimate, vector, created, updated)
            VALUES (%s, %s, %s, %s, %s, %s, %s::vector, now(), now())
            ON CONFLICT (chunk_id, model, dim) DO UPDATE SET
                source_hash = EXCLUDED.source_hash,
                text_chars = EXCLUDED.text_chars,
                token_estimate = EXCLUDED.token_estimate,
                vector = EXCLUDED.vector,
                updated = now()
            """,
            payload,
        )
    conn.commit()


def main():
    parser = argparse.ArgumentParser(description="Generate Qwen v2 full-paper chunk embeddings")
    parser.add_argument("--model", default=DEFAULT_MODEL)
    parser.add_argument("--dim", type=int, default=DEFAULT_DIM)
    parser.add_argument("--scope", default="pdf_text")
    parser.add_argument("--limit", type=int, default=10000)
    parser.add_argument("--batch-size", type=int, default=8)
    parser.add_argument("--order", choices=["recent", "oldest", "random"], default="recent")
    parser.add_argument("--service-url", default=os.environ.get("QWEN_EMBEDDING_SERVICE_URL", ""))
    parser.add_argument("--timeout", type=float, default=300)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    if args.dim != DEFAULT_DIM:
        raise SystemExit("This first v2 table is fixed to 1024d; use --dim 1024.")
    if not args.service_url:
        raise SystemExit("--service-url or QWEN_EMBEDDING_SERVICE_URL is required")

    conn = db_connect()
    ensure_schema(conn)
    rows = fetch_chunks(conn, args.model, args.dim, args.scope, args.limit, args.order)
    if not rows:
        print("No chunks need v2 embeddings.")
        return

    print(f"using embedding service={args.service_url} model={args.model} dim={args.dim}")
    started = time.time()
    processed = 0
    for i in range(0, len(rows), args.batch_size):
        batch = rows[i : i + args.batch_size]
        texts = [" ".join((text or "").split()) for _, text, _, _, _ in batch]
        embeddings = encode_remote(args.service_url, texts, args.timeout)
        if not args.dry_run:
            store_batch(conn, batch, embeddings, args.model, args.dim)
        processed += len(batch)
        elapsed = time.time() - started
        rate = processed / elapsed if elapsed else 0
        print(
            f"processed={processed}/{len(rows)} rate={rate:.2f}/s "
            f"elapsed={elapsed:.1f}s dry_run={args.dry_run}",
            flush=True,
        )

    conn.close()
    print(f"done processed={processed} seconds={time.time() - started:.1f}")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
