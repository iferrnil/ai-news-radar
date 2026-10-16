# MyGo native UI — feasibility note (MyGo v0.4.3)

Studied: `github.com/egoist/mygo@v0.4.3/ui` (≈200 files), `docs/ui/*.md` (70 widget pages), `examples/gallery`, `examples/counter-native`.

## What `mygo/ui` is
An immediate-mode, GPU-drawn widget toolkit (Metal on macOS, OpenGL/GtkGLArea on Linux, D3D on Windows; software raster fallback),
still **cgo-free** (purego). A view is `func(*ui.Context)` rebuilt after input / `Window.Update()`; flexbox + grid layout,
theme follows system light/dark, accessibility tree, `ui.Tester` for headless view tests.

## Widgets we need — all present
| Need (web app)                        | MyGo native                                                                 |
|---------------------------------------|-----------------------------------------------------------------------------|
| Sidebar 全部 + sections, badges       | `ui.Sidebar` / `SidebarSection(*bool collapsible)` / `SidebarItem` + `Badge` |
| collapsible 数据源 group per section   | `SidebarSection` open flag, or `ui.Collapsible` / `Accordion`               |
| Virtualized news list (hundreds+)     | `ui.List` (only rows in view are built, variable row heights, `ListState`)   |
| Card grid                             | `ui.GridView` / `Row().Wrap()` cards                                         |
| Scroll                                | `ui.Scroll`, built into List/Sidebar                                         |
| Text input / search / textarea        | `TextInput`, `SearchField`, `TextArea`, `Password()`                         |
| Toggle / checkbox / select / segmented| `Switch`, `Checkbox`, `Select`, `Combobox`, `Segmented`, `Radio`             |
| Menus / context menu                  | `MenuButton`, context menus; app menu bar via `mygo.NewMenu`                |
| Dialogs, toasts, tooltips, popovers   | `Dialog`, `AlertDialog`, `Toast`, `Tooltip`, `Popover`                       |
| Tabs / split view / toolbar / table   | `Tabs`, `Split`, `Toolbar`, `Table`                                          |
| Native file open/save, notifications  | `mygo.Dialog.Open/Save`, `mygo.NewNotification`, tray, open URL             |

## Gaps vs current web UI
- **Remote images** (B站/煎蛋 thumbnails): `ui.Image` draws decoded bitmaps; we must download + decode (jpeg/png; webp needs
  `golang.org/x/image/webp`) ourselves and cache. Not done in this first port — cards are text-only.
- **Rich HTML** summaries/hover cards, markdown docs pages: no HTML renderer (by design). Summaries are plain text already.
- **Fonts**: system fonts (PingFang SC on macOS) via the platform text stack; no bundled web fonts needed → THIRD_PARTY_FONTS not relevant.
- **Animations / exact CSS look** (shadcn styling) won't be pixel-identical; MyGo Theme + per-element styling is enough for a native look.
- Charts (health page) exist in `ui/chart` but the health/stats page is not ported yet.
- Regex filters: Go `regexp` is RE2 → lookahead/lookbehind/backrefs rejected (web rejects backrefs too). RE2 is linear-time,
  so the node:vm timeout sandbox is unnecessary.

## Verdict
**Feasible.** No core requirement is blocked by MyGo maturity. The API is young (v0.x, breaking changes possible between
minor versions — pin v0.4.3) and the GPU UI cannot be exercised headless on this Linux box (no display), so UI correctness must
be verified on a Mac (`mygo dev`).
