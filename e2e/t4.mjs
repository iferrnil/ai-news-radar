import { launch, rec, srcs, api, B } from "./lib.mjs";
const all = await srcs();
const { br, page, errors } = await launch(false);
const g = page.getByRole("group", { name: "按状态筛选" });
const exp = { "在线": all.filter((s) => s.enabled && s.supported).length, "报错": all.filter((s) => s.enabled && s.supported && s.last_error).length, "停用": all.filter((s) => !s.enabled).length, "暂停": all.filter((s) => s.enabled && !s.supported).length, "全部": all.length };
for (const [k, n] of Object.entries(exp)) {
  const b = g.getByRole("button", { name: new RegExp("^" + k) }); await b.click(); await page.waitForTimeout(250);
  const badge = await b.locator("[data-slot=badge]").innerText(); const rows = await page.getByRole("checkbox", { name: /^选择 / }).count();
  rec(`status filter ${k}: badge=${n}, rows=${n}, pressed`, Number(badge) === n && rows === n && (await b.getAttribute("aria-pressed")) === "true", `badge ${badge} rows ${rows}`);
}
const others = await g.getByRole("button").evaluateAll((bs) => bs.filter((b) => b.getAttribute("aria-pressed") === "true").length);
rec("only one status pressed", others === 1);
const hs = await g.getByRole("button").evaluateAll((bs) => [...new Set(bs.map((b) => Math.round(b.getBoundingClientRect().height) + "/" + getComputedStyle(b).fontWeight))]);
rec("status buttons uniform height/weight", hs.length === 1, hs.join(","));
// feed
await page.goto(B + "/"); await page.waitForSelector("[data-slot=card]"); await page.waitForTimeout(800);
const p0 = page.locator("main button[aria-pressed]").first();
await p0.click(); await page.waitForTimeout(300); const onAll = await p0.getAttribute("aria-pressed");
rec("P0 toggle on in 全部 (URL p0=1)", onAll === "true" && page.url().includes("p0=1"), page.url());
await page.getByRole("complementary").getByRole("button", { name: /^娱乐/ }).first().click(); await page.waitForTimeout(500);
const onEnt = await page.locator("main button[aria-pressed]").first().getAttribute("aria-pressed");
await page.getByRole("complementary").getByRole("button", { name: /^全部/ }).first().click(); await page.waitForTimeout(500);
const back = await page.locator("main button[aria-pressed]").first().getAttribute("aria-pressed");
rec("P0 per section (娱乐 off, 全部 restored on)", onEnt === "false" && back === "true", `${onEnt}/${back}`);
await page.locator("main button[aria-pressed]").first().click(); await page.waitForTimeout(300);
const entCards = await page.locator("[data-slot=card]").filter({ hasText: "娱乐" }).count();
rec("娱乐 excluded from 全部", entCards === 0, `ent cards in 全部: ${entCards}`);
// unsubscribe + undo via sidebar
await page.getByRole("complementary").getByRole("button", { name: "展开数据源" }).click(); await page.waitForTimeout(300); // 数据源 group is collapsed by default
const row = page.getByRole("button", { name: /^取消订阅 / }).first(); const label = (await row.getAttribute("aria-label")).replace("取消订阅 ", "");
await row.click({ force: true }); await page.getByRole("alertdialog").getByRole("button", { name: "取消订阅" }).click(); await page.waitForTimeout(1200);
const off = (await srcs()).find((s) => label.includes(s.name.replace(/^X @/, "").split(" ")[0]) || s.name.includes(label));
await page.getByRole("button", { name: "撤销" }).click(); await page.waitForTimeout(1200);
const on = (await srcs()).find((s) => s.id === off?.id);
rec("sidebar unsubscribe + undo", off && !off.enabled && on.enabled, `${label}: ${off?.enabled}->${on?.enabled}`);
// drawer badges, palette
await page.keyboard.press("j"); await page.keyboard.press("Enter"); await page.waitForTimeout(600);
rec("drawer opens", await page.getByRole("dialog").count() > 0); await page.keyboard.press("Escape");
await page.keyboard.press("Control+k"); await page.locator("[data-slot=command-input]").waitFor(); await page.waitForTimeout(150); await page.keyboard.type("zzzzqqq"); await page.waitForTimeout(500);
await page.screenshot({ path: "/tmp/t4pal.png" }); rec("palette empty state", await page.getByText("没有结果").count() > 0); await page.keyboard.press("Escape");
// delete + restore via 已删除的内置源 dialog
await page.goto(B + "/sources"); await page.getByRole("heading", { name: "数据源" }).waitFor();
await page.getByRole("row").filter({ hasText: "Techmeme" }).getByRole("button", { name: "操作" }).click();
await page.getByRole("menuitem", { name: "删除" }).click(); await page.getByRole("alertdialog").getByRole("button", { name: "删除" }).click(); await page.waitForTimeout(1200);
await page.getByRole("button", { name: /已删除的内置源/ }).click(); await page.waitForTimeout(400);
await page.getByRole("dialog").getByRole("button", { name: /恢复/ }).first().click(); await page.waitForTimeout(1500);
rec("delete + restore built-in via dialog", (await srcs()).some((s) => s.name === "Techmeme") && (await api("/api/sources/deleted")).length === 0);
// add-source dialog error alert
await page.keyboard.press("Escape"); await page.waitForTimeout(500); await page.getByRole("button", { name: /添加数据源/ }).click(); await page.screenshot({ path: "/tmp/t4add.png" });
await page.getByRole("dialog").getByRole("textbox").first().fill("ftp://bad"); await page.getByRole("dialog").getByRole("button", { name: /检测|识别|添加|测试/ }).first().click(); await page.getByRole("dialog").getByRole("alert").waitFor({ timeout: 20000 }).catch(() => {});
rec("add dialog shows Alert on bad input", await page.getByRole("dialog").getByRole("alert").count() > 0);
const errs4 = errors.filter((e) => !/status of 400/.test(e)); // 400 = expected rejection of the bad add input
rec("no console errors (functional, except expected 400)", errs4.length === 0, errs4.join(" | ").slice(0, 300));
await br.close();
