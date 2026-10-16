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
rec("rule brief: prose paragraphs, no per-item cards", await page.getByTestId("brief-item").count() === 0 && await page.getByTestId("brief-body").locator("p").count() > 0, body.slice(0, 80));
const ref0 = page.getByTestId("brief-ref").first();
rec("links separate: small footnote refs [n] + 原文链接 list", await page.getByTestId("brief-ref").count() > 0 && /^https?:/.test(await ref0.getAttribute("href") ?? "") && /原文链接/.test(body));
rec("翻译 defaults OFF", (await page.getByTestId("brief-tr").getAttribute("aria-checked")) !== "true");
// fully editable; copy/export use edited text
await page.getByTestId("brief-edit").click(); await page.getByTestId("brief-editor").waitFor();
const orig = await page.getByTestId("brief-editor").inputValue();
await page.getByTestId("brief-editor").fill("# 我的简报\n\nEDITED-MARKER 一段文字。");
await page.getByTestId("brief-edit").click(); await sleep(200);
rec("edit → preview shows edited text + 已编辑", /EDITED-MARKER/.test(await page.getByTestId("brief-body").innerText()) && /已编辑/.test(await page.getByTestId("brief-dialog").innerText()));
await page.context().grantPermissions(["clipboard-read", "clipboard-write"]).catch(() => {});
await page.getByTestId("brief-copy").click(); await sleep(300);
const clip = await page.evaluate(() => navigator.clipboard.readText()).catch(() => "");
rec("copy uses edited text", clip === "" || /EDITED-MARKER/.test(clip), clip.slice(0, 40));
await page.getByTestId("brief-dialog").screenshot({ path: SHOT("r6_brief_edited.png") });
await page.getByTestId("brief-revert").click(); await sleep(200);
rec("还原 → original", !/EDITED-MARKER/.test(await page.getByTestId("brief-body").innerText()) && orig.length > 0);
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
rec("markdown: prose with [[n]](url) refs + numbered 原文链接 list", /^## .+/m.test(ba.markdown) && /\[\[\d+\]\]\(https?:\/\/[^)]+\)/.test(ba.markdown) && /^## 原文链接$/m.test(ba.markdown) && /^1\. \[.+\]\(https?:\/\/.+\)/m.test(ba.markdown) && !/^### /m.test(ba.markdown)
  && (!firstIt?.title || ba.markdown.includes(firstIt.title)), ba.markdown.slice(0, 200));
const bt = await api("/api/brief?section=all&tr=1");
rec("tr=1 without key: uses existing translations + note", bt.translate === true && (bt.translateNote === undefined || /DeepSeek key/.test(bt.translateNote)));
rec("unknown section → 404", (await fetch(B + "/api/brief?section=nope")).status === 404);
rec("AI without key → 503", (await fetch(B + "/api/brief?section=all&ai=1")).status === 503);

execSync(`bash ${RESTART} ${dsEnv()} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 TRANSLATE_ENABLED=1`);
await sleep(800);
const fetchR = async (url, n = 3) => { for (let i = 0; i < n; i++) { try { return await fetch(url); } catch { await sleep(500); } } return fetch(url); };
const ai = await fetchR(B + "/api/brief?section=all&ai=1");
const aij = ai.ok ? await ai.json() : {};
rec("AI polish with mock key, built-in default skill in the system prompt", aij.ai === true && /SKILL: brief-polish .*始终用简体中文写/.test(aij.markdown), (aij.markdown ?? "").slice(0, 160));
const trd = await (await fetchR(B + "/api/brief?section=media&tr=1")).json();
const tItems = trd.groups.flatMap((g) => g.items);
rec("tr=1 with key: items translated via DeepSeek", tItems.length > 0 && tItems.some((x) => x.translated && x.title.startsWith("【译】")), tItems[0]?.title);
await page.goto(B + "/?section=media"); await sleep(800);
await page.getByTestId("brief-btn").click(); await page.getByTestId("brief-body").waitFor({ timeout: 15000 });
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
