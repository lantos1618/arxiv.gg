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
python3 tools/qwen_embeddings_v2.py --limit 700000 --batch-size 16
```

This writes Qwen 1024d abstract vectors into `embeddings_v2` and does not touch
the existing production MiniLM embeddings.

## 4. Run Full-Body Search Prep

First chunk extracted paper text on the app/DB box:

```bash
cd /home/ubuntu/arxiv
source .venv/bin/activate
python3 tools/chunk_full_papers.py --limit 100000 --select-batch-size 200
```

Then embed those chunks through the GPU service:

```bash
QWEN_EMBEDDING_SERVICE_URL=http://127.0.0.1:8010 \
python3 tools/qwen_chunk_embeddings_v2.py --limit 500000 --batch-size 16
```

Full-body search is available after sign-in and free while we test it.

## 5. Operational Notes

- Keep Postgres on the main app box. Do not copy or replace the production DB.
- Keep the GPU service bound to `127.0.0.1`; expose it only through SSH or a
  private network.
- Start with small limits, check DB growth and query speed, then scale up.
- If the tunnel dies, backfill scripts will fail safely; restart the tunnel and
  rerun the same command.
