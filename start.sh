#!/bin/sh
set -eu

python3 /app/tools/embedding_service.py &
sleep 3
exec ./arxiv-server serve -port "${PORT:-8080}" -embedding-service http://localhost:8001 -embedding-worker
