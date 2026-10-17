// Import/export of subscriptions — isomorphic (browser + server). No items, no secrets: only source settings.
import { z } from "zod";
import { XMLParser } from "fast-xml-parser";
import { APP, SCHEMA_VERSION, MAX_BYTES, MAX_ENTRIES, SECTION_KEYS, SECTION_ZH } from "./portable-export";
export * from "./portable-export";

const httpUrl = z.string().trim().max(2000).refine((u) => { try { return ["http:", "https:"].includes(new URL(u).protocol); } catch { return false; } }, "只允许 http(s) URL");
/** strip control chars / angle brackets, collapse whitespace, cap length */
export const cleanName = (s: string) => s.replace(/[\u0000-\u001f\u007f<>]/g, "").replace(/\s+/g, " ").trim().slice(0, 120);

export const EntrySchema = z.object({
  name: z.string().transform(cleanName).pipe(z.string().min(1, "名称为空")),
  type: z.enum(["rss", "api", "x", "x_list"]),
  url: httpUrl.optional(),
  username: z.string().trim().regex(/^@?[A-Za-z0-9_]{1,15}$/, "X 用户名不合法").transform((u) => u.replace(/^@/, "")).optional(),
  homepage: httpUrl.optional().catch(undefined),
  section: z.string().trim().max(40).catch("media"), // built-in key, or a custom section (resolved by sectionName)
  sectionName: z.string().trim().min(1).max(24).optional().catch(undefined),
  sectionIcon: z.string().trim().max(8).optional().catch(undefined),
  sectionInAll: z.boolean().optional().catch(undefined),
  sectionTranslate: z.boolean().optional().catch(undefined),
  translate: z.enum(["inherit", "on", "off"]).optional().catch(undefined), // only "on" is written now; inherit/off (older exports) = off
  filter: z.object({
    include: z.string().max(200).optional(), exclude: z.string().max(200).optional(),
    scope: z.enum(["title", "all"]).catch("title"), caseSensitive: z.boolean().catch(false),
  }).optional(),
  priority: z.enum(["P0", "P1", "P2"]).catch("P1"),
  interval: z.number().int().min(60).max(86400).optional().catch(undefined),
  enabled: z.boolean().catch(true),
}).superRefine((e, ctx) => {
  if (e.type === "x" && !e.username) ctx.addIssue({ code: "custom", message: "X 账号缺少 username" });
  if (e.type !== "x" && !e.url) ctx.addIssue({ code: "custom", message: "缺少 URL" });
});
export type Entry = z.output<typeof EntrySchema>;

export const ExportSchema = z.object({
  app: z.literal(APP), version: z.number().int().min(1).max(SCHEMA_VERSION), exportedAt: z.string().optional(),
  sources: z.array(z.unknown()).max(MAX_ENTRIES),
  sections: z.array(z.object({ key: z.string(), name: z.string(), icon: z.string().nullish(), inAll: z.boolean().optional(), translate: z.boolean().optional() })).max(200).optional().catch(undefined),
  translateAll: z.boolean().optional().catch(undefined),
  favorites: z.array(z.unknown()).max(MAX_ENTRIES).optional().catch(undefined),
});

export interface ParsedRow { raw: unknown; entry?: Entry; error?: string }

export type KnownSection = { key: string; name: string };
let KNOWN: KnownSection[] = [];
/** folder / category name -> current section key (by the user's current names, then the legacy default names) */
const zhToKey = (s?: string): Entry["section"] | undefined => {
  if (!s) return undefined; const t = s.trim(); const parts = t.split(/[,/]/).map((x) => x.trim());
  for (const k of KNOWN) if (t === k.key || parts.includes(k.name)) return k.key;
  for (const k of SECTION_KEYS) if (t === k || parts.includes(SECTION_ZH[k])) return k;
  return undefined;
};

function opmlRows(xml: string, fallback: Entry["section"]): unknown[] {
  const doc = new XMLParser({ ignoreAttributes: false, attributeNamePrefix: "", isArray: (n) => n === "outline", processEntities: true, htmlEntities: true }).parse(xml);
  const body = doc?.opml?.body; if (!body) throw new Error("不是有效的 OPML 文件（缺少 <opml><body>）");
  const rows: unknown[] = [];
  const walk = (nodes: Record<string, unknown>[] | undefined, folder?: string) => {
    for (const o of nodes ?? []) {
      const a = o as Record<string, string> & { outline?: Record<string, unknown>[] };
      const name = a.title || a.text || "";
      // this app's own custom sections carry sectionCustom="1"; other readers' folders fall back to the chosen default
      const custom = a.sectionCustom === "1" && a.category ? { section: a.sectionKey || "custom", sectionName: a.category, sectionIcon: a.sectionIcon, sectionInAll: a.sectionInAll ? a.sectionInAll !== "false" : undefined, sectionTranslate: a.sectionTranslate ? a.sectionTranslate !== "false" : undefined } : null;
      const sec = custom ? custom.section : zhToKey(a.category) ?? zhToKey(folder) ?? fallback;
      const filter = a.filterInclude || a.filterExclude ? { include: a.filterInclude, exclude: a.filterExclude, scope: a.filterScope, caseSensitive: a.filterCase === "true" } : undefined;
      const extra = { ...custom, section: sec, priority: a.priority, interval: a.interval ? Number(a.interval) : undefined, enabled: a.enabled ? a.enabled !== "false" : undefined, filter, translate: a.translate || undefined };
      if (a.type === "x") rows.push({ name, type: "x", username: a.xUsername, ...extra });
      else if (a.xmlUrl) rows.push({ name: name || a.xmlUrl, type: a.type === "x_list" ? "x_list" : a.type === "api" ? "api" : "rss", url: a.xmlUrl, homepage: a.htmlUrl, ...extra });
      if (a.outline) walk(a.outline, a.outline.length && !a.xmlUrl ? name : folder);
    }
  };
  walk(body.outline);
  return rows;
}

/** Parse a user-chosen file (JSON from this app, or OPML from any reader) into validated rows. */
export function parseImport(text: string, fileName: string, opmlSection: Entry["section"] = "media", known: KnownSection[] = []): { format: "json" | "opml"; rows: ParsedRow[]; translateAll?: boolean; favorites?: unknown[] } {
  if (text.length > MAX_BYTES) throw new Error("文件超过 1 MB");
  KNOWN = known;
  const t = text.replace(/^\uFEFF/, "").trim();
  const isJson = /\.json$/i.test(fileName) || t.startsWith("{");
  let raws: unknown[]; let translateAll: boolean | undefined; let favorites: unknown[] | undefined;
  if (isJson) {
    let j: unknown; try { j = JSON.parse(t); } catch { throw new Error("JSON 解析失败"); }
    const e = ExportSchema.safeParse(j);
    if (!e.success) throw new Error(`不是本应用导出的 JSON（需要 app="${APP}" 且 version≤${SCHEMA_VERSION}）`);
    // v2: attach custom section metadata (by key) to each entry so the server can match/create sections by name
    translateAll = e.data.translateAll; favorites = e.data.favorites?.length ? e.data.favorites : undefined;
    const meta = new Map((e.data.sections ?? []).map((x) => [x.key, x]));
    raws = e.data.sources.map((r) => {
      const m = r && typeof r === "object" ? meta.get(String((r as { section?: unknown }).section)) : undefined;
      return m ? { ...(r as object), sectionName: m.name, sectionIcon: m.icon ?? undefined, sectionInAll: m.inAll, sectionTranslate: m.translate } : r;
    });
  } else { raws = opmlRows(t, opmlSection); translateAll = /<head>[\s\S]*<radarTranslateAll>true<\/radarTranslateAll>[\s\S]*<\/head>/.test(t) || undefined; }
  if (raws.length > MAX_ENTRIES) throw new Error(`条目过多（>${MAX_ENTRIES}）`);
  return { format: isJson ? "json" : "opml", ...(translateAll ? { translateAll } : {}), ...(favorites ? { favorites } : {}), rows: raws.map((raw) => {
    const r = EntrySchema.safeParse(raw);
    return r.success ? { raw, entry: r.data } : { raw, error: r.error.issues.map((i) => i.message).join("；") };
  }) };
}

/** section key -> stored `grp` (what sectionOf() maps back) */
export const SECTION_GRP: Record<string, string> = { lab: "官方实验室", cn: "国内", media: "媒体与聚合", x: "X 快讯", ent: "娱乐" };
export const xProfileUrl = (u: string) => `https://api.fxtwitter.com/2/profile/${u}/statuses`;

export type RowStatus = "new" | "restore" | "dup" | "update" | "invalid";
export interface PlanRow { i: number; name: string; status: RowStatus; reason?: string; match?: string }
export interface ImportPlan { rows: PlanRow[]; remove: { id: number; name: string }[]; counts: Record<RowStatus | "remove", number> }
