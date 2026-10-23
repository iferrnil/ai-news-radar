// Round 14 (0.14.0): Trendshift 日榜 / 周榜 (seed sources, items, add-source preview, not in 热点), section icon library
// (brand + line icons, lazy chunk, sidebar render), 多模态 settings without the video-task API, favorites surviving the
// retention prune (scheduler path), source avatars through /api/icon (no foreign icon service in the page), compressed
// static assets.
import { execSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { chromium } from "playwright";
import { rec, B, RESTART, srcs, api , settingsOpen} from "./lib.mjs";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const SH = process.env.R14_SHOTS ?? "/tmp/shots-r14"; fs.mkdirSync(SH, { recursive: true });
const shot = (n) => path.join(SH, n);
const J = { "Content-Type": "application/json" };
const errors = [];
const br = await chromium.launch();

// ---------------- Trendshift
{
  const all = await srcs();
  const ts = all.filter((s) => s.name.startsWith("Trendshift"));
  rec("Trendshift: 日榜 + 周榜 seeded in 国外媒体, P1, hours apart", ts.length === 2 && ts.every((s) => s.section === "media" && s.priority === "P1" && s.type === "api" && s.poll_interval >= 10800 && s.enabled),
    JSON.stringify(ts.map((s) => [s.name, s.section, s.priority, s.poll_interval])));
  const items = await api("/api/items?limit=1000");
  const d = items.filter((it) => it.source === "Trendshift 日榜").sort((a, b) => b.published - a.published);
  const w = items.filter((it) => it.source === "Trendshift 周榜");
  rec("Trendshift: latest period only, by rank (4 + 4 of the mock; weekly #4 = daily #2 is shown once)", d.length === 4 && w.length === 3 && !w.some((it) => /repo-2$/i.test(it.title) && /mock-d1/i.test(it.title)) && d.every((it, i) => it.summary.startsWith(`#${i + 1} · Go · 今日 +`)) && w.every((it) => /本周 \+/.test(it.summary)),
    JSON.stringify(d.map((it) => it.title + " | " + it.summary)).slice(0, 400));
  rec("Trendshift: title owner/repo, link to GitHub", d.every((it) => /^mock-d1\/repo-\d$/.test(it.title) && it.url === "https://github.com/" + it.title), d.map((it) => it.url).join(" "));
  // re-poll: no duplicates
  for (const s of ts) await fetch(`${B}/api/sources/${s.id}/fetch`, { method: "POST", headers: J });
  await sleep(500);
  const again = (await api("/api/items?limit=1000")).filter((it) => it.source.startsWith("Trendshift"));
  rec("Trendshift: re-poll adds nothing, the twin stays hidden (stable)", again.length === 7, String(again.length));
  const wsrc = (await srcs()).find((s) => s.name === "Trendshift 周榜");
  rec("Trendshift: the hidden twin is stored (dedup is per section at listing time)", wsrc.item_count === 4, String(wsrc.item_count));
  const hot = await api("/api/hot");
  rec("Trendshift: rankings stay out of 热点", !hot.topics.some((t) => t.rows.some((r) => r.source.startsWith("Trendshift"))));
  // feed card + GitHub mark, add dialog preview of a pasted URL
  const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(B + `/?source=${ts[0].id}`); await p.locator("[data-item]").first().waitFor({ timeout: 15000 }); await sleep(600);
  rec("feed: Trendshift cards show a GitHub mark (no fetched icon)", await p.locator("[data-item]").first().locator("svg path").count() > 0 && await p.locator("[data-item] img").count() === 0);
  await p.screenshot({ path: shot("trendshift-feed.png") });
  await p.goto(B + "/sources"); await p.getByRole("heading", { name: "数据源" }).waitFor();
  await p.getByRole("button", { name: /添加数据源/ }).click(); const dlg = p.getByRole("dialog"); await dlg.waitFor();
  await dlg.locator("#src-input").fill("https://trendshift.io/weekly"); await dlg.getByRole("button", { name: "测试" }).click();
  await dlg.getByText("Trendshift 周榜").first().waitFor({ timeout: 15000 }).catch(() => {});
  const txt = await dlg.innerText();
  rec("add dialog: pasting trendshift.io/weekly previews 周榜 (API, 4 条)", /Trendshift 周榜/.test(txt) && /https:\/\/trendshift\.io\/weekly · 4 条/.test(txt) && /mock-w1\/repo-1/.test(txt), txt.replace(/\s+/g, " ").slice(0, 300));
  await dlg.screenshot({ path: shot("trendshift-add.png") });
  const t2 = await (await fetch(B + "/api/sources/test", { method: "POST", headers: J, body: JSON.stringify({ input: "trendshift.io" }) })).json();
  rec("detect: trendshift.io → 日榜", t2.name === "Trendshift 日榜" && t2.type === "api" && t2.url === "https://trendshift.io/", JSON.stringify(t2).slice(0, 200));
  await p.keyboard.press("Escape"); await sleep(300);

  // ---------------- icon library
  await p.getByRole("button", { name: "分区", exact: true }).click();
  const sd = p.getByRole("dialog", { name: "管理分区" }); await sd.waitFor();
  await sd.getByLabel("新分区名称").fill("开源");
  await sd.getByRole("button", { name: "新分区图标" }).click();
  const pk = p.getByTestId("icon-picker"); await pk.waitFor();
  const tabs = await pk.getByRole("tab").allInnerTexts();
  const brand = await pk.getByTestId("icon-grid").getByRole("button").count();
  let total = 0;
  for (const t of tabs) { await pk.getByRole("tab", { name: t }).click(); total += await pk.getByTestId("icon-grid").getByRole("button").count(); }
  rec("icon picker: 品牌 tab first with ≥40 marks, ≥300 icons in all", tabs[0] === "品牌 / 平台" && brand >= 40 && total >= 300, `${tabs.join(",")} brand=${brand} total=${total}`);
  await pk.getByRole("tab", { name: "品牌 / 平台" }).click(); await sleep(150); await pk.screenshot({ path: shot("icon-picker-brand.png") });
  await pk.getByLabel("搜索图标").fill("github");
  rec("icon picker: search github → the GitHub mark", await pk.getByRole("button", { name: /^图标 github / }).count() === 1);
  await pk.getByRole("button", { name: /^图标 github / }).click(); await sleep(200);
  await sd.getByRole("button", { name: "新建分区" }).click(); await sleep(800);
  const sec = (await api("/api/sections")).find((x) => x.name === "开源");
  rec("icon picker: line icon stored as lucide:github", sec?.icon === "lucide:github", JSON.stringify(sec));
  await p.keyboard.press("Escape"); await sleep(300);
  // move Trendshift into it, then the sidebar row renders the icon (lazy chunk), never the token
  for (const s of ts) await fetch(`${B}/api/sources/${s.id}`, { method: "PATCH", headers: J, body: JSON.stringify({ section: sec.key }) });
  await p.goto(B + `/?section=${sec.key}`); await p.locator("[data-item]").first().waitFor({ timeout: 15000 }); await sleep(800);
  const side = p.getByRole("complementary", { name: "侧边栏" });
  const row = side.getByRole("button", { name: /^开源/ }).first();
  rec("sidebar: library icon rendered as SVG, no token text", await row.locator("svg path").count() > 0 && !/lucide:/.test(await p.locator("body").innerText()), await row.innerHTML().catch(() => ""));
  await p.screenshot({ path: shot("section-github.png") });
  rec("dedup: 开源 holds 日榜 + 周榜 → 7 cards", await p.locator("[data-item]").count() === 7, String(await p.locator("[data-item]").count()));
  const wk = ts.find((s) => s.name === "Trendshift 周榜");
  await fetch(`${B}/api/sources/${wk.id}`, { method: "PATCH", headers: J, body: JSON.stringify({ section: "lab" }) });
  const after = await api("/api/items?limit=1000");
  const inSec = (k, src) => after.filter((it) => it.section === k && it.source === src).length;
  rec("dedup: 周榜 moved to 实验室 → both lists complete again (4 + 4)", inSec(sec.key, "Trendshift 日榜") === 4 && inSec("lab", "Trendshift 周榜") === 4, `${inSec(sec.key, "Trendshift 日榜")} ${inSec("lab", "Trendshift 周榜")}`);
  await fetch(`${B}/api/sources/${wk.id}`, { method: "PATCH", headers: J, body: JSON.stringify({ section: sec.key }) });
  // avatars come from /api/icon (same origin) — never a foreign icon service
  const srcsInDom = await p.evaluate(() => [...document.querySelectorAll("img")].map((i) => i.getAttribute("src") ?? ""));
  rec("page: no Google favicon / unavatar URLs", !srcsInDom.some((u) => /google\.com|unavatar\.io/.test(u)), srcsInDom.slice(0, 5).join(" "));
  const ir = await fetch(B + "/api/icon?host=evil.example.com");
  rec("/api/icon: unknown hosts refused (no open proxy)", ir.status === 404);
  await ctx.close();
}

// ---------------- compressed static assets
{
  const html = await (await fetch(B + "/")).text();
  const js = html.match(/\/_next\/static\/chunks\/[^"]+\.js/)?.[0];
  const r = await fetch(B + js, { headers: { "Accept-Encoding": "br" } });
  rec("static: JS chunks served brotli + immutable", r.headers.get("content-encoding") === "br" && /immutable/.test(r.headers.get("cache-control") ?? ""), `${r.headers.get("content-encoding")} ${r.headers.get("cache-control")}`);
}

// ---------------- 多模态 settings: no video-task API (desktop)
execSync(`bash ${RESTART} DISABLE_POLLER=1 AI_RADAR_DESKTOP=1`);
{
  const ctx = await br.newContext({ viewport: { width: 1280, height: 900 } });
  await ctx.addInitScript(() => { window.__radarDesktop = true; });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  await settingsOpen(p, "model"); await p.getByTestId("mm-card").waitFor({ timeout: 15000 });
  const t = await p.getByTestId("mm-card").innerText();
  rec("多模态 card: image + vision only (no 视频 fields)", !/视频|任务接口|提交路径/.test(t) && await p.locator("#mm-videoModel, #mm-videoSubmit, #mm-videoPoll").count() === 0 && await p.locator("#mm-imageModel").count() === 1, t.replace(/\s+/g, " "));
  await p.getByTestId("mm-card").screenshot({ path: shot("settings-multimodal.png") });
  const r = await fetch(B + "/api/multimodal", { method: "PATCH", headers: J, body: JSON.stringify({ imageModel: "img-x", videoModel: "v", videoSubmit: "/x" }) });
  const j = await r.json();
  rec("API: old video fields accepted and ignored", r.status === 200 && j.imageModel === "img-x" && !("videoModel" in j) && !("video" in j), JSON.stringify(j));
  await ctx.close();
}

// ---------------- favorites survive the retention prune (the scheduler's prune on start)
execSync(`bash ${RESTART} DISABLE_POLLER=1`);
{
  const items = await api("/api/items?limit=1000");
  const fav = items.find((it) => !it.is_x && !it.source.startsWith("Trendshift"));
  await fetch(B + "/api/favorites", { method: "POST", headers: J, body: JSON.stringify({ id: fav.id, on: true }) });
  execSync(`bash ${RESTART} RETENTION_HOURS=0.005`); // 18 s: everything of the fixture is older
  let left = [];
  for (let i = 0; i < 40; i++) { await sleep(500); left = await api("/api/items?limit=1000"); if (left.length <= 1) break; }
  const favs = await api("/api/items?fav=1&limit=50");
  rec("prune: the favorite survives, the rest is gone", favs.length === 1 && favs[0].id === fav.id && !left.some((it) => it.id !== fav.id), `fav=${favs.length} left=${left.length}`);
  const ex = await api("/api/favorites/export");
  rec("prune: favorite still exportable with its source", ex.length === 1 && ex[0].source, JSON.stringify(ex).slice(0, 200));
  const p = await br.newPage({ viewport: { width: 1280, height: 900 } }); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(B + "/?view=fav"); await p.locator("[data-item]").first().waitFor({ timeout: 15000 });
  rec("收藏 view after prune: 1 card", await p.locator("[data-item]").count() === 1);
  await p.goto(B + "/?view=hot"); await p.getByTestId("hot-view").waitFor({ timeout: 15000 }); await sleep(500);
  rec("热点 view after prune: renders (empty state)", await p.getByTestId("hot-topic").count() === 0);
  await p.close();
}
rec("no page errors", errors.length === 0, errors.join(" | ").slice(0, 300));
await br.close();
