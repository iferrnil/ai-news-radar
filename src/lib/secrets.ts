// Runtime DeepSeek configuration. Precedence: in-app setting (0600 JSON file next to the DB) > env vars.
// The key never leaves the server: APIs only report `configured` + its source; never logged, never exported.
// SETTINGS_LOCKED=1 makes the in-app setting read-only (public deployments; the Docker image defaults to it).
import fs from "node:fs";
import path from "node:path";

export const DEFAULT_BASE = "https://api.deepseek.com";
export const DEFAULT_MODEL = "deepseek-chat";
const FILE = process.env.SECRETS_FILE ?? path.join(path.dirname(process.env.NEWS_DB ?? path.join(/*turbopackIgnore: true*/ process.cwd(), "data", "news.db")), "secrets.json");

export interface Stored { deepseekKey?: string; deepseekBaseUrl?: string; deepseekModel?: string }
const g = globalThis as unknown as { __secrets?: Stored };
function load(): Stored {
  if (g.__secrets) return g.__secrets;
  try { g.__secrets = JSON.parse(fs.readFileSync(/*turbopackIgnore: true*/ FILE, "utf8")) as Stored; } catch { g.__secrets = {}; }
  return g.__secrets;
}
function save(s: Stored) {
  fs.mkdirSync(path.dirname(FILE), { recursive: true });
  const tmp = `${FILE}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, JSON.stringify(s), { mode: 0o600 });
  fs.chmodSync(tmp, 0o600);
  fs.renameSync(tmp, FILE); // atomic replace, stays 0600
  g.__secrets = s;
}

export const settingsLocked = () => process.env.SETTINGS_LOCKED === "1";
export const deepseekKey = () => load().deepseekKey || process.env.DEEPSEEK_API_KEY || "";
export const keySource = (): "app" | "env" | null => (load().deepseekKey ? "app" : process.env.DEEPSEEK_API_KEY ? "env" : null);
/** base URL without the endpoint; DEEPSEEK_BASE_URL may be the full chat/completions URL (old style) */
export const deepseekBase = () => (load().deepseekBaseUrl || process.env.DEEPSEEK_BASE_URL || DEFAULT_BASE).replace(/\/+$/, "").replace(/\/chat\/completions$/, "");
export const deepseekModel = () => load().deepseekModel || process.env.DEEPSEEK_MODEL || DEFAULT_MODEL;
export const chatUrl = (base = deepseekBase()) => `${base}/chat/completions`;

/** public view: no secret material at all */
export const deepseekPublic = () => ({
  configured: !!deepseekKey(), source: keySource(), baseUrl: deepseekBase(), model: deepseekModel(),
  baseUrlCustom: !!load().deepseekBaseUrl, modelCustom: !!load().deepseekModel, locked: settingsLocked(),
});

export function cleanKey(v: unknown): string | null {
  const k = String(v ?? "").trim();
  return /^[\x21-\x7e]{16,200}$/.test(k) ? k : null; // printable ASCII, no spaces
}
export function cleanBase(v: unknown): string | null {
  const s = String(v ?? "").trim(); if (!s) return "";
  try {
    const u = new URL(s);
    const local = ["localhost", "127.0.0.1", "[::1]"].includes(u.hostname);
    if (u.protocol !== "https:" && !(u.protocol === "http:" && local)) return null;
    if (u.username || u.password || u.search || u.hash) return null;
    return u.toString().replace(/\/+$/, "").replace(/\/chat\/completions$/, "");
  } catch { return null; }
}
export const cleanModel = (v: unknown): string | null => { const s = String(v ?? "").trim(); return !s ? "" : /^[\w.:\-/]{1,64}$/.test(s) ? s : null; };

export function updateDeepseek(p: { key?: string | null; clearKey?: boolean; baseUrl?: string; model?: string }) {
  const s = { ...load() };
  if (p.clearKey) delete s.deepseekKey;
  if (p.key) s.deepseekKey = p.key;
  if (p.baseUrl !== undefined) { if (p.baseUrl) s.deepseekBaseUrl = p.baseUrl; else delete s.deepseekBaseUrl; }
  if (p.model !== undefined) { if (p.model) s.deepseekModel = p.model; else delete s.deepseekModel; }
  save(s);
}

/** Cheap authenticated call (GET /models). Returns no response body, only status. */
export async function testDeepseek(key = deepseekKey(), base = deepseekBase()): Promise<{ ok: boolean; status?: number; error?: string }> {
  if (!key) return { ok: false, error: "未配置 API key" };
  try {
    const r = await fetch(`${base}/models`, { headers: { Authorization: `Bearer ${key}` }, signal: AbortSignal.timeout(15_000) });
    if (r.ok) return { ok: true, status: r.status };
    return { ok: false, status: r.status, error: r.status === 401 ? "API key 无效（401）" : r.status === 402 ? "余额不足（402）" : `HTTP ${r.status}` };
  } catch (e) { return { ok: false, error: `连接失败：${(e as Error).name === "TimeoutError" ? "超时" : (e as Error).message.replace(key, "***")}` }; }
}
