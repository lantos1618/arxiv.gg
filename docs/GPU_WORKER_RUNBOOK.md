# GPU Worker Runbook

Use this when a separate GPU box is ready for Qwen embedding work. The app and
Postgres stay on the main arxiv.gg box; the GPU box only runs the embedding
HTTP service.

## 1. Prepare The GPU Box

```bash
sudo apt-get update
sudo apt-get install -y git python3-venv python3-pip
git clone https://github.com/lantos1618/arxiv.gg.git ~/arxiv-embedding-worker
cd ~/arxiv-embedding-worker
python3 -m venv .venv
source .venv/bin/activate
pip install -U pip
pip install -r tools/requirements.txt
```

Start the service on the GPU box:

```bash
cd ~/arxiv-embedding-worker
source .venv/bin/activate
export PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True
export QWEN_MAX_BATCH_SIZE=128
export QWEN_MAX_TEXT_CHARS=4096
QWEN_EMBEDDING_DEVICE=cuda \
uvicorn tools.qwen_embedding_service:app --host 127.0.0.1 --port 8010
```

Health check on the GPU box:

```bash
curl http://127.0.0.1:8010/health
```

## 2. Attach It To The App Box

From the arxiv.gg app box, open an SSH tunnel:

```bash
ssh -N -L 8010:127.0.0.1:8010 ubuntu@GPU_HOST
```

Then verify the app box can reach the GPU service:

```bash
curl http://127.0.0.1:8010/health
```

For a long-running tunnel, put the SSH command behind `systemd` or `autossh`.

## 3. Run Abstract Embeddings

From the arxiv.gg app box:

```bash
cd /home/ubuntu/arxiv
source .venv/bin/activate
QWEN_EMBEDDING_SERVICE_URL=http://127.0.0.1:8010 \
python3 tools/qwen_embeddings_v2.py --limit 700000 --batch-size 128
```

This writes Qwen 1024d abstract vectors into `embeddings_v2` and does not touch
the existing production MiniLM embeddings.

The backfill script retries transient service failures and splits oversized
batches recursively. A single CUDA OOM should shrink the work unit instead of
killing the whole run.

## 4. Run Full-Body Search Prep

First chunk extracted paper text on the app/DB box:

```bash
cd /home/ubuntu/arxiv
source .venv/bin/activate
python3 tools/chunk_full_papers.py --limit 100000 --select-batch-size 200
```

Use `--refresh-existing` after changing chunk size or text extraction. The
chunker removes stale chunk rows and their old vectors, and the embedder
refreshes vectors when a chunk's `text_hash` changes.

Then embed those chunks through the GPU service:

```bash
QWEN_EMBEDDING_SERVICE_URL=http://127.0.0.1:8010 \
python3 tools/qwen_chunk_embeddings_v2.py --limit 500000 --batch-size 16
```

Full-body search is available after sign-in and free while we test it.

## 5. Pipeline Checks

Run a manual check from the app box:

```bash
cd /home/ubuntu/arxiv
QWEN_EMBEDDING_SERVICE_URL=http://127.0.0.1:8010 \
python3 tools/qwen_pipeline_check.py --scope both --window-minutes 15 --min-recent 1
```

For systemd or cron, run the same command every few minutes and alert on a
non-zero exit. Use `--scope abstract` when only the abstract backfill is
expected to be moving; use `--scope chunks` when full-paper chunks are queued.

The check fails when the GPU service is not ready or when pending rows exist but
no new vectors were written in the selected window.

## 6. Operational Notes

- Keep Postgres on the main app box. Do not copy or replace the production DB.
- Keep the GPU service bound to `127.0.0.1`; expose it only through SSH or a
  private network.
- Start with small limits, check DB growth and query speed, then scale up.
- If the tunnel dies, backfill scripts will fail safely; restart the tunnel and
  rerun the same command.
