// Round 10: selection mode only selects (corner click never navigates), 生成文章 ≤2, settings 抓取 API (fake providers:
// keys, Jina base URL, quota refresh, 不可用), provider failover + 原文 files, drawer 翻译 switch (desktop; hidden on web).
import { execSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { chromium } from "playwright";
import { rec, B, RESTART, DATA, mock } from "./lib.mjs";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const SH = process.env.R10_SHOTS ?? "/tmp/shots-r10"; fs.mkdirSync(SH, { recursive: true });
const shot = (n) => path.join(SH, n);
const ART = path.join(DATA, "artifacts"); fs.rmSync(ART, { recursive: true, force: true });
const errors = [];
const br = await chromium.launch();

// ---------------- web
execSync(`bash ${RESTART}`);
const items = await (await fetch(B + "/api/items?limit=1000")).json();
const nonX = new Set(items.filter((it) => !it.is_x).map((it) => it.id));
const en = items.find((it) => !it.is_x && /update #\d+: new model/.test(it.title));
{
  const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  const popups = []; ctx.on("page", (pg) => popups.push(pg.url()));
  await p.goto(`${B}/`); await p.locator("[data-item]").first().waitFor({ timeout: 20000 });
  const url0 = p.url();
  await p.getByTestId("brief-manual-btn").click(); await p.getByTestId("pick-bar").waitFor();
  const card = p.locator("[data-item]").first();
  rec("selection mode: no 打开原文 link inside cards", await p.locator("[data-item] a").count() === 0, String(await p.locator("[data-item] a").count()));
  const bb = await card.boundingBox();
  const corner = async () => { await p.mouse.click(bb.x + bb.width - 5, bb.y + bb.height - 5); await sleep(250); };
  await corner();
  const picked1 = await card.getAttribute("data-picked");
  rec("corner click selects the card", picked1 === "true" && await card.getByTestId("pick-mark").getAttribute("aria-checked") === "true", String(picked1));
  rec("corner click: no navigation, no new page, no drawer", p.url() === url0 && popups.length === 0 && await p.getByRole("dialog").count() === 0, `${p.url()} popups=${popups.length}`);
  await corner();
  rec("second corner click deselects (toggles once per click)", await card.getAttribute("data-picked") === null && (await p.getByTestId("pick-count").innerText()).startsWith("0/"));
  const mk = await card.getByTestId("pick-mark").boundingBox();
  rec("check mark is a larger hit target (≥24px)", mk.width >= 24 && mk.height >= 24, `${mk.width}x${mk.height}`);
  await p.mouse.click(mk.x + mk.width / 2, mk.y + mk.height / 2); await sleep(200);
  rec("clicking the check toggles selection", await card.getAttribute("data-picked") === "true");
  await p.keyboard.press("Escape"); await sleep(300);
  rec("leaving selection mode restores the link", await card.locator("a[aria-label=打开原文]").count() === 1);

  // drawer: no 翻译 switch without a translation backend
  await p.locator(`[data-item="${en.id}"]`).click(); await p.getByRole("dialog").waitFor();
  rec("web drawer: 翻译 switch hidden when translation is unavailable", await p.getByTestId("sheet-tr").count() === 0);
  await p.keyboard.press("Escape"); await sleep(200);
  rec("web: fetch API endpoints are desktop-only", (await fetch(B + "/api/fetchapi")).status === 404 && (await fetch(B + "/api/fetchapi/quota", { method: "POST" })).status === 404);
  rec("web: per-item translate refuses without a key", (await fetch(B + "/api/translate/item", { method: "POST", body: JSON.stringify({ id: en.id }) })).status === 503);
  await ctx.close();
}

// ---------------- desktop
execSync(`bash ${RESTART} AI_RADAR_DESKTOP=1 ${["DEEPSEEK_API_KEY", process.env.E2E_DS_KEY].join("=")} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 BRIEF_CACHE_MS=1 ARTIFACTS_DIR=${ART}`);
{
  const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
  await ctx.addInitScript(() => { window.__radarDesktop = true; });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));

  // settings › 抓取 API
  await p.goto(B + "/settings"); await p.getByTestId("fetchapi-card").waitFor({ timeout: 15000 }); await sleep(600);
  const card = p.getByTestId("fetchapi-card");
  rec("fetch API card: three providers, all 未配置", (await card.locator("[data-testid$=-status]").allInnerTexts()).join(",") === "未配置,未配置,未配置");
  const keys = { jina: "jina_" + "e2efakekey000000000000", firecrawl: "fc-" + "e2efakekey000000000000", crawl4ai: "c4-" + "e2efakekey000000000000" };
  for (const [id, k] of Object.entries(keys)) await p.locator(`#fa-${id}`).fill(k);
  rec("key fields are masked", (await p.locator("#fa-jina").getAttribute("type")) === "password");
  await p.getByTestId("fetchapi-jina-base").fill("https://r.jinaai.cn");
  await p.getByTestId("fetchapi-save").click(); await sleep(600);
  rec("saved: all three 可用", (await card.locator("[data-testid$=-status]").allInnerTexts()).join(",") === "可用,可用,可用");
  const pub = await (await fetch(B + "/api/fetchapi")).json();
  rec("keys never returned; Jina base URL stored", !JSON.stringify(pub).includes("e2efakekey") && pub.providers[0].baseUrl === "https://r.jinaai.cn", pub.providers[0].baseUrl);
  await p.getByTestId("fetchapi-refresh").click(); await p.getByTestId("fetchapi-firecrawl-quota").waitFor({ timeout: 10000 }); await sleep(300);
  const q = async (id) => (await p.getByTestId(`fetchapi-${id}-quota`).innerText()).trim();
  const fq = await q("firecrawl"), cq = await q("crawl4ai"), jq = await q("jina");
  rec("Firecrawl quota: remaining / plan · reset date", /剩余 4,321 \/ 10万/.test(fq) && /11\/1重置/.test(fq), fq);
  rec("crawl4ai quota: balance + monthly allowance", /剩余 812 \/ 1,000/.test(cq) && /本月 300/.test(cq), cq);
  rec("Jina quota (dashboard, best effort)", /剩余 987\.7万 tokens/.test(jq), jq);
  await card.screenshot({ path: shot("settings-fetch-api.png") });
  await mock("fail=dash.jina.ai&mode=500");
  await p.getByTestId("fetchapi-refresh").click(); await sleep(800);
  rec("Jina quota failure shows 不可用", (await q("jina")) === "额度 不可用", await q("jina"));
  await mock("fail=dash.jina.ai&mode=ok");

  // 生成文章: at most 2, Jina out of credit → Firecrawl
  await p.goto(`${B}/`); await p.locator("[data-item]").first().waitFor({ timeout: 20000 });
  await p.getByTestId("article-btn").click(); await p.getByTestId("pick-bar").waitFor();
  const ids = (await p.locator("[data-item]").evaluateAll((els) => els.map((e) => Number(e.getAttribute("data-item"))))).filter((id) => nonX.has(id)).slice(0, 3);
  for (const id of ids) { await p.locator(`[data-item="${id}"]`).click(); await sleep(60); }
  rec("article selection capped at 2 (2/2, third not selected)", (await p.getByTestId("pick-count").innerText()) === "2/2" && await p.locator("[data-picked]").count() === 2 && await p.locator(`[data-item="${ids[2]}"]`).getAttribute("data-picked") === null);
  await p.evaluate(() => window.scrollTo(0, 0)); await sleep(400);
  await p.screenshot({ path: shot("selection-article-2.png") });
  rec("server rejects 3 ids for an article", (await fetch(B + "/api/compose", { method: "POST", body: JSON.stringify({ platform: "twitter", ids, section: "all" }) })).status === 400);
  await mock("fail=r.jinaai.cn&mode=402");
  await p.getByTestId("pick-go").click(); await p.getByTestId("article-dialog").waitFor();
  rec("article dialog shows 2 items", /2 条/.test(await p.getByTestId("article-dialog").innerText()));
  await p.getByTestId("pf-twitter").click(); await p.getByTestId("article-go").click(); await p.getByTestId("article-body").waitFor({ timeout: 30000 }); await sleep(300);
  await p.getByTestId("article-dialog").screenshot({ path: shot("article-2-items.png") });
  const origDir = path.join(ART, "原文");
  const files = fs.existsSync(origDir) ? execSync(`find "${origDir}" -name '*.md'`).toString().trim().split("\n").filter(Boolean) : [];
  const txt = files.map((f) => fs.readFileSync(f, "utf8"));
  rec("原文 saved: one file per item under 原文/<date>/", files.length === 2 && files.every((f) => /原文\/\d{4}-\d{2}-\d{2}\/\d{4,6}-/.test(f)), files.map((f) => path.relative(ART, f)).join(" | "));
  rec("原文 files record method + text (Jina 402 → Firecrawl)", txt.every((t) => /method: firecrawl/.test(t) && /url: https?:\/\//.test(t) && /source: /.test(t) && t.includes("E2E-FIRECRAWL")), txt[0]?.slice(0, 160).replace(/\n/g, " ⏎ "));
  const st = await (await fetch(B + "/api/fetchapi")).json();
  rec("Jina marked 额度用尽 after 402", st.providers[0].status === "exhausted", st.providers.map((x) => x.status).join(","));
  await mock("fail=r.jinaai.cn&mode=ok");
  await p.keyboard.press("Escape"); await sleep(300); await p.keyboard.press("Escape"); await sleep(300);

  // drawer 翻译 switch
  await p.locator("[data-sonner-toast]").first().waitFor({ state: "detached", timeout: 10000 }).catch(() => {});
  await p.locator(`[data-item="${en.id}"]`).click(); await p.getByRole("dialog").waitFor(); await sleep(300);
  const sw = p.getByTestId("sheet-tr"); const title = p.getByRole("dialog").locator("[data-slot=sheet-title]");
  rec("desktop drawer: 翻译 switch shown, off by default, original title", await sw.count() === 1 && (await sw.getAttribute("aria-checked")) !== "true" && (await title.innerText()) === en.title, await title.innerText());
  await sw.click(); await p.waitForFunction(() => document.querySelector("[data-slot=sheet-title]")?.textContent?.startsWith("【译】"), null, { timeout: 10000 }).catch(() => {});
  rec("switch on: title and body translated", (await title.innerText()).startsWith("【译】") && (await p.getByRole("dialog").innerText()).includes("【译】Summary"), await title.innerText());
  await p.getByRole("dialog").screenshot({ path: shot("drawer-translate.png") });
  await sw.click(); await sleep(300);
  rec("switch off: original shown again", (await title.innerText()) === en.title && !(await p.getByRole("dialog").innerText()).includes("【译】"));
  await ctx.close();
}
await br.close();
rec("no page errors (t23)", errors.length === 0, errors.slice(0, 2).join(" | "));
