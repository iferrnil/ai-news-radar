#!/usr/bin/env bash
# Build the macOS app of AI News Radar.
#   desktop/scripts/package.sh [platforms]      default: darwin/universal
#   SKIP_UI_BUILD=1  reuse an existing out/ (npm run build)
#   NATIVE_UI=1      experimental pure-MyGo native shell: skip Next.js embed (API-only seed)
# Works on macOS (signs + .dmg) and on Linux (cross-compiles an UNSIGNED .app; signing/dmg are skipped by mygo).
set -euo pipefail
PLATFORMS="${1:-darwin/universal}"
DESK="$(cd "$(dirname "$0")/.." && pwd)"; ROOT="$(cd "$DESK/.." && pwd)"
W="$DESK/webui"

if [ "${NATIVE_UI:-}" = 1 ]; then
  echo "== 1. native UI: seed sources only (no Next.js embed)"
  rm -rf "$W/out" "$W/sources.json"
  mkdir -p "$W/out"
  # keep go:embed happy: a tiny placeholder so Sub(webui/out) exists
  printf '%s\n' '<!doctype html><title>AI News Radar Native</title><p>native shell — no webview</p>' > "$W/out/index.html"
  cp "$ROOT/sources/sources.json" "$W/sources.json"
  echo "seed: $(wc -c < "$W/sources.json") bytes"
else
  echo "== 1. static UI (Next.js export)"
  if [ -z "${SKIP_UI_BUILD:-}" ]; then (cd "$ROOT" && NEXT_TELEMETRY_DISABLED=1 npm run build); fi
  [ -f "$ROOT/out/index.html" ] || { echo "no out/index.html (npm run build)"; exit 1; }

  echo "== 2. embed UI + sources.json (desktop/webui)"
  rm -rf "$W/out" "$W/sources.json"
  cp -a "$ROOT/out" "$W/out"; cp "$ROOT/sources/sources.json" "$W/sources.json"
  find "$W/out/_next/static/media" -type f \( -name '*-cyrillic*' -o -name '*-greek*' -o -name '*-vietnamese*' \) -delete 2>/dev/null || true
  find "$W/out" -name '*.map' -delete
  echo "ui: $(( $(du -sk "$W/out" | cut -f1) / 1024 )) MB"
fi

echo "== 3. mygo build ($PLATFORMS)"
cd "$DESK"
CGO_ENABLED=0 go tool mygo build -platform "$PLATFORMS" ${MYGO_FLAGS:-}
echo "== done: $DESK/build"
