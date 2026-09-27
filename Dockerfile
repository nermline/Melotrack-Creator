# syntax=docker/dockerfile:1

# ---- frontend ----
FROM node:24-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- server (pure Go, no cgo) ----
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/melotrack ./cmd/melotrack

# ---- deno: yt-dlp needs a JavaScript runtime for YouTube ----
FROM denoland/deno:bin AS deno

# ---- runtime ----
FROM debian:trixie-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg python3 python3-venv ca-certificates curl tini \
 && rm -rf /var/lib/apt/lists/*
COPY --from=deno /deno /usr/local/bin/deno

# yt-dlp lives in its own virtualenv so it can be upgraded at container start
# (YouTube regularly breaks older releases).
RUN useradd --system --uid 10001 --create-home --home-dir /home/melotrack melotrack \
 && python3 -m venv /opt/yt-dlp \
 && /opt/yt-dlp/bin/pip install --no-cache-dir --upgrade pip "yt-dlp[default,curl-cffi]" \
 && mkdir -p /data \
 && chown -R melotrack:melotrack /opt/yt-dlp /data

WORKDIR /app
COPY --from=server /out/melotrack /app/melotrack
COPY --from=web /src/frontend/dist /app/web
COPY docker/entrypoint.sh /app/entrypoint.sh

ENV HOST=0.0.0.0 \
    PORT=8080 \
    DATA_DIR=/data \
    STATIC_DIR=/app/web \
    YTDLP_PATH=/opt/yt-dlp/bin/yt-dlp \
    YTDLP_AUTO_UPDATE=true

USER melotrack
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=90s \
  CMD curl -fsS "http://127.0.0.1:${PORT}/api/health" || exit 1
ENTRYPOINT ["/usr/bin/tini", "--", "/app/entrypoint.sh"]
