import { clean, httpGet, normUrl, parseDate, sleep, UA } from "./util";
import { discoverFeeds, parseFeed } from "./feed";
import type { FetchedItem, SourceRow } from "./types";

type Src = Pick<SourceRow, "type" | "url" | "username" | "name">;

// Conditional GET: remember ETag / Last-Modified per URL (in memory) and send them back; a 304 costs ~no bandwidth.
const g = globalThis as unknown as { __validators?: Map<string, { etag?: string; lm?: string }> };
const validators = (g.__validators ??= new Map());
async function condGet(url: string, headers: Record<string, string> = {}) {
  const v = validators.get(url);
  const h = { ...headers, ...(v?.etag ? { "If-None-Match": v.etag } : {}), ...(v?.lm ? { "If-Modified-Since": v.lm } : {}) };
  const r = await httpGet(url, { headers: h });
  if (r.ok) {
    const etag = r.headers.get("etag") ?? undefined, lm = r.headers.get("last-modified") ?? undefined;
    if (etag || lm) validators.set(url, { etag, lm }); else validators.delete(url);
  }
  return r;
}

// ---------------------------------------------------------------- RSS / Atom
export async function fetchRss(src: Src): Promise<FetchedItem[]> {
  const r = await condGet(src.url);
  if (r.status === 304) return []; // unchanged since last poll
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  const f = parseFeed(await r.text());
  if (!f) throw new Error("不是有效的 RSS/Atom");
  return f.items;
}

// ---------------------------------------------------------------- JSON APIs
export async function fetchApi(src: Src): Promise<FetchedItem[]> {
  const u = src.url;
  const r = await condGet(u, { Accept: "application/json" });
  if (r.status === 304) return [];
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  // some endpoints are JSONP (e.g. 网易 data_callback([...])) — strip the wrapper
  const raw = (await r.text()).trim().replace(/^[\w$.]+\(([\s\S]*)\)\s*;?$/, "$1");
  const d = JSON.parse(raw) as any;
  const out: FetchedItem[] = [];
  if (u.includes("ent.163.com/special/")) {
    // 网易娱乐 newsdata: time "MM/DD/YYYY HH:mm:ss" in Beijing time
    for (const a of (Array.isArray(d) ? d : []) as any[]) {
      if (!a.docurl || !a.title) continue;
      const m = String(a.time ?? "").match(/(\d\d)\/(\d\d)\/(\d{4}) (\d\d):(\d\d):(\d\d)/);
      const pub = m ? Date.parse(`${m[3]}-${m[1]}-${m[2]}T${m[4]}:${m[5]}:${m[6]}+08:00`) / 1000 : null;
      out.push({ uid: normUrl(a.docurl), title: clean(a.title, 200), url: a.docurl, summary: (a.keywords ?? []).map((k: any) => k.keyname ?? k).filter(Boolean).join(" · "), published: pub, isX: false });
    }
  } else if (u.includes("api.bilibili.com/x/web-interface/popular")) {
    for (const v of d.data?.list ?? []) {
      const link = `https://www.bilibili.com/video/${v.bvid}`;
      out.push({ uid: normUrl(link), title: clean(v.title, 200), url: link, summary: clean(`UP主 ${v.owner?.name ?? ""} · ${v.desc ?? ""}`), published: v.pubdate ?? null, isX: false, author: v.owner?.name, image: v.pic ? String(v.pic).replace(/^http:/, "https:") + "@480w_270h_1c.webp" : null });
    }
  } else if (u.includes("api.jandan.net/api/v1/comment/list/")) {
    // 煎蛋 无聊图 etc.: user posts with images; title = text or "<栏目> · <作者>"
    for (const c of d.data ?? []) {
      const link = `https://jandan.net/t/${c.id}`;
      const text = clean(String(c.content ?? "").replace(/#img#/g, ""), 300);
      const img = c.images?.[0]?.url as string | undefined;
      out.push({ uid: `jandan:${c.id}`, title: text.slice(0, 80) || `${c.post_title ?? "煎蛋"} · ${c.author ?? ""}`, url: link,
        summary: `${text ? text + " · " : ""}OO ${c.vote_positive ?? 0} / XX ${c.vote_negative ?? 0}`, published: parseDate(c.date), isX: false, author: c.author, image: img ?? null });
    }
  } else if (u.includes("hotsearch/billboard/word")) {
    // 抖音热搜: hot words only (no time) -> new entries are dated at fetch time; the first fetch is treated as backlog
    for (const w of d.word_list ?? []) {
      if (!w.word) continue;
      const link = `https://www.douyin.com/search/${encodeURIComponent(w.word)}`;
      out.push({ uid: `douyin:${w.word}`, title: w.word, url: link, summary: `抖音热搜 · 热度 ${w.hot_value ?? "?"}`, published: null, isX: false });
    }
  } else if (u.includes("hn.algolia.com")) {
    for (const h of d.hits ?? []) {
      const link = h.url || `https://news.ycombinator.com/item?id=${h.objectID}`;
      out.push({ uid: normUrl(link), title: h.title ?? "", url: link, isX: false, author: h.author,
        summary: `HN ${h.points ?? 0} points · ${h.num_comments ?? 0} comments · https://news.ycombinator.com/item?id=${h.objectID}`,
        published: h.created_at_i ?? parseDate(h.created_at) });
    }
  } else if (u.includes("huggingface.co/api/daily_papers")) {
    for (const x of d as any[]) {
      const p = x.paper ?? {};
      const link = `https://huggingface.co/papers/${p.id}`;
      out.push({ uid: normUrl(link), title: clean(p.title ?? x.title, 300), url: link, summary: clean(p.summary),
        published: parseDate(x.publishedAt ?? p.publishedAt), isX: false });
    }
  } else if (u.includes("jiqizhixin.com")) {
    for (const a of d.articles ?? []) {
      const link = `https://www.jiqizhixin.com/articles/${a.slug}`;
      const m = String(a.publishedAt ?? "").match(/(\d{4})\/(\d\d)\/(\d\d) (\d\d):(\d\d)/); // Beijing time
      const pub = m ? Date.parse(`${m[1]}-${m[2]}-${m[3]}T${m[4]}:${m[5]}:00+08:00`) / 1000 : null;
      out.push({ uid: normUrl(link), title: a.title ?? "", url: link, summary: clean(a.content), published: pub, isX: false, author: a.author });
    }
  } else if (u.includes("openrouter.ai/api/v1/models")) {
    for (const m of d.data ?? []) {
      const link = `https://openrouter.ai/${m.id}`;
      const pr = m.pricing ?? {};
      const price = pr.prompt != null ? ` · $${(Number(pr.prompt) * 1e6).toFixed(2)}/$${(Number(pr.completion) * 1e6).toFixed(2)} 每百万 token` : "";
      out.push({ uid: normUrl(link), title: `OpenRouter 上线新模型：${m.name ?? m.id}`, url: link,
        summary: `${m.id} · context ${m.context_length ?? "?"}${price} · ${clean(m.description, 200)}`, published: m.created ?? null, isX: false });
    }
  } else {
    throw new Error("未知 API（没有适配器）");
  }
  return out;
}

// ---------------------------------------------------------------- X via FxEmbed
const FX = "https://api.fxtwitter.com/2";
/** FxEmbed returns spurious 404s ("User not found" for real users) – retry 404/429/5xx; only 404 on every try is a real miss. */
async function fxGet(url: string, tries = 5): Promise<Response> {
  let last: Response | Error | null = null;
  for (let i = 0; i < tries; i++) {
    try {
      const r = await fetch(url, { headers: { "User-Agent": "ai-news-crawler/1.0", Accept: "application/json" }, signal: AbortSignal.timeout(20000) });
      if (![404, 429, 500, 502, 503, 504].includes(r.status)) return r;
      last = r;
      if (r.status === 404 && i === tries - 1) return r;
    } catch (e) {
      last = e as Error;
    }
    await sleep(1000 * (i + 1));
  }
  if (last instanceof Response) throw new Error(`FxEmbed HTTP ${last.status}`);
  throw last ?? new Error("FxEmbed failed");
}

function xItem(text: string, url: string, id: string, author: string, ts: number | null, links: string[]): FetchedItem {
  const first = text.replace(/https?:\/\/\S+/g, "").trim().split("\n")[0];
  return { uid: `x:${id}`, title: (first.length > 120 ? first.slice(0, 120) + "…" : first) || url, url, summary: text.slice(0, 4000), // full post text (capped)
    published: ts, isX: true, author, links };
}

export async function fxProfile(handle: string): Promise<any | null> {
  const r = await fxGet(`${FX}/profile/${encodeURIComponent(handle)}`);
  if (r.status === 404) return null;
  if (!r.ok) throw new Error(`FxEmbed HTTP ${r.status}`);
  return ((await r.json()) as any).user ?? null;
}

export async function fetchX(src: Src): Promise<FetchedItem[]> {
  const handle = src.username!;
  const r = await fxGet(`${FX}/profile/${encodeURIComponent(handle)}/statuses?count=20`);
  if (r.status === 204) return [];
  if (r.status === 404) throw new Error("FxEmbed 404（连续 5 次，账号可能不存在）");
  if (!r.ok) throw new Error(`FxEmbed HTTP ${r.status}`);
  const d = (await r.json()) as any;
  const out: FetchedItem[] = [];
  for (const p of d.results ?? []) {
    const author: string = p.author?.screen_name ?? "";
    if (author.toLowerCase() !== handle.toLowerCase()) continue; // reposts of others
    const rt = p.replying_to?.screen_name;
    if (rt && rt.toLowerCase() !== handle.toLowerCase()) continue; // replies to others
    const links = (p.raw_text?.facets ?? []).filter((f: any) => f.type === "url" && f.replacement).map((f: any) => f.replacement as string);
    out.push(xItem(p.text ?? "", p.url, String(p.id), author, p.created_timestamp ?? null, links));
  }
  return out;
}

// ---------------------------------------------------------------- X list via syndication (undocumented, ~30 calls / 15 min / IP)
export async function syndicationList(listId: string) {
  const r = await fetch(`https://syndication.twitter.com/srv/timeline-list/list-id/${listId}`, {
    headers: { "User-Agent": UA }, signal: AbortSignal.timeout(20000) });
  if (!r.ok) throw new Error(`syndication HTTP ${r.status}`);
  const html = await r.text();
  const m = html.match(/<script id="__NEXT_DATA__"[^>]*>([\s\S]*?)<\/script>/);
  if (!m) throw new Error("syndication 响应里没有 __NEXT_DATA__");
  const pp = JSON.parse(m[1]).props.pageProps;
  return { header: pp.headerProps ?? {}, entries: (pp.timeline?.entries ?? []) as any[] };
}

export async function fetchXList(src: Src): Promise<FetchedItem[]> {
  const id = src.url.match(/(\d{5,})/)?.[1];
  if (!id) throw new Error("无法识别列表 id");
  const { entries } = await syndicationList(id);
  const out: FetchedItem[] = [];
  for (const e of entries) {
    if (e.type !== "tweet") continue;
    const t = e.content.tweet;
    const text: string = t.full_text ?? t.text ?? "";
    if (t.retweeted_status || text.startsWith("RT @")) continue;
    const sn = t.user?.screen_name ?? "";
    const links = (t.entities?.urls ?? []).map((u: any) => u.expanded_url).filter(Boolean);
    out.push(xItem(text, `https://x.com/${sn}/status/${t.id_str}`, t.id_str, sn, parseDate(t.created_at), links));
  }
  return out;
}

export const FETCHERS: Record<string, (s: Src) => Promise<FetchedItem[]>> = { rss: fetchRss, api: fetchApi, x: fetchX, x_list: fetchXList };

// ---------------------------------------------------------------- add-source detection
export interface Detected { type: "rss" | "x" | "x_list"; name: string; url: string; homepage: string; username: string | null; sample: FetchedItem[]; count: number }

export async function detect(input: string): Promise<Detected> {
  const s = input.trim();
  let m = s.match(/^(?:https?:\/\/)?(?:www\.)?(?:x|twitter)\.com\/i\/lists\/(\d+)/);
  if (m) {
    const url = `https://x.com/i/lists/${m[1]}`;
    const { header } = await syndicationList(m[1]);
    const items = await fetchXList({ type: "x_list", url, username: null, name: "" });
    return { type: "x_list", name: `X 列表 ${header?.listName ?? header?.name ?? m[1]}`.trim(), url, homepage: url, username: null, sample: items.slice(0, 5), count: items.length };
  }
  m = s.match(/^(?:@|(?:https?:\/\/)?(?:www\.)?(?:x|twitter)\.com\/)?([A-Za-z0-9_]{1,15})\/?$/);
  if (m && (!s.includes(".") || /(x|twitter)\.com/.test(s))) {
    const prof = await fxProfile(m[1]);
    if (!prof) throw new Error(`X 账号 @${m[1]} 不存在或 FxEmbed 无法获取`);
    const h: string = prof.screen_name ?? m[1];
    const src = { type: "x" as const, url: `https://api.fxtwitter.com/2/profile/${h}/statuses`, username: h, name: "" };
    const items = await fetchX(src);
    return { ...src, name: `X @${h} (${prof.name})`, homepage: `https://x.com/${h}`, sample: items.slice(0, 5), count: items.length };
  }
  const url = /^https?:\/\//.test(s) ? s : `https://${s}`;
  const r = await httpGet(url);
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  const body = await r.text();
  const f = parseFeed(body);
  if (f && f.items.length)
    return { type: "rss", name: f.title || new URL(url).host, url: r.url, homepage: f.link || r.url, username: null, sample: f.items.slice(0, 5), count: f.items.length };
  for (const fu of discoverFeeds(body, r.url)) {
    const r2 = await httpGet(fu).catch(() => null);
    if (!r2?.ok) continue;
    const f2 = parseFeed(await r2.text());
    if (f2 && f2.items.length) {
      const title = f2.title || body.match(/<title[^>]*>([^<]*)/i)?.[1]?.trim() || new URL(url).host;
      return { type: "rss", name: clean(title, 80), url: r2.url, homepage: r.url, username: null, sample: f2.items.slice(0, 5), count: f2.items.length };
    }
  }
  throw new Error("页面没有可用的 RSS/Atom（未找到 <link rel=alternate> 或没有条目）；该站需要爬取，暂不支持");
}
