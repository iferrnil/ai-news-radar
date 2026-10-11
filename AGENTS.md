# AGENTS.md — AI News Radar（iferrnil/ai-news-radar）

Guide for coding agents working in this repo. 中文 UI，代码与注释以英文为主。

## Purpose / 项目目的
Real-time aggregator of domestic + international AI news for an AI content creator: ~22 curated sources (lean P0 list) (lab blogs & API changelogs, scoop/benchmark X accounts, key people, Chinese media, global media/newsletters, entertainment), polled by priority, pushed live to the browser via SSE. Only the last 48 h of items are kept.

## Stack
- Next.js 16 (App Router) · TypeScript · React 19
- shadcn/ui **Base UI** variant (`components.json` style `base-nova`; components use `render` props, not Radix `asChild`) · Tailwind CSS v4 (CSS-first tokens in `src/app/globals.css`) · lucide-react · sonner · tw-animate-css (no framer-motion) · `cn` (shadcn's clsx+tailwind-merge replacement) · cmdk · next-themes
- SQLite via `better-sqlite3` (WAL), `fast-xml-parser` for RSS/Atom
- **Node ≥ 22 required** (better-sqlite3 v13 has no build for Node 20). On the shared box Node 22 lives in `~/.local/node22`; run `export PATH=~/.local/node22/bin:$PATH` first.

## Directory map
```
src/app/                 pages: / (feed), /sources, /docs (index), /docs/<slug> (doc pages) ; api/* route handlers
src/app/api/             items, stats, sources (+ test, [id], [id]/fetch), stream (SSE)
src/components/          app-shell.tsx (sidebar, topbar, ⌘K palette, item drawer, context), Feed.tsx, Sources.tsx, common.ts
src/components/ui/       shadcn/ui (Base UI) primitives — generated, edit sparingly
src/lib/db.ts            schema, seed/sync from sources/sources.json, tier intervals
src/lib/fetchers.ts      FETCHERS by type (rss, api, x, x_list) + detect() for add-source
src/lib/feed.ts          RSS/Atom/RDF parser + <link rel=alternate> autodiscovery
src/lib/store.ts         insert/dedupe/merge ("also on X"), queryItems, prune
src/lib/scheduler.ts     poll loop, limiters, pollOne
src/instrumentation.ts   starts the scheduler once (nodejs runtime only)
src/lib/docs-registry.ts DOCS array → sidebar "文档" group children, /docs index, ⌘K entries
src/data/rate-limits.json + src/lib/rate-usage.ts   data for /docs/rate-limits and docs/rate-limits.md
sources/sources.json     source catalog (core list enabled + disabled reference entries; the 开发 section was removed 2026-10-10 (its sources disabled))
docs/                    reports (sources.md, recommendations.md, x_vs_blog.md, rate-limits.md), screenshots/
scripts/                 screenshots.mjs (playwright), gen-rate-limits-md.mts
data/                    runtime SQLite (gitignored)
```

## Commands
```bash
export PATH=~/.local/node22/bin:$PATH
npm ci
npm run dev                      # dev server (poller runs too)
npm run build && npm run start -- -p 3000   # production (what we deploy)
npm run lint                     # eslint (must be clean)
npm run typecheck                # tsc --noEmit
npm run screenshots              # needs running server + `npx playwright install chromium`
npm run docs:rate-limits         # regenerate docs/rate-limits.md
```
Restart prod after a build: stop the old `next start` process first (only one instance may run — see below).

## Architecture
- **Poller lives inside the Next server process**: `instrumentation.ts` → `startScheduler()`; guarded by a `globalThis` flag so it starts once. `DISABLE_POLLER=1` disables it. Run **exactly one instance** (no pm2 cluster, no second `next start` on the same DB).
- Scheduler ticks every 10 s and polls sources where `last_fetched + poll_interval <= now`; X concurrency 3, feeds 8 (`X_CONCURRENCY`, `FEED_CONCURRENCY`).
- Priority tiers (seconds): P0 X 120 / feed 300; P1 X 300 / feed 600; P2 1800 (env `P0_X`, `P0_FEED`, …).
- **SSE**: `GET /api/stream`, events `item | update | removed`, 15 s ping; fed by an in-process EventEmitter (`src/lib/bus.ts`, on `globalThis`). The client opens one EventSource in `app-shell.tsx` and fans out via `onStream`.
- **Retention**: items older than 48 h (`RETENTION_HOURS`) are pruned every 10 min and skipped on insert. **`seen` table** keeps every uid for 30 days (`SEEN_DAYS`) so pruned items never come back as "new".
- **Seed sync**: on every start, `sources/sources.json` is imported: new seed sources are added; for existing `origin='seed'` rows url/username are re-synced, group too unless overridden (import sets `grp`), and enabled/priority/interval only if the user hasn't changed them. Any UI change (`PATCH /api/sources/:id`: sidebar 取消订阅, sources-page switch/interval/priority) appends the field to `sources.overrides` (e.g. `enabled,poll_interval`), and the seed never overwrites those fields (`SYNC_SEED=0` disables sync entirely). PATCH also publishes SSE `sources` so other tabs refresh. User-added sources (`origin='user'`) are never touched (`SYNC_DISABLE_USER=1` disables them).
- DB path `data/news.db` (`NEWS_DB`), seed path `SOURCES_JSON`.

## Data rules (keep these)
- Dedupe: web items by normalized URL (`normUrl`: no scheme/www/trailing slash/utm etc.), X posts by `x:<tweet id>`.
- Cross-source merge: a tweet whose expanded link matches an article → stored with `merged_into`, shown as "也在 X" on the article (also reverse when the article arrives later).
- Future-dated items are clamped to fetch time; undated items on a source's first fetch are ignored (backlog).
- X: keep only the account's own posts and self-replies (drop reposts and replies to others).
- **No scrape-only sources**: a source must have RSS/Atom or a JSON API with an adapter. HTML-only sites stay `supported=0`.

## Adding a new source type / fetcher
1. Implement `async (src) => FetchedItem[]` in `src/lib/fetchers.ts` (use `httpGet` from `util.ts` for UA/timeout/retry; produce `uid`, `title`, `url`, `summary` via `clean()`, `published` in epoch seconds or `null`, `isX`, optional `links` for X).
2. Register it in `FETCHERS` and add the type to `SUPPORTED` in `db.ts` (and `SourceType` in `types.ts`).
3. For a new JSON API, add a URL-matched branch in `fetchApi` instead of a new type.
4. If users should add it from the UI, extend `detect()`.
5. Add its rate-limit entry to `src/data/rate-limits.json` (official link or "no official documentation found"), run `npm run docs:rate-limits`.
6. Add to `sources/sources.json` with `type`, `priority`, `pollInterval`, `group` if it's a curated source.

## Adding a doc page
1. Create `src/app/docs/<slug>/page.tsx` (reuse the header/breadcrumb pattern of `docs/rate-limits/page.tsx`).
2. Append `{ slug, title, desc, updated }` to `DOCS` in `src/lib/docs-registry.ts` — the sidebar group (collapsible, open state in localStorage `sidebar.docs.open`), the /docs index and ⌘K pick it up automatically.

## Feed filter model
- `src/components/filters.ts`: ONE view at a time, chosen in the sidebar — 全部 / an exclusive section / a single source — plus an optional 仅 P0 toggle. **Sections are exclusive, assigned per SOURCE** (`sectionOf` in `common.ts`): seed `group` wins → 实验室 lab / 国内媒体 cn / 国外媒体 media / 娱乐 ent (group `娱乐`, incl. entertainment X accounts like @PopBase); otherwise any X source → `x`（X 快讯）; otherwise region/category fallback. A section may set `includeInAll: false` in `SECTIONS` (currently 娱乐): it is shown only in its own view and is excluded from 全部 everywhere (`passes`, sidebar 全部 count, P0 count, 'N 条新内容', day headers, ⌘K 最新 list; ⌘K text search can still hit it, labelled). So the 全部 count = sum of the in-全部 sections; such sections sit below a divider in the sidebar. There is no 开发 section any more.
- State lives in the URL (`/?section=cn`, `/?section=x`, `/?source=12`, `&p0=1`); use `setF` / `navigate` from `useApp()`.
- Feed loads the whole 48 h window once (`/api/items?limit=1000`) and filters client-side; sidebar counts come from it.
- Layout: day headers + a grid of equal-size shadcn `Card`s (fixed height, clamped title/summary): 1 col → 2 at md → 3 at xl → 4 at ≥1600px. j/k/←/→ move in reading order; Enter opens the drawer.
- Read state: item ids in localStorage `read.ids` (marked on open), rendered dimmed.

## UI conventions
- All UI copy in Chinese. Times shown in **Beijing time** (`Asia/Shanghai`) plus relative time (`bj()`, `ago()` in `common.ts`).
- Keep design tokens in `globals.css` (`--brand`, `--faint`, `--hover`, `--selected`, `--shadow-card`, cool-tinted dark / warm-neutral light). Use `text-faint` for meta, `text-muted-foreground` for summaries; maintain WCAG AA.
- Use shadcn/ui components (Base UI API: `render={<a … />}` instead of `asChild`; Select/ToggleGroup values are arrays/unknown).
- Motion: `duration-150 ease-out` for colors, `duration-200 ease-out` for transforms. Icons: lucide `size-4` (`size-3.5` in dense meta) with `strokeWidth={1.75}`. Counts/times use `tabular-nums`; spacing on a 4px grid.
- Dark mode is the default (`next-themes`, class strategy).

## Verification checklist
- [ ] `npm run lint` and `npm run typecheck` clean; `npm run build` succeeds
- [ ] docker compose maps host 3080 → container 3000 (http://localhost:3080); the box's local prod server stays on :3000
- [ ] restart prod on :3000; `curl localhost:3000/api/stats` → `sources.errors` is 0 and `fetched` equals active count after ~1 min
- [ ] SSE: `curl -N localhost:3000/api/stream` receives `event: item` when new items arrive (or force via "立即抓取")
- [ ] feed, /sources, /docs, /docs/rate-limits render in dark and light; mobile layout OK (`npm run screenshots`)

## Rate-limit etiquette
See **docs/rate-limits.md** (or `/docs/rate-limits` in the app). Don't poll faster than upstream cache times (hnrss 600 s, OpenRouter 120 s); X syndication lists allow only ~30 req / 15 min / IP; honor 429 backoff; current total load ≈ 11 req/min.

## Performance notes
- SQLite: WAL, `synchronous=NORMAL`, `busy_timeout=5000`, 4 MB cache, `auto_vacuum=INCREMENTAL`; `prune()` runs `incremental_vacuum` + `wal_checkpoint(TRUNCATE)`.
- Fetchers use conditional GET (`condGet` in `fetchers.ts`: ETag / If-Modified-Since, 304 → no work).
- `/api/items` gzips responses and ships a 420-char summary preview (`truncated: true`); the drawer fetches the full item with `?id=`. X post text is stored up to 4000 chars.
- Feed cards have a hover preview (`ui/hover-card` = Base UI PreviewCard, delay 450/150 ms, side right with collision flip) only on `(hover: hover) and (pointer: fine)`; content mounts only while open. Full bodies come from the shared cache in `item-preview.tsx` (`fetchFull` / `useFullItem`), also used by the drawer.
- The card grid renders 60 cards at a time (IntersectionObserver); `ItemCard` is memoized.
- Docker: `Dockerfile` (standalone output, non-root, healthcheck on `/api/health`) + `docker-compose.yml`.

## Do / Don't
- ✅ Do keep changes small and typed; reuse `common.ts` helpers and existing tokens.
- ✅ Do update `sources/sources.json` (not the DB) for curated source changes.
- ❌ Don't commit `data/` (SQLite) or `.next/`.
- ❌ Don't run multiple server instances or a second poller against the same DB/IP.
- ❌ Don't add scrape-only/HTML sources or invent feed URLs — verify by fetching.
- ❌ Don't replace the design system or restyle components ad hoc; extend tokens instead.

## Fonts

- `src/app/layout.tsx`: Inter (Latin/numbers) + 霞鹜文楷 LXGW WenKai Regular+Medium (CJK, `lxgw-wenkai-webfont`, unicode-range slices) + JetBrains Mono (times), all imported as npm CSS. `--font-sans` = Inter → LXGW WenKai → system CJK. WenKai Medium is declared as weight 700, so `font-semibold` titles use it. Kai reads small: card title 15px, dek 13px, tweet 14px. Do not add HarmonyOS Sans / MiSans (non-OFL licenses).

## Translation (DeepSeek, optional)

- `src/lib/translate.ts`: `storeItems` → `enqueueTranslation(newIds)`; worker batches (10, concurrency 2) to DeepSeek `chat/completions` with JSON output; cache `tcache(h=sha1(title,summary))`, 7-day prune. `items.title_zh/summary_zh`: NULL = not processed, '' = not needed (mostly CJK, see `needsTranslation`), else translation. On completion it publishes SSE `update` with zh fields.
- Enabled only if `DEEPSEEK_API_KEY` is set; DB `settings.translate` ('1'/'0', via `PATCH /api/settings`) overrides `TRANSLATE_ENABLED`. Startup backfills untranslated retained items once.
- API: `/api/items?zh=1` adds `title_zh`/`summary_zh` (list payload omits them otherwise). Client: `zh` in AppCtx (localStorage `zh`), `localize()` swaps fields and keeps `orig`; `TrBadge`/`Original` in `item-preview.tsx`.
- Never commit keys; `.env*` is gitignored except `.env.example`.

## Add-source dialog

- One 分区 select (= `SECTIONS`), auto-suggested (X handle/URL → X 快讯, .cn/中文 → 国内媒体, else 国外媒体); POST `/api/sources` stores it as `grp`, region/category derived. Dialog uses `grid-cols-[minmax(0,1fr)]` + `min-w-0` so long previews truncate instead of widening it.

## Sidebar unsubscribe

- `SourceRowNav` in `app-shell.tsx`: hover/focus (always on touch) shows a MinusCircle button → popover confirm → `PATCH enabled:false` → Sonner toast with 撤销. Disabled sources vanish from the sidebar and the feed hides their items immediately (Feed filters by enabled source ids; re-enabling triggers a reload). Sources are never deleted here; they stay on the sources page under 停用.

## Deleting sources

- Any source can be deleted (`DELETE /api/sources/:id`, `POST /api/sources/bulk {action:'delete', ids}` → `deleteSources` in `store.ts`): removes items, post_links and their `seen` uids, then `incremental_vacuum`, publishes SSE `sources`. Built-ins (`origin='seed'`) get a tombstone in `deleted_seeds(name)`; `seed()` skips tombstoned names. Restore: `GET/POST /api/sources/deleted {names: [...] | 'all'}` removes tombstones and re-runs `seed()`.
- UI: ⋯ → 删除 (AlertDialog), row checkboxes + 删除所选, 「已删除的内置源」 dialog. Sidebar unsubscribe confirm is inline (inside the sidebar) so it never covers the feed.
- Feed grid: max 3 columns (1/2/3 at base/md/xl), content capped at 1480px.
- `/api/sources` is gzip'd and null-stripped (polled by every tab).

## Lean source list & per-view P0 (2026-10-10)

- `sources/sources.json` now ships only P0 AI sources + 4 Chinese 娱乐 feeds (网易娱乐, 抖音热搜, B站热门, 煎蛋无聊图); the old catalog lives only in docs/sources.md (history). Funny/meme feeds go to 娱乐 (outside 全部) rather than a new section: same audience/mood, and keeps the AI sections clean.
- 仅 P0 is per view: `p0For/setP0For/viewKey` in `filters.ts` (localStorage `p0ByView`); `navigate` restores the target view's value; sidebar counts use each section's own value.
- Items may carry `image` (jandan images, bilibili covers); cards show a fixed-height lazy thumbnail (`referrerPolicy=no-referrer`), hover card/drawer show it larger. `@next/next/no-img-element` is off on purpose.

## Performance notes (Oct 2026 pass)
- `/api/items` and `/api/sources` go through `src/lib/http.ts#jsonResponse`: weak content ETag + `Cache-Control: private, no-cache` (polls revalidate → 304), brotli q5 (≈14% smaller than gzip, ~1 ms) or gzip > 1 KB, nulls dropped.
- ⌘K palette (`components/palette.tsx`, cmdk) is `next/dynamic` and mounted only after first open. Don't import `ui/command` from `app-shell.tsx` or it lands back in the main chunk.
- framer-motion was removed (−74 KB raw on the feed): the new-items pill and back-to-top button use tw-animate `animate-in` classes. Don't re-add a motion lib for small enter effects.
- Code-split on first use (`next/dynamic`, `ssr:false`, mounted via a `*Used` flag so exit animations still run): palette, item drawer (`item-sheet.tsx`), mobile nav sheet (`mobile-nav.tsx`), translate popover (`translate-menu.tsx`; `TrButton` placeholder looks identical and opens it on first click), add-source dialog (`add-source-dialog.tsx`), delete-confirm/restore dialogs (`sources-dialogs.tsx`), import dialog. app-shell warms the drawer + palette chunks on `requestIdleCallback`. Keep Dialog/Sheet/Popover/Switch imports out of `app-shell.tsx`.
- First-load JS (gzip -9, from prerendered HTML script tags; measure with `bash`-sum of chunk files): feed 268 KB, /sources 309 KB, /docs 255 KB (Oct 11 after icon picker + rename + sidebar tree; 267/307/254 after custom sections + filter rules; 266/305/253 before them, 310/315/272 before the perf pass). Edit/sections dialogs and rule preview are code-split (`source-edit-dialogs.tsx`). ~444 KB raw of every route is Next/React itself.
- Source list refresh: structural changes arrive via the SSE `sources` event; the sidebar polls only every 60 s (sources page 15 s) for status dots, skipped while the tab is hidden.
- Fonts: WenKai is the biggest cold-load cost (~2.1 MB / 41 slices on the feed, Regular+Medium; 0 B warm). Dropping Medium would roughly halve it but titles would lose weight (faux bold); the alternative is system CJK fonts.
- Thumbnails: fixed-height box + `loading=lazy decoding=async` → CLS 0.
- `npm ci` needs `make`/`g++` if the better-sqlite3 prebuild download fails.

## Mirror-accelerated build (README: 镜像加速构建说明)
- Fonts come from `@fontsource-variable/{inter,jetbrains-mono} + lxgw-wenkai-webfont (Regular + Medium as weight 700; semibold titles resolve to it)` imported in `app/layout.tsx`; CSS vars `--font-latin/--font-cjk/--font-mono-face` are set in `globals.css`. Never reintroduce `next/font/google` — the build must not touch Google.
- Compose builds `ai-news-radar:local` with `pull_policy: build`; build args `NPM_REGISTRY`, `APT_MIRROR`.

## Type scale (WenKai + Inter)
- Tokens in `globals.css` `@theme`, used via `@utility type-*` classes: `type-2xs` 12 · `type-meta` 12.5 · `type-caption` 13.5 · `type-body` 14.5 · `type-ui` 15 · `type-title` 16.5 · `type-h3` 20 · `type-h1` 24 px (each with its own line-height; raised 2026-10-11 for readability — nothing below 12px, incl. shadcn `text-xs`).
- Below 13px (`type-2xs`, `type-meta` without `font-mono`) CJK falls back to the system sans (PingFang / Noto Sans CJK / YaHei) instead of thin 霞鹜文楷. Body uses `-webkit-font-smoothing: auto` (antialiased looked thin on macOS). Muted/faint tokens: ≥ 5.9:1 contrast on card/background in both themes (AA+).
- Feed card height `CARD_H` = 216px (header 44 + footer 40 + body: title 2 lines + summary 3 lines, or X text 5 lines, or 1-line title + 92px image). Re-check it whenever the scale changes. Use these instead of `text-[Npx]`; shadcn primitives keep Tailwind's `text-xs/sm/base`.
- They are named `type-*`, not `text-*`, on purpose: `cn()` would treat unknown `text-*` classes as colours and drop them. Leading utilities (`leading-*`) still override the token line-height.
- Latin uses "Inter Adj" (`@font-face` over the @fontsource files with `size-adjust: 94%`) so it sits level with 霞鹜文楷. `font-synthesis: none` globally — WenKai has only Regular (400) + Medium (registered as 700); `font-medium` renders Regular, `font-semibold` renders the real Medium face, never a faux bold.

## Import / export of subscriptions
- `src/lib/portable.ts` (isomorphic): zod `EntrySchema` (name sanitised, http(s) only, X username regex, section/priority/interval), `buildJson` (`{app:"ai-news-radar", version:1, exportedAt:+08:00, sources}`), `buildOpml` (OPML 2.0; X as custom `type="x"`/`"x_list"` outlines), `parseImport` (JSON or OPML via fast-xml-parser; 1 MB / 2000 entries caps). Bump `SCHEMA_VERSION` and keep old versions parseable when the entry shape changes.
- `POST /api/sources/import { entries, mode: merge|update|replace, dryRun, test }`: re-validates every entry server-side, matches existing by X username → normalized URL → name, `dryRun` returns the plan (new / restore / dup / update / invalid + remove list). Apply runs in one transaction: replace deletes unmatched sources via `deleteSources` (tombstones for built-ins), tombstoned built-ins in the file are restored through `seed()`, inserted/updated rows get `overrides='enabled,priority,poll_interval,grp'` (seed sync never reverts imported settings or section). X URLs are rebuilt from the username (never taken from the file). Publishes the `sources` SSE event; `test` polls new sources in the background (4 at a time).
- UI: export DropdownMenu + `components/import-dialog.tsx` (next/dynamic, mounted on open) on the sources page.
- Manual test recipe (destructive, back up `data/news.db` first): export JSON → bulk-delete all → import (expect 22 restore, identical set) → OPML sample → replace with the original JSON.
- Bundle rule: the sources page imports only `lib/portable-export.ts` (dependency-free export builders). zod + fast-xml-parser live in `lib/portable.ts`, which only the lazily-loaded import dialog and the API route import. Don't import `@/lib/portable` from always-loaded client code.

## UI tokens (consistency rules)
- Radii: `rounded-xs` (0.3rem) chips/kbd/tiny badges · `rounded-md` small controls/rows · `rounded-lg` inputs, menus, nav items · `rounded-xl` cards/panels · `rounded-full` pills/avatars. No `rounded-[Npx]`.
- Elevation: `shadow-card` resting → `shadow-raised` hover → `shadow-pop` popovers/menus/hover card; `shadow-hair` = inset 1px border for chips. Sheets keep `shadow-lg`.
- Borders: `border-border` for structure, `border-border/50` for in-panel hairline dividers only.
- Focus: `focus-visible:ring-3 focus-visible:ring-ring/50` everywhere (same as shadcn primitives).
- Type: `type-*` scale only (no `text-[Npx]`).
- Spacing: 4px grid for container padding and gaps between blocks (`p-3/p-4`, `gap-2/3/4`, `py-3`…). Half steps (`*-0.5/1.5/2.5`, i.e. 2/6/10px) only inside compact controls (chips, badges, icon↔label gaps, nav rows). Allowed geometric exceptions: sidebar tree guide lines `ml-[11px]`/`ml-[17px]`/`-left-[9.5px]` (align to icon centres), `w-[3px]` active indicator, avatar `p-[3px]` inset, import-mode hint `pl-5.5` (aligns with the radio label).
- Icons: lucide only, one stroke (`svg.lucide { stroke-width: 1.75 }` in globals.css — don't pass `strokeWidth`). Sizes: `size-4` in buttons/menus (shadcn default), `size-3.5` next to meta/caption text, `size-3` inside 2xs badges, `size-5` in empty-state tiles; the filled X glyph is sized to its chip (2–2.5).
- Chips/badges: always shadcn `<Badge>` (`ui/badge.tsx`). Inline chips in cards / hover card / palette / docs use `size="sm"` (18px, `rounded-xs`); drawer, tables and counts use the default 20px. Variants: stock `default/secondary/outline/destructive` + app additions `brand` (P0), `muted` (译, 也在 X, counts), `success` (在线/新增/低风险), `warning` (暂停/等待/中风险). The old `chip` utility is gone.
- States: use `components/empty-state.tsx` (built on shadcn `<Empty>`; icon tile + title + hint + actions; `tone="error"` for failures) for feed empty/error, palette no-results, sources empty filter. Inline form errors/warnings use shadcn `<Alert variant="destructive" | "warning">` (warning is an app addition in `ui/alert.tsx`). SSE disconnected = amber dot + "重连中".
- Toasts: Sonner bottom-right, `shadow-pop`, title `type-body`, description `type-caption`. Wording: short past-tense title ("已导出 22 个来源", "已复制链接"); failures = `toast.error("<动作>失败", { description: message })`.

## shadcn components (Base UI, `base-nova`)
- Prefer the official component over hand-rolled markup: Button, ButtonGroup (status filter + 导入/导出 split button on /sources), Badge, Toggle (feed 仅 P0), Kbd/KbdGroup (shortcut hints, ⌘K, N), RadioGroup (import mode cards), Table (sources, import preview, docs rate-limits), Alert, Empty, Separator, Select, Switch, Checkbox, Input, Dialog/AlertDialog, Sheet, Tooltip, HoverCard, Popover, DropdownMenu, Command, Skeleton, Sonner.
- Add new ones with `npx shadcn@latest add <name>`; generated files import `{ cn } from "cn"` — that is the real `cn` npm package (already a dependency; `@/lib/utils` just re-exports it), either import works; answer **no** to overwriting existing files (e.g. `separator.tsx`).
- Intentional exceptions: sidebar nav rows / docs tree / source rows in `app-shell.tsx` stay custom (shadcn `Sidebar` would pull in its provider/cookie layer for no gain); the import drop zone is a native `<button>` + hidden file input.
- /sources status filter = ButtonGroup of outline Buttons with `aria-pressed` and a count Badge: 在线 (enabled & supported, success) · 报错 (在线 with last_error, destructive) · 停用 (enabled=0, muted) · 暂停 (enabled but supported=0, warning) · 全部 (outline). All toolbar controls are h-8.

## Custom sections & per-source filter rules (Oct 2026)
- **Sections**: 5 built-ins (`lab/cn/media/x/ent`, stored on sources as seed-style `grp`) + user-created ones in the SQLite `sections` table (`key` c1, c2…, `name` unique, `icon` emoji, `in_all`, `pos`). A source points at a custom section with `grp = "sec:<key>"`; `sectionOf()` maps that back. Seed sync never touches `sections`, and any UI/import section change marks `grp` in `overrides`, so `sources.json` never reverts it.
- Client registry: `common.ts#setCustomSections/allSections/secZh/inAll` (app-shell loads `/api/sections` together with `/api/sources` and exposes `sections` in context to re-render). `parseFilters` accepts `c\d+` keys before the list loads. Per-view P0 uses the same `viewKey`, so custom sections get their own 仅 P0.
- API: `GET/POST /api/sections`, `PATCH/DELETE /api/sections/[key]` (delete → 409 if sources still use it unless `?moveTo=<key>`; moved sources get the `grp` override). `POST /api/sources` and `PATCH /api/sources/[id]` accept `section` or `newSection:{name,icon,inAll}`.
- UI: section select in add/edit dialogs has 「＋ 新建分区…」 (inline name / emoji / 显示在「全部」). /sources toolbar 「分区」 opens the manage dialog (rename, icon, 全部 toggle, delete with move-to confirm); custom sidebar rows have a hover ⚙ that opens it via `/sources?sections=1`. Note: don't put `-translate-y-1/2` on a shadcn `Button` — its `active:translate-y-px` overrides the translate and the button jumps away mid-click.
- **Filter rules** (`lib/filter-rule.ts` isomorphic, `lib/rule-guard.ts` server): sources columns `f_include`, `f_exclude`, `f_scope` (`title` | `all` = title+summary), `f_cs` (case-sensitive, default off). Kept = matches include (if set) AND not exclude (if set). Validation: ≤200 chars, rejects nested quantifiers / backreferences / syntax errors, then runs under `node:vm` with a hard timeout on real samples + adversarial probes (catastrophic patterns → 400). Fetch time: rejected items are marked seen but not stored (`storeItems`). Rule change: `reapplyRule()` sets `items.hidden` on stored items (retroactive hide/unhide); list queries and `item_count` exclude hidden (`hidden_count` is returned for the row badge). A rule that times out at fetch time keeps everything.
- Preview: `POST /api/sources/rule-check` (validates + returns keep flags for the source's last 40 stored items incl. hidden, or for the add dialog's sample titles). Rows with a rule show a 「过滤 N」 badge (tooltip shows the patterns).
- Import/export: JSON `version: 2` adds top-level `sections:[{key,name,icon,inAll}]` and per-entry `filter:{include,exclude,scope,caseSensitive}` (v1 files still import). OPML: custom sections are folders with `sectionCustom="1"` (+ `sectionIcon`, `sectionInAll`), entries carry `filterInclude/filterExclude/filterScope/filterCase`. On import, custom sections are matched/created **by name** (keys differ per install); other readers' folders still map to the chosen default section.
- Tests: the external Playwright suite (t7, not in the repo) covers create/rename/delete sections, Smol AI News with an exclude rule, invalid/catastrophic regex, retroactive hide, JSON+OPML round-trip, seed-sync persistence.

## Icon picker, section delete, source rename, sidebar tree (Oct 2026)
- **Icon picker** (`icon-picker.tsx`, lazy via `next/dynamic` inside a shadcn `Popover`; trigger = `IconPicker` in `source-form-parts.tsx`): ~90 curated emoji in 5 categories (AI/科技, 视频/图像, 资讯/写作, 商业/安全, 娱乐), zh/en keyword search, 8 recents (`localStorage iconPicker.recent`), preview + 清除图标. Lucide icons deliberately not offered (bundle cost).
- **Section delete**: from /sources 「分区」 dialog or the sidebar ⚙ (opens the same dialog). AlertDialog warns it can't be undone, shows affected source count, has 「先导出备份（JSON）」 (`exportBackup()`), then the move-to select (defaults to 国外媒体 when present) and destructive confirm. Applies to every section (see "All sections are user-managed").
- **Rename sources**: click the name on /sources (inline Input; Enter/blur saves, Esc cancels), row menu 「重命名」, or the edit dialog. `PATCH /api/sources/[id] {name}`: trimmed, 1–80 chars, unique (case-insensitive), marks `name` override. Seed sources keep their sources.json name in `sources.seed_name`; `seed()` and delete tombstones match on `COALESCE(seed_name, name)`, so seed sync never reverts a rename or re-adds a duplicate. Feed/sidebar/export read `sources.name`, so the new name shows everywhere.
- **Sidebar**: top section rows are plain filters (+ hover ⚙ → 管理分区). 管理 › 「数据源」 is a `NavGroup` (same component as 「文档」) that only toggles (no navigation) and holds one `SourceFolder` per section (user order, trigger aria-label `文件夹：<name>`) listing its sources (counts, status, unsubscribe+undo, click = filter by source). Group open state: `localStorage sidebar.sources.open` (default collapsed); collapsed folders: `sidebar.folders.closed`. There is no separate bottom 订阅源 list and no 管理数据源 row; /sources is reached via the header 数据源 button (≥sm), the command palette (管理数据源 / 添加数据源), and a section row's ⚙ (opens 管理分区).
- Section selects (add/edit, /sources filter, move-to, import preview) and the /sources section column show the NAME only (no icon); icons appear in sidebar rows/folders, manage dialog, icon picker, feed/drawer/palette badges.
- NavItem merges `className` last (so `pr-8` beats the base `px-2.5`).
- Tests: external Playwright t8 (icon picker, delete + backup download + cancel, rename + restart, tree persistence).

## All sections are user-managed (Oct 2026)
- There are no built-in sections any more; only 全部 is fixed. `lib/sections.ts#migrateSections` (run in `getDb()` before `seed()`) seeds the former defaults ONCE (settings flag `sections_seeded`): keys `lab/cn/media/x/ent`, names 实验室/国内媒体/国外媒体/X 快讯/娱乐, icons `lucide:<key>` (rendered by `secIcon` until the user picks an emoji), 娱乐 `in_all=0`. Keys are kept so old URLs (`?section=x`) and v1 files resolve.
- Each source stores its section KEY in `sources.section` (legacy `grp` is only seed metadata now). Migration fills NULL `section` from the old derivation (`legacyKey`: grp → region/category/type).
- Seed sync never creates/edits/deletes sections and never touches `section` of existing rows. A NEW seed source (first start or restored built-in) goes to its default key if that section still exists, else to `未分类` (auto-created, 📥) — `seedTarget()`. So deleted sections are never resurrected and renames stick (tracked by key, not name).
- API: `PATCH /api/sections/[key] {name, icon, inAll, move: -1|1}` for every section (reorder rewrites `pos`); `DELETE` needs `?moveTo` when non-empty. Name 「全部」 is reserved.
- Client: `common.ts` holds the runtime list (`setSections/allSections/sectionsReady`); `sectionOf(item) = item.section`. The feed shows skeletons until sections load so 全部 never flashes excluded sections. `parseFilters` accepts any key-like `?section=`.
- Import/export: JSON v2 `sections[]` lists every section used by the exported sources (renamed defaults included). Import resolves by NAME, then default key (lab/cn/media/x/ent) if it still exists, else creates the section by name (v1 legacy keys use the default names, 娱乐 created outside 全部), else 未分类. OPML: every section folder carries `sectionCustom="1" sectionKey=… sectionInAll=… sectionIcon=…`; foreign OPML folders match current section names, then legacy names, else the chosen default.
- Tests: external Playwright t9 (menu label, rename/re-icon/reorder/delete 娱乐, restart no-resurrection, 未分类 fallback, v1 import, v2 roundtrip with renamed default).
- Section icons: render ONLY via `components/section-icon.tsx` (`SectionIcon`, `SectionName`, `secIcon`). Stored icons may be `lucide:<key>` tokens; never print `section.icon` as text. Exports keep the token verbatim. Playwright t10 asserts no visible `lucide:` text across pages/dialogs/selects/menus.
- Playwright: run the external e2e `runall.sh <backup.db>` (kept outside the repo) — restores the DB before EACH test file (no order dependency). t11 covers the 数据源 group and plain-text section selects.
