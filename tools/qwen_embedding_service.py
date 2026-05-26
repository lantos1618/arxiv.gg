#!/usr/bin/env python3
"""
Persistent Qwen embedding service for GPU workers.

Run this on the GPU box and call it from the app/backfill host through a
private network or SSH tunnel. The service deliberately has no database access:
it only turns text into normalized vectors, so losing the worker does not risk
the production database.

Example:
    QWEN_EMBEDDING_DEVICE=cuda uvicorn qwen_embedding_service:app --host 127.0.0.1 --port 8010
"""

import logging
import os
import time
from contextlib import asynccontextmanager
from typing import List, Optional

import numpy as np
import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from sentence_transformers import SentenceTransformer


logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger("qwen-embedding-service")

os.environ.setdefault("TOKENIZERS_PARALLELISM", "false")

MODEL_NAME = os.environ.get("QWEN_EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-8B")
EMBEDDING_DIM = int(os.environ.get("QWEN_EMBEDDING_DIM", "1024"))
DEVICE = os.environ.get("QWEN_EMBEDDING_DEVICE", "cuda")
MAX_BATCH_SIZE = int(os.environ.get("QWEN_MAX_BATCH_SIZE", "128"))
MAX_TEXT_CHARS = int(os.environ.get("QWEN_MAX_TEXT_CHARS", "24000"))

model: Optional[SentenceTransformer] = None
started_at = time.time()


class EmbedBatchRequest(BaseModel):
    texts: List[str] = Field(min_length=1)


class EmbedBatchResponse(BaseModel):
    embeddings: List[List[float]]
    dimension: int
    model: str
    device: str
    count: int
    seconds: float


class HealthResponse(BaseModel):
    status: str
    ready: bool
    model: str
    dimension: int
    device: str
    max_batch_size: int
    uptime_seconds: float
    cuda_allocated_gb: float = 0
    cuda_reserved_gb: float = 0


@asynccontextmanager
async def lifespan(app: FastAPI):
    global model

    logger.info("loading model=%s dim=%s device=%s", MODEL_NAME, EMBEDDING_DIM, DEVICE)
    start = time.time()
    model = SentenceTransformer(
        MODEL_NAME,
        device=DEVICE,
        model_kwargs={"torch_dtype": torch.bfloat16},
        processor_kwargs={"padding_side": "left"},
        truncate_dim=EMBEDDING_DIM,
    )
    logger.info("model loaded in %.1fs", time.time() - start)
    yield
    model = None
    if torch.cuda.is_available():
        torch.cuda.empty_cache()


app = FastAPI(
    title="arXiv.gg Qwen Embedding Service",
    version="1.0.0",
    lifespan=lifespan,
)


@app.get("/health", response_model=HealthResponse)
async def health():
    allocated = 0.0
    reserved = 0.0
    if torch.cuda.is_available():
        allocated = torch.cuda.memory_allocated() / 1024**3
        reserved = torch.cuda.memory_reserved() / 1024**3
    return HealthResponse(
        status="healthy" if model is not None else "loading",
        ready=model is not None,
        model=MODEL_NAME,
        dimension=EMBEDDING_DIM,
        device=DEVICE,
        max_batch_size=MAX_BATCH_SIZE,
        uptime_seconds=time.time() - started_at,
        cuda_allocated_gb=round(allocated, 3),
        cuda_reserved_gb=round(reserved, 3),
    )


@app.post("/embed/batch", response_model=EmbedBatchResponse)
async def embed_batch(request: EmbedBatchRequest):
    if model is None:
        raise HTTPException(status_code=503, detail="model not ready")
    if len(request.texts) > MAX_BATCH_SIZE:
        raise HTTPException(status_code=413, detail=f"batch too large; max {MAX_BATCH_SIZE}")

    texts = [" ".join(text.split())[:MAX_TEXT_CHARS] for text in request.texts]
    if any(text == "" for text in texts):
        raise HTTPException(status_code=400, detail="texts cannot be empty")

    start = time.time()
    embeddings = model.encode(
        texts,
        batch_size=len(texts),
        normalize_embeddings=True,
        convert_to_numpy=True,
        show_progress_bar=False,
    )
    embeddings = np.asarray(embeddings, dtype=np.float32)
    seconds = time.time() - start

    return EmbedBatchResponse(
        embeddings=[row.tolist() for row in embeddings],
        dimension=int(embeddings.shape[1]),
        model=MODEL_NAME,
        device=DEVICE,
        count=len(texts),
        seconds=seconds,
    )
