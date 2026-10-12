#!/usr/bin/env bash
# Build the macOS app of AI News Radar (MyGo shell + bundled Node 22 + Next standalone server).
#   desktop/scripts/package.sh [platforms]      default: darwin/universal
#   MIRROR=cn   use npmmirror for Node + better-sqlite3 downloads (China)
#   SKIP_NEXT_BUILD=1  reuse an existing .next/standalone
# Works on macOS (signs + .dmg) and on Linux (cross-compiles an UNSIGNED .app; signing/dmg are skipped by mygo).
set -euo pipefail
PLATFORMS="${1:-darwin/universal}"
DESK="$(cd "$(dirname "$0")/.." && pwd)"; ROOT="$(cd "$DESK/.." && pwd)"
CACHE="$DESK/.cache"; RES="$DESK/resources"
NODE_VERSION="${NODE_VERSION:-$(node -v)}"; NODE_VERSION="v${NODE_VERSION#v}"
BSQ_VERSION="$(node -p "require('$ROOT/node_modules/better-sqlite3/package.json').version")"
ABI="$(node -e "console.log(process.versions.modules)")"   # must match NODE_VERSION's major (22 -> 127)
if [ "${MIRROR:-}" = cn ]; then
  NODE_BASE="https://npmmirror.com/mirrors/node"; BSQ_BASE="https://registry.npmmirror.com/-/binary/better-sqlite3"
else
  NODE_BASE="https://nodejs.org/dist"; BSQ_BASE="https://github.com/WiseLibs/better-sqlite3/releases/download"
fi
mkdir -p "$CACHE"
fetch() { [ -s "$2" ] || { echo "↓ $1"; curl -fL --retry 3 -o "$2.part" "$1" && mv "$2.part" "$2"; }; }

echo "== 1. Next standalone build"
if [ -z "${SKIP_NEXT_BUILD:-}" ]; then (cd "$ROOT" && STANDALONE=1 NEXT_TELEMETRY_DISABLED=1 npm run build); fi
[ -f "$ROOT/.next/standalone/server.js" ] || { echo "no .next/standalone (build with STANDALONE=1)"; exit 1; }

echo "== 2. assemble resources/server (shared by all platforms)"
rm -rf "$RES/server" "$RES"/darwin "$RES"/darwin-* "$RES"/linux-*
mkdir -p "$RES/server"
cp -a "$ROOT/.next/standalone/." "$RES/server/"
rm -rf "$RES/server/data" "$RES/server/.env"* "$RES/server/desktop"   # never ship a DB, env files or ourselves
mkdir -p "$RES/server/.next" && cp -a "$ROOT/.next/static" "$RES/server/.next/static"
[ -d "$ROOT/public" ] && cp -a "$ROOT/public" "$RES/server/public"
mkdir -p "$RES/server/sources" && cp "$ROOT/sources/sources.json" "$RES/server/sources/"
# mygo leaves hidden files out of app resources, so ship .next as "next-dist" and point the standalone config at it
mv "$RES/server/.next" "$RES/server/next-dist"
grep -q '"distDir":"./.next"' "$RES/server/server.js" || { echo "server.js distDir marker not found"; exit 1; }
sed -i.bak 's#"distDir":"./.next"#"distDir":"./next-dist"#' "$RES/server/server.js" && rm -f "$RES/server/server.js.bak"
find "$RES/server" -name ".gitkeep" -delete
if find "$RES/server" -name '.*' -not -name '.' | grep -q .; then echo "hidden files would be dropped by mygo:"; find "$RES/server" -name '.*'; exit 1; fi
rm -rf "$RES/server/node_modules/better-sqlite3" && cp -a "$ROOT/node_modules/better-sqlite3" "$RES/server/node_modules/better-sqlite3"
B="$RES/server/node_modules/better-sqlite3"; rm -rf "$B/build" "$B/deps" "$B/src" "$B/docs"
# better-sqlite3 >= 13 ships Node-API prebuilds for every platform in prebuilds/ (lib/index.js picks one by
# process.platform/arch), so nothing is compiled or downloaded for it; each target only gets its own files.
mkdir -p "$CACHE/prebuilds" && mv "$B/prebuilds/"*.node "$CACHE/prebuilds/"
rm -rf "$RES/server/node_modules/@img" "$RES/server/node_modules/sharp"   # next/image is not used; sharp is linux-only native code
if find "$RES/server" -name '*.node' | grep -q .; then echo "unexpected native addons:"; find "$RES/server" -name '*.node'; exit 1; fi

echo "== 2b. slim the server (nothing here is loaded by \`next start\`)"
S="$RES/server"; before=$(du -sk "$S" | cut -f1)
# source maps, typings, docs (licenses stay)
find "$S" -type f \( -name '*.map' -o -name '*.d.ts' -o -name '*.d.mts' -o -name '*.d.cts' -o -name '*.tsbuildinfo' \
  -o -iname 'README*' -o -iname 'CHANGELOG*' -o -iname 'HISTORY*' -o -name '*.md' -o -name '*.markdown' \) -delete
# Next: the experimental (PPR / React canary) runtimes and dev-only refresh/mock helpers are never used by a production build
rm -f "$S/node_modules/next/dist/compiled/next-server/"*experimental*
rm -rf "$S/node_modules/next/dist/compiled/@next/react-refresh-utils" "$S/node_modules/next/dist/compiled/@mswjs"
# fonts: drop Cyrillic/Greek/Vietnamese subsets of the UI/mono fonts (the app is zh/en; CSS just skips missing ranges)
find "$S/next-dist/static/media" -type f \( -name '*-cyrillic*' -o -name '*-greek*' -o -name '*-vietnamese*' \) -delete
find "$S" -type d -empty -delete
echo "server: $((before / 1024)) MB -> $(( $(du -sk "$S" | cut -f1) / 1024 )) MB"

echo "== 3. per-arch Node $NODE_VERSION + better-sqlite3 $BSQ_VERSION prebuilds"
for P in ${PLATFORMS//,/ }; do
  OS="${P%/*}"; A="${P#*/}"
  ARCHS="$A"; [ "$A" = universal ] && ARCHS="arm64 amd64"
  [ "$OS" = windows ] && { echo "windows is not supported by this script"; exit 1; }
  for ARCH in $ARCHS; do
    NA="$ARCH"; [ "$ARCH" = amd64 ] && NA=x64
    TGZ="$CACHE/node-$NODE_VERSION-$OS-$NA.tar.gz"
    fetch "$NODE_BASE/$NODE_VERSION/node-$NODE_VERSION-$OS-$NA.tar.gz" "$TGZ"
    D="$RES/$OS-$ARCH"; mkdir -p "$D/node/bin"
    tar -xzf "$TGZ" -C "$D/node/bin" --strip-components=2 "node-$NODE_VERSION-$OS-$NA/bin/node"
    # strip the symbol table (~15% of the binary; mygo signs the result). Mach-O needs Apple's strip, so only on macOS.
    n0=$(wc -c < "$D/node/bin/node")
    if [ "$OS" = darwin ] && [ "$(uname)" = Darwin ]; then strip -x "$D/node/bin/node"
    elif [ "$OS" = linux ] && command -v strip >/dev/null; then strip -s "$D/node/bin/node"; fi
    echo "node $OS-$ARCH: $((n0 / 1048576)) MB -> $(( $(wc -c < "$D/node/bin/node") / 1048576 )) MB"
    # prebuilt addon for this platform (darwin: under resources/darwin so the universal app carries both arches' files untouched)
    PD="$RES/$OS-$ARCH/server/node_modules/better-sqlite3/prebuilds"; [ "$OS" = darwin ] && PD="$RES/darwin/server/node_modules/better-sqlite3/prebuilds"
    mkdir -p "$PD"; cp "$CACHE/prebuilds/$OS-$NA.node" "$PD/"
    [ -n "${BSQ_FETCH:-}" ] && echo "(BSQ_BASE=$BSQ_BASE kept for older better-sqlite3 versions)"
  done
done

echo "== 4. mygo build ($PLATFORMS)"
cd "$DESK"
go tool mygo build -platform "$PLATFORMS" ${MYGO_FLAGS:-}
echo "== done: $DESK/build"
