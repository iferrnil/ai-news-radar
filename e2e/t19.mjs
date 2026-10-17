// Round 5: sidebar collapse (no ghost, both directions, web + desktop), settings redesign + lucide settings icon,
// 简报 Skill (desktop only; install / upload / activate / preview / delete; AI 润色 sends the active skill), notification status UI.
import { execSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { chromium } from "playwright";
import { rec, B, RESTART, SHOT, DATA } from "./lib.mjs";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const dsEnv = () => ["DEEPSEEK_API_KEY", process.env.E2E_DS_KEY].join("=");
const fetchR = async (url, o, n = 4) => { for (let i = 0; i < n; i++) { try { return await fetch(url, o); } catch { await sleep(500); } } return fetch(url, o); };
const J = { "Content-Type": "application/json" };

fs.rmSync(path.join(DATA, "skills"), { recursive: true, force: true });
execSync(`bash ${RESTART}`);
const br = await chromium.launch();
const errors = [];

// ---- sidebar collapse, web and desktop, both directions
for (const desktop of [false, true]) {
  const ctx = await br.newContext({ viewport: { width: 1280, height: 820 } });
  if (desktop) await ctx.addInitScript(() => { window.__radarDesktop = true; window.__radarTitlebar = true;
    document.addEventListener("DOMContentLoaded", () => document.documentElement.style.setProperty("--mygo-titlebar-inset-left", "78px")); });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(B + "/"); await p.locator("[data-slot=card]").first().waitFor({ timeout: 20000 }); await sleep(400);
  const tag = desktop ? "desktop" : "web";
  const state = () => p.evaluate(() => { const a = document.querySelector(".app-sidebar"); const cs = getComputedStyle(a); const r = a.getBoundingClientRect(); return { vis: cs.visibility, w: r.width, header: document.querySelector(".app-header").getBoundingClientRect().left }; });
  // where the brand sat: after collapsing nothing of the sidebar may be hit-testable / painted there
  const brandHit = () => p.evaluate(() => { const el = document.elementFromPoint(150, 28); return !!el?.closest(".app-sidebar"); });
  const s0 = await state();
  rec(`${tag}: sidebar visible at start`, s0.vis === "visible" && s0.w > 200 && await brandHit());
  await p.getByTestId("sidebar-toggle").click(); await sleep(450);
  const s1 = await state();
  rec(`${tag}: collapse → hidden (visibility hidden, width 0, nothing left under the title bar)`, s1.vis === "hidden" && s1.w === 0 && !(await brandHit()), JSON.stringify(s1));
  if (desktop) {
    const bg = await p.evaluate(() => { const cs = getComputedStyle(document.querySelector(".app-header")); return [cs.backdropFilter, cs.backgroundColor]; });
    rec("desktop: header opaque, no backdrop-filter", (bg[0] === "none" || bg[0] === "") && !/\/ 0\.|, 0\.\d+\)$/.test(bg[1]), bg.join(" | "));
  }
  await p.screenshot({ path: SHOT(`r5_sidebar_collapsed_${tag}.png`), clip: { x: 0, y: 0, width: 640, height: 220 } });
  await p.getByTestId("sidebar-toggle").click(); await sleep(450);
  const s2 = await state();
  rec(`${tag}: expand → visible again`, s2.vis === "visible" && s2.w > 200 && await brandHit(), JSON.stringify(s2));
  await p.screenshot({ path: SHOT(`r5_sidebar_expanded_${tag}.png`), clip: { x: 0, y: 0, width: 640, height: 220 } });
  // settings icon is lucide "settings"
  rec(`${tag}: sidebar 设置 uses lucide settings icon`, await p.locator(".app-sidebar").getByRole("button", { name: "设置" }).locator("svg.lucide-settings").count() === 1);
  await ctx.close();
}

// ---- web settings: grouped cards, no desktop-only bits
{
  const p = await br.newPage({ viewport: { width: 1280, height: 1000 } }); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(B + "/settings"); await p.getByTestId("ai-card").waitFor(); await sleep(600);
  const titles = await p.locator("[data-slot=card-title]").allInnerTexts();
  rec("web settings: cards 常规 / 通知 / 关键词 / 热点词表 / 翻译 / AI / 关于 (no 数据 on web) (in order)", titles.join(",") === "常规,通知,关键词,热点词表,翻译 / AI,关于", titles.join(","));
  rec("web: no 简报 Skill / 更新 cards", await p.getByTestId("skill-card").count() === 0 && await p.getByTestId("update-card").count() === 0);
  rec("web: skills API is desktop-only", (await fetch(B + "/api/skills")).status === 404);
  await p.screenshot({ path: SHOT("r5_settings_web.png"), fullPage: true });
  await p.close();
}

// ---- desktop: skills + notifications UI
execSync(`bash ${RESTART} AI_RADAR_DESKTOP=1 AI_RADAR_VERSION=0.5.0 ${dsEnv()} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 BRIEF_CACHE_MS=1`);
await sleep(600);
const ctx = await br.newContext({ viewport: { width: 1280, height: 1000 } });
await ctx.addInitScript(() => { window.__radarDesktop = true; });
const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
await p.goto(B + "/settings"); await p.getByTestId("skill-card").waitFor({ timeout: 15000 }); await sleep(800);
const titles = await p.locator("[data-slot=card-title]").allInnerTexts();
rec("desktop settings: 常规 / 通知 / 关键词 / 热点词表 / 翻译 / AI / 简报·推特·公众号·小红书 Skill / 写作风格 / 多模态模型 / 抓取 API / 工作目录 / 数据 / 关于", titles.join(",") === "常规,通知,关键词,热点词表,翻译 / AI,简报 Skill,推特 Skill,公众号 Skill,小红书 Skill,写作风格,多模态模型,抓取 API,工作目录,数据,关于", titles.join(","));
rec("desktop: 更新 is a section of the 数据 card (version + menu hint)", await p.getByTestId("update-card").count() === 0 && /更新\s*v0\.5\.0 · 自动检查/.test(await p.getByTestId("data-card").getByTestId("update-section").innerText()), await p.getByTestId("data-card").innerText());
{ const sh = process.env.R11_SHOTS ?? "/tmp/shots-r11"; fs.mkdirSync(sh, { recursive: true }); await p.getByTestId("data-card").scrollIntoViewIfNeeded(); await p.getByTestId("data-card").screenshot({ path: path.join(sh, "settings-data-update.png") }); }
rec("desktop: no 发送方式 / AppleScript option (native only)", await p.getByTestId("notify-method").count() === 0 && !/AppleScript/.test(await p.getByTestId("notify-card").innerText()));
{ const r = (await fetchR(B + "/api/desktop/notify", { method: "PATCH", headers: J, body: JSON.stringify({ method: "applescript" }) })).status; rec("notify method API removed", r === 405 || r === 404, String(r)); }
rec("desktop: notification status + 发送测试通知 button", /不支持|尚未询问|已允许|已拒绝/.test(await p.getByTestId("notify-perm").innerText()) && await p.getByTestId("notify-test").count() === 1);
rec("desktop: data dir shown, no import/export", (await p.getByTestId("data-card").getByRole("button").count()) === 1 && /打开数据目录/.test(await p.getByTestId("data-card").innerText()));
rec("default skill active", (await p.getByTestId("skill-row").count()) === 1 && /brief-polish/.test(await p.getByTestId("skill-list").innerText()) && await p.getByTestId("skill-active").count() === 1);

// install via URL (https only, validated)
rec("install: http refused", (await fetchR(B + "/api/skills/install", { method: "POST", headers: J, body: JSON.stringify({ url: "http://skills.mock-ai.com/a/SKILL.md" }) })).status === 400);
rec("install: HTML page refused", (await fetchR(B + "/api/skills/install", { method: "POST", headers: J, body: JSON.stringify({ url: "https://skills.mock-ai.com/a/page" }) })).status === 422);
await p.getByTestId("skill-url").fill("https://skills.mock-ai.com/editors/SKILL.md");
await p.getByTestId("skill-install").click(); await p.getByTestId("skill-list").getByText("e2e-editors").waitFor({ timeout: 10000 });
rec("install from URL → listed", (await p.getByTestId("skill-row").count()) === 2);
// upload a local SKILL.md
const f = path.join(os.tmpdir(), "SKILL.md"); fs.writeFileSync(f, "# 无 frontmatter\n");
rec("upload SKILL.md without frontmatter → rejected", (await fetchR(B + "/api/skills/upload", { method: "POST", body: (() => { const fd = new FormData(); fd.append("file", new Blob([fs.readFileSync(f)]), "SKILL.md"); return fd; })() })).status >= 400);
// upload a local multi-file skill folder
const dir = path.join(os.tmpdir(), "local-skill"); fs.rmSync(dir, { recursive: true, force: true });
fs.mkdirSync(path.join(dir, "references"), { recursive: true }); fs.mkdirSync(path.join(dir, "scripts"));
fs.writeFileSync(path.join(dir, "SKILL.md"), "---\nname: local-skill\ndescription: folder upload\n---\nLOCAL-UPLOAD-MARKER 参见 references/rules.md\n");
fs.writeFileSync(path.join(dir, "references", "rules.md"), "RULES\n"); fs.writeFileSync(path.join(dir, "scripts", "x.sh"), "echo hi\n");
await p.getByTestId("skill-folder").setInputFiles(dir); await p.getByTestId("skill-list").getByText("local-skill").waitFor({ timeout: 10000 });
rec("upload local folder → listed with 参考/脚本 counts", (await p.getByTestId("skill-row").count()) === 3 && /1 个参考 · 1 个脚本/.test(await p.getByTestId("skill-row").filter({ hasText: "local-skill" }).innerText()), await p.getByTestId("skill-row").filter({ hasText: "local-skill" }).innerText());
// activate the URL one, preview it
await p.getByRole("button", { name: "使用 e2e-editors" }).click(); await sleep(600);
rec("activate → 使用中", (await p.getByTestId("skill-row").filter({ hasText: "e2e-editors" }).getByTestId("skill-active").count()) === 1);
await p.getByRole("button", { name: "预览 e2e-editors" }).click(); await p.getByTestId("skill-preview").waitFor();
rec("preview shows SKILL.md", /SKILL-E2E-MARKER-42/.test(await p.getByTestId("skill-preview").innerText()));
await p.keyboard.press("Escape"); await sleep(300);
await p.getByTestId("skill-card").screenshot({ path: SHOT("r5_settings_skill.png") });
await p.screenshot({ path: SHOT("r5_settings_desktop.png"), fullPage: true });
// AI 润色 must carry the active skill in the system prompt (mock echoes it)
const ai = await (await fetchR(B + "/api/brief?section=all&ai=1")).json();
rec("AI 润色 system prompt contains the active skill text", ai.ai === true && /SKILL-E2E-MARKER-42/.test(ai.markdown) && ai.skill === "e2e-editors", (ai.markdown ?? "").slice(0, 160));
// delete it → back to the built-in default
await p.getByRole("button", { name: "删除 e2e-editors" }).click(); await sleep(700);
rec("delete active → default active again", (await p.getByTestId("skill-row").count()) === 2 && /brief-polish/.test(await p.getByTestId("skill-row").filter({ has: p.getByTestId("skill-active") }).innerText()));
const ai2 = await (await fetchR(B + "/api/brief?section=all&ai=1")).json();
rec("pipeline: summaries → brief → verification; per-item fetch status", /【摘要】/.test(ai2.markdown) && /【已核对】/.test(ai2.markdown) && ai2.verified === true
  && Array.isArray(ai2.noFullText) && ai2.groups.flatMap((g) => g.items).every((x) => x.fetch && x.fetch.status), JSON.stringify(ai2.noFullText?.[0] ?? null));
rec("…and polishing uses the default skill", /SKILL: brief-polish .*始终用简体中文写/.test(ai2.markdown) && !/SKILL-E2E-MARKER-42/.test(ai2.markdown));
// the AI brief renders in the dialog
await p.goto(B + "/"); await p.locator("[data-slot=card]").first().waitFor({ timeout: 15000 }); await sleep(500);
await p.getByTestId("brief-btn").click(); await p.getByTestId("brief-body").waitFor({ timeout: 15000 });
await p.getByTestId("brief-ai").click(); await p.getByTestId("brief-rule").waitFor({ timeout: 15000 });
rec("dialog: AI result rendered (skill named)", /AI 润色（brief-polish）/.test(await p.getByTestId("brief-dialog").innerText()));
{ const sh = process.env.R11_SHOTS ?? "/tmp/shots-r11"; await p.getByTestId("brief-nofull").locator("summary").click().catch(() => {}); await sleep(200); await p.getByTestId("brief-dialog").screenshot({ path: path.join(sh, "brief-dropped-items.png") }); }
rec("dialog: lists items without full text / left out", /未取到原文全文|未取到内容，未写入/.test(await p.getByTestId("brief-nofull").innerText().catch(() => "")));
await p.getByTestId("brief-nofull").locator("summary").click().catch(() => {}); await sleep(200);
await p.getByTestId("brief-dialog").screenshot({ path: SHOT("r7_brief_nofull.png") });
await p.getByTestId("brief-dialog").screenshot({ path: SHOT("r5_brief_ai.png") });
await ctx.close();
fs.rmSync(f, { force: true }); fs.rmSync(dir, { recursive: true, force: true });

await br.close();
execSync(`bash ${RESTART}`);
rec("no page errors (t19)", errors.length === 0, errors.slice(0, 2).join(" | "));
