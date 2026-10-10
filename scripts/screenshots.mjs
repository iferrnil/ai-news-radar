// npm run screenshots  (server must be running; BASE_URL defaults to http://localhost:3000)
import { chromium } from "playwright";
import fs from "node:fs";
const BASE = process.env.BASE_URL ?? "http://localhost:3000";
const OUT = new URL("../docs/screenshots/", import.meta.url).pathname;
fs.mkdirSync(OUT, { recursive: true });
for (const f of fs.readdirSync(OUT)) if (f.endsWith(".png") && !f.includes("/")) fs.unlinkSync(OUT + f);
const b = await chromium.launch();
async function shot(name, path, { theme = "dark", mobile = false, act, w = 1280, h = 800 } = {}) {
  const ctx = await b.newContext({ viewport: mobile ? { width: 390, height: 844 } : { width: w, height: h }, deviceScaleFactor: mobile ? 2 : 1, locale: "zh-CN" });
  await ctx.addInitScript((t) => localStorage.setItem("theme", t), theme);
  const pg = await ctx.newPage();
  await pg.goto(BASE + path, { waitUntil: "networkidle" });
  await pg.waitForSelector(path.startsWith("/?") || path === "/" ? "[data-slot=card]" : path === "/docs" ? "section" : "tbody tr td");
  await pg.waitForTimeout(1200);
  if (act) await act(pg);
  await pg.waitForTimeout(900);
  await pg.screenshot({ path: OUT + name });
  await ctx.close();
}
await shot("feed_dark.png", "/");
await shot("feed_light.png", "/", { theme: "light" });
await shot("feed_1920.png", "/", { w: 1920, h: 1080 });
await shot("section_cn.png", "/?section=cn");
await shot("section_x.png", "/?section=x");
const hover = async (pg) => { const c = pg.locator("[data-slot=card]").nth(2); await c.hover(); await pg.waitForSelector("[data-slot=hover-card-content]"); await pg.waitForTimeout(900); };
await shot("hover_card.png", "/", { act: hover });
await shot("hover_card_light.png", "/", { theme: "light", act: hover });
await shot("feed_mobile.png", "/", { mobile: true });
await shot("mobile_nav.png", "/", { mobile: true, act: (pg) => pg.click("button[aria-label='菜单']") });
await shot("drawer.png", "/", { act: async (pg) => { await pg.keyboard.press("j"); await pg.keyboard.press("j"); await pg.keyboard.press("Enter"); } });
await shot("command_palette.png", "/", { act: async (pg) => { await pg.keyboard.press("Control+k"); await pg.waitForTimeout(400); await pg.keyboard.type("Claude"); await pg.waitForTimeout(800); } });
await shot("sources_dark.png", "/sources");
await shot("sources_light.png", "/sources", { theme: "light" });
await shot("docs_page.png", "/docs/rate-limits", { act: async () => {} });
await shot("add_source.png", "/sources", { act: async (pg) => {
  await pg.click("button:has-text('添加数据源')");
  await pg.fill("#src-input", "https://simonwillison.net");
  await pg.click("[role=dialog] button:has-text('测试')");
  await pg.waitForSelector("[role=dialog] ul li", { timeout: 60000 });
} });
await b.close();
console.log("screenshots ->", OUT);
