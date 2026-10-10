import type { ItemOut } from "@/lib/types";

// Exclusive sections: every item belongs to exactly one. ALL X accounts -> "x"; websites by their group.
// Exclusive sections: each SOURCE maps to exactly one section (seed `group`), so every item is in exactly one.
// X accounts default to "x" (X 快讯) unless their group says otherwise (e.g. entertainment X accounts -> 娱乐).
export const SECTIONS = [
  { key: "lab", zh: "实验室", en: "Labs" },
  { key: "cn", zh: "国内媒体", en: "China media" },
  { key: "media", zh: "国外媒体", en: "Global media" },
  { key: "x", zh: "X 快讯", en: "X" },
  { key: "ent", zh: "娱乐", en: "Entertainment" },
] as const;
export type SectionKey = (typeof SECTIONS)[number]["key"];

const GRP: Record<string, SectionKey> = {
  官方实验室: "lab", 国内: "cn", 开发与工具: "media", 媒体与聚合: "media", 娱乐: "ent", Newsletter: "media", "X 快讯": "x", 爆料与榜单X: "x", 人物与产品X: "x",
};
export function sectionOf(it: { grp?: string | null; region?: string | null; category?: string | null; is_x?: number | boolean }): SectionKey {
  if (it.grp && GRP[it.grp]) return GRP[it.grp];
  if (it.is_x) return "x";
  if (it.region === "cn") return "cn";
  if (it.category === "official") return "lab";
  return "media";
}
export const secZh = (k: SectionKey) => SECTIONS.find((s) => s.key === k)!.zh;
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
  enabled: number; supported: number; unsupported_reason: string | null; priority: string | null; poll_interval: number; grp: string | null;
  last_fetched: number | null; last_error: string | null; last_new: number; origin: string; item_count: number; latest: number | null;
};
