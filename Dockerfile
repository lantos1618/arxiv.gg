FROM golang:1.25-alpine AS builder
WORKDIR /build
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy && go build -o arxiv-server ./cmd/arxiv && go build -o arxiv-migrate ./cmd/migrate

FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates poppler-utils curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN groupadd --gid 1000 arxiv \
    && useradd --uid 1000 --gid 1000 --create-home --home-dir /home/arxiv --shell /usr/sbin/nologin arxiv

COPY tools/ /app/tools/
RUN pip install --no-cache-dir torch --index-url https://download.pytorch.org/whl/cpu && \
    pip install --no-cache-dir -r /app/tools/requirements.txt

COPY --from=builder /build/arxiv-server .
COPY --from=builder /build/arxiv-migrate .

EXPOSE 8080
ENV ARXIV_CACHE=/data/arxiv \
    HOME=/home/arxiv \
    HF_HOME=/data/arxiv/huggingface \
    SENTENCE_TRANSFORMERS_HOME=/data/arxiv/sentence-transformers \
    TRANSFORMERS_CACHE=/data/arxiv/huggingface/transformers \
    PYTHONDONTWRITEBYTECODE=1 \
    PORT=8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD curl -fsS http://127.0.0.1:8080/health || exit 1

COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh \
    && chown -R 1000:1000 /app /home/arxiv

USER 1000:1000

CMD ["/app/start.sh"]
