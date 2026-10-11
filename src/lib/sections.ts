// Sidebar sections: 5 built-ins (keys lab/cn/media/x/ent, stored on sources as seed-style `grp`) + user-created ones
// (SQLite `sections` table, keys c1, c2, …; sources point at them with grp = "sec:<key>"). Seed sync never touches either.
import type Database from "better-sqlite3";

export const BUILTIN_GRP: Record<string, string> = { lab: "官方实验室", cn: "国内", media: "媒体与聚合", x: "X 快讯", ent: "娱乐" };
export const BUILTIN_ZH: Record<string, string> = { lab: "实验室", cn: "国内媒体", media: "国外媒体", x: "X 快讯", ent: "娱乐" };
export interface CustomSection { key: string; name: string; icon: string | null; in_all: number; pos: number }

export const listSections = (db: Database.Database) => db.prepare("SELECT key, name, icon, in_all, pos FROM sections ORDER BY pos, key").all() as CustomSection[];
export const isSectionKey = (db: Database.Database, k: unknown): k is string =>
  typeof k === "string" && (k in BUILTIN_GRP || !!db.prepare("SELECT 1 FROM sections WHERE key=?").get(k));
export const grpFor = (k: string) => BUILTIN_GRP[k] ?? `sec:${k}`;
/** region/category are internal-only; derive them from the section like the add dialog always did */
export const metaFor = (k: string, isX: boolean) => ({ region: k === "cn" ? "cn" : "global", category: k === "lab" ? "official" : k === "x" || (isX && !(k in BUILTIN_GRP)) ? "community" : "media" });

export const cleanSectionName = (s: unknown) => String(s ?? "").replace(/[\u0000-\u001f\u007f<>]/g, "").replace(/\s+/g, " ").trim().slice(0, 24);
export const cleanIcon = (s: unknown) => { const t = String(s ?? "").trim(); return t ? [...t].slice(0, 2).join("") : null; }; // one emoji (may be 2 code points)

/** Create (or reuse by name) a custom section. Returns its key, or an error. */
export function createSection(db: Database.Database, raw: { name?: unknown; icon?: unknown; inAll?: unknown }): { key: string } | { error: string } {
  const name = cleanSectionName(raw.name);
  if (!name) return { error: "分区名称不能为空" };
  if (Object.values(BUILTIN_ZH).includes(name)) return { error: "与内置分区重名" };
  const ex = db.prepare("SELECT key FROM sections WHERE name=?").get(name) as { key: string } | undefined;
  if (ex) return { key: ex.key };
  const n = (db.prepare("SELECT COALESCE(MAX(CAST(substr(key,2) AS INTEGER)),0)+1 n FROM sections").get() as { n: number }).n;
  const pos = (db.prepare("SELECT COALESCE(MAX(pos),0)+1 p FROM sections").get() as { p: number }).p;
  db.prepare("INSERT INTO sections(key,name,icon,in_all,pos,created) VALUES(?,?,?,?,?,?)").run(`c${n}`, name, cleanIcon(raw.icon), raw.inAll === false ? 0 : 1, pos, Date.now() / 1000);
  return { key: `c${n}` };
}
