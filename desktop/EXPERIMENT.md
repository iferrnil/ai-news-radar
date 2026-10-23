# Experimental native MyGo UI

Branch: `experiment/mygo-native-ui`

Pure MyGo native shell (no webview for primary chrome). Demo-quality preview of look/feel using:

- https://mygo.egoist.dev/docs/ui
- Dark theme + violet accent (`ui.Oklch`)
- `plugins/glass` Liquid Glass on toolbar / detail / about
- Window `VibrancySidebar` (macOS / Win11) with transparent root

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

## Release channel

- App: **AI News Radar Native** (`com.iferrnil.ainewsradar.native`)
- Tags: `desktop-native-v*` → workflow `desktop-native-release.yml`
- Published as **prerelease** so GitHub Latest stays on web `desktop-v0.15.x`

Credit: Lang
