// Shared helpers of the end-to-end suite (run with e2e/runall.sh; the server under test is started by e2e/restart.sh).
import { chromium } from "playwright";
import fs from "fs";
import os from "os";
import path from "path";
import { fileURLToPath } from "url";
export const E2E = path.dirname(fileURLToPath(import.meta.url));
export const ROOT = path.dirname(E2E);
export const RESTART = path.join(E2E, "restart.sh");
export const PORT = Number(process.env.PORT ?? 3000);
export const DATA = process.env.E2E_DATA ?? path.join(ROOT, "data");
export const B = process.env.E2E_BASE ?? `http://localhost:${PORT}`;
export const MOCK = process.env.UPSTREAM_MOCK ?? "http://127.0.0.1:4010";
export const LOG = process.env.E2E_LOG ?? path.join(os.tmpdir(), "radar-server.log");
const SHOTS = process.env.E2E_SHOTS ?? path.join(os.tmpdir(), "radar-e2e-shots");
fs.mkdirSync(SHOTS, { recursive: true });
export const SHOT = (name) => path.join(SHOTS, name);
/** control the mock upstream (see mock-upstream.mjs) */
export const mock = (q) => fetch(`${MOCK}/__ctl?${q}`).then((r) => r.json());
export const results = [];
export const rec = (t, ok, note = "") => { results.push({ t, ok, note }); console.log(ok ? "PASS" : "FAIL", t, note); };
export const api = (p, o) => fetch(B + p, o).then((r) => r.json());
export async function launch(mobile) {
  const br = await chromium.launch();
  const ctx = await br.newContext(mobile ? { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, acceptDownloads: true } : { viewport: { width: 1280, height: 800 }, acceptDownloads: true });
  const page = await ctx.newPage(); const errors = [];
  page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto(B + "/sources"); await page.getByRole("heading", { name: "数据源" }).waitFor();
  return { br, page, errors };
}
export async function exportFile(page, scopeLabel, fmt) {
  await page.getByRole("button", { name: "导出" }).click();
  await page.getByRole("menuitemradio", { name: new RegExp(scopeLabel) }).click();
  const [dl] = await Promise.all([page.waitForEvent("download"), page.getByRole("menuitem", { name: fmt === "json" ? /JSON/ : /OPML/ }).click()]);
  const p = path.join(os.tmpdir(), `dl-${Date.now()}-${dl.suggestedFilename()}`); await dl.saveAs(p);
  await page.keyboard.press("Escape").catch(() => {});
  return { name: dl.suggestedFilename(), text: fs.readFileSync(p, "utf8"), path: p };
}
export async function openImport(page, file) {
  await page.getByRole("button", { name: "导入", exact: true }).click();
  const dlg = page.getByRole("dialog", { name: "导入订阅" }); await dlg.waitFor();
  await page.locator('input[type=file]').setInputFiles(file);
  return dlg;
}
export async function waitPlan(dlg) { await dlg.locator("table tbody tr").first().waitFor({ timeout: 10000 }); await dlg.page().waitForTimeout(300); }
export async function counts(dlg) { return (await dlg.locator("tbody tr td:first-child").allInnerTexts()).reduce((a, k) => ((a[k] = (a[k] || 0) + 1), a), {}); }
export async function setMode(dlg, label) { await dlg.getByRole("radio", { name: new RegExp("^" + label) }).click(); await dlg.page().waitForTimeout(800); }
export async function applyImport(dlg) { await dlg.getByRole("button", { name: /^(导入|覆盖导入)/ }).click(); }
export const srcs = async () => (await api("/api/sources"));
export const deleted = () => api("/api/sources/deleted");

/** Open /settings and switch to a category (general|notify|model|writing|data|about). Only the active panel is mounted. */
export async function settingsOpen(page, cat = "general") {
  await page.goto(B + "/settings");
  await page.getByTestId("settings-page").waitFor({ timeout: 15000 });
  await page.evaluate((k) => { try { localStorage.setItem("settings.category", k); } catch {} }, cat);
  const nav = page.getByTestId(`settings-nav-${cat}`);
  if (await nav.count()) {
    await nav.click();
  } else {
    // narrow layout: top select
    const trigger = page.getByTestId("settings-cat-select");
    await trigger.waitFor({ timeout: 5000 });
    await trigger.click();
    await page.getByRole("option", { name: { general: "常规", notify: "通知与推送", model: "模型与抓取", writing: "写作与 Skill", data: "数据", about: "关于" }[cat] }).click();
  }
  await page.getByTestId("settings-panel").waitFor();
  await page.waitForTimeout(200);
}
