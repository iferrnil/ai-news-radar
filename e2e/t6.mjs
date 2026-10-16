import { chromium } from "playwright";
import { rec, B } from "./lib.mjs";
 const b = await chromium.launch(); const errs = [];
const page = async (vp, theme = "dark") => { const c = await b.newContext({ viewport: vp }); await c.addInitScript((t) => localStorage.setItem("theme", t), theme); const p = await c.newPage();
  p.on("console", (m) => m.type() === "error" && errs.push(`${p.url()} ${m.text()}`)); p.on("pageerror", (e) => errs.push(String(e))); return p; };
// overflow matrix
const bad = [];
for (const w of [320, 390, 768, 1024, 1280, 1920]) for (const path of ["/", "/?section=cn", "/?section=ent", "/sources", "/docs", "/docs/rate-limits"]) for (const theme of w === 1280 ? ["dark", "light"] : ["dark"]) {
  const p = await page({ width: w, height: 800 }, theme); await p.goto(B + path, { waitUntil: "load" }); await p.waitForTimeout(900);
  if (await p.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)) bad.push(`${w}${path}${theme}`); await p.context().close(); }
rec("no horizontal overflow 320–1920 × 6 pages (+light at 1280)", bad.length === 0, bad.join(","));
// section filters: each section view shows only its section; counts match sidebar
const p = await page({ width: 1280, height: 900 });
await p.goto(B + "/", { waitUntil: "load" }); await p.waitForSelector("[data-slot=card]"); await p.waitForTimeout(800);
const side = await p.getByRole("complementary").locator("button:not([aria-label^='文件夹'])").evaluateAll((bs) => Object.fromEntries(bs.map((b) => [b.innerText.split("\n")[0].trim(), b.innerText.split("\n").pop().trim()])));
const secMap = { "实验室": "lab", "国内媒体": "cn", "国外媒体": "media", "X 快讯": "x", "娱乐": "ent" };
let sum = 0; const notes = [];
for (const [zh, key] of Object.entries(secMap)) {
  await p.goto(B + "/?section=" + key, { waitUntil: "load" }); await p.waitForTimeout(900);
  const n = Number((await p.getByRole("button", { name: "刷新" }).innerText()).match(/\d+/)?.[0] ?? -1);
  const chips = await p.locator("[data-slot=card] [data-slot=badge]").allInnerTexts();
  const wrong = chips.filter((t) => Object.keys(secMap).includes(t) && t !== zh).length;
  notes.push(`${zh}:${n}/${side[zh]}${wrong ? " wrong=" + wrong : ""}`); if (key !== "ent") sum += n;
  if (n !== Number(side[zh]) || wrong) rec(`section ${zh} filter`, false, notes.at(-1));
}
const allN = Number(side["全部"]);
rec("section views match sidebar counts; 全部 = sum of non-娱乐 sections", notes.every((x) => !x.includes("wrong")) && allN === sum, notes.join(" ") + ` 全部=${allN} sum=${sum}`);
// translate menu lazy popover
await p.goto(B + "/", { waitUntil: "load" }); await p.waitForTimeout(600);
await p.getByTestId("tr-toggle").click(); await p.waitForTimeout(1500);
rec("translate menu loads on first click and opens", await p.getByText("翻译「全部」").count() > 0);
await p.keyboard.press("Escape"); await p.waitForTimeout(300);
// theme toggle
const before = await p.evaluate(() => document.documentElement.classList.contains("dark"));
await p.getByRole("button", { name: "切换主题" }).click(); await p.waitForTimeout(300);
rec("theme toggle switches dark/light", before !== await p.evaluate(() => document.documentElement.classList.contains("dark")));
// hover card
await p.locator("[data-slot=card]").nth(1).hover(); await p.waitForTimeout(1300);
rec("hover card appears", await p.locator("[data-testid=hover-preview]").count() > 0);
await p.mouse.move(5, 790);
// drawer via click (lazy)
await p.locator("[data-slot=card]").nth(0).click(); await p.waitForTimeout(1200);
rec("drawer opens on card click (lazy chunk)", await p.getByRole("button", { name: "复制链接" }).count() > 0);
await p.keyboard.press("Escape"); await p.waitForTimeout(400);
// mobile nav
const m = await page({ width: 390, height: 844 }); await m.goto(B + "/", { waitUntil: "load" }); await m.waitForTimeout(600);
await m.getByRole("button", { name: "菜单" }).click(); await m.waitForTimeout(1200);
const navOk = await m.getByRole("dialog").getByRole("button", { name: /^国内媒体/ }).count() > 0;
await m.getByRole("dialog").getByRole("button", { name: /^国内媒体/ }).click(); await m.waitForTimeout(800);
rec("mobile nav sheet opens (lazy) and navigates", navOk && m.url().includes("section=cn"), m.url());
// docs
await p.goto(B + "/docs", { waitUntil: "load" }); await p.waitForTimeout(500);
await p.getByRole("link", { name: /抓取限制/ }).first().click(); await p.waitForURL(/rate-limits/); await p.waitForTimeout(500);
rec("docs index → rate-limits page, table rendered", await p.locator("table tbody tr").count() > 5);
// API compression
const r = await fetch(B + "/api/items?limit=1000", { headers: { "Accept-Encoding": "br" } });
rec("API brotli + ETag", r.headers.get("content-encoding") === "br" && !!r.headers.get("etag"));
const r2 = await fetch(B + "/api/items?limit=1000", { headers: { "If-None-Match": r.headers.get("etag") } });
rec("API 304 on If-None-Match", r2.status === 304);
rec("no console errors (t6)", errs.length === 0, errs.slice(0, 3).join(" | "));
await b.close();
