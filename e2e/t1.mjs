import { launch, exportFile, rec, results, srcs } from "./lib.mjs";
import { XMLParser, XMLValidator } from "fast-xml-parser";
import fs from "fs";
const all = await srcs(); const en = all.filter((s) => s.enabled);
const { br, page, errors } = await launch(false);
const fnRe = (e) => new RegExp(`^ai-news-radar-sources-\\d{8}-\\d{4}\\.${e}$`);
const KEY = process.env.DEEPSEEK_API_KEY || "\u0000none"; const bad = { test: (t) => t.includes(KEY) || /DEEPSEEK_API_KEY|sk-[A-Za-z0-9]{10}|last_error|item_count|"id"|token|password|\/home\/|\/workspace\//i.test(t) };
function checkJson(f, n, label) {
  const j = JSON.parse(f.text);
  const keys = new Set(j.sources.flatMap((s) => Object.keys(s)));
  const okKeys = [...keys].every((k) => ["name","type","url","username","homepage","section","priority","interval","enabled"].includes(k));
  rec(`export JSON ${label}`, fnRe("json").test(f.name) && j.app === "ai-news-radar" && j.version === 2 && j.count === n && j.sources.length === n && okKeys && !bad.test(f.text) && !j.items, `${f.name} n=${j.sources.length}/${n}`);
  return j;
}
function checkOpml(f, n, label) {
  const v = XMLValidator.validate(f.text);
  const d = new XMLParser({ ignoreAttributes: false, attributeNamePrefix: "", isArray: (x) => x === "outline" }).parse(f.text);
  const leaves = (d.opml.body.outline || []).flatMap((g) => g.outline || []);
  const okLeaves = leaves.every((o) => o.text && (o.type === "x" ? o.xUsername : o.xmlUrl));
  rec(`export OPML ${label}`, v === true && d.opml.version === "2.0" && d.opml.head?.title && leaves.length === n && okLeaves && fnRe("opml").test(f.name) && !bad.test(f.text), `${f.name} n=${leaves.length}/${n}`);
}
fs.writeFileSync("/tmp/export-all.json", (await (async () => { const f = await exportFile(page, "全部来源", "json"); checkJson(f, all.length, "all"); return f.text; })()));
checkOpml(await exportFile(page, "全部来源", "opml"), all.length, "all");
fs.writeFileSync("/tmp/export-all.opml", fs.readFileSync((await exportFile(page, "全部来源", "opml")).path));
checkJson(await exportFile(page, "仅启用", "json"), en.length, "enabled");
checkOpml(await exportFile(page, "仅启用", "opml"), en.length, "enabled");
// current filter: status 全部 + section 实验室
await page.getByRole("group", { name: "按状态筛选" }).getByRole("button", { name: /^全部/ }).click();
await page.getByRole("combobox").filter({ hasText: "全部分区" }).click();
await page.getByRole("option", { name: "实验室" }).click(); await page.waitForTimeout(300);
const nLab = all.filter((s) => s.section === "lab").length;
const jv = checkJson(await exportFile(page, "当前筛选", "json"), nLab, "current filter (实验室, 全部)");
rec("export filter rows are lab only", jv.sources.every((s) => s.section === "lab"));
checkOpml(await exportFile(page, "当前筛选", "opml"), nLab, "current filter");
await page.getByRole("checkbox", { name: /^选择 / }).nth(0).click(); await page.getByRole("checkbox", { name: /^选择 / }).nth(1).click();
checkJson(await exportFile(page, "已选中的行", "json"), 2, "selected (2)");
checkOpml(await exportFile(page, "已选中的行", "opml"), 2, "selected");
rec("desktop export: no console errors", errors.length === 0, errors.join(" | ").slice(0, 300));
await br.close();
fs.writeFileSync("/tmp/r1.json", JSON.stringify(results));
