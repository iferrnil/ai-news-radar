# Vendored MyGo v0.4.3 (github.com/egoist/mygo, MIT) — local patches

Used through `replace github.com/egoist/mygo => ./third_party/mygo` in `desktop/go.mod`. Docs, website,
examples, JS packages and `_test.go` files of upstream are left out. Patches (search for `[ai-news-radar patch]`):

1. `internal/darwin/window.go` (createWebView): upstream sets `drawsBackground = NO` on the WKWebView whenever
   `WindowOptions.BackgroundColor` is set. That makes the web view non-opaque, so WebKit composites the page —
   every layer and every scroll tile — with alpha over the window instead of as an opaque surface, which Safari
   never does. This was the scroll / hover jank seen only in the app (the same UI was smooth in Safari at
   http://127.0.0.1:47821). Now only `Transparent` / `Vibrancy` windows get a see-through web view; a background
   colour only sets `underPageBackgroundColor` (and the window's colour). `MYGO_WEBVIEW_TRANSPARENT=1` restores
   upstream behaviour (the app sets it when `<data dir>/debug-webview-transparent` exists).
2. `internal/darwin/notify.go` + `notify_auth.go`: `RequestNotificationAuthorization` and
   `NotificationAuthorizationStatus` (UNUserNotificationCenter requestAuthorization / getNotificationSettings,
   results delivered on the main thread). Upstream only asks lazily inside the first `Show` and ignores the
   answer.

To update MyGo: copy the new release here, re-apply the patches above, keep this file.

3. `internal/darwin/window.go` (`SetBackgroundColor`): upstream always sets `drawsBackground = NO` when updating
   the background (including light-dark appearance flips). That undid patch (1). Now it uses the same see-through
   gate as `createWebView` (`Transparent` / `Vibrancy` / `MYGO_WEBVIEW_TRANSPARENT`).

