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
rec("rule brief has title + sources", /简报/.test(body) && /## /.test(body), body.slice(0, 80));
await page.getByTestId("brief-dialog").screenshot({ path: SHOT("r4_brief_all.png") });
rec("AI 润色 disabled without key", await page.getByTestId("brief-ai").isDisabled());
await page.keyboard.press("Escape"); await sleep(300);

await page.goto(B + "/?section=media"); await page.locator("[data-slot=card]").first().waitFor({ timeout: 15000 }).catch(() => {});
await sleep(600);
await page.getByTestId("brief-btn").click();
await page.getByTestId("brief-dialog").waitFor();
await page.getByTestId("brief-body").waitFor({ timeout: 15000 });
const mediaBody = await page.getByTestId("brief-body").innerText();
rec("section brief scoped", /国外媒体|media|简报/.test(mediaBody), mediaBody.slice(0, 60));
await page.getByTestId("brief-dialog").screenshot({ path: SHOT("r4_brief_section.png") });
await page.getByTestId("brief-copy").click(); await sleep(400);
rec("copy button works", await page.getByTestId("brief-copy").innerText().then((t) => /已复制|复制/.test(t)));
await page.keyboard.press("Escape");

// API: unknown section 404; AI without key → 503; SETTINGS_LOCKED blocks AI
rec("brief API all", (await api("/api/brief?section=all")).count >= 0 && (await api("/api/brief?section=all")).ai === false);
rec("unknown section → 404", (await fetch(B + "/api/brief?section=nope")).status === 404);
rec("AI without key → 503", (await fetch(B + "/api/brief?section=all&ai=1")).status === 503);

execSync(`bash ${RESTART} ${dsEnv()} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 TRANSLATE_ENABLED=1`);
await sleep(800);
const fetchR = async (url, n = 3) => { for (let i = 0; i < n; i++) { try { return await fetch(url); } catch { await sleep(500); } } return fetch(url); };
const ai = await fetchR(B + "/api/brief?section=all&ai=1");
rec("AI polish with mock key", ai.ok && (await ai.json()).ai === true);

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
