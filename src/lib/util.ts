export const UA =
  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36 ai-news-crawler/1.0";

const TRACK = /^(utm_|ref$|ref_src$|source$|fbclid$|gclid$|s$|t$)/;

/** host(without www)+path(no trailing slash)+filtered query; scheme dropped. */
export function normUrl(u: string): string {
  try {
    const p = new URL(u.trim());
    const q = [...p.searchParams.entries()].filter(([k]) => !TRACK.test(k));
    const qs = q.length ? "?" + new URLSearchParams(q).toString() : "";
    const path = p.pathname.replace(/\/+$/, "") || "/";
    return (p.host.toLowerCase().replace(/^www\./, "") + path + qs).toLowerCase();
  } catch {
    return u.trim().toLowerCase();
  }
}

const ENT: Record<string, string> = { amp: "&", lt: "<", gt: ">", quot: '"', apos: "'", nbsp: " ", mdash: "—", ndash: "–", hellip: "…", rsquo: "’", lsquo: "‘", ldquo: "“", rdquo: "”" };
export const decode = (t: string) => t.replace(/&(#x?[0-9a-f]+|\w+);/gi, (m, e: string) =>
  e[0] === "#" ? String.fromCodePoint(e[1].toLowerCase() === "x" ? parseInt(e.slice(2), 16) : parseInt(e.slice(1), 10)) : ENT[e.toLowerCase()] ?? m);
export function clean(s: unknown, n = 280): string {
  let t = decode(String(s ?? ""))
    .replace(/<script[\s\S]*?<\/script>|<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&(#x?[0-9a-f]+|\w+);/gi, (m, e: string) =>
      e[0] === "#" ? String.fromCodePoint(e[1].toLowerCase() === "x" ? parseInt(e.slice(2), 16) : parseInt(e.slice(1), 10)) : ENT[e.toLowerCase()] ?? m)
    .replace(/\s+/g, " ")
    .trim();
  if (t.length > n) t = t.slice(0, n) + "…";
  return t;
}

export function parseDate(v: unknown): number | null {
  if (v == null || v === "") return null;
  if (typeof v === "number") return v > 1e12 ? Math.floor(v / 1000) : v;
  const t = Date.parse(String(v).trim());
  return Number.isNaN(t) ? null : Math.floor(t / 1000);
}

export { sleep } from "./net";
import { backoff, netFetch, retryAfter, sleep } from "./net";

/** GET with a per-attempt timeout; 429/5xx and network errors are retried with jittered exponential backoff (Retry-After wins). */
export async function httpGet(url: string, opts: { timeout?: number; tries?: number; headers?: Record<string, string> } = {}) {
  const tries = opts.tries ?? 3;
  let last: unknown;
  for (let i = 0; i < tries; i++) {
    try {
      const r = await netFetch(url, {
        headers: {
          "User-Agent": UA,
          Accept: "application/rss+xml, application/atom+xml, application/xml;q=0.9, text/html;q=0.8, application/json;q=0.8, */*;q=0.5",
          "Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8",
          ...opts.headers,
        },
        redirect: "follow",
        signal: AbortSignal.timeout(opts.timeout ?? 15000),
      });
      if ([429, 500, 502, 503, 504].includes(r.status) && i < tries - 1) {
        await r.body?.cancel().catch(() => {});
        await sleep(retryAfter(r) ?? backoff(i, 1500));
        continue;
      }
      return r;
    } catch (e) {
      last = e;
      if (i < tries - 1) await sleep(backoff(i, 1500));
    }
  }
  throw last instanceof Error ? last : new Error(String(last));
}

export function errMsg(e: unknown) {
  if (e instanceof Error) return `${e.name}: ${e.message}${e.cause instanceof Error ? ` (${e.cause.message})` : ""}`.slice(0, 300);
  return String(e).slice(0, 300);
}
