// Round 6: scroll performance on a long feed (web + desktop) and the back-to-top button in both.
import fs from "node:fs";
import { execSync } from "node:child_process";
import { chromium } from "playwright";
import { rec, B, RESTART, SHOT, DATA } from "./lib.mjs";
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
const errors = []; const out = {};
for (const desktop of [false, true]) {
  const tag = desktop ? "desktop" : "web";
  const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
  if (desktop) await ctx.addInitScript(() => { window.__radarDesktop = true; window.__radarTitlebar = true; });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(B + "/"); await p.locator("[data-item]").first().waitFor({ timeout: 20000 }); await sleep(800);
  for (let i = 0; i < 12; i++) { await p.evaluate(() => window.scrollTo(0, document.body.scrollHeight)); await sleep(350); }
  await p.evaluate(() => window.scrollTo(0, 0)); await sleep(300);
  if (process.env.PERF_CSS) await p.addStyleTag({ content: process.env.PERF_CSS });
  const cdp = await ctx.newCDPSession(p); await cdp.send("Emulation.setCPUThrottlingRate", { rate: 4 });
  const m = await p.evaluate(async () => {
    const lt = []; const po = new PerformanceObserver((l) => { for (const e of l.getEntries()) lt.push(e.duration); }); po.observe({ type: "longtask", buffered: false });
    const fr = []; let last = performance.now();
    for (let i = 0; i < 240; i++) { await new Promise((r) => requestAnimationFrame(r)); const n = performance.now(); fr.push(n - last); last = n; window.scrollBy(0, i < 120 ? 140 : -140); }
    po.disconnect(); fr.sort((a, b) => a - b);
    return { cards: document.querySelectorAll("[data-item]").length, avg: +(fr.reduce((a, b) => a + b, 0) / fr.length).toFixed(1), p95: +fr[Math.floor(fr.length * 0.95)].toFixed(1), jank: fr.filter((x) => x > 50).length, longTasks: lt.length, longMs: Math.round(lt.reduce((a, b) => a + b, 0)) };
  });
  await cdp.send("Emulation.setCPUThrottlingRate", { rate: 1 });
  out[tag] = m; rec(`${tag}: scroll perf measured (4x CPU throttle)`, m.cards > 20, JSON.stringify(m));
  // back-to-top
  await p.evaluate(() => window.scrollTo(0, 3000)); await sleep(400);
  const btn = p.getByTestId("back-to-top");
  rec(`${tag}: back-to-top appears after scrolling`, await btn.count() === 1);
  if (await btn.count()) { await btn.click(); await sleep(1200); rec(`${tag}: back-to-top scrolls to the top`, await p.evaluate(() => window.scrollY) < 5); }
  if (desktop) await p.screenshot({ path: SHOT("r6_back_to_top_desktop.png") });
  await ctx.close();
}
if (process.env.PERF_OUT) fs.writeFileSync(process.env.PERF_OUT, JSON.stringify(out));
await br.close();
rec("no page errors (t20)", errors.length === 0, errors.slice(0, 2).join(" | "));
