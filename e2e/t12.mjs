// in-app DeepSeek settings: masked key, never echoed, 0600 file, clear, locked mode
import fs from "node:fs"; import { execSync } from "node:child_process";
import { launch, rec, B, SHOT, RESTART, LOG, DATA } from "./lib.mjs";
const KEY = "sk-" + "e2efake" + Math.random().toString(36).slice(2, 14) + "zz";
const leak = (t) => t.includes(KEY) || t.includes(KEY.slice(3, 20));
const get = async (p) => (await fetch(B + p)).text();
const SF = DATA + "/secrets.json";
let { br, page, errors } = await launch(false);
await page.goto(B + "/settings"); await page.waitForTimeout(1200);
rec("settings page renders, unconfigured", (await page.getByTestId("key-status").textContent()) === "未配置");
const inp = page.locator("#ds-key"); await inp.fill(KEY);
rec("key input masked (type=password)", (await inp.getAttribute("type")) === "password");
await page.getByTestId("ai-card").getByRole("button", { name: "保存" }).click(); await page.waitForTimeout(1000);
rec("saved → 已配置, input cleared", (await page.getByTestId("key-status").textContent()) === "已配置" && (await inp.inputValue()) === "");
const html = await page.content(); rec("key not in DOM after save", !leak(html));
const s = JSON.parse(await get("/api/settings"));
rec("GET /api/settings: configured only", s.deepseek.configured === true && s.deepseek.source === "app" && !leak(JSON.stringify(s)));
const t = await (await fetch(B + "/api/settings/test", { method: "POST", headers: { "content-type": "application/json" }, body: "{}" })).text();
rec("test response has no key", !leak(t) && "ok" in JSON.parse(t), t.slice(0, 80));
let all = ""; for (const p of ["/api/sources", "/api/items?limit=50", "/api/health", "/api/sections", "/api/sources/deleted"]) all += await get(p);
rec("key absent from sources/items/health (JSON + OPML export source)", !leak(all));
// UI exports (JSON + OPML) from /sources
await page.goto(B + "/sources"); await page.waitForTimeout(1200);
let exp = "";
for (const n of [/JSON/, /OPML/]) { try { await page.getByRole("button", { name: /导出/ }).first().click(); const [d] = await Promise.all([page.waitForEvent("download", { timeout: 4000 }), page.getByRole("menuitem", { name: n }).click()]); exp += fs.readFileSync(await d.path(), "utf8"); } catch { await page.keyboard.press("Escape"); } }
rec("exports downloaded and contain no key", exp.length > 100 && !leak(exp), `${exp.length}B`);
const st = fs.statSync(SF); rec("secrets.json mode 0600", (st.mode & 0o777) === 0o600, (st.mode & 0o777).toString(8));
rec("server log has no key", !leak(fs.readFileSync(LOG, "utf8")));
// persisted across restart, no env key
await br.close(); execSync("bash " + RESTART + "");
rec("persisted after restart", JSON.parse(await get("/api/settings")).deepseek.configured === true);
({ br, page, errors } = await launch(false));
await page.goto(B + "/settings"); await page.waitForTimeout(1200);
await page.getByRole("button", { name: "清除" }).click(); await page.waitForTimeout(1000);
rec("clear → 未配置, key removed from file", (await page.getByTestId("key-status").textContent()) === "未配置" && !leak(fs.readFileSync(SF, "utf8")));
// locked mode
await br.close(); execSync("bash " + RESTART + " SETTINGS_LOCKED=1");
const r = await fetch(B + "/api/settings", { method: "PATCH", headers: { "content-type": "application/json" }, body: JSON.stringify({ deepseek: { key: KEY } }) });
rec("locked: PATCH key → 403", r.status === 403 && JSON.parse(await get("/api/settings")).deepseek.configured === false && JSON.parse(await get("/api/settings")).deepseek.locked === true);
const rt = await fetch(B + "/api/settings/test", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ key: KEY }) });
rec("locked: test with typed key refused", rt.status === 403, `${rt.status}`);
({ br, page, errors } = await launch(false));
await page.goto(B + "/settings"); await page.waitForTimeout(1200);
rec("locked: UI shows notice, inputs/save disabled", await page.getByText("密钥由部署方配置").count() === 1 && await page.locator("#ds-key").isDisabled() && await page.getByTestId("ai-card").getByRole("button", { name: "保存" }).isDisabled());
await page.screenshot({ path: SHOT("settings_locked.png") });
rec("no console errors (t12)", errors.filter((e) => !/status of (400|401|403)/.test(e)).length === 0, errors.join("|").slice(0, 200));
await br.close(); execSync("bash " + RESTART + "");
