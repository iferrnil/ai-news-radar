# desktop-native — AI News Radar, pure Go + MyGo native UI

Experimental rewrite of `desktop/` (MyGo shell + embedded Node 22 Next.js server + WKWebView) as a single Go binary:
no Node, no webview, no localhost port. See [FEASIBILITY.md](FEASIBILITY.md).

```
core/              backend port of src/lib (fetchers, SQLite via modernc.org/sqlite, sections, filters, notify, translate, import/export)
app/               native UI (sidebar, card/list feed, 数据源管理, 设置, dialogs) + ui.Tester tests
cmd/radar-native/  the desktop app (embeds sources.json); mygo.json for `go tool mygo build`
cmd/radar-cli/     headless smoke tool: -dir DIR [-seed sources.json] [-poll] [-export json|opml]
```

```sh
go test ./...                                         # parsers, filters, storage, import/export, notifier, UI flows
cd cmd/radar-native && go tool mygo dev               # run (macOS / Linux desktop)
go tool mygo build -platform darwin/universal         # .app (sign + dmg need macOS)
RADAR_SNAPSHOT=<data dir> RADAR_SNAPSHOT_OUT=/tmp/snaps go test ./app -run Snapshot   # PNG screenshots, headless
```

Data: `<UserData>/native/news.db` (override `AI_RADAR_DATA`). Same schema as the web app, so an existing `news.db`
opens as-is; kept in a separate folder until migration from the webview build is decided.
Translation is off by default (needs a DeepSeek key in 设置; key stored in `secrets.json`, 0600).

## Desktop integration (v0.4.0)

- **Migration:** on first run, if `<UserData>/news.db` from the webview build exists and `<UserData>/native/` has no DB,
  it is copied with `VACUUM INTO` (old DB opened read-only) together with `secrets.json`; `native/.migrated` makes it one-time.
  Covers sources, sections, filter rules, tombstones, server-side settings (notify/translate), items, seen ids and read state.
  Not covered: the web UI's localStorage preferences (inside WKWebView storage).
- **Thumbnails:** `<Cache>/thumbs`, 200 MB cap, LRU + pruned with 48h retention; decoded off the UI thread, downscaled to 360 px.
- **Tray:** unread count (macOS title / tooltip), 显示窗口 · 立即抓取 · 检查更新… · 退出. Closing the window hides it.
  Global shortcut ⌘⇧R / Ctrl+Shift+R toggles the window.
- **Updates:** MyGo ed25519 updater with the existing key pair (`~/.config/ai-news-radar/update-keys`), native update window,
  GitHub releases tagged `desktop-native-v*`. A plain GitHub check shows a “发现新版本 … 前往下载” banner as fallback.
- **App id:** reuses `com.iferrnil.ainewsradar` (same bundle id as the webview build). See the trade-off in the report /
  FEASIBILITY.md. To migrate existing users automatically later, publish a native release under the `desktop-v` prefix.
