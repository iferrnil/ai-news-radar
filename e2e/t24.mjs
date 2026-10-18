// Rounds 12–13: 热点追踪 (multi-source topics, Chinese / English names via the lexicon, heat + velocity, spark, jump to
// items, 生成文章 — no 简报 button since 0.13.0), 热点词表 (settings; locked → hidden / 403), 关键词 (settings, tags on
// cards, filter chip / tag click, rematch on change; no notifications since 0.13.0), 选题收藏夹 (card + drawer star,
// 收藏 view, export / import, lock), 写作风格 (desktop only), the article dialog's copy / export (0.11 behaviour again —
// the 0.12 platform copy was removed).
import { execSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { chromium } from "playwright";
import { rec, B, RESTART, DATA, mock, srcs, exportFile } from "./lib.mjs";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const SH = process.env.R13_SHOTS ?? "/tmp/shots-r13"; fs.mkdirSync(SH, { recursive: true });
const shot = (n) => path.join(SH, n);
const J = { "Content-Type": "application/json" };
const ART = path.join(DATA, "artifacts"); fs.rmSync(ART, { recursive: true, force: true });
const byName = async (n) => (await srcs()).find((s) => s.name.startsWith(n));
const poll = async (n) => (await fetch(`${B}/api/sources/${(await byName(n)).id}/fetch`, { method: "POST", headers: J })).json();
const patch = (b) => fetch(B + "/api/settings", { method: "PATCH", headers: J, body: JSON.stringify(b) });
const errors = [];
const br = await chromium.launch();
const HANDLES = ["X @OpenAI", "X @AnthropicAI", "X @deepseek_ai"];

execSync(`bash ${RESTART} DISABLE_POLLER=1`);
for (const h of HANDLES) await poll(h); // backlog poll (never notifies)
await sleep(1800);

// ---------------- web: 关键词 settings
{
  const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
  await ctx.addInitScript(() => {
    window.__notes = []; window.__bg = false;
    class N { constructor(t, o) { this.title = t; this.body = o?.body; this.tag = o?.tag; window.__notes.push(this); } close() {} }
    N.permission = "granted"; N.requestPermission = async () => "granted"; window.Notification = N;
    const hf = document.hasFocus.bind(document); document.hasFocus = () => (window.__bg ? false : hf());
  });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(B + "/settings"); await p.getByTestId("watch-card").waitFor({ timeout: 15000 });
  await p.getByTestId("watch-input").fill("Zephyrion, 求索、zephyrion"); await p.getByTestId("watch-add").click(); await sleep(600);
  const w = (await (await fetch(B + "/api/settings")).json()).watch;
  rec("关键词: comma / 、 split, case-insensitive dedupe, saved", JSON.stringify(w.keywords) === '["Zephyrion","求索"]' && w.scope === "all" && !w.caseSensitive, JSON.stringify(w));
  rec("关键词: chips listed", (await p.getByTestId("watch-list").innerText()).includes("Zephyrion"));
  rec("关键词 card: no 通知 switch, no 提醒 wording", await p.getByTestId("watch-notify").count() === 0 && !/提醒|通知/.test(await p.getByTestId("watch-card").innerText()), await p.getByTestId("watch-card").innerText());
  await patch({ watch: { notify: true } });
  rec("关键词 API: a notify flag is not stored", !("notify" in (await (await fetch(B + "/api/settings")).json()).watch));
  // 热点词表
  await p.getByTestId("hotlex-card").waitFor();
  await p.getByTestId("hotlex-input").fill("星澜 = Xinglan\n星澜"); await p.getByTestId("hotlex-card").screenshot({ path: shot("settings-hotlex.png") });
  await p.getByTestId("hotlex-save").click(); await sleep(600);
  rec("热点词表: saved normalized", (await (await fetch(B + "/api/settings")).json()).hotLexicon === "星澜=Xinglan", JSON.stringify((await (await fetch(B + "/api/settings")).json()).hotLexicon));
  await p.getByTestId("watch-card").screenshot({ path: shot("settings-watch.png") });
  rec("web: 写作风格 card hidden, API 404", await p.getByTestId("style-card").count() === 0 && (await fetch(B + "/api/style?kind=wechat")).status === 404);
  await p.evaluate(() => localStorage.setItem("notify.web", JSON.stringify({ enabled: true, sections: [] })));

  // three sources report the same story → one hot topic; titles match the keyword
  await p.goto(B + "/"); await p.locator("[data-item]").first().waitFor({ timeout: 20000 }); await sleep(800);
  await p.evaluate(() => { window.__bg = true; });
  // keep the new posts > 2 min away from each account's other posts (a few seconds apart would read as one thread)
  const others = (await (await fetch(B + "/api/items?limit=1000")).json()).filter((it) => it.is_x).map((it) => it.published);
  const clear = (age) => { let a = age; while (others.some((p) => Math.abs(Date.now() / 1000 - a - p) < 150)) a += 160; return a; };
  const [a1, a2, a3] = [clear(6000), clear(3000), clear(600)];
  await mock(`add=OpenAI&age=${a1}&title=` + encodeURIComponent("Orbitlab releases Zephyrion-9, an open model with 2M context"));
  await mock(`add=AnthropicAI&age=${a2}&title=` + encodeURIComponent("Zephyrion-9 from Orbitlab tops the coding leaderboard"));
  await mock(`add=deepseek_ai&age=${a3}&title=` + encodeURIComponent("Orbitlab's Zephyrion-9 is out: weights and benchmarks"));
  // a Chinese and an English report of one story: 「字节跳动 / 豆包」 = ByteDance / Doubao (lexicon), 「星澜」 = Xinglan (user lexicon)
  const [b1, b2] = [clear(9000), clear(4500)];
  await mock(`add=OpenAI&age=${b1}&title=` + encodeURIComponent("字节跳动发布豆包 Quill-4 与星澜引擎"));
  await mock(`add=AnthropicAI&age=${b2}&title=` + encodeURIComponent("ByteDance ships Doubao Quill-4 and the Xinglan engine"));
  for (const h of HANDLES) await poll(h);
  await sleep(6500); // a keyword batch would have collected for 5 s
  const notes = await p.evaluate(() => window.__notes.map((n) => ({ title: n.title, body: n.body, tag: n.tag })));
  const kn = notes.filter((n) => n.tag === "ai-radar-kw" || /关键词/.test(n.title ?? ""));
  const stn = (await (await fetch(B + "/api/stats")).json()).notify ?? {};
  rec("关键词: matches raise no notification (channel removed)", kn.length === 0 && !("keyword" in stn), JSON.stringify(notes).slice(0, 300) + " " + JSON.stringify(stn).slice(0, 200));
  await p.evaluate(() => { window.__bg = false; }); await p.reload(); await p.locator("[data-item]").first().waitFor(); await sleep(600);
  const items = await (await fetch(B + "/api/items?limit=1000")).json();
  const zs = items.filter((it) => /Zephyrion-9/.test(it.title));
  rec("关键词: matched items carry kw", zs.length === 3 && zs.every((it) => (it.kw ?? []).includes("Zephyrion")), JSON.stringify(zs.map((z) => z.kw)));
  const badge = p.locator(`[data-item="${zs[0].id}"] [data-testid=kw-badge]`);
  const shown = (await Promise.all(zs.map((z) => p.locator(`[data-item="${z.id}"]`).count()))).reduce((a, b) => a + b, 0);
  rec("feed: every matched card shown is highlighted with a keyword badge", await badge.count() === 1 && (await badge.innerText()).includes("Zephyrion") && shown >= 1 && await p.locator("[data-testid=kw-badge]").count() === shown, `${await p.locator(`[data-item="${zs[0].id}"]`).count()} ${await p.locator("[data-testid=kw-badge]").count()} ${await p.locator("[data-item]").count()}`);
  await p.locator(`[data-item="${zs[0].id}"]`).screenshot({ path: shot("feed-keyword-card.png") });
  // filter: the toolbar chip (any keyword) and a tag click (that keyword)
  const chip = p.getByTestId("kw-filter");
  rec("关键词 chip: shown with the match count", await chip.count() === 1 && (await chip.innerText()).includes(String(zs.length)), await chip.innerText().catch(() => ""));
  await chip.click(); await sleep(500);
  const visKw = await p.locator("[data-item]").count();
  rec("关键词 chip: only the matched cards, ?kw=* in the URL", visKw === zs.length && visKw === await p.locator("[data-testid=kw-badge]").count() && new URL(p.url()).searchParams.get("kw") === "*", `${visKw} ${p.url()}`);
  await p.screenshot({ path: shot("feed-keyword-filter.png") });
  await chip.click(); await sleep(400);
  rec("关键词 chip: off again", !new URL(p.url()).searchParams.get("kw") && await p.locator("[data-item]").count() > visKw);
  await p.locator(`[data-item="${zs[0].id}"] [data-testid=kw-badge] button`).first().click(); await sleep(500);
  rec("tag click → that keyword only, drawer not opened", new URL(p.url()).searchParams.get("kw") === "Zephyrion" && await p.getByRole("dialog").count() === 0 && await p.locator("[data-item]").count() === visKw && (await chip.innerText()).includes("Zephyrion"));
  rec("api/items?kw= filters server-side too", (await (await fetch(B + "/api/items?kw=Zephyrion&limit=50")).json()).length === 3 && (await (await fetch(B + "/api/items?kw=*&limit=50")).json()).length === 3);
  await p.goto(B + "/?kw=Zephyrion"); await p.locator("[data-item]").first().waitFor(); await sleep(400);
  rec("?kw= survives a reload (web parity: URL state)", await p.locator("[data-item]").count() === visKw);
  await p.getByTestId("kw-filter").click(); await sleep(300);
  await patch({ watch: { keywords: ["Orbitlab"], scope: "title" } }); await sleep(500);
  const re = await (await fetch(B + `/api/items?id=${zs[0].id}`)).json();
  rec("关键词 change rematches retained items", JSON.stringify(re[0]?.kw) === '["Orbitlab"]', JSON.stringify(re[0]?.kw));
  await patch({ watch: { keywords: ["Zephyrion"], scope: "all" } }); await sleep(300);

  // 热点
  await p.getByRole("complementary", { name: "侧边栏" }).getByRole("button", { name: "热点" }).click(); await p.getByTestId("hot-topic").first().waitFor({ timeout: 10000 });
  rec("热点: sidebar entry → ?view=hot", new URL(p.url()).searchParams.get("view") === "hot");
  const top = p.getByTestId("hot-topic").filter({ hasText: "Zephyrion" });
  const hots = (await (await fetch(B + "/api/hot")).json()).topics;
  const hot = hots.find((t) => /Zephyrion/.test(t.title));
  rec("热点: the 3-source story is one topic (3 源 · 3 条)", await top.count() === 1 && await top.getByTestId("hot-count").innerText() === "3 源 · 3 条", (await top.first().innerText()).replace(/\n/g, " "));
  rec("热点: sorted by heat", hots.every((t, i) => i === 0 || hots[i - 1].heat >= t.heat), hots.map((t) => t.heat).join(","));
  rec("热点: 首发 = the earliest source", /首发 OpenAI/.test(await top.innerText()), (await top.innerText()).replace(/\n/g, " "));
  rec("热点: 24 h spark bars", await top.getByTestId("spark").locator("rect").count() >= 3);
  rec("api/hot: spark sums to the reports, heat > 0", hot.spark.reduce((a, b) => a + b, 0) === 3 && hot.heat > 0 && hot.first.source.includes("OpenAI"), JSON.stringify({ spark: hot.spark, heat: hot.heat }));
  rec("web 热点: no 简报 button, 生成文章 hidden", await p.getByTestId("hot-brief").count() === 0 && await top.getByTestId("hot-article").count() === 0);
  const zh = hots.find((t) => t.rows.some((r) => /豆包/.test(r.title)));
  rec("热点: Chinese + English names (字节跳动/豆包 = ByteDance/Doubao) → one topic", !!zh && zh.rows.some((r) => /Doubao/.test(r.title)) && zh.sources.length === 2, JSON.stringify(hots.map((t) => t.rows.map((r) => r.title))).slice(0, 400));
  rec("api/hot: velocity + first report fields", typeof hot.velocity === "number" && hot.first.title && hot.first.url && hot.core, JSON.stringify({ v: hot.velocity, first: hot.first, core: hot.core }));
  await top.getByRole("button", { name: "展开" }).click(); await top.getByTestId("hot-items").waitFor();
  rec("热点: expanded rows list each report", await top.getByTestId("hot-items").locator("li").count() === 3);
  await p.getByTestId("hot-view").screenshot({ path: shot("hot-view.png") });
  await top.getByTestId("hot-items").locator("button").first().click(); await p.getByRole("dialog").waitFor();
  rec("热点: a row opens the item drawer", /Zephyrion/.test(await p.getByRole("dialog").innerText()));

  // 收藏 from the drawer, then from a card
  await p.getByTestId("sheet-fav").click(); await sleep(400);
  rec("drawer star → favorited", await p.getByTestId("sheet-fav").getAttribute("aria-pressed") === "true");
  await p.keyboard.press("Escape"); await sleep(300);
  await p.getByRole("button", { name: /^全部/ }).first().click(); await p.locator("[data-item]").first().waitFor();
  const oc = p.locator("[data-item]").filter({ hasNotText: "Zephyrion" }).first(); await oc.scrollIntoViewIfNeeded(); await oc.hover();
  await oc.getByTestId("fav-btn").click(); await sleep(400);
  rec("card star → favorited", await oc.getByTestId("fav-btn").getAttribute("aria-pressed") === "true");
  await p.getByRole("complementary", { name: "侧边栏" }).getByRole("button", { name: "收藏" }).click(); await sleep(800);
  rec("收藏 view: exactly the 2 favorites", await p.locator("[data-item]").count() === 2 && new URL(p.url()).searchParams.get("view") === "fav", String(await p.locator("[data-item]").count()));
  await p.screenshot({ path: shot("fav-view.png") });
  const ex = await (await fetch(B + "/api/favorites/export")).json();
  rec("favorites export: 2 rows with uid / url / source", ex.length === 2 && ex.every((r) => r.uid && r.url && r.source), String(ex.length));
  // JSON subscriptions export carries them
  await p.goto(B + "/sources"); await p.getByRole("heading", { name: "数据源" }).waitFor();
  const dl = await exportFile(p, "全部", "json");
  const exj = JSON.parse(dl.text);
  rec("subscriptions JSON export includes favorites", exj.favorites?.length === 2, String(exj.favorites?.length));
  // unfavorite from the 收藏 view
  await p.goto(B + "/?view=fav"); await p.locator("[data-item]").first().waitFor();
  const fc = p.locator("[data-item]").first(); await fc.hover(); await fc.getByTestId("fav-btn").click(); await sleep(500);
  rec("unfavorite in 收藏 view removes the card", await p.locator("[data-item]").count() === 1);
  // import restores it (dry-run shows the count)
  const imp = "/tmp/r12-fav-import.json"; fs.writeFileSync(imp, dl.text);
  await p.goto(B + "/sources"); await p.getByRole("heading", { name: "数据源" }).waitFor();
  await p.getByRole("button", { name: "导入", exact: true }).click(); await p.getByRole("dialog", { name: "导入订阅" }).waitFor();
  await p.locator("input[type=file]").setInputFiles(imp); await p.getByTestId("import-fav").waitFor({ timeout: 10000 });
  rec("import preview: 收藏 count", (await p.getByTestId("import-fav").innerText()).includes("2"));
  await p.getByRole("dialog", { name: "导入订阅" }).getByRole("button", { name: /^导入/ }).last().click(); await sleep(1200);
  rec("import restores the unfavorited item", (await (await fetch(B + "/api/favorites/export")).json()).length === 2);
  rec("fav view filter via API", (await (await fetch(B + "/api/items?fav=1&limit=50")).json()).length === 2);
  await ctx.close();
}

// ---------------- locked web: favorite toggles allowed, import refused
execSync(`bash ${RESTART} DISABLE_POLLER=1 SETTINGS_LOCKED=1`);
{
  const one = (await (await fetch(B + "/api/items?limit=5")).json())[0];
  const t1 = await fetch(B + "/api/favorites", { method: "POST", headers: J, body: JSON.stringify({ id: one.id, on: true }) });
  const t2 = await fetch(B + "/api/favorites/import", { method: "POST", headers: J, body: JSON.stringify({ favorites: [] }) });
  rec("locked: favorite toggle allowed, favorites import 403", t1.status === 200 && t2.status === 403, `${t1.status} ${t2.status}`);
  await fetch(B + "/api/favorites", { method: "POST", headers: J, body: JSON.stringify({ id: one.id, on: false }) });
  rec("locked: 写作风格 still 404 on web", (await fetch(B + "/api/style?kind=xhs")).status === 404);
  rec("locked: 热点词表 PATCH 403, 关键词 still editable", (await patch({ hotLexicon: "x" })).status === 403 && (await patch({ watch: { keywords: ["Zephyrion"] } })).status === 200);
  const p = await br.newPage({ viewport: { width: 1280, height: 900 } }); await p.goto(B + "/settings"); await p.getByTestId("watch-card").waitFor({ timeout: 15000 }); await sleep(500);
  rec("locked web: 热点词表 card hidden", await p.getByTestId("hotlex-card").count() === 0);
  await p.close();
}

// ---------------- desktop: 写作风格 + 生成文章 from a hot topic + platform copy
execSync(`bash ${RESTART} DISABLE_POLLER=1 AI_RADAR_DESKTOP=1 ${["DEEPSEEK_API_KEY", process.env.E2E_DS_KEY].join("=")} DEEPSEEK_BASE_URL=https://ds.mock-ai.com/v1 BRIEF_CACHE_MS=1 ARTIFACTS_DIR=${ART}`);
{
  const ctx = await br.newContext({ viewport: { width: 1440, height: 900 } });
  await ctx.grantPermissions(["clipboard-read", "clipboard-write"], { origin: B });
  await ctx.addInitScript(() => { window.__radarDesktop = true; });
  const p = await ctx.newPage(); p.on("pageerror", (e) => errors.push(String(e)));
  await p.goto(B + "/settings"); await p.getByTestId("style-card").waitFor({ timeout: 15000 });
  const sc = p.getByTestId("style-card");
  await sc.getByTestId("style-tab-wechat").click();
  const sample = "我写东西有个习惯：先说结论，再讲为什么。\n\n" + "上周我们团队试了三款新模型，结果出乎意料，最便宜的那款反而最稳。".repeat(4);
  await sc.getByTestId("style-paste").fill(sample); await sc.getByTestId("style-add").click(); await sleep(600);
  rec("写作风格: sample added (desktop)", (await sc.getByTestId("style-samples").innerText()).includes("先说结论"));
  await sc.getByTestId("style-gen").click(); await p.waitForFunction(() => /STYLE-E2E-PROFILE/.test(document.querySelector("[data-testid=style-profile]")?.value ?? ""), null, { timeout: 15000 });
  rec("写作风格: profile generated via the model, editable", /STYLE-E2E-PROFILE/.test(await sc.getByTestId("style-profile").inputValue()) && await sc.getByTestId("style-profile").isEditable());
  const st = await (await fetch(B + "/api/style?kind=wechat")).json();
  rec("写作风格: per platform (xhs has none)", st.samples?.length === 1 && (await (await fetch(B + "/api/style?kind=xhs")).json()).samples.length === 0);
  await sc.screenshot({ path: shot("settings-style.png") });

  await p.goto(B + "/?view=hot"); await p.getByTestId("hot-topic").first().waitFor({ timeout: 15000 });
  const top = p.getByTestId("hot-topic").first();
  rec("desktop 热点: 生成文章 shown", await top.getByTestId("hot-article").count() === 1);
  await top.getByTestId("hot-article").click(); await p.getByTestId("article-dialog").waitFor();
  const gen = async (pf) => { await p.getByTestId(`pf-${pf}`).click(); await p.getByTestId("article-go").click(); await p.getByTestId("article-body").waitFor({ timeout: 20000 }); await sleep(400); };
  const clip = () => p.evaluate(async () => { const out = {}; for (const it of await navigator.clipboard.read()) for (const t of it.types) out[t] = await (await it.getType(t)).text(); return out; });
  const req = p.waitForRequest((r) => r.url().includes("/api/compose") || r.url().includes("/api/article"), { timeout: 5000 }).catch(() => null);
  await gen("wechat"); const rq = await req;
  const ids = rq ? JSON.parse(rq.postData() ?? "{}").ids ?? [] : [];
  rec("hot → 生成文章 auto-picks 2 of the topic's items", ids.length === 2, JSON.stringify(ids));
  rec("wechat article: the style profile reached the prompt", (await p.getByTestId("article-body").innerText()).includes("STYLE-IN"));
  // copy / export as in 0.11: one 复制 (the Markdown as plain text) and 导出; no platform-specific copy
  rec("article dialog: no platform copy buttons", await p.locator("[data-testid^=copy-], [data-testid=tweet-copy]").count() === 0);
  await p.getByTestId("article-dialog").getByRole("button", { name: "复制" }).click(); await sleep(400);
  const c1 = await clip();
  rec("复制: plain Markdown text only (0.11 behaviour)", !c1["text/html"] && (c1["text/plain"] ?? "").includes("STYLE-IN"), Object.keys(c1).join(","));
  rec("native clipboard hook removed", [404, 405].includes((await fetch(B + "/api/desktop/clipboard", { method: "POST", headers: J, body: "{}" })).status));
  await p.getByTestId("article-dialog").screenshot({ path: shot("article-dialog.png") });
  await ctx.close();
}

rec("no page errors", errors.length === 0, errors.slice(0, 3).join(" | "));
await br.close();
