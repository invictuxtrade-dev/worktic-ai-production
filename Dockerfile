# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worktic-ai .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata curl && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/worktic-ai /app/worktic-ai
COPY static /app/static
RUN mkdir -p /var/data && chown -R 65532:65532 /app /var/data
USER 65532:65532
ENV APP_ENV=production DATA_DIR=/var/data PORT=10000
EXPOSE 10000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD curl -fsS http://127.0.0.1:${PORT}/healthz || exit 1
ENTRYPOINT ["/app/worktic-ai"]
