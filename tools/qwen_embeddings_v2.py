#!/usr/bin/env python3
"""
Generate Qwen v2 embeddings into the Postgres embeddings_v2 table.

This script is intended for GPU workers. It keeps the current production
embeddings table untouched and writes 1024d paper-level abstract vectors to a
separate table for evaluation/backfill.
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
            CREATE TABLE IF NOT EXISTS embeddings_v2 (
                paper_id text NOT NULL,
                scope text NOT NULL,
                model text NOT NULL,
                dim integer NOT NULL,
                source_hash text,
                text_chars integer DEFAULT 0,
                token_estimate integer DEFAULT 0,
                created timestamptz DEFAULT now(),
                updated timestamptz DEFAULT now(),
                vector vector(1024),
                PRIMARY KEY (paper_id, scope, model, dim)
            )
            """
        )
        cur.execute(
            "CREATE INDEX IF NOT EXISTS idx_embeddings_v2_lookup "
            "ON embeddings_v2(scope, model, dim, paper_id)"
        )
    conn.commit()


def fetch_papers(conn, model, scope, dim, limit, order):
    order_sql = "p.fetched_at DESC NULLS LAST, p.created DESC NULLS LAST"
    if order == "random":
        order_sql = "random()"
    query = f"""
        SELECT p.id, p.title, p.abstract
        FROM papers p
        WHERE COALESCE(p.title, '') <> ''
          AND COALESCE(p.abstract, '') <> ''
          AND NOT EXISTS (
              SELECT 1
              FROM embeddings_v2 e
              WHERE e.paper_id = p.id
                AND e.scope = %s
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


def paper_text(title, abstract):
    title = (title or "").strip()
    abstract = " ".join((abstract or "").split())
    if title and abstract:
        return f"{title}. {abstract}"
    return title or abstract


def source_hash(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def store_batch(conn, rows, embeddings, model, scope, dim):
    payload = []
    for (paper_id, title, abstract), embedding in zip(rows, embeddings):
        text = paper_text(title, abstract)
        payload.append(
            (
                paper_id,
                scope,
                model,
                dim,
                source_hash(text),
                len(text),
                max(1, len(text) // 4),
                vector_literal(embedding),
            )
        )

    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO embeddings_v2
                (paper_id, scope, model, dim, source_hash, text_chars,
                 token_estimate, vector, created, updated)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s::vector, now(), now())
            ON CONFLICT (paper_id, scope, model, dim) DO UPDATE SET
                source_hash = EXCLUDED.source_hash,
                text_chars = EXCLUDED.text_chars,
                token_estimate = EXCLUDED.token_estimate,
                vector = EXCLUDED.vector,
                updated = now()
            """,
            payload,
        )
    conn.commit()


def load_local_model(model_name, dim):
    import torch
    from sentence_transformers import SentenceTransformer

    return SentenceTransformer(
        model_name,
        device="cuda",
        model_kwargs={"torch_dtype": torch.bfloat16},
        processor_kwargs={"padding_side": "left"},
        truncate_dim=dim,
    )


def encode_local(model, texts, batch_size):
    return model.encode(
        texts,
        batch_size=batch_size,
        normalize_embeddings=True,
        convert_to_numpy=True,
        show_progress_bar=False,
    )


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


def main():
    parser = argparse.ArgumentParser(description="Generate Qwen v2 abstract embeddings")
    parser.add_argument("--model", default=DEFAULT_MODEL)
    parser.add_argument("--dim", type=int, default=DEFAULT_DIM)
    parser.add_argument("--scope", default="abstract")
    parser.add_argument("--limit", type=int, default=10000)
    parser.add_argument("--batch-size", type=int, default=8)
    parser.add_argument("--order", choices=["recent", "random"], default="recent")
    parser.add_argument("--service-url", default=os.environ.get("QWEN_EMBEDDING_SERVICE_URL", ""))
    parser.add_argument("--timeout", type=float, default=300)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    if args.dim != DEFAULT_DIM:
        raise SystemExit("This first v2 table is fixed to 1024d; use --dim 1024.")

    local_model = None
    if args.service_url:
        print(f"using embedding service={args.service_url} model={args.model} dim={args.dim}")
    else:
        print(f"loading local model={args.model} dim={args.dim}")
        local_model = load_local_model(args.model, args.dim)

    conn = db_connect()
    ensure_schema(conn)
    started = time.time()
    processed = 0
    while processed < args.limit:
        remaining = args.limit - processed
        batch_limit = min(args.batch_size, remaining)
        batch = fetch_papers(conn, args.model, args.scope, args.dim, batch_limit, args.order)
        if not batch:
            if processed == 0:
                print("No papers need v2 embeddings.")
            break
        texts = [paper_text(title, abstract) for _, title, abstract in batch]
        if args.service_url:
            embeddings = encode_remote(args.service_url, texts, args.timeout)
        else:
            embeddings = encode_local(local_model, texts, args.batch_size)
        if not args.dry_run:
            store_batch(conn, batch, embeddings, args.model, args.scope, args.dim)
        processed += len(batch)
        elapsed = time.time() - started
        rate = processed / elapsed if elapsed else 0
        print(
            f"processed={processed}/{args.limit} rate={rate:.2f}/s "
            f"elapsed={elapsed:.1f}s dry_run={args.dry_run}",
            flush=True,
        )
        if args.dry_run:
            print("dry-run stops after one streamed batch to avoid reselecting unchanged rows.")
            break

    conn.close()
    total_elapsed = time.time() - started
    print(f"done processed={processed} seconds={total_elapsed:.1f}")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
