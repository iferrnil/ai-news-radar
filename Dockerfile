# syntax=docker/dockerfile:1
# AI News Radar — production image (multi-arch: amd64 / arm64, e.g. Oracle Ampere A1)
#   docker build -t ai-news-radar:local .
#   If network access is restricted, use mirrors: --build-arg NPM_REGISTRY=https://registry.npmmirror.com --build-arg APT_MIRROR=mirrors.tuna.tsinghua.edu.cn
# The build needs only the npm registry (+ Debian apt); fonts are vendored via npm, nothing is fetched from Google.
FROM node:22-bookworm-slim AS deps
ARG NPM_REGISTRY=https://registry.npmjs.org
ARG APT_MIRROR=
WORKDIR /app
RUN if [ -n "$APT_MIRROR" ]; then sed -i "s#deb.debian.org#$APT_MIRROR#g" /etc/apt/sources.list.d/debian.sources; fi
# toolchain only needed if no prebuilt better-sqlite3 binary exists for the platform
RUN apt-get update && apt-get install -y --no-install-recommends python3 make g++ && rm -rf /var/lib/apt/lists/*
COPY package.json package-lock.json ./
RUN npm config set registry "$NPM_REGISTRY" && npm ci --no-audit --no-fund

FROM deps AS build
COPY . .
ENV STANDALONE=1 NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-bookworm-slim AS runner
WORKDIR /app
ENV NODE_ENV=production PORT=3000 HOSTNAME=0.0.0.0 NEXT_TELEMETRY_DISABLED=1 \
    NEWS_DB=/app/data/news.db SOURCES_JSON=/app/sources/sources.json
# standalone server + static assets + seed catalog; native module compiled for this arch
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
COPY --from=build --chown=node:node /app/public ./public
COPY --from=build --chown=node:node /app/sources ./sources
COPY --from=build --chown=node:node /app/node_modules/better-sqlite3 ./node_modules/better-sqlite3
RUN mkdir -p /app/data && chown node:node /app/data
USER node
VOLUME /app/data
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:'+(process.env.PORT||3000)+'/api/health').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"
CMD ["node", "server.js"]
