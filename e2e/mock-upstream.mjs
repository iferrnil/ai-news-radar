// Deterministic stand-in for every upstream the poller talks to (RSS, JSON APIs, FxEmbed, X syndication, DeepSeek).
// The server is started with UPSTREAM_MOCK=http://127.0.0.1:<port>, which sends each request here as /?u=<url>.
// Control (tests only): GET /__ctl?add=<handle>[&title=]  → a new post on that X timeline
//                       GET /__ctl?fail=<handle|host>&mode=500|429|404|false404|gone|timeout|ok  → failure injection
//                       GET /__ctl?reset=1 · GET /__ctl?state=1
import http from "node:http";

const PORT = Number(process.env.MOCK_PORT ?? 4010);
const T0 = Math.floor(Date.now() / 1000);
const extra = new Map(); // handle -> [{id, text, ts}]
const fail = new Map();  // key -> mode
let seq = 1;
const hits = new Map();

const hash = (s) => [...s].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7);
const esc = (s) => s.replace(/[<>&]/g, (c) => ({ "<": "&lt;", ">": "&gt;", "&": "&amp;" })[c]);

function rss(u) {
  const host = u.hostname.replace(/^www\./, "");
  const zh = /qbitai|jiqizhixin/.test(host);
  const items = Array.from({ length: 6 }, (_, i) => {
    const ts = T0 - (i + 1) * 1800 - (hash(host) % 900);
    const title = host === "news.smol.ai" && i % 2 ? `[AINews] not much happened today #${i + 1}` // t7 rule preview
      : zh ? `${host} 中文资讯 第 ${i + 1} 条：大模型新进展` : `${host} update #${i + 1}: new model release notes`;
    return `<item><title>${esc(title)}</title><link>https://${host}/post-${i + 1}</link><guid>https://${host}/post-${i + 1}</guid>
      <pubDate>${new Date(ts * 1000).toUTCString()}</pubDate><description>${esc(`Summary of ${title}. Lorem ipsum dolor sit amet.`)}</description></item>`;
  }).join("\n");
  return `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>${esc(host)} feed</title><link>https://${host}/</link>${items}</channel></rss>`;
}

function fxPosts(handle) {
  const base = hash(handle.toLowerCase()) % 1e6;
  const posts = Array.from({ length: 3 }, (_, i) => ({ id: String(1e12 + base * 10 + i), text: `${handle} says: shipping update ${i + 1} https://example.com/${handle}/${i}`, ts: T0 - (i + 1) * 2400 }));
  for (const p of extra.get(handle.toLowerCase()) ?? []) posts.unshift(p);
  return posts.map((p) => ({ id: p.id, url: `https://x.com/${handle}/status/${p.id}`, text: p.text, created_timestamp: p.ts, author: { screen_name: handle, name: handle } }));
}

function api(u) {
  const s = u.href;
  if (s.includes("jiqizhixin.com")) return { articles: Array.from({ length: 4 }, (_, i) => ({ slug: `mock-${i}`, title: `机器之心 模拟文章 ${i + 1}`, content: "内容摘要", author: "机器之心", publishedAt: new Date((T0 - i * 3600 + 8 * 3600) * 1000).toISOString().replace("T", " ").slice(0, 16).replace(/-/g, "/") })) };
  if (s.includes("ent.163.com")) return Array.from({ length: 4 }, (_, i) => ({ docurl: `https://ent.163.com/mock/${i}.html`, title: `网易娱乐 模拟 ${i + 1}`, time: (() => { const d = new Date((T0 - i * 3600 + 8 * 3600) * 1000); const p = (n) => String(n).padStart(2, "0"); return `${p(d.getUTCMonth() + 1)}/${p(d.getUTCDate())}/${d.getUTCFullYear()} ${p(d.getUTCHours())}:${p(d.getUTCMinutes())}:00`; })(), keywords: [{ keyname: "明星" }] }));
  if (s.includes("bilibili.com")) return { data: { list: Array.from({ length: 4 }, (_, i) => ({ bvid: `BVmock${i}`, title: `B站热门 模拟 ${i + 1}`, owner: { name: "UP" }, desc: "简介", pubdate: T0 - i * 3600, pic: "" })) } };
  if (s.includes("hotsearch/billboard")) return { word_list: Array.from({ length: 5 }, (_, i) => ({ word: `热搜词${i + 1}`, hot_value: 1000 - i })) };
  if (s.includes("jandan.net")) return { data: Array.from({ length: 4 }, (_, i) => ({ id: 900 + i, content: `煎蛋 模拟 ${i + 1}`, author: "蛋友", date: new Date((T0 - i * 3600) * 1000).toISOString(), vote_positive: 10, vote_negative: 1, images: [] })) };
  if (s.includes("hn.algolia.com")) return { hits: [] };
  if (s.includes("huggingface.co")) return [];
  if (s.includes("openrouter.ai")) return { data: [] };
  return null;
}

function send(res, status, body, type = "application/json") {
  res.writeHead(status, { "Content-Type": type, "Cache-Control": "no-store" });
  res.end(typeof body === "string" ? body : JSON.stringify(body));
}

http.createServer((req, res) => {
  const me = new URL(req.url, "http://mock");
  if (me.pathname === "/__ctl") {
    const q = me.searchParams;
    if (q.get("reset")) { extra.clear(); fail.clear(); hits.clear(); }
    if (q.get("add")) { const h = q.get("add").toLowerCase(); const id = String(2e12 + seq++); extra.set(h, [...(extra.get(h) ?? []), { id, text: q.get("title") ?? `${q.get("add")} breaking: mock news ${id}`, ts: Math.floor(Date.now() / 1000) - Number(q.get("age") ?? 0) }]); return send(res, 200, { id }); }
    if (q.get("fail")) { const m = q.get("mode") ?? "500"; if (m === "ok") fail.delete(q.get("fail").toLowerCase()); else fail.set(q.get("fail").toLowerCase(), m); }
    return send(res, 200, { fail: Object.fromEntries(fail), extra: [...extra.keys()], hits: Object.fromEntries(hits) });
  }
  const raw = me.searchParams.get("u");
  if (!raw) return send(res, 400, { error: "missing u" });
  let u;
  try { u = new URL(raw); } catch { return req.socket.destroy(); } // like a DNS / connection failure
  if (!/^https?:$/.test(u.protocol) || !u.hostname.includes(".") || u.hostname.endsWith(".example.invalid")) return req.socket.destroy();
  const host = u.hostname.replace(/^www\./, "");
  const fx = host.endsWith("fxtwitter.com") || host.endsWith("fixupx.com") || host.endsWith(".invalid");
  const handle = fx ? (u.pathname.match(/\/profile\/([^/]+)/)?.[1] ?? "") : "";
  const key = (handle || host).toLowerCase();
  hits.set(key, (hits.get(key) ?? 0) + 1);
  if (host.endsWith(".invalid")) return send(res, 503, { error: "dead mirror" });
  const mode = fail.get(key);
  const isStatuses = u.pathname.endsWith("/statuses");
  if (mode === "timeout") return; // never answers (client timeout)
  if (mode === "500") return send(res, 500, { error: "mock 500" });
  if (mode === "429") { res.writeHead(429, { "Retry-After": "1" }); return res.end(); }
  if (mode === "404" || mode === "gone" || (mode === "false404" && isStatuses)) return send(res, 404, { code: 404, message: "User not found" });
  if (mode === "402") return send(res, 402, { error: "Payment required: insufficient credits" });

  // fake 抓取 API providers (t23): Jina Reader, Firecrawl, crawl4ai and the Jina dashboard balance
  const md = (who, target) => `# ${who} 抓取\n\n` + Array.from({ length: 8 }, (_, i) => `E2E-${who.toUpperCase()} 第 ${i + 1} 段：${target} 的正文内容，包含足够多的文字用来通过全文长度判断。`).join("\n\n");
  if (host === "r.jina.ai" || host === "r.jinaai.cn") return send(res, 200, { code: 200, data: { title: "J", url: u.pathname.slice(1), content: md("jina", u.pathname.slice(1)) } });
  if (host === "api.firecrawl.dev") {
    if (u.pathname === "/v2/team/credit-usage") return send(res, 200, { success: true, data: { remainingCredits: 4321, planCredits: 100000, billingPeriodStart: "2026-10-01T00:00:00Z", billingPeriodEnd: "2026-11-01T00:00:00Z" } });
    if (u.pathname === "/v2/scrape") return send(res, 200, { success: true, data: { markdown: md("firecrawl", "page"), metadata: { title: "F", statusCode: 200 } } });
  }
  if (host === "api.crawl4ai.com") {
    res.setHeader("x-c4-balance", "812"); res.setHeader("x-c4-allowance-left", "300");
    if (u.pathname === "/v1/billing/balance") return send(res, 200, { balance: { credits: 812 }, allowance_per_month: 1000, allowance_left: 300, resets_at: "2026-11-01T00:00:00Z" });
    if (u.pathname === "/scrape") return send(res, 200, { ok: true, markdown: md("crawl4ai", "page") });
  }
  if (host === "dash.jina.ai" && u.pathname === "/api/v1/api_key/fe_user") return send(res, 200, { wallet: { total_balance: 9876543 } });

  if (fx) {
    if (isStatuses) return send(res, 200, { results: fxPosts(handle) });
    return send(res, 200, { user: { screen_name: handle, name: handle } });
  }
  if (host === "syndication.twitter.com") {
    const data = { props: { pageProps: { headerProps: { listName: "Mock list" }, timeline: { entries: [] } } } };
    return send(res, 200, `<html><script id="__NEXT_DATA__" type="application/json">${JSON.stringify(data)}</script></html>`, "text/html");
  }
  // 简报 Skill install from URL (t19)
  if (host === "skills.mock-ai.com") {
    if (u.pathname.endsWith("/SKILL.md")) return send(res, 200, "---\nname: e2e-editors\ndescription: e2e skill\n---\n# E2E 编辑部\n\nSKILL-E2E-MARKER-42：每条只保留一句话。\n", "text/markdown");
    return send(res, 200, "<!DOCTYPE html><html><body>not a skill</body></html>", "text/html");
  }
  // fake DeepSeek-compatible translator (t17): echoes every title/summary prefixed with 【译】
  if (host === "ds.mock-ai.com" && u.pathname.endsWith("/chat/completions")) {
    let b = ""; req.on("data", (c) => (b += c)).on("end", () => {
      try {
        const msgs = JSON.parse(b).messages;
        const content = msgs.at(-1).content;
        const sys = msgs[0]?.role === "system" ? msgs[0].content : "";
        const sm = /<skill name="([^"]*)">([\s\S]*?)<\/skill>/.exec(sys); const skill = (sm ? sm[1] + " " + sm[2] : "").replace(/\s+/g, " ").trim();
        const reply = (c) => send(res, 200, { choices: [{ message: { content: c } }], usage: { prompt_tokens: 10, completion_tokens: 10 } });
        // brief pipeline stages: per-item summaries (JSON), then verification (returns the draft + a marker)
        if (sys.includes("<stage>summarize</stage>")) {
          const ns = [...content.matchAll(/<item n="(\d+)"[^>]*title="([^"]*)"/g)];
          return reply(JSON.stringify({ items: ns.map((m) => ({ n: Number(m[1]), summary: "【摘要】" + m[2] })) }));
        }
        if (sys.includes("<stage>verify</stage>")) return reply((/<brief>\n?([\s\S]*?)\n?<\/brief>/.exec(content)?.[1] ?? "") + "\n\n> 【已核对】");
        try {
          const items = JSON.parse(content);
          const r = items.map((x) => ({ i: x.i, t: "【译】" + x.t, s: x.s ? "【译】" + x.s : "" }));
          send(res, 200, { choices: [{ message: { content: JSON.stringify({ r }) } }], usage: { prompt_tokens: 10, completion_tokens: 10 } });
        } catch {
          // brief AI 润色: user content is Markdown, not a translate batch
          // 生成文章: a Skill that asks for a cover gets a cover marker line (t22)
          const cover = content.includes("## 选中的新闻") && /cover-image/.test(sys) ? "\n\n<!-- cover-image: e2e cover prompt -->" : "";
          send(res, 200, { choices: [{ message: { content: "# 【润色】\n\n> SKILL: " + skill.slice(0, 2000) + "\n\n" + content + cover } }], usage: { prompt_tokens: 10, completion_tokens: 10 } });
        }
      } catch { send(res, 400, { error: "bad request" }); }
    });
    return;
  }
  if (host === "api.deepseek.com" || u.pathname.endsWith("/models")) return send(res, 401, { error: { message: "Authentication Fails (mock)" } });
  const j = api(u);
  if (j) return send(res, 200, j);
  // site roots / .html pages: HTML with an RSS <link> (add-source detection); any other path is a feed
  if (u.pathname !== "/" && !/\.html?$/.test(u.pathname)) return send(res, 200, rss(u), "application/rss+xml");
  return send(res, 200, `<html><head><title>${esc(host)}</title><link rel="alternate" type="application/rss+xml" href="https://${host}/feed.xml"></head><body>mock</body></html>`, "text/html");
}).listen(PORT, "127.0.0.1", () => console.log(`mock upstream on :${PORT}`));
