import type { ItemOut } from "@/lib/types";

// Exclusive sections: each SOURCE belongs to exactly one section (sources.section = section key), so every item is in exactly one.
// ALL sections are user-manageable rows from /api/sections (the former built-ins lab/cn/media/x/ent were seeded once);
// only 全部 is fixed. app-shell registers them at runtime via setSections(); `sectionsReady()` gates 全部 filtering.
export type Section = { key: string; zh: string; includeInAll?: boolean; icon?: string | null; pos?: number };
export type SectionKey = string;
export type SectionApiRow = { key: string; name: string; icon: string | null; in_all: number; pos: number };
let LIST: Section[] = []; let READY = false;
export function setSections(rows: SectionApiRow[]) { LIST = rows.map((r) => ({ key: r.key, zh: r.name, icon: r.icon, includeInAll: !!r.in_all, pos: r.pos })); READY = true; }
export const allSections = (): Section[] => LIST;
export const sectionsReady = () => READY;
export const isSectionKeyLike = (k: string) => /^[a-z][a-z0-9]{0,15}$/.test(k);
// includeInAll: false -> the section only shows in its own view; 全部 (feed, counts, P0 count, "N 条新内容", day headers) excludes it.
export const inAll = (k: SectionKey) => LIST.find((s) => s.key === k)?.includeInAll !== false;
export function sectionOf(it: { section?: string | null; is_x?: number | boolean }): SectionKey { return it.section ?? (it.is_x ? "x" : "media"); }
export const secZh = (k: SectionKey) => LIST.find((s) => s.key === k)?.zh ?? "未分类";
export const sourceSection = (s: { section?: string | null; type: string }) => sectionOf({ ...s, is_x: s.type.startsWith("x") });
export const CAT: Record<string, string> = { official: "官方", media: "媒体", research: "论文", community: "社区", infra: "开发者" };
export const REG: Record<string, string> = { global: "国际", cn: "国内" };
export const TYPE: Record<string, string> = { rss: "RSS", api: "API", x: "X 账号", x_list: "X 列表", html: "HTML" };

const mdhm = new Intl.DateTimeFormat("zh-CN", { timeZone: "Asia/Shanghai", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });
const full = new Intl.DateTimeFormat("zh-CN", { timeZone: "Asia/Shanghai", year: "numeric", month: "long", day: "numeric", weekday: "short", hour: "2-digit", minute: "2-digit", hour12: false });
export const bj = (ts?: number | null) => (ts ? mdhm.format(new Date(ts * 1000)).replace(/\//g, "-") : "—");
export const bjFull = (ts?: number | null) => (ts ? full.format(new Date(ts * 1000)) : "—");
export function ago(ts?: number | null, short = false) {
  if (!ts) return "—";
  const s = Date.now() / 1000 - ts;
  if (s < 60) return "刚刚";
  if (s < 3600) return `${Math.floor(s / 60)}${short ? "分" : " 分钟前"}`;
  if (s < 86400) return `${Math.floor(s / 3600)}${short ? "时" : " 小时前"}`;
  return `${Math.floor(s / 86400)}${short ? "天" : " 天前"}`;
}
export const handleOf = (it: ItemOut) => it.author || it.username || "";
export const srcName = (it: ItemOut) => it.source.replace(/^X @(\S+) \((.*)\)$/, "$2").replace(/^X @/, "@");
export const xText = (it: ItemOut) => (it.summary || it.title).replace(/https?:\/\/\S+/g, "").replace(/\s+/g, " ").trim() || it.title;
export const dekText = (s: string) => (/^Article URL:/.test(s) ? "" : s.replace(/^\s*(编辑|作者|来源|文|撰文|编译)\s*[｜|丨:：]\s*(\S{1,12}\s+)?/, "").trim());
export function avatarUrl(o: { username?: string | null; homepage?: string | null; url?: string }) {
  if (o.username) return `https://unavatar.io/x/${o.username}?fallback=false`;
  try { return `https://www.google.com/s2/favicons?domain=${new URL(o.homepage || o.url || "").hostname}&sz=64`; } catch { return undefined; }
}
export const initial = (name: string) => name.replace(/^X @/, "").replace(/[^\p{L}\p{N}]/gu, "").slice(0, 1).toUpperCase() || "?";
export async function api<T = any>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(path, { ...init, headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) } });
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(j.detail || r.statusText);
  return j as T;
}
export type SourceRow = {
  id: number; name: string; type: string; username: string | null; category: string | null; region: string | null; url: string; homepage: string | null;
  enabled: number; supported: number; unsupported_reason: string | null; priority: string | null; poll_interval: number; grp: string | null; section: string | null;
  last_fetched: number | null; last_error: string | null; last_new: number; origin: string; item_count: number; latest: number | null;
  hidden_count?: number; f_include?: string | null; f_exclude?: string | null; f_scope?: "title" | "all" | null; f_cs?: number;
};

/** Show the Chinese translation (if any) in place of title/summary, keeping the original in `orig`. */
export function localize(it: ItemOut): ItemOut {
  if (!it.title_zh || it.tr) return it;
  return { ...it, title: it.title_zh, summary: it.summary_zh || it.summary, tr: true, orig: { title: it.title, summary: it.summary } };
}
