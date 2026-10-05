# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worktic-ai . \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worktic-migrate ./cmd/migrate

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates tzdata curl \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/worktic-ai /app/worktic-ai
COPY --from=builder /out/worktic-migrate /app/worktic-migrate
COPY static /app/static
COPY scripts /app/scripts
RUN mkdir -p /var/data/uploads /var/data/wa_sessions /var/data/backups \
 && chown -R 65532:65532 /app /var/data \
 && chmod +x /app/scripts/*.sh
USER 65532:65532
ENV APP_ENV=production DATA_DIR=/var/data PORT=10000
EXPOSE 10000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD curl -fsS http://127.0.0.1:${PORT}/healthz || exit 1
ENTRYPOINT ["/app/worktic-ai"]
