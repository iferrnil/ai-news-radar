import { launch, rec, SHOT, B } from "./lib.mjs";
const { br, page, errors } = await launch(false);
await page.goto(B + "/"); await page.waitForSelector("[data-slot=card]"); await page.waitForTimeout(800);
await page.locator("body").click({ position: { x: 640, y: 30 } }).catch(() => {});
await page.getByRole("button", { name: /搜索新闻/ }).click(); await page.waitForTimeout(500); await page.keyboard.type("zzzzqqqxx"); await page.waitForTimeout(800);
await page.screenshot({ path: SHOT("palette_empty.png") });
rec("palette empty state", await page.getByText("没有结果").count() > 0); await page.keyboard.press("Escape");
await page.goto(B + "/sources"); await page.getByRole("heading", { name: "数据源" }).waitFor();
await page.getByRole("button", { name: /添加数据源/ }).click(); const d = page.getByRole("dialog"); await d.waitFor();
await d.getByRole("textbox").first().fill("not a url at all ###"); await d.getByRole("button", { name: "添加并订阅" }).click(); await d.locator("[data-slot=alert]").waitFor({ timeout: 20000 }).catch(() => {});
await page.screenshot({ path: SHOT("add_error.png") });
rec("add dialog shows Alert on bad input", await d.locator("[data-slot=alert]").count() > 0);
const errs5 = errors.filter((e) => !/status of 400/.test(e)); // 400 = expected server rejection of the bad add input
rec("no console errors (except expected 400)", errs5.length === 0, errs5.join("|").slice(0, 300));
await br.close();
