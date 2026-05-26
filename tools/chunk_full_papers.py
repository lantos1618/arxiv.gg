#!/usr/bin/env python3
"""
Create full-paper text chunks for the paid semantic-search layer.

This does not generate embeddings. It writes deterministic chunks to
paper_chunks so a GPU worker can embed chunk_embeddings_v2 later.
"""

import argparse
import hashlib
import os
import re
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


def ensure_schema(conn):
    with conn.cursor() as cur:
        cur.execute(
            """
            CREATE TABLE IF NOT EXISTS paper_chunks (
                id text PRIMARY KEY,
                paper_id text NOT NULL,
                scope text NOT NULL,
                section text,
                chunk_index integer NOT NULL,
                text text NOT NULL,
                text_hash text,
                text_chars integer DEFAULT 0,
                token_estimate integer DEFAULT 0,
                created timestamptz DEFAULT now(),
                updated timestamptz DEFAULT now()
            )
            """
        )
        cur.execute(
            "CREATE INDEX IF NOT EXISTS idx_paper_chunks_paper_scope "
            "ON paper_chunks(paper_id, scope, chunk_index)"
        )
    conn.commit()


def normalize_text(text):
    text = re.sub(r"\s+", " ", text or "").strip()
    return text


def chunk_text(text, chunk_chars, overlap_chars):
    text = normalize_text(text)
    if not text:
        return []
    chunks = []
    start = 0
    while start < len(text):
        end = min(len(text), start + chunk_chars)
        if end < len(text):
            boundary = text.rfind(" ", start + int(chunk_chars * 0.75), end)
            if boundary > start:
                end = boundary
        chunk = text[start:end].strip()
        if chunk:
            chunks.append(chunk)
        if end >= len(text):
            break
        start = max(0, end - overlap_chars)
    return chunks


def stable_chunk_id(paper_id, scope, index, text):
    raw = f"{paper_id}\x00{scope}\x00{index}\x00{text}".encode("utf-8")
    return "chk_" + hashlib.sha256(raw).hexdigest()[:32]


def text_hash(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def fetch_papers(conn, limit):
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT id, pdf_text
            FROM papers p
            WHERE p.pdf_text IS NOT NULL
              AND length(p.pdf_text) > 0
              AND NOT EXISTS (
                  SELECT 1 FROM paper_chunks c
                  WHERE c.paper_id = p.id AND c.scope = 'pdf_text'
              )
            ORDER BY p.fetched_at DESC NULLS LAST, p.created DESC NULLS LAST
            LIMIT %s
            """,
            (limit,),
        )
        return cur.fetchall()


def store_chunks(conn, paper_id, chunks):
    rows = []
    for i, chunk in enumerate(chunks):
        rows.append(
            (
                stable_chunk_id(paper_id, "pdf_text", i, chunk),
                paper_id,
                "pdf_text",
                "body",
                i,
                chunk,
                text_hash(chunk),
                len(chunk),
                max(1, len(chunk) // 4),
            )
        )
    if not rows:
        return 0
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO paper_chunks
                (id, paper_id, scope, section, chunk_index, text, text_hash,
                 text_chars, token_estimate, created, updated)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, now(), now())
            ON CONFLICT (id) DO UPDATE SET
                text = EXCLUDED.text,
                text_hash = EXCLUDED.text_hash,
                text_chars = EXCLUDED.text_chars,
                token_estimate = EXCLUDED.token_estimate,
                updated = now()
            """,
            rows,
        )
    conn.commit()
    return len(rows)


def main():
    parser = argparse.ArgumentParser(description="Chunk extracted full-paper text")
    parser.add_argument("--limit", type=int, default=1000)
    parser.add_argument("--chunk-chars", type=int, default=3000)
    parser.add_argument("--overlap-chars", type=int, default=300)
    args = parser.parse_args()

    conn = db_connect()
    ensure_schema(conn)
    papers = fetch_papers(conn, args.limit)
    total_chunks = 0
    for paper_id, pdf_text in papers:
        chunks = chunk_text(pdf_text, args.chunk_chars, args.overlap_chars)
        stored = store_chunks(conn, paper_id, chunks)
        total_chunks += stored
        print(f"{paper_id}: chunks={stored}", flush=True)
    conn.close()
    print(f"done papers={len(papers)} chunks={total_chunks}")


if __name__ == "__main__":
    main()
