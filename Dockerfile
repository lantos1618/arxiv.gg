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

COPY tools/ /app/tools/
RUN pip install --no-cache-dir torch --index-url https://download.pytorch.org/whl/cpu && \
    pip install --no-cache-dir -r /app/tools/requirements.txt

COPY --from=builder /build/arxiv-server .
COPY --from=builder /build/arxiv-migrate .

EXPOSE 80
ENV ARXIV_CACHE=/data/arxiv
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD curl -fsS http://127.0.0.1/health || exit 1

COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

CMD ["/app/start.sh"]
