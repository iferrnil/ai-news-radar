// no raw `lucide:` icon tokens anywhere in the UI; exports keep them
import { launch, rec, B, exportFile, openImport, waitPlan, SHOT } from "./lib.mjs";
const { br, page, errors } = await launch(false);
const bad = [];
const check = async (where) => { const t = await page.evaluate(() => document.body.innerText + [...document.querySelectorAll("[aria-label],[title]")].map((e) => (e.getAttribute("aria-label") || "") + (e.getAttribute("title") || "")).join(" ")); if (/lucide:/.test(t)) bad.push(where); };
for (const theme of ["dark", "light"]) {
  await page.emulateMedia({ colorScheme: theme });
  for (const u of ["/", "/sources", "/docs", "/docs/rate-limits"]) { await page.goto(B + u); await page.waitForTimeout(1200); await check(`${theme} ${u}`); }
}
await page.goto(B + "/sources"); await page.waitForTimeout(1200);
// sources filter select
await page.getByRole("combobox").filter({ hasText: "全部分区" }).click(); await page.waitForTimeout(300); await check("sources section filter open");
await page.keyboard.press("Escape");
// edit dialog select (trigger + items)
const row = page.getByRole("row").filter({ hasText: "www.techmeme.com" });
await row.getByRole("button", { name: "操作" }).click(); await page.getByRole("menuitem", { name: "编辑", exact: true }).click();
const dlg = page.getByRole("dialog"); await dlg.waitFor(); await check("edit dialog");
await dlg.getByRole("combobox").first().click(); await page.waitForTimeout(400);
const opts = await page.getByRole("option").allInnerTexts(); await check("edit select open");
await page.screenshot({ path: SHOT("edit_select_open.png") });
rec("edit-dialog select items show names, not tokens", opts.some((o) => o.includes("实验室")) && !opts.some((o) => /lucide:/.test(o)), opts.join("|"));
await page.keyboard.press("Escape"); await page.keyboard.press("Escape"); await page.waitForTimeout(300);
// add dialog select
await page.getByRole("button", { name: /添加数据源/ }).click(); const ad = page.getByRole("dialog"); await ad.waitFor(); await check("add dialog");
await ad.getByRole("combobox").first().click().catch(() => {}); await page.waitForTimeout(300); await check("add select open");
await page.keyboard.press("Escape"); await page.keyboard.press("Escape"); await page.waitForTimeout(300);
// manage sections + delete/move-to dialog + icon picker preview
await page.getByRole("button", { name: "分区", exact: true }).click(); const sd = page.getByRole("dialog", { name: "管理分区" }); await sd.waitFor(); await check("manage sections");
await sd.getByRole("button", { name: "「实验室」的图标" }).click(); await page.waitForTimeout(500); await check("icon picker open");
await page.keyboard.press("Escape"); await page.waitForTimeout(200);
await sd.getByRole("button", { name: "删除分区「实验室」" }).click(); const al = page.getByRole("alertdialog"); await al.waitFor(); await check("delete dialog");
await al.getByRole("combobox").click(); await page.waitForTimeout(300); await check("move-to select open");
await page.keyboard.press("Escape"); await al.getByRole("button", { name: "取消" }).click(); await page.keyboard.press("Escape");
// export menu + exported JSON/OPML keep tokens
await page.goto(B + "/sources"); await page.waitForTimeout(1000);
await page.getByRole("button", { name: "导出" }).click(); await page.waitForTimeout(300); await check("export menu"); await page.keyboard.press("Escape");
const ej = await exportFile(page, "全部来源", "json"); const j = JSON.parse(ej.text);
const eo = await exportFile(page, "全部来源", "opml");
rec("JSON + OPML exports keep lucide:<key> icon tokens", j.sections.some((s) => s.icon === "lucide:lab") && /sectionIcon="lucide:lab"/.test(eo.text));
// import preview: re-import the export
const idl = await openImport(page, ej.path); await waitPlan(idl); await check("import preview");
await idl.getByRole("combobox").first().click().catch(() => {}); await page.waitForTimeout(300); await check("import select open");
await page.keyboard.press("Escape"); await page.keyboard.press("Escape");
// feed: palette, drawer
await page.goto(B + "/"); await page.waitForTimeout(1500);
await page.keyboard.press("Control+k"); await page.waitForTimeout(500); await check("palette");
await page.keyboard.press("Escape");
await page.locator("[data-slot=card]").first().click(); await page.waitForTimeout(800); await check("drawer");
rec("no visible lucide: token on any page / dialog / select / menu (dark+light)", bad.length === 0, bad.join(", "));
rec("no console errors (t10)", errors.filter((e) => !/status of (400|409)|ERR_INCOMPLETE/.test(e)).length === 0);
await br.close();
