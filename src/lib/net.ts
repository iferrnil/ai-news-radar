// Outbound HTTP for all fetchers: one place for timeouts, retry backoff with jitter and the test upstream.
// UPSTREAM_MOCK=http://127.0.0.1:4010 (tests/CI only) sends every request to a local mock as `/?u=<url>`,
// so the suite never depends on the network.
export const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/** Exponential backoff with "full jitter": random in [base·2^i / 2, base·2^i], capped. */
export function backoff(attempt: number, base = 1000, cap = 15_000) {
  const exp = Math.min(cap, base * 2 ** attempt);
  return Math.round(exp / 2 + Math.random() * (exp / 2));
}

export function netFetch(url: string, init: RequestInit = {}): Promise<Response> {
  const mock = process.env.UPSTREAM_MOCK;
  if (!mock) return fetch(url, init);
  // keep Response.url = the real URL (detection stores it as the feed address)
  return fetch(`${mock.replace(/\/$/, "")}/?u=${encodeURIComponent(url)}`, init).then((r) => Object.defineProperty(r, "url", { value: url }));
}

/** Retry-After (seconds or HTTP date) in ms, if the server sent one. */
export function retryAfter(r: Response): number | null {
  const v = r.headers.get("retry-after");
  if (!v) return null;
  const n = Number(v);
  if (Number.isFinite(n)) return Math.min(60_000, n * 1000);
  const t = Date.parse(v);
  return Number.isNaN(t) ? null : Math.max(0, Math.min(60_000, t - Date.now()));
}

export type FailKind = "timeout" | "network" | "http" | "rate_limit" | "not_found" | "false_404" | "parse" | "other";
/** Error with a coarse kind, kept in per-source health. */
export class FetchError extends Error {
  constructor(message: string, public kind: FailKind) { super(message); this.name = "FetchError"; }
}
export function kindOf(e: unknown): FailKind {
  if (e instanceof FetchError) return e.kind;
  const n = e instanceof Error ? `${e.name} ${e.message}` : String(e);
  if (/Timeout|aborted/i.test(n)) return "timeout";
  if (/HTTP 429/.test(n)) return "rate_limit";
  if (/HTTP \d/.test(n)) return "http";
  if (/fetch failed|ECONN|ENOTFOUND|EAI_AGAIN|socket/i.test(n)) return "network";
  if (/JSON|RSS|Unexpected token|有效/.test(n)) return "parse";
  return "other";
}
