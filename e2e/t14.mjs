// Read / unread: unread dots, 只看未读 (snapshot while browsing), 全部已读 per view + undo, sidebar unread counts,
// persistence, hover-to-read option, server mirror (READ_SYNC / desktop).
import { execSync } from "node:child_process";
import { chromium } from "playwright";
import { rec, B, RESTART, SHOT } from "./lib.mjs";
const br = await chromium.launch();
const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage(); const errors = [];
page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", (e) => errors.push(String(e)));
await page.goto(B + "/"); await page.evaluate(() => localStorage.clear()); await page.reload();
await page.waitForSelector("[data-slot=card]"); await page.waitForTimeout(800);
const total = async () => Number(await page.getByTestId("unread-total").innerText());
const cards = page.locator("[data-item]");
const side = page.getByRole("complementary");
const allCount = async () => (await side.getByRole("button", { name: /^全部/ }).first().innerText()).match(/\d+/)?.[0];

const n0 = await total();
rec("all unread at first: dots on every rendered card", n0 > 10 && await page.getByTestId("unread-dot").count() === await cards.count(), `${n0}`);
rec("sidebar 全部 shows the unread count (brand)", Number(await allCount()) === n0 && await side.getByTestId("unread-count").count() >= 2, `${await allCount()} vs ${n0}`);

// open → read
const first = cards.first(); const id1 = await first.getAttribute("data-item");
await first.click(); await page.getByRole("dialog").waitFor(); await page.keyboard.press("Escape"); await page.waitForTimeout(400);
rec("opening a card marks it read (dot gone, counts −1)", (await page.locator(`[data-item="${id1}"]`).getAttribute("data-read")) === "true" && await total() === n0 - 1 && Number(await allCount()) === n0 - 1);

// 只看未读
await page.getByRole("button", { name: "只看未读" }).click(); await page.waitForTimeout(500);
rec("只看未读: URL unread=1, read card hidden, list = unread", page.url().includes("unread=1") && await page.locator(`[data-item="${id1}"]`).count() === 0 && await cards.count() === Math.min(n0 - 1, 60));
const id2 = await cards.first().getAttribute("data-item");
await cards.first().click(); await page.getByRole("dialog").waitFor(); await page.keyboard.press("Escape"); await page.waitForTimeout(400);
rec("card read while browsing 只看未读 stays visible (snapshot)", await page.locator(`[data-item="${id2}"]`).count() === 1 && (await page.locator(`[data-item="${id2}"]`).getAttribute("data-read")) === "true");
await side.getByRole("button", { name: /^X 快讯/ }).click(); await page.waitForTimeout(400);
await side.getByRole("button", { name: /^全部/ }).first().click(); await page.waitForTimeout(500);
rec("view change refreshes the snapshot; 只看未读 kept across views", page.url().includes("unread=1") && await page.locator(`[data-item="${id2}"]`).count() === 0);

// 全部已读 per section + undo
await side.getByRole("button", { name: /^实验室/ }).click(); await page.waitForTimeout(500);
const labU = await total(); const allBefore = Number(await allCount());
await page.getByRole("button", { name: "全部标为已读" }).click(); await page.waitForTimeout(500);
rec("全部已读 (实验室): section unread 0, 全部 decreased by the same", await total() === 0 && Number(await allCount()) === allBefore - labU && await page.getByText("全部读完了").count() === 1, `${labU}`);
await page.screenshot({ path: SHOT("all_read.png") });
await page.getByRole("button", { name: "撤销" }).click(); await page.waitForTimeout(500);
rec("undo restores the section's unread items", await total() === labU && await cards.count() === Math.min(labU, 60));

// persistence
await page.reload(); await page.waitForSelector("[data-slot=card]"); await page.waitForTimeout(800);
rec("read state + 只看未读 persist across reload", await page.locator(`[data-item="${id1}"]`).count() === 0 && page.url().includes("unread=1"));
await page.getByRole("button", { name: "只看未读" }).click(); await page.waitForTimeout(400);
await page.goto(B + "/"); await page.waitForTimeout(800);
rec("turning 只看未读 off is remembered", !page.url().includes("unread=1"));

// hover option
await page.goto(B + "/settings"); await page.getByRole("switch", { name: "悬停预览时标为已读" }).click();
await page.goto(B + "/"); await page.waitForSelector("[data-slot=card]"); await page.waitForTimeout(600);
const unreadCard = page.locator('[data-item][data-read="false"]').nth(2); const id3 = await unreadCard.getAttribute("data-item");
await unreadCard.hover(); await page.waitForTimeout(1200); await page.mouse.move(5, 5); await page.waitForTimeout(300);
rec("悬停预览时标为已读: hover preview marks read", (await page.locator(`[data-item="${id3}"]`).getAttribute("data-read")) === "true");

// mark-all perf on the whole feed
await page.goto(B + "/"); await page.waitForTimeout(800);
const t0 = await page.evaluate(() => performance.now());
await page.getByRole("button", { name: "全部标为已读" }).click(); await page.getByTestId("unread-total").filter({ hasText: /^0$/ }).waitFor();
const ms = (await page.evaluate(() => performance.now())) - t0;
rec("全部已读 on the whole feed is fast (< 600 ms)", ms < 600, `${Math.round(ms)}ms`);

// server mirror: off on the web …
const g0 = await (await fetch(B + "/api/read")).json();
const p0 = await fetch(B + "/api/read", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ add: [1] }) });
rec("web: /api/read sync off, POST 404", g0.sync === false && g0.desktop === false && p0.status === 404);
await ctx.close();
// … on with READ_SYNC=1: a fresh browser gets the read state from the server
execSync(`bash ${RESTART} READ_SYNC=1`);
const c1 = await br.newContext(); const p1 = await c1.newPage();
await p1.goto(B + "/"); await p1.waitForSelector("[data-slot=card]"); await p1.waitForTimeout(2200);
const idS = await p1.locator("[data-item]").nth(1).getAttribute("data-item");
await p1.locator(`[data-item="${idS}"]`).click(); await p1.getByRole("dialog").waitFor(); await p1.keyboard.press("Escape"); await p1.waitForTimeout(1500);
const g1 = await (await fetch(B + "/api/read")).json();
rec("READ_SYNC=1: marking read mirrors to the server", g1.sync === true && g1.ids.includes(Number(idS)), `${g1.ids?.length}`);
const c2 = await br.newContext(); const p2 = await c2.newPage();
await p2.goto(B + "/"); await p2.waitForSelector("[data-slot=card]"); await p2.waitForTimeout(2500);
rec("READ_SYNC=1: another browser (empty localStorage) shows it read", (await p2.locator(`[data-item="${idS}"]`).getAttribute("data-read")) === "true");
rec("no console errors (t14)", errors.length === 0, errors.join("|").slice(0, 200));
await br.close();
execSync(`bash ${RESTART}`);
