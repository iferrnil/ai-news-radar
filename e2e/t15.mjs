// P0 notifications: no start-up backlog, batching, old items skipped, per-section filter, web Notification (background)
// vs toast (foreground), click opens the item, server prefs for the desktop, locked mode, desktop flag.
import { execSync } from "node:child_process";
import { chromium } from "playwright";
import { rec, B, mock, RESTART, SHOT, srcs , settingsOpen} from "./lib.mjs";
const J = { "Content-Type": "application/json" };
const byName = async (n) => (await srcs()).find((s) => s.name.startsWith(n));
const poll = async (n) => (await fetch(`${B}/api/sources/${(await byName(n)).id}/fetch`, { method: "POST", headers: J })).json();
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
// the scheduler would make the first poll itself (racing the backlog check): only manual polls in this test
execSync(`bash ${RESTART} DISABLE_POLLER=1`);

const br = await chromium.launch();
const ctx = await br.newContext({ viewport: { width: 1280, height: 860 } });
// fake Notification API (records instances) + a switch for "tab in background"
await ctx.addInitScript(() => {
  window.__notes = []; window.__bg = false;
  class N { constructor(t, o) { this.title = t; this.body = o?.body; this.tag = o?.tag; window.__notes.push(this); } close() {} }
  N.permission = "granted"; N.requestPermission = async () => "granted";
  window.Notification = N;
  const hf = document.hasFocus.bind(document); document.hasFocus = () => (window.__bg ? false : hf());
});
const page = await ctx.newPage(); const errors = [];
page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", (e) => errors.push(String(e)));
const notes = () => page.evaluate(() => window.__notes.map((n) => ({ title: n.title, body: n.body, tag: n.tag })));

// opt in (web prefs live in this browser)
await settingsOpen(page, "notify"); await page.getByTestId("notify-card").waitFor();
rec("web: notifications off by default", !(await page.getByRole("switch", { name: "P0 通知" }).isChecked()) && (await page.getByTestId("notify-perm").innerText()).includes("已允许"));
await page.getByRole("switch", { name: "P0 通知" }).click(); await page.waitForTimeout(300);
rec("opt-in stored locally", (await page.evaluate(() => JSON.parse(localStorage.getItem("notify.web")))).enabled === true);
await page.screenshot({ path: SHOT("settings_notify.png") });
await page.goto(B + "/"); await page.waitForSelector("[data-slot=card]"); await page.waitForTimeout(1200);
await page.evaluate(() => { window.__bg = true; });

// start-up backlog: the first poll after (re)start never notifies
await mock("add=OpenAI&title=backlog%20post");
await poll("X @OpenAI"); await sleep(2500);
rec("first poll after start-up is backlog: no notification", (await notes()).length === 0);
// a new P0 item afterwards → one notification (batched after NOTIFY_BATCH_MS)
await mock("add=OpenAI&title=GPT-next%20is%20here");
await poll("X @OpenAI"); await sleep(2500);
let n = await notes();
rec("new P0 item → system notification (tab in background)", n.length === 1 && n[0].title.startsWith("P0 · ") && n[0].body.includes("GPT-next") && n[0].tag === "ai-radar-p0", JSON.stringify(n));
// batching: two sources within the window → one notification
await poll("X @AnthropicAI"); await poll("X @deepseek_ai"); // first polls (backlog)
await mock("add=AnthropicAI&title=Claude%20news"); await mock("add=deepseek_ai&title=DeepSeek%20news");
await Promise.all([poll("X @AnthropicAI"), poll("X @deepseek_ai")]); await sleep(2500);
n = await notes();
rec("two new P0 items within the batch window → one notification '2 条新 P0 动态'", n.length === 2 && n[1].title === "2 条新 P0 动态" && /Claude news/.test(n[1].body) && /DeepSeek news/.test(n[1].body), n[1]?.title);
// old item (published 5 h ago) → skipped
await mock("add=OpenAI&age=18000&title=old%20news"); await poll("X @OpenAI"); await sleep(2200);
rec("items older than 3 h are not notified", (await notes()).length === 2);
// non-P0 source → nothing
await poll("网易娱乐"); await sleep(300);
// per-section filter: only 实验室 → an X post is filtered out
await page.evaluate(() => localStorage.setItem("notify.web", JSON.stringify({ enabled: true, sections: ["lab"] })));
await mock("add=OpenAI&title=filtered"); await poll("X @OpenAI"); await sleep(2200);
rec("per-section filter: X post not notified when only 实验室 is chosen", (await notes()).length === 2);
await page.evaluate(() => localStorage.setItem("notify.web", JSON.stringify({ enabled: true, sections: [] })));
// click → opens the item
await mock("add=OpenAI&title=Click%20me"); await poll("X @OpenAI"); await sleep(2200);
n = await notes();
await page.evaluate(() => window.__notes.at(-1).onclick());
await page.getByRole("dialog").waitFor({ timeout: 5000 }).catch(() => {});
const dlg = await page.getByRole("dialog").innerText().catch(() => "");
rec("clicking the notification opens that item", n.length === 3 && /Click me/.test(dlg) && !page.url().includes("item="), dlg.slice(0, 80));
await page.keyboard.press("Escape");
// foreground → toast instead of a system notification
await page.evaluate(() => { window.__bg = false; window.focus(); });
await page.bringToFront();
await mock("add=OpenAI&title=Foreground%20news"); await poll("X @OpenAI");
const toast = page.locator("[data-sonner-toast]").filter({ hasText: "P0 · " });
await toast.first().waitFor({ timeout: 6000 }).catch(() => {});
rec("tab in front → toast with 查看 (no system notification)", await toast.count() >= 1 && await toast.first().getByRole("button", { name: "查看" }).count() === 1 && (await notes()).length === 3);
await page.screenshot({ path: SHOT("notify_toast.png") });
// 发送测试通知
await page.evaluate(() => { window.__bg = true; });
const tr = await fetch(B + "/api/settings/notify-test", { method: "POST" }); await sleep(800);
rec("发送测试通知 delivers a batch", tr.ok && (await notes()).length === 4);
// off → nothing
await page.evaluate(() => localStorage.setItem("notify.web", JSON.stringify({ enabled: false, sections: [] })));
await mock("add=OpenAI&title=muted"); await poll("X @OpenAI"); await sleep(2200);
rec("disabled → no notification", (await notes()).length === 4);

// server prefs (used by the desktop shell)
const s0 = await (await fetch(B + "/api/settings")).json();
const pr = await (await fetch(B + "/api/settings", { method: "PATCH", headers: J, body: JSON.stringify({ notify: { enabled: true, sections: ["x", "lab"] } }) })).json();
rec("server notify prefs: default off on the web, PATCH saves", s0.notify.enabled === false && s0.desktop === false && pr.notify.enabled === true && pr.notify.sections.join() === "x,lab");
await ctx.close();

// locked: prefs + test notification refused
execSync(`bash ${RESTART} SETTINGS_LOCKED=1`);
const lp = await fetch(B + "/api/settings", { method: "PATCH", headers: J, body: JSON.stringify({ notify: { enabled: false } }) });
const lt = await fetch(B + "/api/settings/notify-test", { method: "POST" });
rec("locked: notify prefs + test notification → 403", lp.status === 403 && lt.status === 403);

// desktop flag: settings page shows the desktop variant, the page never shows web notifications itself
execSync(`bash ${RESTART} AI_RADAR_DESKTOP=1 AI_RADAR_VERSION=9.9.9`);
const c2 = await br.newContext(); await c2.addInitScript(() => { window.__notes = []; class N { constructor(t) { window.__notes.push(t); } close() {} } N.permission = "granted"; window.Notification = N; document.hasFocus = () => false; localStorage.setItem("notify.web", JSON.stringify({ enabled: true, sections: [] })); });
const p2 = await c2.newPage();
await settingsOpen(p2, "notify"); await p2.getByTestId("notify-card").waitFor(); await p2.waitForTimeout(800);
const card = await p2.getByTestId("notify-card").innerText();
rec("desktop: settings show native-notification text + permission status", /系统通知/.test(card) && /不支持/.test(await p2.getByTestId("notify-perm").innerText()), card.slice(0, 120));
await p2.getByTestId("settings-nav-data").click(); await p2.getByTestId("data-card").waitFor(); await p2.waitForTimeout(400);
rec("desktop: 数据 shows version and 检查更新", await p2.getByText("v9.9.9").count() >= 1 && await p2.getByText(/检查更新/).count() >= 1, await p2.getByTestId("data-card").innerText());
const ds = await (await fetch(B + "/api/settings")).json();
rec("desktop: prefs on the server (default on)", ds.desktop === true && ds.notify.enabled === true);
await p2.goto(B + "/"); await p2.waitForTimeout(2500);
await fetch(B + "/api/settings/notify-test", { method: "POST" }); await sleep(800);
rec("desktop: the page leaves notifications to the shell", (await p2.evaluate(() => window.__notes.length)) === 0);
rec("no console errors (t15)", errors.length === 0, errors.join("|").slice(0, 200));
await br.close();
execSync(`bash ${RESTART}`);
