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
  section: z.enum(SECTION_KEYS).catch("media"),
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
});

export interface ParsedRow { raw: unknown; entry?: Entry; error?: string }

const zhToKey = (s?: string): Entry["section"] | undefined => {
  if (!s) return undefined; const t = s.trim();
  for (const k of SECTION_KEYS) if (t === k || t === SECTION_ZH[k] || t.split(/[,/]/).map((x) => x.trim()).includes(SECTION_ZH[k])) return k;
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
      const sec = zhToKey(a.category) ?? zhToKey(folder) ?? fallback;
      const extra = { section: sec, priority: a.priority, interval: a.interval ? Number(a.interval) : undefined, enabled: a.enabled ? a.enabled !== "false" : undefined };
      if (a.type === "x") rows.push({ name, type: "x", username: a.xUsername, ...extra });
      else if (a.xmlUrl) rows.push({ name: name || a.xmlUrl, type: a.type === "x_list" ? "x_list" : a.type === "api" ? "api" : "rss", url: a.xmlUrl, homepage: a.htmlUrl, ...extra });
      if (a.outline) walk(a.outline, a.outline.length && !a.xmlUrl ? name : folder);
    }
  };
  walk(body.outline);
  return rows;
}

/** Parse a user-chosen file (JSON from this app, or OPML from any reader) into validated rows. */
export function parseImport(text: string, fileName: string, opmlSection: Entry["section"] = "media"): { format: "json" | "opml"; rows: ParsedRow[] } {
  if (text.length > MAX_BYTES) throw new Error("文件超过 1 MB");
  const t = text.replace(/^\uFEFF/, "").trim();
  const isJson = /\.json$/i.test(fileName) || t.startsWith("{");
  let raws: unknown[];
  if (isJson) {
    let j: unknown; try { j = JSON.parse(t); } catch { throw new Error("JSON 解析失败"); }
    const e = ExportSchema.safeParse(j);
    if (!e.success) throw new Error(`不是本应用导出的 JSON（需要 app="${APP}" 且 version≤${SCHEMA_VERSION}）`);
    raws = e.data.sources;
  } else raws = opmlRows(t, opmlSection);
  if (raws.length > MAX_ENTRIES) throw new Error(`条目过多（>${MAX_ENTRIES}）`);
  return { format: isJson ? "json" : "opml", rows: raws.map((raw) => {
    const r = EntrySchema.safeParse(raw);
    return r.success ? { raw, entry: r.data } : { raw, error: r.error.issues.map((i) => i.message).join("；") };
  }) };
}

/** section key -> stored `grp` (what sectionOf() maps back) */
export const SECTION_GRP: Record<Entry["section"], string> = { lab: "官方实验室", cn: "国内", media: "媒体与聚合", x: "X 快讯", ent: "娱乐" };
export const xProfileUrl = (u: string) => `https://api.fxtwitter.com/2/profile/${u}/statuses`;

export type RowStatus = "new" | "restore" | "dup" | "update" | "invalid";
export interface PlanRow { i: number; name: string; status: RowStatus; reason?: string; match?: string }
export interface ImportPlan { rows: PlanRow[]; remove: { id: number; name: string }[]; counts: Record<RowStatus | "remove", number> }
