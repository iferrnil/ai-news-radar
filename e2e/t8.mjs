// icon picker, section delete w/ backup, source rename, sidebar tree
import { execSync } from "child_process";
import fs from "fs";
import { launch, rec, api, srcs, B, SHOT, RESTART } from "./lib.mjs";
const J = { "content-type": "application/json" };
const { br, page, errors } = await launch(false);
const shot = (n) => page.screenshot({ path: SHOT(`${n}.png`) });

// 1) icon picker: create section via manage dialog, pick via search, then clear on the row
await page.getByRole("button", { name: "分区", exact: true }).click();
const sd = page.getByRole("dialog", { name: "管理分区" }); await sd.waitFor();
await sd.getByLabel("新分区名称").fill("视频生成");
await sd.getByRole("button", { name: "新分区图标" }).click();
const pk = page.getByTestId("icon-picker"); await pk.waitFor();
await pk.getByRole("tab", { name: "视频 / 图像" }).click(); await page.waitForTimeout(200); await shot("icon_picker");
await pk.getByLabel("搜索图标").fill("视频"); await pk.getByRole("button", { name: /^图标 🎬/ }).click();
const trig = await sd.getByRole("button", { name: "新分区图标" }).innerText();
await sd.getByRole("button", { name: "新建分区" }).click(); await page.waitForTimeout(800);
let secs = await api("/api/sections"); const key = secs.find((x) => x.name === "视频生成")?.key;
rec("icon picker: search + pick sets emoji", trig.includes("🎬") && secs.find((x) => x.name === "视频生成")?.icon === "🎬", trig);
await sd.getByRole("button", { name: "「视频生成」的图标" }).click(); await pk.waitFor();
await pk.getByRole("button", { name: "清除图标" }).click(); await page.waitForTimeout(800);
secs = await api("/api/sections");
rec("icon picker: clear removes icon", secs.find((x) => x.name === "视频生成")?.icon == null);
await sd.getByRole("button", { name: "「视频生成」的图标" }).click(); await pk.waitFor();
const hasRecent = await pk.getByText("最近使用").count() > 0;
await pk.getByRole("button", { name: "图标 🎬 最近" }).click(); await page.waitForTimeout(600);
rec("icon picker: recent list remembers last pick", hasRecent && (await api("/api/sections")).find((x) => x.name === "视频生成")?.icon === "🎬");
await page.keyboard.press("Escape"); await page.waitForTimeout(300);

// 2) rename a source inline (click name) + via menu; persists across restart; shows in feed + export
const tm = (await srcs()).find((s) => s.name === "Techmeme");
await fetch(B + `/api/sources/${tm.id}`, { method: "PATCH", headers: J, body: JSON.stringify({ section: key }) }); // also populate the custom section
await page.reload(); await page.waitForTimeout(800);
const row = page.getByRole("row").filter({ hasText: "www.techmeme.com" });
await row.getByRole("button", { name: "Techmeme" }).waitFor(); await page.waitForTimeout(300);
await row.getByRole("button", { name: "Techmeme" }).click(); await row.getByLabel("重命名 Techmeme").waitFor({ timeout: 5000 }).catch(() => row.getByRole("button", { name: "Techmeme" }).click());
await row.getByLabel("重命名 Techmeme").fill("   "); await row.getByLabel("重命名 Techmeme").press("Enter"); await page.waitForTimeout(400);
const emptyRejected = (await srcs()).some((s) => s.name === "Techmeme");
await row.getByLabel("重命名 Techmeme").press("Escape"); await page.waitForTimeout(200);
await row.getByRole("button", { name: "操作" }).click(); await page.getByRole("menuitem", { name: "重命名" }).click();
await row.getByLabel("重命名 Techmeme").fill("Techmeme 科技头条"); await row.getByLabel("重命名 Techmeme").press("Enter"); await page.waitForTimeout(800);
rec("rename: empty rejected, menu rename saved", emptyRejected && (await srcs()).some((s) => s.id === tm.id && s.name === "Techmeme 科技头条"));
const long = await fetch(B + `/api/sources/${tm.id}`, { method: "PATCH", headers: J, body: JSON.stringify({ name: "x".repeat(81) }) });
rec("rename: >80 chars rejected by API", long.status === 400);
execSync("bash " + RESTART + "", { stdio: "ignore" }); await new Promise((r) => setTimeout(r, 1500));
const after = (await srcs()).filter((s) => /techmeme/i.test(s.name));
rec("rename survives restart (seed sync keeps it, no duplicate)", after.length === 1 && after[0].name === "Techmeme 科技头条", after.map((s) => s.name).join(","));
const items = await api(`/api/items?limit=5&source=${tm.id}`); const it = (items.items ?? items)[0];
rec("renamed name appears in feed items", !it || it.source === "Techmeme 科技头条", it?.source);
await page.goto(B + "/sources"); await page.waitForTimeout(800);
await page.getByRole("button", { name: "导出" }).click(); await page.getByRole("menuitemradio", { name: /全部来源/ }).click();
const [dl0] = await Promise.all([page.waitForEvent("download"), page.getByRole("menuitem", { name: /JSON/ }).click()]);
await dl0.saveAs("/tmp/t8-exp.json"); await page.keyboard.press("Escape").catch(() => {});
rec("renamed name appears in export", JSON.parse(fs.readFileSync("/tmp/t8-exp.json", "utf8")).sources.some((s) => s.name === "Techmeme 科技头条"));

// 3) sidebar 订阅源 folders: open by default, each independently collapsible, persisted; source click filters; unsubscribe from folder
await page.goto(B + "/"); await page.waitForTimeout(1200);
const side = page.getByRole("complementary");
await side.getByRole("button", { name: "展开数据源" }).click(); await page.waitForTimeout(300);
const grp = side.getByRole("group", { name: "视频生成 的订阅源" }), xgrp = side.getByRole("group", { name: "X 快讯 的订阅源" });
const openDefault = await grp.count() > 0 && await xgrp.count() > 0;
await side.getByRole("button", { name: "文件夹：视频生成" }).click(); await page.waitForTimeout(400);
const indep = await grp.count() === 0 && await xgrp.count() > 0;
await shot("sidebar_folders");
await page.reload(); await page.waitForTimeout(1200);
const persisted = await grp.count() === 0 && await xgrp.count() > 0;
rec("sidebar folders: open by default, independent collapse, persisted", openDefault && indep && persisted);
rec("sidebar: top section rows have no chevrons", await side.getByRole("button", { name: /^(展开|收起)「/ }).count() === 0);
await side.getByRole("button", { name: "文件夹：视频生成" }).click(); await page.waitForTimeout(400);
await grp.getByRole("button", { name: /Techmeme 科技头条/ }).first().click(); await page.waitForTimeout(600);
rec("sidebar folders: source click filters feed", page.url().includes(`source=${tm.id}`));
await side.getByRole("button", { name: /^视频生成/ }).first().click(); await page.waitForTimeout(500);
rec("sidebar: clicking section still filters", page.url().includes(`section=${key}`));
{ const u = xgrp.getByRole("button", { name: /^取消订阅 / }).first(); const label = (await u.getAttribute("aria-label")).replace("取消订阅 ", "");
  await u.click({ force: true }); await page.getByRole("alertdialog").getByRole("button", { name: "取消订阅" }).click(); await page.waitForTimeout(1200);
  const off = (await srcs()).find((s) => s.type.startsWith("x") && label.includes(s.name.replace(/^X @/, "").split(" ")[0]));
  await page.getByRole("button", { name: "撤销" }).click(); await page.waitForTimeout(1200);
  const on = (await srcs()).find((s) => s.id === off?.id);
  rec("sidebar folders: unsubscribe + undo from folder", off && !off.enabled && on?.enabled, label); }

// 4) delete from sidebar ⚙: warning + count + backup download, cancel path, then confirm with move
await side.getByRole("button", { name: /^视频生成/ }).first().hover();
{ const bb = await side.getByRole("button", { name: "管理分区「视频生成」" }).boundingBox(); await page.mouse.click(bb.x + bb.width / 2, bb.y + bb.height / 2); }
await sd.waitFor();
await sd.getByRole("button", { name: "删除分区「视频生成」" }).click();
const ad = page.getByRole("alertdialog"); await ad.waitFor();
const warn = await ad.getByTestId("delete-warning").innerText();
const [dl] = await Promise.all([page.waitForEvent("download"), ad.getByRole("button", { name: /先导出备份/ }).click()]);
await dl.saveAs("/tmp/t8-backup.json"); const bk = JSON.parse(fs.readFileSync("/tmp/t8-backup.json", "utf8"));
await page.waitForTimeout(300); await shot("delete_dialog");
rec("delete dialog: irreversible warning + affected count + backup JSON downloaded", /无法撤销/.test(warn) && /影响 1 个来源/.test(warn) && bk.sources.length >= 22 && bk.sections?.some((x) => x.name === "视频生成"), dl.suggestedFilename() + " " + warn.replace(/\s+/g, " ") + " n=" + bk.sources.length);
await ad.getByRole("button", { name: "取消" }).click(); await page.waitForTimeout(400);
rec("delete dialog: cancel keeps section", (await api("/api/sections")).some((x) => x.key === key));
await sd.getByRole("button", { name: "删除分区「视频生成」" }).click(); await ad.waitFor();
await ad.getByRole("combobox", { name: "分区" }).click(); await page.getByRole("option", { name: "国外媒体" }).click();
await ad.getByRole("button", { name: "删除" }).click(); await page.waitForTimeout(1000);
const t3 = (await srcs()).find((s) => s.id === tm.id);
rec("delete confirm: section gone, sources moved", !(await api("/api/sections")).some((x) => x.key === key) && t3.section === "media");
rec("former built-in sections are deletable/manageable too", await sd.getByRole("button", { name: /删除分区「(实验室|娱乐)」/ }).count() === 2);
await page.keyboard.press("Escape");
// restore name
await fetch(B + `/api/sources/${tm.id}`, { method: "PATCH", headers: J, body: JSON.stringify({ name: "Techmeme" }) });
const errs = errors.filter((e) => !/status of (400|409)|ERR_INCOMPLETE_CHUNKED_ENCODING/.test(e));
rec("no console errors (t8)", errs.length === 0, errs.slice(0, 2).join(" | "));
await br.close();
