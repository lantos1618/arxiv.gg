#!/bin/sh
set -eu

python3 /app/tools/embedding_service.py &
sleep 3
exec ./arxiv-server serve -port 80 -embedding-service http://localhost:8001 -embedding-worker
