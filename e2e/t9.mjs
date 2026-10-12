// all sections user-manageable (former built-ins), seed sync never resurrects, fallback 未分类, v1 import, menu label
import { execSync } from "child_process";
import fs from "fs";
import { launch, rec, api, srcs, openImport, waitPlan, applyImport, B, SHOT, RESTART } from "./lib.mjs";
const J = { "content-type": "application/json" };
const restart = async () => { execSync("bash " + RESTART + "", { stdio: "ignore" }); await new Promise((r) => setTimeout(r, 1500)); };
const { br, page, errors } = await launch(false);
const shot = (n) => page.screenshot({ path: SHOT(`${n}.png`) });
const entIds = (await srcs()).filter((s) => s.section === "ent").map((s) => s.id);

// 1) row menu label + dialog title
const row = page.getByRole("row").filter({ hasText: "www.techmeme.com" });
await row.getByRole("button", { name: "操作" }).click();
const menu = await page.getByRole("menuitem").allInnerTexts(); await shot("row_menu");
await page.getByRole("menuitem", { name: "编辑", exact: true }).click();
const dt = await page.getByRole("dialog").getByRole("heading").first().innerText();
rec("row menu says 编辑 (old label gone) + dialog title 编辑「name」", menu.includes("编辑") && !menu.some((m) => m.includes("编辑分区与过滤")) && dt === "编辑「Techmeme」", dt);
await page.keyboard.press("Escape"); await page.waitForTimeout(300);

// 2) former built-ins are regular sections: rename 娱乐 -> 吃瓜, re-icon 🍉, move up, toggle 全部
const secs0 = await api("/api/sections");
rec("migration seeded the 5 former built-ins as rows", ["实验室", "国内媒体", "国外媒体", "X 快讯", "娱乐"].every((n) => secs0.some((s) => s.name === n)) && secs0.find((s) => s.key === "ent").in_all === 0);
await page.getByRole("button", { name: "分区", exact: true }).click();
const sd = page.getByRole("dialog", { name: "管理分区" }); await sd.waitFor();
const ent = sd.getByTestId("section-ent");
await ent.getByLabel("分区名称").fill("吃瓜"); await ent.getByLabel("分区名称").press("Enter"); await page.waitForTimeout(700);
await ent.getByRole("button", { name: "「吃瓜」的图标" }).click();
const pk = page.getByTestId("icon-picker"); await pk.getByLabel("搜索图标").fill("吃瓜"); await pk.getByRole("button", { name: /^图标 🍉/ }).click(); await page.waitForTimeout(600);
await ent.getByRole("button", { name: "上移「吃瓜」" }).click(); await page.waitForTimeout(600);
await shot("manage_sections");
let s1 = await api("/api/sections"); const e1 = s1.find((s) => s.key === "ent");
rec("rename + re-icon + reorder a former built-in", e1.name === "吃瓜" && e1.icon === "🍉" && s1.findIndex((s) => s.key === "ent") === 3, JSON.stringify(s1.map((s) => s.name)));
await page.keyboard.press("Escape"); await page.waitForTimeout(400);
rec("sidebar shows renamed section, not 娱乐", await page.getByRole("complementary").getByRole("button", { name: /^吃瓜/ }).count() > 0 && await page.getByRole("complementary").getByRole("button", { name: /^娱乐/ }).count() === 0);
await restart();
s1 = await api("/api/sections");
rec("restart: edits persist, 娱乐 not resurrected", s1.find((s) => s.key === "ent")?.name === "吃瓜" && !s1.some((s) => s.name === "娱乐") && s1.length === 5);

// 3) delete it from the sidebar ⚙ with move-to 国外媒体
await page.goto(B + "/"); await page.waitForTimeout(1200);
const side = page.getByRole("complementary");
await side.getByRole("button", { name: /^吃瓜/ }).first().hover();
{ const bb = await side.getByRole("button", { name: "管理分区「吃瓜」" }).boundingBox(); await page.mouse.click(bb.x + bb.width / 2, bb.y + bb.height / 2); }
await sd.waitFor(); await sd.getByRole("button", { name: "删除分区「吃瓜」" }).click();
const ad = page.getByRole("alertdialog"); await ad.waitFor();
const warn = await ad.getByTestId("delete-warning").innerText();
const mv = await ad.getByRole("combobox", { name: "分区" }).innerText();
await ad.getByRole("button", { name: "删除" }).click(); await page.waitForTimeout(1000);
await restart();
const s2 = await api("/api/sections"); const moved = (await srcs()).filter((s) => entIds.includes(s.id));
rec("delete former built-in (warning, move-to default 国外媒体) + no resurrection after restart", /无法撤销/.test(warn) && mv.includes("国外媒体") && !s2.some((s) => s.key === "ent" || s.name === "娱乐" || s.name === "吃瓜") && moved.every((s) => s.section === "media"), `${moved.length} moved`);

// 4) seed fallback: a seed source whose default section (ent) no longer exists -> 未分类 (auto-created)
const victim = moved.find((s) => s.origin === "seed");
await fetch(B + `/api/sources/${victim.id}`, { method: "DELETE" });
await fetch(B + "/api/sources/deleted", { method: "POST", headers: J, body: JSON.stringify({ names: [victim.name] }) });
const back = (await srcs()).find((s) => s.name === victim.name); const s3 = await api("/api/sections");
const unc = s3.find((s) => s.name === "未分类");
rec("seed fallback: re-added seed source goes to auto-created 未分类", !!unc && back?.section === unc.key, `${victim.name} -> ${back?.section}`);
await restart();
rec("restart: no section/source resurrection, 未分类 kept", (await api("/api/sections")).length === s3.length && (await srcs()).filter((s) => s.name === victim.name).length === 1);

// 5) v1 file still imports: legacy key "ent" recreated by its default name
fs.writeFileSync("/tmp/v1.json", JSON.stringify({ app: "ai-news-radar", version: 1, sources: [{ name: "V1 Test Feed", type: "rss", url: "https://example.com/v1-feed.xml", section: "ent", priority: "P2", enabled: false }, { name: "V1 Lab Feed", type: "rss", url: "https://example.com/v1-lab.xml", section: "lab", priority: "P2", enabled: false }] }));
await page.goto(B + "/sources"); await page.waitForTimeout(600);
const dlg = await openImport(page, "/tmp/v1.json"); await waitPlan(dlg); await applyImport(dlg); await page.waitForTimeout(1200);
const v1 = (await srcs()).filter((s) => s.name.startsWith("V1 ")); const s4 = await api("/api/sections");
rec("v1 import: legacy keys map (lab kept, deleted ent recreated as 娱乐 outside 全部)", v1.find((s) => s.name === "V1 Lab Feed")?.section === "lab" && s4.some((s) => s.name === "娱乐" && s.in_all === 0 && v1.find((x) => x.name === "V1 Test Feed")?.section === s.key));
for (const s of v1) await fetch(B + `/api/sources/${s.id}`, { method: "DELETE" });

// 6) v2 roundtrip of all sections incl. renamed defaults: rename x -> 推特, export JSON, delete X sources, import -> back under 推特
await fetch(B + "/api/sections/x", { method: "PATCH", headers: J, body: JSON.stringify({ name: "推特", icon: "🐦" }) });
await page.reload(); await page.waitForTimeout(800);
await page.getByRole("button", { name: "导出" }).click(); await page.getByRole("menuitemradio", { name: /全部来源/ }).click();
const [dl] = await Promise.all([page.waitForEvent("download"), page.getByRole("menuitem", { name: /JSON/ }).click()]);
await dl.saveAs("/tmp/t9.json"); await page.keyboard.press("Escape").catch(() => {});
const jx = JSON.parse(fs.readFileSync("/tmp/t9.json", "utf8"));
const xs = (await srcs()).filter((s) => s.section === "x");
for (const s of xs) await fetch(B + `/api/sources/${s.id}`, { method: "DELETE" });
await fetch(B + "/api/sections/x", { method: "PATCH", headers: J, body: JSON.stringify({ name: "推特2" }) });
await page.goto(B + "/sources"); await page.waitForTimeout(800);
const d2 = await openImport(page, "/tmp/t9.json"); await waitPlan(d2); await applyImport(d2); await page.waitForTimeout(1200);
const s5 = await api("/api/sections");
rec("v2 export carries used sections (renamed defaults incl.); re-import maps renamed default by key, no duplicate section", jx.sections.some((s) => s.key === "x" && s.name === "推特" && s.icon === "🐦") && jx.sections.every((x) => jx.sources.some((e) => e.section === x.key)) && !s5.some((s) => s.name === "推特") && (await srcs()).filter((s) => s.section === "x").length === xs.length, JSON.stringify({ secs: jx.sections.map((s) => s.key + ":" + s.name), now: s5.map((s) => s.key + ":" + s.name), x: (await srcs()).filter((s) => s.section === "x").length, xs: xs.length }));
const errs = errors.filter((e) => !/status of (400|409)|ERR_INCOMPLETE_CHUNKED_ENCODING|ERR_CONNECTION_REFUSED|Failed to fetch|network/i.test(e));
rec("no console errors (t9)", errs.length === 0, errs.slice(0, 2).join(" | "));
await br.close();
