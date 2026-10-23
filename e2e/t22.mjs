// Round 9: 自动简报 / 手动简报 / 生成文章 toolbar, manual selection (max 10), stale-brief race (#1), article dialog with
// per-platform Skills + cover notice, settings (Skill ×4, 多模态模型, 制品目录), web gating.
import { execSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { chromium } from "playwright";
import { rec, B, RESTART, DATA } from "./lib.mjs";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const SH = process.env.R11_SHOTS ?? "/tmp/shots-r11"; fs.mkdirSync(SH, { recursive: true });
const shot = (n) => path.join(SH, n);
const ART = path.join(DATA, "artifacts"); fs.rmSync(ART, { recursive: true, force: true });
const errors = [];
const br = await chromium.launch();

// ---------------- web
execSync(`bash ${RESTART}`);
const secs = await (await fetch(B + "/api/sections")).json();
const items = await (await fetch(B + "/api/items?limit=1000")).json();
const bySec = {}; for (const it of items) (bySec[it.section] ??= []).push(it);
const rich = secs.filter((s) => (bySec[s.key]?.length ?? 0) >= 2).map((s) => s.key);
const [A, Bk] = rich; const nameOf = (k) => secs.find((s) => s.key === k)?.name ?? k;
{
  const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(`${B}/?section=${A}`); await p.locator("[data-item]").first().waitFor({ timeout: 20000 });
  const tools = await p.getByTestId("brief-tools").innerText();
  rec("web toolbar: 自动简报 + 手动简报, no 生成文章", /自动简报/.test(tools) && /手动简报/.test(tools) && await p.getByTestId("article-btn").count() === 0, tools.replace(/\n/g, " "));
  rec("web: compose API is desktop-only", (await fetch(B + "/api/compose", { method: "POST", body: "{}" })).status === 404);

  // #1 race: section A's brief answers late (dialog left open, ⌘K switches to B) → B's brief must stay
  await p.route(`**/api/brief?section=${A}*`, async (r) => { await sleep(2500); await r.continue(); });
  await p.getByTestId("brief-btn").click(); await p.getByTestId("brief-dialog").waitFor();
  await p.keyboard.press("Control+k"); await p.getByPlaceholder("搜索新闻、来源、操作…").fill(nameOf(Bk)); await sleep(300); await p.getByRole("option", { name: nameOf(Bk) }).first().click();
  for (let i = 0; i < 50 && !p.url().includes(`section=${Bk}`); i++) await sleep(100);
  rec("#1 setup: ⌘K switched section while the brief dialog was open", p.url().includes(`section=${Bk}`), p.url());
  if (await p.getByTestId("brief-dialog").count() === 0) await p.getByTestId("brief-btn").click();
  await p.getByTestId("brief-body").waitFor({ timeout: 10000 }); await sleep(3200); // A's late answer has arrived by now
  const title = await p.getByTestId("brief-dialog").locator("[data-slot=dialog-title]").innerText();
  const bodyB = await p.getByTestId("brief-body").innerText();
  const aOnly = (bySec[A] ?? []).filter((it) => !(bySec[Bk] ?? []).some((x) => x.title === it.title)).map((it) => it.title.slice(0, 20));
  rec("#1: brief after switching section shows the new section, not the late previous one", title.includes(nameOf(Bk)) && !title.includes(nameOf(A)) && !aOnly.some((t) => bodyB.includes(t)), title);
  await p.keyboard.press("Escape"); await sleep(300); await p.unrouteAll({ behavior: "ignoreErrors" });

  // manual selection: max 10 with a counter
  await p.goto(`${B}/`); await p.locator("[data-item]").first().waitFor();
  await p.getByTestId("brief-tools").locator("xpath=..").screenshot({ path: shot("toolbar-web.png") });
  await p.getByTestId("brief-manual-btn").click(); await p.getByTestId("pick-bar").waitFor();
  const cards = p.locator("[data-item]"); const n = Math.min(12, await cards.count());
  for (let i = 0; i < n; i++) { await cards.nth(i).click(); await sleep(40); }
  const cnt = await p.getByTestId("pick-count").innerText();
  rec("manual: selection capped at 10 with count shown", cnt === `${Math.min(10, n)}/10` && await p.locator("[data-picked]").count() === Math.min(10, n), cnt);
  rec("manual: clicking a card selects instead of opening the drawer", await p.getByRole("dialog").count() === 0);
  await p.evaluate(() => window.scrollTo(0, 0)); await sleep(300); await p.screenshot({ path: shot("manual-selection.png") });
  const picked = await p.locator("[data-picked]").evaluateAll((els) => els.map((e) => Number(e.getAttribute("data-item"))));
  const req = p.waitForRequest((r) => r.url().includes("/api/brief") && r.url().includes("ids="));
  await p.getByTestId("pick-go").click(); const u = new URL((await req).url());
  await p.getByTestId("brief-body").waitFor({ timeout: 10000 });
  const ids = u.searchParams.get("ids").split(",").map(Number);
  const meta = await p.getByTestId("brief-dialog").locator("[data-slot=dialog-description]").innerText();
  rec("manual: brief from exactly the selected items", ids.length === picked.length && picked.every((x) => ids.includes(x)) && meta.startsWith(`${picked.length} 条`), meta);
  await p.keyboard.press("Escape"); await sleep(300);
  await p.keyboard.press("Escape"); await sleep(200);
  rec("manual: Esc leaves selection mode", await p.getByTestId("pick-bar").count() === 0);
  // 自动简报 API: ≤20 picked (24h pool)
  const auto = await (await fetch(B + "/api/brief?section=all")).json();
  rec("auto brief: at most 20 items", auto.count > 0 && auto.count <= 20, String(auto.count));
  await ctx.close();
}

// ---------------- desktop
execSync(`bash ${RESTART} AI_RADAR_DESKTOP=1 ${["DEEPSEEK_API_KEY", process.env.E2E_DS_KEY].join("=")} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 BRIEF_CACHE_MS=1 ARTIFACTS_DIR=${ART}`);
{
  const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
  await ctx.addInitScript(() => { window.__radarDesktop = true; });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(`${B}/?section=${A}`); await p.locator("[data-item]").first().waitFor({ timeout: 20000 });
  rec("desktop toolbar: 自动简报 · 手动简报 · 生成文章 (in order)", (await p.getByTestId("brief-tools").getByRole("button").allInnerTexts()).join(",") === "自动简报,手动简报,生成文章");
  await p.getByTestId("brief-tools").locator("xpath=..").screenshot({ path: shot("toolbar-desktop.png") });
  await p.getByTestId("article-btn").click();
  const cards = p.locator("[data-item]"); const k = Math.min(2, await cards.count());
  for (let i = 0; i < k; i++) await cards.nth(i).click();
  await p.getByTestId("pick-go").click(); await p.getByTestId("article-dialog").waitFor();
  rec("article dialog: 推特 / 公众号 / 小红书 segmented options, no AI 润色", (await p.getByTestId("article-platforms").getByRole("radio").allInnerTexts()).join(",") === "推特文章,公众号文章,小红书文章" && await p.getByTestId("article-dialog").getByText("AI 润色").count() === 0);
  const runPf = async (pf) => {
    await p.getByTestId(`pf-${pf}`).click();
    await p.getByTestId("article-go").click(); await p.getByTestId("article-body").waitFor({ timeout: 20000 }); await sleep(300);
    return { body: await p.getByTestId("article-body").innerText(), meta: await p.getByTestId("article-meta").innerText(), note: await p.getByTestId("article-cover-note").allInnerTexts() };
  };
  for (const [pf, sk] of [["twitter", "twitter-article"], ["wechat", "wechat-article"], ["xhs", "xhs-article"]]) {
    const r = await runPf(pf);
    rec(`article ${pf}: written with ${sk}`, r.meta.startsWith(sk) && r.body.includes(sk) && !["twitter-article", "wechat-article", "xhs-article"].filter((x) => x !== sk).some((x) => r.body.includes(x)), r.meta);
    if (pf !== "twitter") rec(`article ${pf}: cover requested, skipped without multimodal model`, r.note.some((t) => /未配置图片模型/.test(t)) && !r.body.includes("e2e cover prompt"), r.note.join("|"));
  }
  await p.getByTestId("article-dialog").screenshot({ path: shot("article-dialog.png") });
  const count = (d) => (fs.existsSync(path.join(ART, d)) ? Number(execSync(`find "${ART}/${d}" -name '*.md' | wc -l`).toString().trim()) : 0);
  rec("articles not saved until 导出", count("文章") === 0, String(count("文章")));
  rec("article dialog: no 工作目录 / 制品目录 button", await p.getByTestId("article-open-dir").count() === 0 && !/制品目录|工作目录/.test(await p.getByTestId("article-dialog").innerText()));
  await p.getByTestId("article-export").click(); await p.locator("[data-sonner-toast]").filter({ hasText: "已导出" }).waitFor({ timeout: 10000 });
  rec("导出: the article written to 文章/<date>/ (one file)", count("文章") === 1 && /已导出/.test(await p.getByTestId("article-meta").innerText()), String(count("文章")));
  await p.getByTestId("article-dialog").screenshot({ path: shot("article-dialog-exported.png") });
  await p.keyboard.press("Escape"); await sleep(300);
  await p.getByTestId("brief-btn").click(); await p.getByTestId("brief-body").waitFor({ timeout: 10000 }); await sleep(300);
  rec("desktop brief not saved until 导出", count("简报") === 0 && await p.getByTestId("brief-saved").count() === 0);
  await p.getByTestId("brief-export").click(); await p.getByTestId("brief-saved").waitFor({ timeout: 10000 });
  rec("导出: the brief written to 简报/<date>/", count("简报") === 1, String(count("简报")));
  await p.keyboard.press("Escape");

  // settings
  await p.goto(B + "/settings"); await p.getByTestId("artifacts-card").waitFor({ timeout: 15000 }); await sleep(800);
  rec("settings: 推特 / 公众号 / 小红书 Skill cards with their built-in defaults",
    /twitter-article/.test(await p.getByTestId("skill-card-twitter").innerText()) && /wechat-article/.test(await p.getByTestId("skill-card-wechat").innerText()) && /xhs-article/.test(await p.getByTestId("skill-card-xhs").innerText()));
  rec("settings: 工作目录 shows the configured dir", (await p.getByTestId("artifacts-dir").getAttribute("placeholder")) === ART);
  await p.locator("#mm-baseUrl").fill("https://mm.example.com/v1"); await p.locator("#mm-imageModel").fill("img-model"); await p.locator("#mm-key").fill("sk-" + "e2emultimodalkey0000");
  await p.getByTestId("mm-save").click(); await p.getByTestId("mm-on").waitFor();
  const mm = await (await fetch(B + "/api/multimodal")).json();
  rec("multimodal config saved, key never returned", mm.image === true && mm.configured && !JSON.stringify(mm).includes("e2emultimodalkey"));
  for (const [id, f] of [["skill-card-twitter", "settings-skill-twitter.png"], ["mm-card", "settings-multimodal.png"], ["artifacts-card", "settings-artifacts.png"]]) await p.getByTestId(id).screenshot({ path: shot(f) });
  await p.screenshot({ path: shot("settings-full.png"), fullPage: true });
  await ctx.close();
}
await br.close();
rec("no page errors (t22)", errors.length === 0, errors.slice(0, 2).join(" | "));
