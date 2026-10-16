// Round 6: desktop jank audit — interaction latency (Event Timing, INP-like) + long tasks on a ~1k-item feed, 4x CPU throttle.
import fs from "node:fs";
import { execSync } from "node:child_process";
import { chromium } from "playwright";
import { rec, B, RESTART, DATA } from "./lib.mjs";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
execSync(`bash ${RESTART}`);
// inflate the feed to ~1k cards (copies of the fixture items, spread over the last 40 h)
execSync(`python3 - <<'PY'
import sqlite3, time
c = sqlite3.connect("${DATA}/news.db"); cols = [r[1] for r in c.execute("pragma table_info(items)") if r[1] not in ("id",)]
rows = c.execute("select " + ",".join(cols) + " from items where merged_into is null and hidden=0").fetchall(); now = time.time(); k = 0
for rep in range(12):
  for r in rows:
    d = dict(zip(cols, r)); k += 1; d["uid"] = "perf-%d-%s" % (rep, d["uid"]); d["url"] = (d["url"] or "") + "#p%d" % rep; d["published"] = now - k * 140; d["fetched"] = d["published"]
    c.execute("insert into items(" + ",".join(cols) + ") values(" + ",".join("?" * len(cols)) + ")", [d[x] for x in cols])
c.commit()
PY`);
execSync(`bash ${RESTART}`);
const br = await chromium.launch();
const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
await ctx.addInitScript(() => { window.__radarDesktop = true; window.__radarTitlebar = true;
  window.__lt = []; window.__ev = [];
  new PerformanceObserver((l) => { for (const e of l.getEntries()) window.__lt.push([e.startTime, e.duration]); }).observe({ type: "longtask", buffered: true });
  new PerformanceObserver((l) => { for (const e of l.getEntries()) window.__ev.push([e.startTime, e.duration, e.name]); }).observe({ type: "event", durationThreshold: 16, buffered: true }); });
const p = await ctx.newPage(); const errors = []; p.on("pageerror", (e) => errors.push(String(e)));
const cdp = await ctx.newCDPSession(p); await cdp.send("Emulation.setCPUThrottlingRate", { rate: 4 });
const res = {};
const t0 = Date.now(); await p.goto(B + "/"); await p.locator("[data-item]").first().waitFor({ timeout: 30000 });
res["startup: first card (ms)"] = { wall: Date.now() - t0, ...(await p.evaluate(() => ({ lt: Math.round(window.__lt.reduce((a, x) => a + x[1], 0)), fcp: Math.round(performance.getEntriesByName("first-contentful-paint")[0]?.startTime ?? 0) }))) };
await sleep(1500);
async function measure(name, act, settle) {
  const s = await p.evaluate(() => performance.now()); const w = Date.now();
  await act(); if (settle) await settle(); await sleep(250);
  const r = await p.evaluate((s) => ({ lt: Math.round(window.__lt.filter((x) => x[0] >= s).reduce((a, x) => a + x[1], 0)), inp: Math.round(Math.max(0, ...window.__ev.filter((x) => x[0] >= s).map((x) => x[1]))) }), s);
  res[name] = { wall: Date.now() - w - 250, ...r }; await sleep(300);
}
const side = (t) => p.locator(".app-sidebar").getByText(t, { exact: true }).first();
const secs = await p.locator(".app-sidebar a, .app-sidebar button").allInnerTexts();
const sec = secs.map((x) => x.trim().split("\n")[0]).find((x) => /媒体|实验室|研究/.test(x)) ?? "国外媒体";
await measure(`switch section → ${sec}`, () => side(sec).click(), () => p.locator("[data-item]").first().waitFor());
await measure("switch section → 全部", () => side("全部").click(), () => p.locator("[data-item]").first().waitFor());
await measure("sidebar collapse", () => p.getByTestId("sidebar-toggle").click());
await measure("sidebar expand", () => p.getByTestId("sidebar-toggle").click());
await measure("hover card", () => p.locator("[data-item]").nth(2).hover(), () => sleep(900));
await p.mouse.move(5, 880);
await measure("open brief dialog", () => p.getByTestId("brief-btn").click(), () => p.getByTestId("brief-body").waitFor({ timeout: 20000 }));
await p.keyboard.press("Escape"); await sleep(400);
await measure("open translate menu", () => p.getByTestId("tr-toggle").click(), () => p.getByTestId("tr-menu").waitFor());
await p.keyboard.press("Escape"); await sleep(300);
await measure("open item sheet", () => p.locator("[data-item]").nth(1).click(), () => sleep(500));
await p.keyboard.press("Escape"); await sleep(400);
await measure("theme switch", () => p.getByRole("button", { name: "切换主题" }).first().click());
await measure("theme switch back", () => p.getByRole("button", { name: "切换主题" }).first().click());
await measure("palette open (⌘K)", () => p.keyboard.press("Control+k"), () => p.getByPlaceholder("搜索新闻、来源、操作…").waitFor());
await measure("palette type 6 chars", () => p.keyboard.type("OpenAI", { delay: 60 }), () => sleep(400));
await p.keyboard.press("Escape"); await sleep(300);
await measure("SSE-like burst: mark-all read + refresh", () => p.evaluate(() => window.dispatchEvent(new Event("focus"))));
await measure("navigate → 设置", () => p.locator(".app-sidebar").getByRole("button", { name: "设置" }).click(), () => p.getByTestId("ai-card").waitFor());
await measure("navigate → 数据源", () => p.locator("[aria-label=\"数据源\"]").first().click(), () => p.getByRole("heading", { name: "数据源" }).waitFor());
await measure("navigate back → 全部", () => side("全部").click(), () => p.locator("[data-item]").first().waitFor());
console.table(res);
if (process.env.AUDIT_OUT) fs.writeFileSync(process.env.AUDIT_OUT, JSON.stringify(res, null, 1));
rec("jank audit measured", Object.keys(res).length > 10, JSON.stringify(res));
await br.close();
rec("no page errors (t21)", errors.length === 0, errors.slice(0, 2).join(" | "));
