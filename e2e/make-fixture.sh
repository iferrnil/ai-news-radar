#!/usr/bin/env bash
# Build the baseline DB for the suite from scratch: seed sources (sources/sources.json) + one poll of every
# source against the mock upstream, then 3 sources switched off (22 sources / 19 active, like a lived-in install).
#   bash e2e/make-fixture.sh out.db      (needs the Go server + static UI built and the mock running, see runall.sh)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"; cd "$ROOT"; OUT="$1"; PORT="${PORT:-3000}"; DATA="${E2E_DATA:-$ROOT/data}"
BIN="${RADAR_BIN:-$ROOT/server/bin/radar-server}"
: "${UPSTREAM_MOCK:?UPSTREAM_MOCK must point at e2e/mock-upstream.mjs}"
mkdir -p "$DATA"; rm -f "$DATA/news.db" "$DATA/news.db-wal" "$DATA/news.db-shm" "$DATA/secrets.json"
bash e2e/restart.sh >/dev/null
for i in $(seq 120); do
  n=$(curl -s "http://localhost:$PORT/api/stats" | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const j=JSON.parse(s);console.log(j.sources.active-j.sources.fetched)})')
  [ "$n" = 0 ] && break; sleep 0.5
done
node -e '
const B = "http://localhost:" + (process.env.PORT ?? 3000);
(async () => {
  const src = await fetch(B + "/api/sources").then((r) => r.json());
  for (const name of ["Google DeepMind Blog", "X @testingcatalog", "量子位"]) {
    const s = src.find((x) => x.name === name); if (!s) throw new Error("missing " + name);
    const r = await fetch(B + "/api/sources/" + s.id, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ enabled: false }) });
    if (!r.ok) throw new Error("disable " + name + ": " + r.status);
  }
  const after = await fetch(B + "/api/sources").then((r) => r.json());
  console.log(`fixture: ${after.length} sources / ${after.filter((s) => s.enabled).length} active`);
})().catch((e) => { console.error(e); process.exit(1); });'
if [ -f "$DATA/server.pid" ]; then p=$(cat "$DATA/server.pid"); kill "$p" 2>/dev/null || true
  for i in $(seq 40); do kill -0 "$p" 2>/dev/null || break; sleep 0.25; done; rm -f "$DATA/server.pid"; fi
rm -f "$OUT"
NEWS_DB="$DATA/news.db" "$BIN" vacuum-into "$OUT"
