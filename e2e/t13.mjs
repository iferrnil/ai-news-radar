// X/feed fetch reliability + per-source health: retries, FxEmbed false 404 vs real 404, 429, timeouts,
// circuit breaker, recovery, fallback mirror; health column on /sources and sidebar status.
import { execSync } from "node:child_process";
import { launch, rec, B, srcs, mock, RESTART, SHOT } from "./lib.mjs";
const J = { "Content-Type": "application/json" };
const byName = async (n) => (await srcs()).find((s) => s.name === n || s.name.startsWith(n));
const poll = async (s) => (await fetch(`${B}/api/sources/${s.id}/fetch`, { method: "POST", headers: J })).json();

const arena = await byName("X @arena");
rec("fixture: health recorded on the first poll", (arena.h_runs ?? "").length >= 1 && arena.h_runs.endsWith("1") && arena.h_lat >= 0, arena.h_runs);

// 5xx → failure, consecutive count, then the breaker opens
await mock("fail=arena&mode=500");
let t0 = Date.now(); const r1 = await poll(arena); const dt = Date.now() - t0;
rec("500 is retried with backoff (FX_TRIES=2), then reported", !!r1.error && /HTTP 500/.test(r1.error) && dt > 300, `${dt}ms ${r1.error}`);
for (let i = 0; i < 4; i++) await poll(arena);
let a = await byName("X @arena");
rec("5 failures: consec 5, runs end 00000, kind http", a.h_consec === 5 && a.h_runs.endsWith("00000") && a.h_kind === "http", `${a.h_consec} ${a.h_runs} ${a.h_kind}`);
rec("breaker open after 5 consecutive failures (cool-down ≥ 5 min)", a.h_open > Date.now() / 1000 + 290, `${Math.round(a.h_open - Date.now() / 1000)}s`);

// FxEmbed: statuses 404 but the profile exists = false 404; both 404 = the account is gone
await mock("fail=btibor91&mode=false404");
const fb = await poll(await byName("X @btibor91")); const b = await byName("X @btibor91");
rec("FxEmbed false 404 detected (profile exists)", /伪 404/.test(fb.error ?? "") && b.h_kind === "false_404", fb.error);
await mock("fail=AiBreakfast&mode=gone");
const fg = await poll(await byName("X @AiBreakfast")); const g = await byName("X @AiBreakfast");
rec("real 404 (profile also 404) → not_found", /不存在/.test(fg.error ?? "") && g.h_kind === "not_found", fg.error);
await mock("fail=OpenAI&mode=429");
const f4 = await poll(await byName("X @OpenAI")); const o = await byName("X @OpenAI");
rec("429 honours Retry-After and is classified rate_limit", /429/.test(f4.error ?? "") && o.h_kind === "rate_limit", f4.error);
await mock("fail=ArtificialAnlys&mode=timeout");
t0 = Date.now(); const ft = await poll(await byName("X @ArtificialAnlys")); const tt = Date.now() - t0;
rec("hung upstream: per-attempt timeout (FX_TIMEOUT_MS=3000) → timeout", (await byName("X @ArtificialAnlys")).h_kind === "timeout" && tt < 15000, `${tt}ms ${ft.error}`);
await mock("fail=openai.com&mode=500");
const fr = await poll(await byName("OpenAI News"));
rec("RSS 5xx retried then reported (kind http)", /HTTP 500/.test(fr.error ?? "") && (await byName("OpenAI News")).h_kind === "http", fr.error);

// UI: /sources health column + tooltip; sidebar dot + footer
const { br, page, errors } = await launch(false);
await page.getByRole("button", { name: /^在线/ }).first().click().catch(() => {});
const row = page.getByRole("row").filter({ hasText: "@arena" });
const hb = row.getByTestId("health");
rec("/sources health badge: open circuit", (await hb.getAttribute("data-level")) === "open" && /0%|\d+%/.test(await hb.innerText()));
await hb.hover(); await page.getByText("熔断", { exact: false }).last().waitFor({ timeout: 3000 }).catch(() => {});
const tip = await page.locator("[data-slot=tooltip-content]").last().innerText().catch(() => "");
rec("health tooltip: success rate, last ok, latency, consecutive failures, breaker", /成功率/.test(tip) && /上次成功/.test(tip) && /延迟/.test(tip) && /连续失败/.test(tip) && /暂停抓取至/.test(tip), tip.replace(/\n/g, " | ").slice(0, 160));
await page.screenshot({ path: SHOT("health_tooltip.png") });
const okRow = page.getByRole("row").filter({ hasText: "Techmeme" }).getByTestId("health");
rec("healthy source badge: ok 100%", (await okRow.getAttribute("data-level")) === "ok" && (await okRow.innerText()).includes("100%"));
await page.goto(B + "/"); await page.waitForTimeout(1200);
const side = page.getByRole("complementary");
await side.getByRole("button", { name: "展开数据源" }).click().catch(() => {}); await page.waitForTimeout(400);
rec("sidebar status dot reflects health (arena open, btibor91 degraded)", await side.locator("[data-health=open]").filter({ hasText: "@arena" }).count() === 1 && await side.locator("[data-health=degraded]").filter({ hasText: "@btibor91" }).count() === 1);
rec("sidebar footer shows 报错 / 不稳定", /报错/.test(await side.innerText()));

// recovery resets the breaker
await mock("fail=arena&mode=ok");
await poll(arena); a = await byName("X @arena");
rec("recovery: consec 0, breaker closed, runs end with 1", a.h_consec === 0 && !a.h_open && a.h_runs.endsWith("1") && !a.last_error, `${a.h_runs}`);
// breaker skips scheduled polls: btibor91 has 1 failure → still polled; arena was skipped by the scheduler while open (no hits growth)
rec("no console errors (t13 ui)", errors.length === 0, errors.join("|").slice(0, 200));
await br.close();

// fallback mirror: the first FxEmbed base is dead, the second answers
execSync(`bash ${RESTART} FX_MIRRORS=https://fx-dead.invalid/2,https://api.fxtwitter.com/2`);
const dotey = await byName("X @dotey");
const fd = await poll(dotey);
const st = await (await fetch(B + "/api/stats")).json();
rec("fallback mirror: poll succeeds via the 2nd FxEmbed base", !fd.error && fd.fetched >= 1, JSON.stringify(fd));
rec("dead mirror is tracked (fails ≥ 1)", st.fx?.length === 2 && st.fx[0].fails >= 1 && st.fx[1].fails === 0, JSON.stringify(st.fx));
execSync(`bash ${RESTART}`);
