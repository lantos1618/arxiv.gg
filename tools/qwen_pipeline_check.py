#!/usr/bin/env python3
"""Check Qwen GPU service health and DB backfill progress."""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request

from qwen_backfill_common import db_connect


DEFAULT_MODEL = "Qwen/Qwen3-Embedding-8B"
DEFAULT_DIM = 1024


def fetch_health(service_url, timeout):
    req = urllib.request.Request(service_url.rstrip("/") + "/health", method="GET")
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read())


def check_service(service_url, timeout):
    if not service_url:
        return ["QWEN_EMBEDDING_SERVICE_URL or --service-url is required"], []

    errors = []
    lines = []
    try:
        health = fetch_health(service_url, timeout)
    except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as err:
        return [f"embedding service health failed: {err}"], []

    if not health.get("ready"):
        errors.append("embedding service is not ready")
    if health.get("status") not in ("healthy", "loading"):
        errors.append(f"embedding service status is {health.get('status')!r}")

    lines.append(
        "service "
        f"ready={health.get('ready')} "
        f"model={health.get('model')} "
        f"dim={health.get('dimension')} "
        f"requests={health.get('requests_total', 0)} "
        f"errors={health.get('errors_total', 0)} "
        f"oom={health.get('oom_total', 0)} "
        f"cuda_free_gb={health.get('cuda_free_gb', 0)} "
        f"cuda_reserved_gb={health.get('cuda_reserved_gb', 0)} "
        f"last_success_ago={health.get('last_success_seconds_ago')}"
    )
    if health.get("last_error"):
        lines.append(f"service_last_error={health.get('last_error')}")
    return errors, lines


def check_abstracts(conn, model, dim, window_minutes, min_recent):
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT
                (SELECT count(*)
                 FROM papers
                 WHERE COALESCE(title, '') <> ''
                   AND COALESCE(abstract, '') <> '') AS eligible,
                (SELECT count(*)
                 FROM embeddings_v2
                 WHERE scope = 'abstract'
                   AND model = %s
                   AND dim = %s) AS embedded,
                (SELECT count(*)
                 FROM embeddings_v2
                 WHERE scope = 'abstract'
                   AND model = %s
                   AND dim = %s
                   AND updated > now() - (%s * interval '1 minute')) AS recent
            """,
            (model, dim, model, dim, window_minutes),
        )
        eligible, embedded, recent = cur.fetchone()

    pending = max(int(eligible) - int(embedded), 0)
    errors = []
    if pending > 0 and recent < min_recent:
        errors.append(
            f"abstract backfill stalled: recent={recent} in {window_minutes}m "
            f"min_recent={min_recent} pending={pending}"
        )
    line = (
        f"abstract eligible={eligible} embedded={embedded} pending={pending} "
        f"recent_{window_minutes}m={recent}"
    )
    return errors, [line]


def check_chunks(conn, model, dim, chunk_scope, window_minutes, min_recent):
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT
                (SELECT count(*)
                 FROM paper_chunks
                 WHERE scope = %s
                   AND COALESCE(text, '') <> '') AS eligible,
                (SELECT count(*)
                 FROM paper_chunks c
                 JOIN chunk_embeddings_v2 e
                   ON e.chunk_id = c.id
                  AND e.model = %s
                  AND e.dim = %s
                 WHERE c.scope = %s
                   AND COALESCE(c.text, '') <> '') AS embedded,
                (SELECT count(*)
                 FROM paper_chunks c
                 JOIN chunk_embeddings_v2 e
                   ON e.chunk_id = c.id
                  AND e.model = %s
                  AND e.dim = %s
                 WHERE c.scope = %s
                   AND COALESCE(c.text, '') <> ''
                   AND e.updated > now() - (%s * interval '1 minute')) AS recent
            """,
            (chunk_scope, model, dim, chunk_scope, model, dim, chunk_scope, window_minutes),
        )
        eligible, embedded, recent = cur.fetchone()

    pending = max(int(eligible) - int(embedded), 0)
    errors = []
    if pending > 0 and recent < min_recent:
        errors.append(
            f"chunk backfill stalled: recent={recent} in {window_minutes}m "
            f"min_recent={min_recent} pending={pending}"
        )
    line = (
        f"chunks scope={chunk_scope} eligible={eligible} embedded={embedded} "
        f"pending={pending} recent_{window_minutes}m={recent}"
    )
    return errors, [line]


def main():
    parser = argparse.ArgumentParser(description="Check Qwen embedding pipeline health")
    parser.add_argument("--service-url", default=os.environ.get("QWEN_EMBEDDING_SERVICE_URL", ""))
    parser.add_argument("--timeout", type=float, default=10)
    parser.add_argument("--model", default=DEFAULT_MODEL)
    parser.add_argument("--dim", type=int, default=DEFAULT_DIM)
    parser.add_argument("--scope", choices=["abstract", "chunks", "both"], default="abstract")
    parser.add_argument("--chunk-scope", default="pdf_text")
    parser.add_argument("--window-minutes", type=int, default=15)
    parser.add_argument("--min-recent", type=int, default=1)
    parser.add_argument("--skip-service", action="store_true")
    parser.add_argument("--skip-db", action="store_true")
    args = parser.parse_args()

    errors = []
    lines = []

    if not args.skip_service:
        service_errors, service_lines = check_service(args.service_url, args.timeout)
        errors.extend(service_errors)
        lines.extend(service_lines)

    if not args.skip_db:
        conn = db_connect()
        try:
            if args.scope in ("abstract", "both"):
                db_errors, db_lines = check_abstracts(
                    conn, args.model, args.dim, args.window_minutes, args.min_recent
                )
                errors.extend(db_errors)
                lines.extend(db_lines)
            if args.scope in ("chunks", "both"):
                db_errors, db_lines = check_chunks(
                    conn, args.model, args.dim, args.chunk_scope, args.window_minutes, args.min_recent
                )
                errors.extend(db_errors)
                lines.extend(db_lines)
        finally:
            conn.close()

    for line in lines:
        print(line)
    if errors:
        for error in errors:
            print(f"FAIL {error}", file=sys.stderr)
        return 2
    print("OK qwen pipeline healthy")
    return 0


if __name__ == "__main__":
    sys.exit(main())
