// Export side of subscriptions import/export — dependency-free so the sources page bundle stays small.
// (zod + the OPML parser live in portable.ts and load only with the import dialog.)
export const APP = "ai-news-radar";
export const SCHEMA_VERSION = 2; // v2: custom sections + per-source filter rules (v1 files still import); optional translation scope flags: source `translate:"on"`, section `translate:true`, top-level `translateAll` (older v2 readers ignore them)
export const MAX_BYTES = 1024 * 1024; // 1 MB
export const MAX_ENTRIES = 2000;
/** legacy default section keys/names (v1 files, OPML folders from older exports) */
export const SECTION_KEYS = ["lab", "cn", "media", "x", "ent"] as const;
export const SECTION_ZH: Record<(typeof SECTION_KEYS)[number], string> = { lab: "实验室", cn: "国内媒体", media: "国外媒体", x: "X 快讯", ent: "娱乐" };
export type ImportMode = "merge" | "update" | "replace";

const bjStamp = (d = new Date()) => {
  const p = Object.fromEntries(new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).formatToParts(d).map((x) => [x.type, x.value]));
  return { iso: `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}:${p.second}+08:00`, file: `${p.year}${p.month}${p.day}-${p.hour}${p.minute}` };
};
export const exportFileName = (ext: "json" | "opml", d = new Date()) => `${APP}-sources-${bjStamp(d).file}.${ext}`;

export interface ExportSource {
  name: string; type: string; url: string; username: string | null; homepage: string | null; section: string; priority: string | null; poll_interval: number; enabled: number | boolean;
  f_include?: string | null; f_exclude?: string | null; f_scope?: string | null; f_cs?: number | null;
  tr?: number; // 1 = translation switched on for this source
}
/** user-created section metadata (key -> name/icon/inAll); built-ins are implied by their keys */
export interface ExportSection { key: string; name: string; icon?: string | null; inAll: boolean; translate?: boolean }
/** translation switched on for this source view → translate:"on" (off is omitted) */
export const translateOf = (s: ExportSource) => (s.tr === 1 ? { translate: "on" as const } : {});
const filterOf = (s: ExportSource) => (s.f_include || s.f_exclude ? { filter: { ...(s.f_include ? { include: s.f_include } : {}), ...(s.f_exclude ? { exclude: s.f_exclude } : {}), scope: s.f_scope === "all" ? "all" : "title", caseSensitive: !!s.f_cs } } : {});

export function toEntry(s: ExportSource) {
  return {
    name: s.name, type: s.type, ...(s.type === "x" ? { username: s.username ?? undefined } : { url: s.url }),
    ...(s.homepage ? { homepage: s.homepage } : {}), section: s.section, priority: s.priority ?? "P1",
    interval: s.poll_interval, enabled: !!s.enabled, ...filterOf(s), ...translateOf(s),
  };
}

/** favorites travel as opaque rows from GET /api/favorites/export (the server validates them on import) */
export function buildJson(list: ExportSource[], d = new Date(), custom: ExportSection[] = [], translateAll = false, favorites: unknown[] = []): string {
  const used = custom.filter((c) => list.some((s) => s.section === c.key)).map(({ translate, ...c }) => (translate ? { ...c, translate } : c));
  return JSON.stringify({ app: APP, version: SCHEMA_VERSION, exportedAt: bjStamp(d).iso, count: list.length, ...(translateAll ? { translateAll } : {}), ...(used.length ? { sections: used } : {}), sources: list.map(toEntry), ...(favorites.length ? { favorites } : {}) }, null, 2) + "\n";
}

const esc = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

/** OPML 2.0: RSS/Atom as standard `type="rss"` outlines grouped by section; X accounts/lists as custom
 *  `type="x"`/`type="x_list"` outlines (other readers ignore them; this app re-imports them). */
export function buildOpml(list: ExportSource[], d = new Date(), custom: ExportSection[] = [], translateAll = false): string {
  const out: string[] = [`<?xml version="1.0" encoding="UTF-8"?>`, `<opml version="2.0">`, `  <head>`, `    <title>AI News Radar subscriptions</title>`, `    <dateCreated>${d.toUTCString()}</dateCreated>`, `    <docs>http://opml.org/spec2.opml</docs>`, ...(translateAll ? [`    <radarTranslateAll>true</radarTranslateAll>`] : []), `  </head>`, `  <body>`];
  // one folder per section in sidebar order (folder + category = section name; sectionCustom/sectionKey/sectionIcon/sectionInAll let this app restore it)
  const groups = custom.map((c) => ({ key: c.key, name: c.name, extra: ` sectionCustom="1" sectionKey="${esc(c.key)}" sectionInAll="${c.inAll ? "true" : "false"}"${c.translate ? ` sectionTranslate="true"` : ""}${c.icon ? ` sectionIcon="${esc(c.icon)}"` : ""}` }));
  const orphan = list.filter((s) => !custom.some((c) => c.key === s.section));
  if (orphan.length) groups.push({ key: "__orphan", name: "未分类", extra: ` sectionCustom="1"` });
  for (const g of groups) {
    const rows = g.key === "__orphan" ? orphan : list.filter((s) => s.section === g.key); if (!rows.length) continue;
    out.push(`    <outline text="${esc(g.name)}" title="${esc(g.name)}"${g.extra}>`);
    for (const s of rows) {
      const f = filterOf(s).filter;
      const fa = f ? `${f.include ? ` filterInclude="${esc(f.include)}"` : ""}${f.exclude ? ` filterExclude="${esc(f.exclude)}"` : ""} filterScope="${f.scope}" filterCase="${f.caseSensitive}"` : "";
      const common = `text="${esc(s.name)}" title="${esc(s.name)}" category="${esc(g.name)}"${g.extra} priority="${esc(s.priority ?? "P1")}" interval="${s.poll_interval}" enabled="${s.enabled ? "true" : "false"}"${fa}${s.tr === 1 ? ` translate="on"` : ""}`;
      if (s.type === "x") out.push(`      <outline type="x" ${common} xUsername="${esc(s.username ?? "")}" htmlUrl="https://x.com/${esc(s.username ?? "")}"/>`);
      else if (s.type === "x_list") out.push(`      <outline type="x_list" ${common} xmlUrl="${esc(s.url)}"/>`);
      else out.push(`      <outline type="${s.type === "api" ? "api" : "rss"}" ${common} xmlUrl="${esc(s.url)}"${s.homepage ? ` htmlUrl="${esc(s.homepage)}"` : ""}/>`);
    }
    out.push(`    </outline>`);
  }
  out.push(`  </body>`, `</opml>`, ``);
  return out.join("\n");
}

