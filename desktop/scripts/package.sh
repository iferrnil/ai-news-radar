#!/usr/bin/env bash
# Build the macOS app of AI News Radar: MyGo shell + the Go backend in-process + the static Next.js UI embedded.
#   desktop/scripts/package.sh [platforms]      default: darwin/universal
#   SKIP_UI_BUILD=1  reuse an existing out/ (npm run build)
# Works on macOS (signs + .dmg) and on Linux (cross-compiles an UNSIGNED .app; signing/dmg are skipped by mygo).
# No Node runtime and no native addons are shipped: one Go binary per arch.
set -euo pipefail
PLATFORMS="${1:-darwin/universal}"
DESK="$(cd "$(dirname "$0")/.." && pwd)"; ROOT="$(cd "$DESK/.." && pwd)"

echo "== 1. static UI (Next.js export)"
if [ -z "${SKIP_UI_BUILD:-}" ]; then (cd "$ROOT" && NEXT_TELEMETRY_DISABLED=1 npm run build); fi
[ -f "$ROOT/out/index.html" ] || { echo "no out/index.html (npm run build)"; exit 1; }

echo "== 2. embed UI + sources.json (desktop/webui)"
W="$DESK/webui"; rm -rf "$W/out" "$W/sources.json"
cp -a "$ROOT/out" "$W/out"; cp "$ROOT/sources/sources.json" "$W/sources.json"
# fonts: drop Cyrillic/Greek/Vietnamese subsets of the UI/mono fonts (the app is zh/en; CSS just skips missing ranges)
find "$W/out/_next/static/media" -type f \( -name '*-cyrillic*' -o -name '*-greek*' -o -name '*-vietnamese*' \) -delete
# RSC payloads for client-side navigation are kept (.txt); source maps never ship
find "$W/out" -name '*.map' -delete
echo "ui: $(( $(du -sk "$W/out" | cut -f1) / 1024 )) MB"

echo "== 3. mygo build ($PLATFORMS)"
cd "$DESK"
CGO_ENABLED=0 go tool mygo build -platform "$PLATFORMS" ${MYGO_FLAGS:-}
echo "== done: $DESK/build"
