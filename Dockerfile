# syntax=docker/dockerfile:1
# AI News Radar — production image (multi-arch: amd64 / arm64, e.g. Oracle Ampere A1)
#   docker build -t ai-news-radar:local .
#   If network access is restricted, use mirrors:
#     --build-arg NPM_REGISTRY=https://registry.npmmirror.com --build-arg GOPROXY=https://goproxy.cn,direct \
#     --build-arg APK_MIRROR=mirrors.tuna.tsinghua.edu.cn
# Three stages: the Next.js UI as a static export (npm registry only; fonts are vendored via npm, nothing is fetched
# from Google) -> one pure-Go binary (no cgo) -> a small Alpine runtime. No Node in the final image.
FROM node:22-bookworm-slim AS ui
ARG NPM_REGISTRY=https://registry.npmjs.org
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm config set registry "$NPM_REGISTRY" && npm ci --no-audit --no-fund
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM golang:1.27-alpine AS server
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY CGO_ENABLED=0
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN go build -trimpath -ldflags "-s -w" -o /radar-server ./cmd/radar-server

FROM alpine:3.22 AS runner
ARG APK_MIRROR=
RUN if [ -n "$APK_MIRROR" ]; then sed -i "s#dl-cdn.alpinelinux.org#$APK_MIRROR#g" /etc/apk/repositories; fi \
 && apk add --no-cache ca-certificates tzdata \
 && addgroup -g 1000 radar && adduser -D -u 1000 -G radar radar \
 && mkdir -p /app/data && chown radar:radar /app/data
WORKDIR /app
# uid 1000 like the previous node image, so existing ./data volumes stay writable
ENV PORT=3000 HOSTNAME=0.0.0.0 UI_DIR=/app/out \
    NEWS_DB=/app/data/news.db SOURCES_JSON=/app/sources/sources.json \
    SETTINGS_LOCKED=1
COPY --from=server /radar-server /usr/local/bin/radar-server
COPY --from=ui /app/out ./out
COPY --from=ui /app/sources ./sources
USER radar
VOLUME /app/data
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["radar-server", "healthcheck"]
CMD ["radar-server"]
