// Custom sections + per-source filter rules
import { execSync } from "child_process";
import fs from "fs";
import { launch, rec, api, srcs, exportFile, openImport, waitPlan, applyImport, B, SHOT, RESTART } from "./lib.mjs";
const SMOL = "https://news.smol.ai/rss.xml";
const { br, page, errors } = await launch(false);
const smol = async () => (await srcs()).find((s) => s.url.includes("news.smol.ai"));
const dlgAdd = page.getByRole("dialog", { name: "添加数据源" });

// 1) add Smol AI News into a NEW section with an exclude rule; invalid regex blocks submit
await page.getByRole("button", { name: /添加数据源/ }).click(); await dlgAdd.waitFor();
await dlgAdd.getByLabel("地址 / 账号").fill(SMOL); await dlgAdd.getByRole("button", { name: "测试" }).click();
await dlgAdd.getByText(/条$/).first().waitFor({ timeout: 30000 }).catch(() => {});
await dlgAdd.getByRole("combobox", { name: "分区" }).click(); await page.getByRole("option", { name: "＋ 新建分区…" }).click();
await dlgAdd.getByLabel("新分区名称").fill("视频生成"); await dlgAdd.getByRole("button", { name: "新分区图标" }).click(); await page.getByTestId("icon-picker").getByLabel("搜索图标").fill("视频"); await page.getByTestId("icon-picker").getByRole("button", { name: /^图标 🎬/ }).click();
await dlgAdd.getByRole("button", { name: /过滤规则/ }).click();
await dlgAdd.getByLabel("只保留匹配（包含）").fill("(x"); await dlgAdd.getByTestId("rule-error").waitFor({ timeout: 5000 }).catch(() => {});
const invalidShown = await dlgAdd.getByTestId("rule-error").count() > 0, addDisabled = await dlgAdd.getByRole("button", { name: "添加并订阅" }).isDisabled();
rec("invalid regex rejected in dialog (error + submit disabled)", invalidShown && addDisabled);
const rc = await fetch(B + "/api/sources/rule-check", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ exclude: "^(a|a)*$" }) });
rec("server rejects catastrophic regex (timeout guard)", rc.status === 400, (await rc.json()).detail);
await dlgAdd.getByLabel("只保留匹配（包含）").fill("");
await dlgAdd.getByLabel("隐藏匹配（排除）").fill("not much happened"); await page.waitForTimeout(1500);
const pvText = await dlgAdd.getByTestId("rule-preview").innerText().catch(() => "");
const pvFlat = pvText.replace(/\s+/g, " ");
rec("live preview shows kept/hidden counts", /保留 \d+ \/ 隐藏 \d+/.test(pvFlat), pvFlat.slice(0, 60));
await page.screenshot({ path: SHOT("add_new_section_rule.png") });
await dlgAdd.getByRole("button", { name: "添加并订阅" }).click(); await dlgAdd.waitFor({ state: "hidden", timeout: 40000 });
const secs = await api("/api/sections"); const s1 = await smol();
const key = secs.find((x) => x.name === "视频生成")?.key;
rec("custom section created + source stored in it with rule", !!key && s1?.section === key && s1?.f_exclude === "not much happened" && secs.find((x) => x.key === key)?.icon === "🎬", `${key} ${s1?.section}`);
const all = await api(`/api/items?limit=1000&source=${s1.id}`); const items = all.items ?? all;
rec("fetch-time filter: no stored item matches exclude", items.length >= 0 && !items.some((i) => /not much happened/i.test(i.title)), `${items.length} items`);
await page.reload(); await page.waitForTimeout(1200);
const fbadge = page.getByRole("row").filter({ hasText: "news.smol.ai" }).getByLabel("有过滤规则");
rec("filter badge on source row", await fbadge.count() > 0);

// 2) sidebar shows it; section view + P0 per section works
const nav = page.getByRole("complementary").getByRole("button", { name: /^视频生成/ });
rec("sidebar shows custom section", await nav.count() > 0);
await nav.click(); await page.waitForURL(/section=c\d+/); await page.waitForTimeout(1200);
const chips = await page.locator("[data-slot=card] [data-slot=badge]").allInnerTexts();
await page.screenshot({ path: SHOT("sidebar_custom_section.png") });
rec("custom section view shows only its items", !chips.some((t) => ["实验室", "国内媒体", "国外媒体", "X 快讯", "娱乐"].includes(t)), page.url());
await page.getByRole("button", { name: /^仅 P0/ }).click(); await page.waitForTimeout(500);
const p0on = page.url().includes("p0=1");
await page.getByRole("complementary").getByRole("button", { name: /^全部/ }).click(); await page.waitForTimeout(600);
const allP0 = page.url().includes("p0=1");
await page.getByRole("complementary").getByRole("button", { name: /^视频生成/ }).click(); await page.waitForTimeout(600);
rec("P0 per custom section remembered independently", p0on && !allP0 && page.url().includes("p0=1"));
await page.getByRole("button", { name: /^仅 P0/ }).click(); await page.waitForTimeout(300);

// 3) edit rule retroactively hides/unhides (on Techmeme, which has stored items), via the row's edit dialog
let big = (await srcs()).filter((s) => s.enabled && s.supported).sort((a, b) => b.item_count - a.item_count)[0]; // source with most stored items
if (!big.item_count) for (const s of (await srcs()).filter((s) => s.enabled && s.supported)) { await fetch(B + `/api/sources/${s.id}/fetch`, { method: "POST" }); }
big = (await srcs()).filter((s) => s.enabled && s.supported).sort((a, b) => b.item_count - a.item_count)[0];
const tmId = big.id; const tmHost = big.name;
const tmGet = async () => (await srcs()).find((s) => s.id === tmId);
const before = (await tmGet()).item_count;
await page.goto(B + "/sources"); await page.waitForTimeout(800);
const ed = page.getByRole("dialog", { name: /编辑/ });
const editTm = async (exc) => {
  await page.getByRole("row").filter({ hasText: tmHost }).getByRole("button", { name: "操作" }).click();
  await page.getByRole("menuitem", { name: "编辑", exact: true }).click(); await ed.waitFor();
  await ed.getByLabel("隐藏匹配（排除）").fill(exc); await page.waitForTimeout(1500);
  if (exc) await page.screenshot({ path: SHOT("edit_rule_preview.png") });
  await ed.getByRole("button", { name: "保存" }).click(); await ed.waitFor({ state: "hidden" });
};
await editTm("."); const hid = await tmGet();
await editTm(""); const back = await tmGet();
rec("rule change hides existing items retroactively and restores them", before > 0 && hid.item_count === 0 && hid.hidden_count === before && back.item_count === before && !back.f_exclude, `${before} -> ${hid.item_count}/${hid.hidden_count} -> ${back.item_count}`);

// 4) rename via 管理分区 (sidebar link opens it)
await page.getByRole("complementary").getByRole("button", { name: /^视频生成/ }).hover();
{ const bb = await page.getByRole("button", { name: "管理分区「视频生成」" }).boundingBox(); await page.mouse.click(bb.x + bb.width / 2, bb.y + bb.height / 2); } // real mouse click at the button centre
 await page.waitForURL(/\/sources/);
const sd = page.getByRole("dialog", { name: "管理分区" }); await sd.waitFor();
{ const r = sd.getByTestId(`section-${key}`).getByLabel("分区名称"); await r.fill("UI设计"); await r.press("Enter"); await page.waitForTimeout(1200); }
await page.screenshot({ path: SHOT("manage_sections.png") });
await page.keyboard.press("Escape"); await page.waitForTimeout(600); // modal makes the sidebar inert; check after closing
rec("rename section (sidebar updates)", await page.getByRole("complementary").getByRole("button", { name: /^UI设计/ }).count() > 0);

// 5) export JSON + OPML include sections + rules
const j = await exportFile(page, "全部来源", "json"); const jj = JSON.parse(j.text);
const je = jj.sources.find((x) => x.url === SMOL);
rec("JSON export: version 2, sections + filter", jj.version === 2 && jj.sections?.some((x) => x.name === "UI设计" && x.key === key) && je?.section === key && je?.filter?.exclude === "not much happened", JSON.stringify({ v: jj.version, s: jj.sections, je }).slice(0, 400));
const o = await exportFile(page, "全部来源", "opml");
rec("OPML export: custom folder + filter attrs", /<outline text="UI设计" title="UI设计" sectionCustom="1"/.test(o.text) && /filterExclude="not much happened"/.test(o.text));
fs.writeFileSync("/tmp/smol-only.json", JSON.stringify({ ...jj, sources: [je], count: 1 }));
const ox = o.text.split("\n").filter((l) => !/<outline type=/.test(l) || l.includes("news.smol.ai")).join("\n"); fs.writeFileSync("/tmp/smol-only.opml", ox);

// 6) delete section that still has sources -> must choose target (moved), then delete the source
await page.getByRole("button", { name: "分区", exact: true }).click(); await sd.waitFor();
await sd.getByRole("button", { name: "删除分区「UI设计」" }).click();
const ad = page.getByRole("alertdialog"); await ad.waitFor();
const askMove = await ad.getByText(/还有 1 个来源/).count() > 0;
await ad.getByRole("button", { name: "删除" }).click(); await page.waitForTimeout(1200);
const moved = await smol();
rec("delete non-empty section moves sources (confirm + target)", askMove && moved.section === "media" && !(await api("/api/sections")).some((x) => x.name === "UI设计"), moved.section);
await page.keyboard.press("Escape");
const del409 = await (async () => { const c = await fetch(B + "/api/sections", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ name: "临时" }) }).then((r) => r.json());
  await fetch(B + `/api/sources/${moved.id}`, { method: "PATCH", headers: { "content-type": "application/json" }, body: JSON.stringify({ section: c.key }) });
  const r = await fetch(B + `/api/sections/${c.key}`, { method: "DELETE" }); const st = r.status;
  await fetch(B + `/api/sections/${c.key}?moveTo=media`, { method: "DELETE" }); return st; })();
rec("API refuses deleting non-empty section without moveTo", del409 === 409);
await fetch(B + `/api/sources/${moved.id}`, { method: "DELETE" });

// 7) import roundtrip (JSON then OPML): section re-created by name, rule restored
for (const [f, label] of [["/tmp/smol-only.json", "JSON"], ["/tmp/smol-only.opml", "OPML"]]) {
  await page.goto(B + "/sources"); await page.waitForTimeout(600);
  const dlg = await openImport(page, f); await waitPlan(dlg); await applyImport(dlg); await page.waitForTimeout(1500);
  const s = await smol(); const sc = await api("/api/sections");
  rec(`import ${label} roundtrip: section + rule restored`, !!s && sc.some((x) => x.name === "UI设计" && s.section === x.key) && s.f_exclude === "not much happened", `${s?.section} ${s?.f_exclude}`);
  if (s) await fetch(B + `/api/sources/${s.id}`, { method: "DELETE" });
  for (const x of sc.filter((x) => x.name === "UI设计")) await fetch(B + `/api/sections/${x.key}?moveTo=media`, { method: "DELETE" });
}

// 8) seed sync never overwrites: move a built-in into a custom section, restart, still there
const c = await fetch(B + "/api/sections", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ name: "视频生成", icon: "🎬" }) }).then((r) => r.json());
const tm = (await srcs()).find((s) => s.name === "Techmeme");
await fetch(B + `/api/sources/${tm.id}`, { method: "PATCH", headers: { "content-type": "application/json" }, body: JSON.stringify({ section: c.key }) });
execSync("bash " + RESTART + "", { stdio: "ignore" });
await new Promise((r) => setTimeout(r, 1500)); // let undici drop sockets to the old server
const retry = async (fn) => { for (let k = 0; ; k++) { try { return await fn(); } catch (e) { if (k > 4) throw e; await new Promise((r) => setTimeout(r, 1000)); } } };
const tm2 = (await retry(srcs)).find((s) => s.name === "Techmeme"); const sc2 = await retry(() => api("/api/sections"));
rec("custom section + seed source assignment survive restart (seed sync)", tm2.section === c.key && sc2.some((x) => x.key === c.key));
await fetch(B + `/api/sections/${c.key}?moveTo=media`, { method: "DELETE" });
const errs = errors.filter((e) => !/status of (400|409)|ERR_INCOMPLETE_CHUNKED_ENCODING|ERR_CONNECTION_(REFUSED|RESET|CLOSED)|ERR_EMPTY_RESPONSE|Failed to fetch/.test(e)); // SSE / in-flight requests cut by the deliberate server restart
rec("no console errors (t7, except expected 400/409)", errs.length === 0, errs.slice(0, 2).join(" | "));
await br.close();
