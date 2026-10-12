#!/usr/bin/env bash
# (Re)start the production build under test: stops every next-server, then `next start` on $PORT (default 3000)
# with translation off and no DeepSeek key. Extra VAR=value arguments go into its environment, e.g.
#   bash e2e/restart.sh SETTINGS_LOCKED=1
# Prints the number of running next-server processes (expect 1). Log: $E2E_LOG (default <tmp>/radar-server.log).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"; cd "$ROOT"
PORT="${PORT:-3000}"; LOG="${E2E_LOG:-${TMPDIR:-/tmp}/radar-server.log}"
for p in $(pgrep -f next-server || true); do kill "$p" 2>/dev/null || true; done
for i in $(seq 20); do pgrep -f next-server >/dev/null || break; sleep 0.25; done
for p in $(pgrep -f next-server || true); do kill -9 "$p" 2>/dev/null || true; done
(env -u DEEPSEEK_API_KEY TRANSLATE_ENABLED=0 "$@" nohup npx next start -p "$PORT" > "$LOG" 2>&1 &)
for i in $(seq 120); do curl -sf "http://localhost:$PORT/api/health" -o /dev/null && break; sleep 0.5; done
pgrep -f next-server | wc -l | tr -d ' '
