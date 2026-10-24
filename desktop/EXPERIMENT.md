# Experimental native MyGo UI

Branch: `experiment/mygo-native-ui`

Pure MyGo native shell (no webview for primary chrome). Demo-quality preview of look/feel using:

- https://mygo.egoist.dev/docs/ui
- Dark theme + restrained violet accent (`ui.Oklch`)
- `plugins/glass` Liquid Glass on toolbar / detail / about
- Window `VibrancySidebar` (macOS / Win11) with transparent root

## Visual pass (0.2.0)

Feedback: 「感觉原生的太丑了」— 0.1.0 read as a quick scaffold. This pass pushes MyGo chrome toward a polished macOS news reader:

- Clear hierarchy: vibrancy sidebar / feed list surface / glass detail card
- Roomy type scale (Chinese titles ~14–20pt, line-height), softer muted secondary
- Unread violet dot + selected left bar (no harsh bordered cards)
- Real app toolbar (`ui.Toolbar`): search, count pill, refresh, open
- Comfortable detail reading width (max ~640), hairline dividers, glass depth
- Intentional empty / stub states (glyph + soft pill — not gray boxes)
- Fewer borders; depth via glass, inset hairlines, restrained shadows
- Accent chroma lowered (less neon)

Credit: Lang

## What’s in the demo

- Sidebar sections from `/api/sections`
- Feed list from `/api/items` (filter by section + search)
- Item detail + open URL in browser / copy link
- Basic status + settings/about stubs
- In-process `radar-server` (same backend as the web desktop)

## Known gaps（实验预览）

- No briefing / Telegram / article generation
- No full settings (DeepSeek key, notify prefs)
- No Sparkle auto-update (on purpose — does not touch `desktop-v*` channel)
- Not feature-parity with the WebView app
- Linux builds lack true vibrancy (opaque fallback surfaces)

## Release channel

- App: **AI News Radar Native** (`com.iferrnil.ainewsradar.native`)
- Tags: `desktop-native-v*` → workflow `desktop-native-release.yml`
- Published as **prerelease** so GitHub Latest stays on web `desktop-v0.15.x`

Credit: Lang
