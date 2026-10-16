// Round 3: view-scoped translation (top-right toggle: 全部 / section / source; API, pipeline, UI hints, import/export, unavailable states).
// Round 2: desktop notification prefs (桌面系统通知 + 全部新内容), web icons/manifest, desktop titlebar inset of the sidebar brand.
// Screenshots go to $E2E_SHOTS (r2_*/r3_*.png).
import { execSync } from "node:child_process";
import { chromium } from "playwright";
import { rec, B, RESTART, SHOT, srcs, api } from "./lib.mjs";
const J = { "Content-Type": "application/json" };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const dsEnv = () => ["DEEPSEEK_API_KEY", process.env.E2E_DS_KEY].join("=");
const patch = (p, b) => fetch(B + p, { method: "PATCH", headers: J, body: JSON.stringify(b) });

// Translation is ONE top-bar control scoped to the current view (全部 / section / source); effective = any scope on.
// key + fake DeepSeek endpoint (mock), poller off (manual polls only). Default: every scope off.
execSync(`bash ${RESTART} DISABLE_POLLER=1 ${dsEnv()} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 TRANSLATE_ENABLED=1`);
// retry once: the first request after a restart may hit a pooled keep-alive socket of the old server
const tr = async () => (await api("/api/settings").catch(() => sleep(500).then(() => api("/api/settings")))).translate;
const setScope = (scope, key, on) => patch("/api/settings", { translate: { scope, key, on } });
let t0 = await tr();
rec("translation available; all scopes off by default", t0.available && t0.reason === null && !t0.scopes.all && !t0.scopes.sections.length && !t0.scopes.sources.length, JSON.stringify(t0.scopes));
const sections = await api("/api/sections");
rec("sections expose tr=0, sources tr=0 (old translate columns gone)", sections.every((s) => s.tr === 0 && !("translate" in s)) && (await srcs()).every((s) => s.tr === 0 && !("translate" in s)));
const all = await srcs();
const media = all.find((s) => s.section === "media" && s.enabled && s.type === "rss");
const lab = all.find((s) => s.section === "lab" && s.enabled && s.type === "rss");
rec("bad scope → 400, unknown section → 404", (await setScope("bogus", null, true)).status === 400 && (await setScope("section", "nope", true)).status === 404 && (await setScope("source", 999999, true)).status === 404);
const fresh = async (s) => { await fetch(`${B}/api/sources/${s.id}/fetch`, { method: "POST" }); };
const items = async (s) => (await api(`/api/items?source=${s.id}&limit=50&zh=1`));
const host = (s) => new URL(s.url).hostname.replace(/^www\./, "");
const bump = (s) => fetch(`${process.env.UPSTREAM_MOCK}/__ctl?add=${host(s)}&title=${encodeURIComponent("Fresh English headline about models " + s.id)}`);
await bump(media); await bump(lab); await fresh(media); await fresh(lab); await sleep(2500);
const trOf = (list) => list.filter((i) => (i.title_zh ?? "").startsWith("【译】")).length;
rec("nothing translated while all scopes are off", trOf(await items(media)) === 0 && trOf(await items(lab)) === 0);
rec("section scope on → that section's items translated (backfill)", (await setScope("section", "media", true)).ok && (await sleep(2500), trOf(await items(media)) > 0) && trOf(await items(lab)) === 0, `${trOf(await items(media))}`);
rec("source scope on → only that source", (await setScope("source", lab.id, true)).ok && (await sleep(2500), trOf(await items(lab)) > 0));
t0 = await tr();
rec("scopes stored", t0.scopes.sections.includes("media") && t0.scopes.sources.includes(lab.id) && !t0.scopes.all && (await srcs()).find((s) => s.id === lab.id).tr === 1);
await setScope("source", lab.id, false);

// UI: the top-right toggle follows the view
const br = await chromium.launch();
const page = await br.newPage({ viewport: { width: 1280, height: 900 } }); const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const toggle = () => page.getByTestId("tr-toggle");
const pressed = async () => (await toggle().getAttribute("aria-pressed")) === "true";
const view = async (q) => { await page.goto(B + "/" + q); await page.locator("[data-slot=card]").first().waitFor(); await sleep(700); };
const openMenu = async () => { await toggle().click(); await page.getByTestId("tr-menu").waitFor(); await sleep(300); };
const mediaName = sections.find((s) => s.key === "media").name;
await view("");
rec("全部: toggle off (only a section is on)", !(await pressed()));
await view("?section=media");
rec("section page: toggle on", await pressed());
rec("section page: cards show the translation", (await page.locator("[data-slot=card]").allInnerTexts()).some((t) => t.includes("【译】")));
await openMenu();
rec("section menu: switch named for the section, checked", await page.getByRole("switch", { name: `翻译「${mediaName}」分区` }).isChecked());
await page.getByTestId("tr-menu").screenshot({ path: SHOT("r3_menu_section.png") });
await page.screenshot({ path: SHOT("r3_section_view.png") });
await page.keyboard.press("Escape");
await view(`?source=${media.id}`);
rec("source page in an on section: toggle on", await pressed());
await openMenu();
const sw = page.getByRole("switch", { name: `翻译「${media.name}」` });
rec("…switch locked with hint 已由「…」分区开启", await sw.isChecked() && await sw.isDisabled() && (await page.getByTestId("tr-hint").innerText()).includes(`已由「${mediaName}」分区开启`));
rec("…offers turning the section off", await page.getByTestId("tr-off-section").isVisible());
await page.getByTestId("tr-menu").screenshot({ path: SHOT("r3_menu_inherited.png") });
await page.getByTestId("tr-off-section").click(); await sleep(1000);
rec("turning the broader scope off from the source page", !(await tr()).scopes.sections.includes("media") && !(await pressed()));
// own source scope via the switch
await sw.click(); await sleep(1000);
rec("source switch on → only this source", (await tr()).scopes.sources.includes(media.id) && await pressed());
await page.keyboard.press("Escape");
await view(`?source=${lab.id}`);
rec("another source stays off", !(await pressed()));
// 全部 on via the switch, then lab page shows 已由「全部」开启
await view("");
await openMenu(); await page.getByRole("switch", { name: "翻译「全部」" }).click(); await sleep(1000);
rec("全部 switch on → scopes.all", (await tr()).scopes.all && await pressed());
await page.getByTestId("tr-menu").screenshot({ path: SHOT("r3_menu_all.png") });
await page.keyboard.press("Escape");
await view(`?source=${lab.id}`);
await openMenu();
rec("source page under 全部: hint 已由「全部」开启 + off button", (await page.getByTestId("tr-hint").innerText()).includes("已由「全部」开启") && await page.getByTestId("tr-off-all").isVisible());
await page.getByTestId("tr-off-all").click(); await sleep(1000);
rec("关闭「全部」 from a source page", !(await tr()).scopes.all && !(await pressed()));
await page.keyboard.press("Escape");
// edit / section dialogs no longer carry translation settings
await page.goto(B + "/sources"); await page.getByRole("heading", { name: "数据源" }).waitFor();
await page.getByRole("button", { name: /^分区/ }).first().click(); await page.getByRole("dialog").waitFor(); await sleep(400);
rec("section dialog has no translate switch", await page.getByRole("dialog").getByRole("switch", { name: /^翻译「/ }).count() === 0);
await page.keyboard.press("Escape");
await page.goto(B + "/settings"); await page.getByTestId("settings-translate").waitFor();
await page.getByTestId("settings-translate").screenshot({ path: SHOT("r3_settings_translate.png") });
rec("settings: no separate translate switch", await page.getByRole("switch", { name: "抓取时翻译" }).count() === 0);

// export / import keep the scope flags
await setScope("all", null, true); await setScope("section", "media", true);
const { buildJson, buildOpml } = await import("../src/lib/portable-export.ts").catch(() => ({}));
const secs = (await api("/api/sections")).map((c) => ({ key: c.key, name: c.name, icon: c.icon, inAll: !!c.in_all, translate: c.tr === 1 }));
const json = JSON.parse(buildJson(await srcs(), new Date(), secs, true));
rec("export JSON: translateAll + section translate + source translate:on", json.translateAll === true && json.sections.find((c) => c.key === "media")?.translate === true
  && json.sources.find((e) => e.name === media.name)?.translate === "on" && !("translate" in json.sources.find((e) => e.name === lab.name)), "");
const opml = buildOpml(await srcs(), new Date(), secs, true);
rec("export OPML carries the flags", opml.includes("<radarTranslateAll>true</radarTranslateAll>") && opml.includes('sectionTranslate="true"') && opml.includes('translate="on"'));
await setScope("all", null, false); await setScope("source", media.id, false);
const entry = { ...json.sources.find((e) => e.name === lab.name), translate: "on" };
const imp = await (await fetch(B + "/api/sources/import", { method: "POST", headers: J, body: JSON.stringify({ entries: [entry], mode: "update", dryRun: false, translateAll: true }) })).json();
t0 = await tr();
rec("import (update) restores source + 全部 flags", imp.applied && t0.scopes.sources.includes(lab.id) && t0.scopes.all, JSON.stringify(imp).slice(0, 120));
await setScope("all", null, false); await setScope("source", lab.id, false); await setScope("section", "media", false);

// round 5: the switches are the user's; only a missing key blocks. TRANSLATE_ENABLED / SETTINGS_LOCKED no longer do.
execSync(`bash ${RESTART} DISABLE_POLLER=1 ${dsEnv()} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 TRANSLATE_ENABLED=0`);
t0 = await tr();
rec("TRANSLATE_ENABLED=0 no longer blocks (key set → available)", t0.available && !t0.reason);
await view(""); await openMenu();
rec("…toggle enabled", !(await page.getByRole("switch", { name: "翻译「全部」" }).isDisabled()));
await page.keyboard.press("Escape");
execSync(`bash ${RESTART} DISABLE_POLLER=1 TRANSLATE_ENABLED=1`);
t0 = await tr();
rec("no key → unavailable", !t0.available && /API key/.test(t0.reason ?? ""));
await view(""); await openMenu();
rec("…toggle disabled, reason shown", await page.getByRole("switch", { name: "翻译「全部」" }).isDisabled() && /无法翻译.*API key/.test(await page.getByTestId("tr-hint").innerText()));
await page.getByTestId("tr-menu").screenshot({ path: SHOT("r5_menu_nokey.png") });
await page.keyboard.press("Escape");
execSync(`bash ${RESTART} DISABLE_POLLER=1 ${dsEnv()} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 SETTINGS_LOCKED=1`);
const lk = await setScope("section", "media", true);
rec("SETTINGS_LOCKED: scope PATCH allowed (user control)", lk.status === 200);
await view("?section=media"); await openMenu();
rec("…toggle enabled + on", await page.getByRole("switch", { name: /^翻译「/ }).first().isChecked() && !(await page.getByRole("switch", { name: /^翻译「/ }).first().isDisabled()));
// progress counts for the current scope, live
let pg = null;
for (let i = 0; i < 40; i++) { pg = await api("/api/translate/progress?section=media"); if (pg.pending === 0 && pg.queued === 0 && pg.running === 0) break; await sleep(250); }
rec("progress: scope counts add up, queue drained", pg.total > 0 && pg.translated + pg.pending + pg.skipped === pg.total && pg.pending === 0 && pg.translated > 0, JSON.stringify(pg));
await sleep(1200);
const statsTxt = await page.getByTestId("tr-stats").innerText();
rec("popover shows 本页/已译/待翻译 + queue 排队/进行中/已完成/失败", /本页 \d+ 条/.test(statsTxt) && /待翻译 0/.test(statsTxt) && /排队/.test(statsTxt) && /进行中/.test(statsTxt) && /已完成/.test(statsTxt) && /失败/.test(statsTxt), statsTxt.replace(/\s+/g, " "));
rec("popover count matches API", (await page.getByTestId("tr-done-scope").innerText()) === String(pg.translated));
await page.getByTestId("tr-menu").screenshot({ path: SHOT("r5_menu_progress.png") });
await page.keyboard.press("Escape");
await setScope("section", "media", false);

// web icons + manifest (the desktop app icon)
const man = await (await fetch(B + "/manifest.webmanifest")).json().catch(() => null);
const icon = await fetch(B + "/icons/icon-512.png"); const apple = await page.goto(B + "/").then(() => page.locator('link[rel="apple-touch-icon"]').getAttribute("href"));
rec("manifest + icons served", !!man && man.icons?.length >= 2 && icon.ok && (await fetch(B + "/favicon.ico")).ok && !!apple, apple ?? "");

// desktop mode: titlebar inset (traffic lights ≈78 px) → brand logo starts ≥ 10 px after them
const dctx = await br.newContext({ viewport: { width: 1280, height: 820 } });
await dctx.addInitScript(() => { window.__radarDesktop = true; window.__radarTitlebar = true;
  document.addEventListener("DOMContentLoaded", () => document.documentElement.style.setProperty("--mygo-titlebar-inset-left", "78px")); });
const dp = await dctx.newPage();
await dp.goto(B + "/"); await dp.locator(".app-brand").waitFor(); await sleep(400);
const logoX = await dp.locator(".app-brand > div").first().evaluate((e) => e.getBoundingClientRect().left);
rec("desktop: brand logo clears the traffic lights (≥ 88 px)", logoX >= 88, `${Math.round(logoX)}px`);
// draw where the traffic lights sit, for the screenshot
await dp.evaluate(() => { const d = document.createElement("div"); d.style.cssText = "position:fixed;left:20px;top:20px;display:flex;gap:8px;z-index:99"; for (const c of ["#ff5f57", "#febc2e", "#28c840"]) { const b = document.createElement("span"); b.style.cssText = `width:12px;height:12px;border-radius:50%;background:${c}`; d.append(b); } document.body.append(d); });
await dp.screenshot({ path: SHOT("r2_desktop_header.png"), clip: { x: 0, y: 0, width: 520, height: 120 } });
const webCtx = await br.newPage({ viewport: { width: 1280, height: 820 } });
await webCtx.goto(B + "/"); await webCtx.locator(".app-brand").waitFor();
rec("web: brand padding unchanged (16 px)", Math.round(await webCtx.locator(".app-brand > div").first().evaluate((e) => e.getBoundingClientRect().left)) === 16);
await webCtx.screenshot({ path: SHOT("r2_web_header.png"), clip: { x: 0, y: 0, width: 520, height: 120 } });
await br.close();

// desktop notification prefs: system + scope (desktop flag on)
execSync(`bash ${RESTART} AI_RADAR_DESKTOP=1 AI_RADAR_VERSION=9.9.9`);
let st = await api("/api/settings");
rec("notify prefs default: system on, scope p0", st.notify.system === true && st.notify.scope === "p0");
await patch("/api/settings", { notify: { system: false, scope: "all" } });
st = await api("/api/settings");
rec("notify prefs persist system/scope", st.notify.system === false && st.notify.scope === "all");
await patch("/api/settings", { notify: { scope: "bogus" } });
rec("invalid scope ignored", (await api("/api/settings")).notify.scope === "all");
await patch("/api/settings", { notify: { system: true, enabled: true } });
const b2 = await chromium.launch(); const sp = await b2.newPage({ viewport: { width: 1100, height: 1300 } });
await sp.goto(B + "/settings"); await sp.getByTestId("notify-system").waitFor();
rec("settings: 桌面系统通知 switch + scope radio (desktop)", await sp.getByRole("switch", { name: "桌面系统通知" }).isChecked() && await sp.getByRole("radio", { name: "全部新内容" }).isChecked());
await sp.getByTestId("notify-card").screenshot({ path: SHOT("r2_settings_notify.png") });
// digest SSE event when scope=all
const got = await sp.evaluate((b) => new Promise((res) => { const es = new EventSource(b + "/api/stream"); const t = setTimeout(() => { es.close(); res(false); }, 8000);
  es.addEventListener("digest", () => { clearTimeout(t); es.close(); res(true); }); setTimeout(() => fetch(b + "/api/settings/notify-test", { method: "POST" }), 500); }), B);
rec("scope all: test notification arrives as a digest event", got);
await b2.close();
execSync(`bash ${RESTART}`);
rec("no page errors (t17)", errors.length === 0, errors.slice(0, 3).join(" | "));
