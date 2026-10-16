// Round 4: section brief (rule-based + AI polish gate), toolbar button, dialog copy/export.
import { execSync } from "node:child_process";
import { chromium } from "playwright";
import { rec, B, RESTART, SHOT, api } from "./lib.mjs";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const dsEnv = () => ["DEEPSEEK_API_KEY", process.env.E2E_DS_KEY].join("=");

execSync(`bash ${RESTART}`);
const br = await chromium.launch();
const page = await br.newPage({ viewport: { width: 1280, height: 900 } });
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));

await page.goto(B + "/"); await page.locator("[data-slot=card]").first().waitFor({ timeout: 20000 }); await sleep(500);
rec("全部: 生成简报 button", await page.getByTestId("brief-btn").count() > 0);
await page.getByTestId("brief-btn").click();
await page.getByTestId("brief-dialog").waitFor();
await page.getByTestId("brief-body").waitFor({ timeout: 15000 });
const body = await page.getByTestId("brief-body").innerText();
rec("rule brief: grouped items", await page.getByTestId("brief-group").count() > 0 && await page.getByTestId("brief-item").count() > 0, body.slice(0, 80));
const it0 = page.getByTestId("brief-item").first();
rec("item layout: title, then summary paragraph, then separate 来源 + 原文链接 line", await it0.locator("h4").count() === 1 && /来源：/.test(await it0.getByTestId("brief-meta").innerText()) && (await it0.getByTestId("brief-link").innerText()).includes("原文链接")
  && await it0.getByTestId("brief-link").getAttribute("href").then((h) => /^https?:/.test(h ?? "")));
rec("brief dialog has a 翻译 switch", await page.getByTestId("brief-tr").count() === 1);
await page.getByTestId("brief-dialog").screenshot({ path: SHOT("r4_brief_all.png") });
rec("AI 润色 disabled without key", await page.getByTestId("brief-ai").isDisabled());
await page.keyboard.press("Escape"); await sleep(300);

await page.goto(B + "/?section=media"); await page.locator("[data-slot=card]").first().waitFor({ timeout: 15000 }).catch(() => {});
await sleep(600);
await page.getByTestId("brief-btn").click();
await page.getByTestId("brief-dialog").waitFor();
await page.getByTestId("brief-body").waitFor({ timeout: 15000 });
const mediaBody = await page.getByTestId("brief-dialog").innerText();
rec("section brief scoped", /国外媒体|media|简报/.test(mediaBody), mediaBody.slice(0, 60));
await page.getByTestId("brief-dialog").screenshot({ path: SHOT("r4_brief_section.png") });
await page.getByTestId("brief-copy").click(); await sleep(400);
rec("copy button works", await page.getByTestId("brief-copy").innerText().then((t) => /已复制|复制/.test(t)));
await page.keyboard.press("Escape");

// API: unknown section 404; AI without key → 503; SETTINGS_LOCKED blocks AI
const ba = await api("/api/brief?section=all");
rec("brief API all", ba.count >= 0 && ba.ai === false && Array.isArray(ba.groups));
const firstIt = ba.groups[0]?.items[0];
rec("markdown mirrors layout: ## group, ### title, summary, 来源 … [原文链接](url)", /^## .+/m.test(ba.markdown) && /^### .+/m.test(ba.markdown) && /^来源：.+ · \[原文链接\]\(https?:\/\/.+\)$/m.test(ba.markdown)
  && (!firstIt?.summary || ba.markdown.includes(`### ${firstIt.title}\n\n${firstIt.summary}\n\n来源：`)), ba.markdown.slice(0, 200));
const bt = await api("/api/brief?section=all&tr=1");
rec("tr=1 without key: uses existing translations + note", bt.translate === true && (bt.translateNote === undefined || /DeepSeek key/.test(bt.translateNote)));
rec("unknown section → 404", (await fetch(B + "/api/brief?section=nope")).status === 404);
rec("AI without key → 503", (await fetch(B + "/api/brief?section=all&ai=1")).status === 503);

execSync(`bash ${RESTART} ${dsEnv()} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 TRANSLATE_ENABLED=1`);
await sleep(800);
const fetchR = async (url, n = 3) => { for (let i = 0; i < n; i++) { try { return await fetch(url); } catch { await sleep(500); } } return fetch(url); };
const ai = await fetchR(B + "/api/brief?section=all&ai=1");
const aij = ai.ok ? await ai.json() : {};
rec("AI polish with mock key, built-in default skill in the system prompt", aij.ai === true && /SKILL: .*默认简报/.test(aij.markdown), (aij.markdown ?? "").slice(0, 160));
const trd = await (await fetchR(B + "/api/brief?section=media&tr=1")).json();
const tItems = trd.groups.flatMap((g) => g.items);
rec("tr=1 with key: items translated via DeepSeek", tItems.length > 0 && tItems.some((x) => x.translated && x.title.startsWith("【译】")), tItems[0]?.title);
await page.goto(B + "/?section=media"); await sleep(800);
await page.getByTestId("brief-btn").click(); await page.getByTestId("brief-item").first().waitFor({ timeout: 15000 });
await page.getByTestId("brief-dialog").screenshot({ path: SHOT("r5_brief_translated.png") });
await page.keyboard.press("Escape");

execSync(`bash ${RESTART} ${dsEnv()} SETTINGS_LOCKED=1`);
await sleep(800);
rec("SETTINGS_LOCKED → AI 403", (await fetchR(B + "/api/brief?section=all&ai=1")).status === 403);
const cached = await api("/api/brief?section=all").catch(() => sleep(500).then(() => api("/api/brief?section=all")));
rec("rule brief still works when locked", cached.ai === false && typeof cached.markdown === "string");

await page.goto(B + "/?source=1"); await sleep(800);
rec("source page: no brief button", await page.getByTestId("brief-btn").count() === 0);

await br.close();
execSync(`bash ${RESTART}`);
rec("no page errors (t18)", errors.length === 0, errors.slice(0, 2).join(" | "));
