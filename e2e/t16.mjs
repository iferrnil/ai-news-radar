// Sidebar collapse: header toggle, ⌘B / Ctrl+B, persistence across reloads (no flash), feed grid reflow, tooltips,
// mobile unchanged, desktop-only ⌘F / ⌘, (window.__radarDesktop) and the desktop menu hook window.__radarCmd.
import { chromium } from "playwright";
import { rec, B, SHOT } from "./lib.mjs";

const br = await chromium.launch();
// 1200px: 2 columns next to the sidebar, 3 without it
const ctx = await br.newContext({ viewport: { width: 1200, height: 860 } });
const page = await ctx.newPage(); const errors = [];
page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", (e) => errors.push(String(e)));
const aside = page.locator("#app-sidebar");
const width = () => aside.evaluate((e) => e.getBoundingClientRect().width);
const cols = () => page.locator("main [data-slot=card]").evaluateAll((cs) => new Set(cs.slice(0, 12).map((c) => Math.round(c.getBoundingClientRect().left))).size);
const settle = () => page.waitForTimeout(450);
// wait until the sidebar width settles at w (slow CI runners)
const widthIs = (w) => page.waitForFunction((w) => document.getElementById("app-sidebar").getBoundingClientRect().width === w, w, { timeout: 5000 }).then(() => true, () => false);

await page.goto(B + "/"); await page.waitForSelector("main [data-slot=card]"); await settle();
const toggle = page.getByTestId("sidebar-toggle");
rec("toggle visible, expanded by default", (await toggle.isVisible()) && (await toggle.getAttribute("data-state")) === "expanded" && (await width()) === 244);
const colsOpen = await cols();
await toggle.hover(); await page.getByText("隐藏侧边栏").last().waitFor({ timeout: 3000 }).catch(() => {});
rec("tooltip names the action + shortcut", /隐藏侧边栏\s*Ctrl\+B/.test(await page.locator("[data-slot=tooltip-content]").last().innerText().catch(() => "")));
await page.screenshot({ path: SHOT("sidebar_expanded.png") });

// sample every frame while it animates: the panel slides (transform), the column width changes in ONE step
// (no per-frame re-layout of the feed grid — that was the stutter)
const frames = await page.evaluate(() => new Promise((res) => {
  const a = document.getElementById("app-sidebar"), p = a.firstElementChild, out = [], t0 = performance.now();
  document.querySelector("[data-testid=sidebar-toggle]").click();
  const f = () => { out.push({ w: a.getBoundingClientRect().width, x: new DOMMatrix(getComputedStyle(p).transform).m41 }); if (performance.now() - t0 < 450) requestAnimationFrame(f); else res(out); }; requestAnimationFrame(f);
}));
const midX = frames.find((f) => f.x < 0 && f.x > -244)?.x ?? 0; await settle();
const widths = [...new Set(frames.map((f) => Math.round(f.w)))];
rec("click collapses with a slide transition", midX < 0 && (await width()) === 0, `frames=${frames.length} midX=${Math.round(midX)}`);
rec("column width changes in one step (no per-frame reflow)", widths.length <= 2 && widths.every((w) => w === 0 || w === 244), widths.join(","));
rec("collapsed: aria + inert + persisted", (await toggle.getAttribute("data-state")) === "collapsed" && (await toggle.getAttribute("aria-label")) === "显示侧边栏" && (await aside.getAttribute("inert")) !== null
  && (await page.evaluate(() => localStorage.getItem("sidebar.collapsed"))) === "1");
const colsClosed = await cols();
rec("feed grid reflows to more columns", colsOpen === 2 && colsClosed === 3, `${colsOpen}→${colsClosed}`);
await page.mouse.move(700, 500); await page.waitForTimeout(300);
await page.screenshot({ path: SHOT("sidebar_collapsed.png") });

// reload: collapsed before first paint (inline script), no animation from open
await page.reload(); await page.waitForSelector("main [data-slot=card]");
const first = await page.evaluate(() => document.documentElement.dataset.sidebar);
rec("persisted across reload, no flash", first === "collapsed" && (await width()) === 0);

await page.waitForFunction(() => typeof window.__radarCmd === "function");
await page.keyboard.press("Control+b");
rec("Ctrl+B expands", (await widthIs(244)) && (await page.evaluate(() => localStorage.getItem("sidebar.collapsed"))) === "0");
await page.waitForTimeout(300); await page.keyboard.press("Meta+b");
rec("⌘B collapses", await widthIs(0));
// one ⌘B reaching both the menu accelerator and the page counts once
await page.waitForTimeout(300);
await page.evaluate(() => { window.__radarCmd("sidebar"); document.body.dispatchEvent(new KeyboardEvent("keydown", { key: "b", ctrlKey: true, bubbles: true })); });
rec("menu + key for one press toggles once", (await widthIs(244)) && (await page.waitForTimeout(400), (await width()) === 244));
// ⌘B in an input still toggles (no contenteditable), ⌘F stays native in the browser
await page.keyboard.press("Control+f"); await page.waitForTimeout(300);
rec("browser: Ctrl+F not intercepted", !(await page.getByRole("dialog").isVisible().catch(() => false)));

// sidebar navigation still works after a round trip
await page.getByRole("button", { name: /^X 快讯/ }).first().click(); await page.waitForURL(/section=x/);
rec("sidebar nav works after toggling", true);

// desktop app flag: ⌘F opens search, ⌘, opens 设置
await page.evaluate(() => { window.__radarDesktop = true; });
await page.locator("main").click({ position: { x: 5, y: 5 } }); await page.keyboard.press("Control+f");
rec("desktop: Ctrl+F opens search", await page.locator("[cmdk-input]").waitFor({ timeout: 5000 }).then(() => true, () => false));
await page.keyboard.press("Escape"); await page.waitForTimeout(300);
await page.keyboard.press("Control+,"); await page.waitForURL(/\/settings/, { timeout: 5000 }).catch(() => {});
rec("desktop: Ctrl+, opens 设置", page.url().endsWith("/settings"));
await page.evaluate(() => window.__radarCmd("search"));
rec("desktop menu hook: search", await page.locator("[cmdk-input]").waitFor({ timeout: 5000 }).then(() => true, () => false));
await page.keyboard.press("Escape");

// other routes keep the state
await page.goto(B + "/sources"); await page.getByRole("heading", { name: "数据源" }).waitFor();
rec("state shared across routes", (await width()) === 244);

// mobile: no toggle, sheet nav unchanged, a stored collapsed state does not matter
const m = await br.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
const mp = await m.newPage();
await mp.addInitScript(() => localStorage.setItem("sidebar.collapsed", "1"));
await mp.goto(B + "/"); await mp.waitForSelector("main [data-slot=card]");
rec("mobile: toggle hidden, menu button present", !(await mp.getByTestId("sidebar-toggle").isVisible()) && (await mp.getByRole("button", { name: "菜单" }).isVisible()));
const mcols = await mp.locator("main [data-slot=card]").evaluateAll((cs) => new Set(cs.slice(0, 6).map((c) => Math.round(c.getBoundingClientRect().left))).size);
rec("mobile: single column", mcols === 1);
await mp.getByRole("button", { name: "菜单" }).tap();
rec("mobile: nav sheet opens", await mp.getByRole("dialog").getByRole("button", { name: /^全部/ }).first().waitFor({ timeout: 8000 }).then(() => true, () => false));

rec("no console errors", errors.length === 0, errors.slice(0, 3).join(" | "));
await br.close();
