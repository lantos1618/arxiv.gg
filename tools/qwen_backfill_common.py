#!/usr/bin/env python3
"""Shared helpers for Qwen embedding backfill scripts."""

import hashlib
import json
import os
import time
import urllib.error
import urllib.request
from urllib.parse import urlparse

import psycopg2


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


def normalize_text(text):
    return " ".join((text or "").split())


def source_hash(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def vector_literal(embedding):
    return "[" + ",".join(str(float(value)) for value in embedding) + "]"


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


def run_backfill(limit, batch_size, dry_run, fetch_batch, text_for_row, embed_batch, store_batch):
    started = time.time()
    processed = 0
    while processed < limit:
        remaining = limit - processed
        batch_limit = min(batch_size, remaining)
        batch = fetch_batch(batch_limit)
        if not batch:
            if processed == 0:
                print("No rows need embeddings.")
            break

        texts = [text_for_row(row) for row in batch]
        embeddings = embed_batch(texts)
        if not dry_run:
            store_batch(batch, embeddings)

        processed += len(batch)
        elapsed = time.time() - started
        rate = processed / elapsed if elapsed else 0
        print(
            f"processed={processed}/{limit} rate={rate:.2f}/s "
            f"elapsed={elapsed:.1f}s dry_run={dry_run}",
            flush=True,
        )
        if dry_run:
            print("dry-run stops after one streamed batch to avoid reselecting unchanged rows.")
            break

    total_elapsed = time.time() - started
    print(f"done processed={processed} seconds={total_elapsed:.1f}")
