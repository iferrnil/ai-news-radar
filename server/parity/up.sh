#!/usr/bin/env bash
# Start the Node build (:3091) and the Go server (:3090) on fresh DBs against the mock upstream (:4011) for compare.mjs.
# Only kills processes it started itself (by port / cwd) — never other next-server processes.
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"; cd "$ROOT"; export PATH="$HOME/.local/node22/bin:$PATH"
for p in $(pgrep -f "radar-server" || true); do kill "$p" 2>/dev/null; done
for p in $(pgrep -f next-server || true); do [ "$(readlink /proc/$p/cwd)" = "$ROOT" ] && kill "$p"; done
pgrep -f "mock-upstream" >/dev/null || (MOCK_PORT=4011 nohup node e2e/mock-upstream.mjs > /tmp/mock4011.log 2>&1 &)
curl -s "http://127.0.0.1:4011/__ctl?reset=1" >/dev/null
sleep 1; rm -rf /tmp/nref /tmp/gt; mkdir -p /tmp/nref /tmp/gt
C=(SOURCES_JSON="$ROOT/sources/sources.json" UPSTREAM_MOCK=http://127.0.0.1:4011 TRANSLATE_ENABLED=0 FX_TRIES=2 FX_TIMEOUT_MS=3000 NOTIFY_BATCH_MS=1500 NOTIFY_MIN_GAP_MS=0 "$@")
[ -d .next ] && (env -u DEEPSEEK_API_KEY "${C[@]}" NEWS_DB=/tmp/nref/news.db nohup npx next start -p 3091 > /tmp/nref/server.log 2>&1 &)
(env -u DEEPSEEK_API_KEY "${C[@]}" NEWS_DB=/tmp/gt/news.db PORT=3090 nohup server/bin/radar-server -ui "${UI_DIR:-out}" > /tmp/gt/server.log 2>&1 &)
for i in $(seq 60); do curl -sf localhost:3090/api/health >/dev/null && { [ ! -d .next ] || curl -sf localhost:3091/api/health >/dev/null; } && break; sleep 0.5; done
curl -s -m 2 localhost:3091/api/stream >/dev/null; sleep 6; echo up
