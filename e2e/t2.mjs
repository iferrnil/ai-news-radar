import { launch, rec, results, srcs, deleted, openImport, waitPlan, counts, setMode, applyImport } from "./lib.mjs";
import fs from "fs";
const snap = (l) => Object.fromEntries(l.map((s) => [s.name, JSON.stringify([s.type, s.type === "x" ? s.username : s.url, s.section, s.priority, s.poll_interval, !!s.enabled])]));
const before = snap(await srcs());
const { br, page, errors } = await launch(false);
const sleep = (ms) => page.waitForTimeout(ms);
const closeDlg = async () => { await page.keyboard.press("Escape"); await sleep(400); };
// --- delete all via UI
await page.getByRole("group", { name: "按状态筛选" }).getByRole("button", { name: /^全部/ }).click(); await sleep(300);
await page.getByRole("checkbox", { name: "全选当前列表" }).click();
await page.getByRole("button", { name: /删除所选/ }).click();
await page.getByRole("alertdialog").getByRole("button", { name: "删除" }).click(); await sleep(1500);
const afterDel = await srcs(); const tomb = await deleted();
rec("delete all via UI", afterDel.length === 0 && tomb.length === 22, `left=${afterDel.length} tomb=${tomb.length}`);
// --- JSON round trip
let dlg = await openImport(page, "/tmp/export-all.json"); await waitPlan(dlg);
let c = await counts(dlg); rec("round-trip plan: 22 恢复", c["恢复"] === 22, JSON.stringify(c));
await applyImport(dlg); await sleep(2500);
const after = snap(await srcs());
const diff = Object.keys({ ...before, ...after }).filter((k) => before[k] !== after[k]);
rec("JSON round-trip identical settings", diff.length === 0 && Object.keys(after).length === 22, diff.map((k) => `${k}: ${before[k]} -> ${after[k]}`).join("; "));
rec("deleted built-in list empty after round-trip", (await deleted()).length === 0);
// --- re-import same file
dlg = await openImport(page, "/tmp/export-all.json"); await waitPlan(dlg); c = await counts(dlg);
const btn = dlg.getByRole("button", { name: /没有需要导入的内容/ });
rec("re-import: all existing, nothing to import", c["已存在"] === 22 && await btn.isDisabled(), JSON.stringify(c)); await closeDlg();
// --- OPML external
dlg = await openImport(page, "/tmp/ext.opml"); await waitPlan(dlg); c = await counts(dlg);
const okC = c["新增"] === 4 && c["已存在"] === 1 && c["无效"] === 1;
rec("OPML external: 4 new / 1 dup / 1 invalid", okC, JSON.stringify(c));
await applyImport(dlg); await sleep(15000);
const l = await srcs(); const g = (n) => l.find((s) => s.name === n);
rec("OPML section mapping (folder 国内媒体->cn, category 实验室->lab, default->media)", g("IT之家")?.section === "cn" && g("Simon Willison")?.section === "lab" && g("Hacker News")?.section === "media" && g("The Verge AI")?.section === "media", ["IT之家","Simon Willison","Hacker News","The Verge AI"].map((n) => `${n}:${g(n)?.section}`).join(","));
const nf = ["IT之家","Simon Willison","Hacker News","The Verge AI"].map(g);
rec("OPML test-fetch after import", nf.every((s) => s?.last_fetched && !s.last_error), nf.map((s) => `${s?.name}:${s?.last_fetched ? "fetched" : "no"}${s?.last_error ? " err=" + s.last_error : ""} items=${s?.item_count ?? 0}`).join("; "));
// --- update mode
dlg = await openImport(page, "/tmp/update.json"); await waitPlan(dlg);
c = await counts(dlg); rec("merge mode: existing shown as 已存在", c["已存在"] === 1, JSON.stringify(c));
await setMode(dlg, "合并并更新"); c = await counts(dlg); await applyImport(dlg); await sleep(2000);
let o = (await srcs()).find((s) => s.name === "OpenAI News");
rec("merge-and-update updates existing settings", o.priority === "P2" && o.poll_interval === 1800 && !o.enabled && o.section === "media", `${JSON.stringify(c)} -> ${o.priority}/${o.poll_interval}/${o.enabled}/${o.grp}`);
// --- replace mode
const n0 = (await srcs()).length;
dlg = await openImport(page, "/tmp/subset.json"); await waitPlan(dlg); await setMode(dlg, "覆盖");
c = await counts(dlg); await applyImport(dlg);
const ad = page.getByRole("alertdialog", { name: "覆盖本地订阅？" }); await ad.waitFor();
rec("overwrite: no deletion before 2nd confirm", (await srcs()).length === n0, `remove pill=${c["删除"]}`);
await ad.getByRole("button", { name: "取消" }).click(); await sleep(500);
rec("overwrite: cancel keeps all", (await srcs()).length === n0);
await applyImport(dlg); await ad.waitFor(); await ad.getByRole("button", { name: "确认覆盖" }).click(); await sleep(2500);
const ls = await srcs();
rec("overwrite: only file sources remain after confirm", ls.length === 3, `${n0} -> ${ls.length}`);
// restore everything via import of export-all (built-ins restore)
dlg = await openImport(page, "/tmp/export-all.json"); await waitPlan(dlg); c = await counts(dlg);
rec("import of deleted built-ins shows 恢复", c["恢复"] === 19, JSON.stringify(c));
await applyImport(dlg); await sleep(2500);
rec("deleted built-ins restored", (await srcs()).length === 22 && (await deleted()).length === 0, `n=${(await srcs()).length}`);
// single delete + restore by import
await page.reload(); await page.getByRole("heading", { name: "数据源" }).waitFor();
await page.getByRole("group", { name: "按状态筛选" }).getByRole("button", { name: /^全部/ }).click();
await page.getByRole("row").filter({ hasText: "Techmeme" }).getByRole("button", { name: "操作" }).click();
await page.getByRole("menuitem", { name: "删除" }).click(); await page.getByRole("alertdialog").getByRole("button", { name: "删除" }).click(); await sleep(1200);
const t1 = await deleted();
dlg = await openImport(page, "/tmp/export-all.json"); await waitPlan(dlg); c = await counts(dlg); await applyImport(dlg); await sleep(2000);
rec("single deleted built-in restored by import", t1.some((t) => t.name === "Techmeme") && (await srcs()).some((s) => s.name === "Techmeme") && (await deleted()).length === 0, JSON.stringify(c));
// --- edge cases
dlg = await openImport(page, "/tmp/bad-urls.json"); await waitPlan(dlg); c = await counts(dlg);
rec("javascript:/file: URLs invalid", c["无效"] === 2, JSON.stringify(c)); await closeDlg();
dlg = await openImport(page, "/tmp/big.json"); await sleep(800);
let alert = await dlg.getByRole("alert").innerText().catch(() => "");
rec("oversized file (>1MB) rejected", /超过 1 MB/.test(alert), alert); await closeDlg();
dlg = await openImport(page, "/tmp/malformed.json"); await sleep(800);
alert = await dlg.getByRole("alert").innerText().catch(() => "");
rec("malformed JSON clear error", /JSON 解析失败/.test(alert), alert); await closeDlg();
// server-side direct checks
const r = await fetch("http://localhost:3000/api/sources/import", { method: "POST", body: JSON.stringify({ entries: [{ name: "x", type: "rss", url: "javascript:alert(1)" }], dryRun: true }) }).then((r) => r.json());
rec("server re-validates javascript: URL", r.counts?.invalid === 1);
rec("desktop import: no console errors", errors.length === 0, errors.join(" | ").slice(0, 400));
await br.close();
fs.writeFileSync("/tmp/r2.json", JSON.stringify(results));
