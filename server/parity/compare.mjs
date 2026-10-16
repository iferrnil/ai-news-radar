// Parity check: the same requests against the Node build (A) and the Go server (B), both on fresh DBs fed by the mock upstream.
// Volatile values (timestamps, latencies, ids) are masked; keys, key order, types and the rest must match.
//   node server/parity/compare.mjs http://localhost:3091 http://localhost:3090
const [A, B] = process.argv.slice(2);
const VOL = /^(id|ts|created|last_fetched|latest|published|h_last_ok|h_lat|h_err_at|h_open|lastPrune|at|source_id|item_id|until|key)$/;
const mask = (v, k = "") => {
  if (Array.isArray(v)) return v.map((x) => mask(x));
  if (v && typeof v === "object") return Object.fromEntries(Object.entries(v).map(([kk, x]) => [kk, mask(x, kk)]));
  if (VOL.test(k) && v != null) return typeof v === "number" ? "<num>" : "<str>";
  return v;
};
async function call(base, [method, path, body]) {
  const r = await fetch(base + path, { method, headers: body ? { "content-type": "application/json" } : {}, body: body ? JSON.stringify(body) : undefined });
  const t = await r.text(); let j; try { j = JSON.parse(t); } catch { j = t.slice(0, 80); }
  return { status: r.status, ctype: (r.headers.get("content-type") ?? "").split(";")[0], body: mask(j) };
}
const cases = [
  ["GET", "/api/health"], ["GET", "/api/sections"], ["GET", "/api/settings"], ["GET", "/api/read"], ["POST", "/api/read", { add: [1] }],
  ["GET", "/api/stats"], ["GET", "/api/sources/deleted"],
  ["GET", "/api/items?limit=3"], ["GET", "/api/items?limit=2&stype=x"], ["GET", "/api/items?q=update&limit=2"], ["GET", "/api/items?id=1"],
  ["POST", "/api/sources/rule-check", { exclude: "^(a|a)*$" }], ["POST", "/api/sources/rule-check", { include: "(x" }],
  ["POST", "/api/sources/rule-check", { include: "(a+)+" }], ["POST", "/api/sources/rule-check", { include: "\\1" }],
  ["POST", "/api/sources/rule-check", { exclude: "#[24]", samples: ["a #2", "b #3", "c #4"] }],
  ["POST", "/api/sources/rule-check", { include: "(?<=x)y", samples: ["xy", "zy"] }],
  ["POST", "/api/sources/test", { input: "https://news.smol.ai/rss.xml" }], ["POST", "/api/sources/test", { input: "@OpenAI" }],
  ["POST", "/api/sources/test", { input: "ftp://x" }], ["POST", "/api/sources/test", { input: "https://example.invalid/none" }],
  ["POST", "/api/sources/import", { entries: [{ name: "Smol", type: "rss", url: "https://news.smol.ai/rss.xml" }, { name: "Bad", type: "rss", url: "javascript:x" }, { type: "x" }, { name: "dup", type: "rss", url: "https://openai.com/news/rss.xml" }, 5], mode: "merge" }],
  ["POST", "/api/sources/import", { entries: [{ name: "Smol", type: "rss", url: "https://news.smol.ai/rss.xml", section: "c99", sectionName: "新区" }], mode: "replace" }],
  ["POST", "/api/sources/import", { entries: "x" }], ["POST", "/api/sources/import", { entries: [], mode: "zzz" }],
  ["POST", "/api/sections", { name: "" }], ["POST", "/api/sections", { name: "全部" }], ["POST", "/api/sections", { name: "测试区", icon: "🎬" }],
  ["PATCH", "/api/sections/nope", { name: "x" }], ["PATCH", "/api/sections/lab", { name: "国内媒体" }], ["DELETE", "/api/sections/lab"],
  ["PATCH", "/api/sources/99999", { enabled: false }], ["DELETE", "/api/sources/99999"], ["POST", "/api/sources/99999/fetch"],
  ["PATCH", "/api/sources/1", { name: "x".repeat(81) }], ["PATCH", "/api/sources/1", { rule: { include: "(" } }],
  ["POST", "/api/sources/bulk", { action: "x" }],
  ["PATCH", "/api/settings", { deepseek: { key: "short" } }], ["PATCH", "/api/settings", { deepseek: { baseUrl: "http://evil.com" } }],
  ["PATCH", "/api/settings", { deepseek: { model: "bad model!" } }], ["POST", "/api/settings/test", {}], ["POST", "/api/settings/test", { key: "bad" }],
  ["POST", "/api/settings/notify-test"],
  ["PATCH", "/api/settings", { notify: { enabled: true, sections: ["lab", 5, "x".repeat(70)] } }], ["GET", "/api/settings"],
  ["POST", "/api/sources", { input: "https://news.smol.ai/rss.xml", section: "lab", rule: { exclude: "not much" } }],
  ["GET", "/api/sources"], ["GET", "/api/sections"],
  ["PATCH", "/api/sources/1", { rule: { exclude: "update #1" }, priority: "P2", poll_interval: 10 }],
  ["POST", "/api/sources/1/fetch"], ["DELETE", "/api/sources/2"], ["GET", "/api/sources/deleted"], ["POST", "/api/sources/deleted", { names: "all" }],
  ["POST", "/api/sources/bulk", { action: "delete", ids: [3, "4", "zz"] }], ["GET", "/api/sources"], ["GET", "/api/nope"],
];
let diff = 0;
for (const c of cases) {
  const [a, b] = await Promise.all([call(A, c), call(B, c)]);
  const sa = JSON.stringify(a), sb = JSON.stringify(b);
  if (sa !== sb) {
    diff++;
    console.log(`DIFF ${c[0]} ${c[1]} ${c[2] ? JSON.stringify(c[2]).slice(0, 80) : ""}`);
    // show first differing position
    let i = 0; while (i < sa.length && sa[i] === sb[i]) i++;
    console.log("  node:", sa.slice(Math.max(0, i - 120), i + 200));
    console.log("  go  :", sb.slice(Math.max(0, i - 120), i + 200));
  } else console.log(`same ${c[0]} ${c[1]}`);
}
console.log(`== ${cases.length - diff}/${cases.length} identical`);
