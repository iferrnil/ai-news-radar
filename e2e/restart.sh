#!/usr/bin/env bash
# (Re)start the Go server under test (server/bin/radar-server serving the static UI in out/) on $PORT (default 3000),
# with translation off and no DeepSeek key. Extra VAR=value arguments go into its environment, e.g.
#   bash e2e/restart.sh SETTINGS_LOCKED=1
# Data lives in $E2E_DATA (default data/). Only the process recorded in $E2E_DATA/server.pid is stopped — nothing else.
# Prints the number of running servers it manages (expect 1). Log: $E2E_LOG (default <tmp>/radar-server.log).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"; cd "$ROOT"
PORT="${PORT:-3000}"; LOG="${E2E_LOG:-${TMPDIR:-/tmp}/radar-server.log}"; DATA="${E2E_DATA:-$ROOT/data}"
BIN="${RADAR_BIN:-$ROOT/server/bin/radar-server}"; PIDF="$DATA/server.pid"; mkdir -p "$DATA"
[ -x "$BIN" ] || (cd server && CGO_ENABLED=0 go build -o "$BIN" ./cmd/radar-server)
stop() { [ -f "$PIDF" ] || return 0; local p; p=$(cat "$PIDF"); kill "$p" 2>/dev/null || true
  for i in $(seq 20); do kill -0 "$p" 2>/dev/null || break; sleep 0.25; done; kill -9 "$p" 2>/dev/null || true; rm -f "$PIDF"; }
stop
env -u DEEPSEEK_API_KEY TRANSLATE_ENABLED=0 NEWS_DB="$DATA/news.db" SECRETS_FILE="$DATA/secrets.json" PORT="$PORT" "$@" \
  nohup "$BIN" -ui "${UI_DIR:-$ROOT/out}" > "$LOG" 2>&1 &
echo $! > "$PIDF"
for i in $(seq 120); do curl -sf "http://localhost:$PORT/api/health" -o /dev/null && break; sleep 0.25; done
kill -0 "$(cat "$PIDF")" 2>/dev/null && echo 1 || echo 0
