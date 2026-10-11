// Sidebar sections — ALL user-manageable rows in the SQLite `sections` table (only 全部 is fixed).
// The 5 former built-ins are seeded ONCE by migration (settings flag `sections_seeded`), keeping their keys
// (lab/cn/media/x/ent) so old URLs and v1 exports still resolve. Each source stores its section KEY in
// `sources.section`; seed sync sets it only when inserting a new seed source and never touches sections.
import type Database from "better-sqlite3";

/** the original defaults (migration + legacy mapping for v1 files / seed `group`) */
export const DEFAULT_SECTIONS = [
  { key: "lab", name: "实验室", icon: "lucide:lab", in_all: 1 },
  { key: "cn", name: "国内媒体", icon: "lucide:cn", in_all: 1 },
  { key: "media", name: "国外媒体", icon: "lucide:media", in_all: 1 },
  { key: "x", name: "X 快讯", icon: "lucide:x", in_all: 1 },
  { key: "ent", name: "娱乐", icon: "lucide:ent", in_all: 0 },
];
export const LEGACY_ZH: Record<string, string> = Object.fromEntries(DEFAULT_SECTIONS.map((s) => [s.key, s.name]));
const GRP: Record<string, string> = {
  官方实验室: "lab", 国内: "cn", 开发与工具: "media", 媒体与聚合: "media", 娱乐: "ent", Newsletter: "media", "X 快讯": "x", 爆料与榜单X: "x", 人物与产品X: "x",
};
/** legacy derivation (seed group / old grp / region / category) -> default section key */
export function legacyKey(s: { grp?: string | null; region?: string | null; category?: string | null; type?: string | null }) {
  if (s.grp?.startsWith("sec:")) return s.grp.slice(4);
  if (s.grp && GRP[s.grp]) return GRP[s.grp];
  if (s.type?.startsWith("x")) return "x";
  if (s.region === "cn") return "cn";
  if (s.category === "official") return "lab";
  return "media";
}

export interface SectionRow { key: string; name: string; icon: string | null; in_all: number; pos: number }
export const listSections = (db: Database.Database) => db.prepare("SELECT key, name, icon, in_all, pos FROM sections ORDER BY pos, key").all() as SectionRow[];
export const isSectionKey = (db: Database.Database, k: unknown): k is string => typeof k === "string" && !!db.prepare("SELECT 1 FROM sections WHERE key=?").get(k);
/** region/category are internal-only metadata; derive them from the section like the add dialog always did */
export const metaFor = (k: string) => ({ region: k === "cn" ? "cn" : "global", category: k === "lab" ? "official" : k === "x" ? "community" : "media" });

export const cleanSectionName = (s: unknown) => String(s ?? "").replace(/[\u0000-\u001f\u007f<>]/g, "").replace(/\s+/g, " ").trim().slice(0, 24);
export const cleanIcon = (s: unknown) => { const t = String(s ?? "").trim(); if (/^lucide:[a-z]+$/.test(t)) return t; return t ? [...t].slice(0, 2).join("") : null; };

/** Create (or reuse by name) a section. Returns its key, or an error. */
export function createSection(db: Database.Database, raw: { name?: unknown; icon?: unknown; inAll?: unknown }): { key: string } | { error: string } {
  const name = cleanSectionName(raw.name);
  if (!name) return { error: "分区名称不能为空" };
  if (name === "全部") return { error: "「全部」是保留名称" };
  const ex = db.prepare("SELECT key FROM sections WHERE name=?").get(name) as { key: string } | undefined;
  if (ex) return { key: ex.key };
  let n = (db.prepare("SELECT COALESCE(MAX(CAST(substr(key,2) AS INTEGER)),0)+1 n FROM sections WHERE key GLOB 'c[0-9]*'").get() as { n: number }).n;
  while (db.prepare("SELECT 1 FROM sections WHERE key=?").get(`c${n}`)) n++;
  const pos = (db.prepare("SELECT COALESCE(MAX(pos),0)+1 p FROM sections").get() as { p: number }).p;
  db.prepare("INSERT INTO sections(key,name,icon,in_all,pos,created) VALUES(?,?,?,?,?,?)").run(`c${n}`, name, cleanIcon(raw.icon), raw.inAll === false ? 0 : 1, pos, Date.now() / 1000);
  return { key: `c${n}` };
}

/** Where a NEW seed source goes: its default section if the user still has it, else 未分类 (auto-created). */
export function seedTarget(db: Database.Database, key: string) {
  if (isSectionKey(db, key)) return key;
  const r = createSection(db, { name: "未分类", icon: "📥" });
  return "key" in r ? r.key : key;
}

/** One-off migration: seed the former built-ins as rows, then give every source a section key. */
export function migrateSections(db: Database.Database) {
  if (!db.prepare("SELECT 1 FROM settings WHERE k='sections_seeded'").get()) {
    db.transaction(() => {
      db.prepare("UPDATE sections SET pos = pos + ?").run(DEFAULT_SECTIONS.length); // keep existing custom ones after the defaults
      const ins = db.prepare("INSERT OR IGNORE INTO sections(key,name,icon,in_all,pos,created) VALUES(?,?,?,?,?,?)");
      DEFAULT_SECTIONS.forEach((s, i) => {
        const name = db.prepare("SELECT 1 FROM sections WHERE name=?").get(s.name) ? `${s.name}（默认）` : s.name;
        ins.run(s.key, name, s.icon, s.in_all, i + 1, Date.now() / 1000);
      });
      db.prepare("INSERT INTO settings VALUES('sections_seeded','1')").run();
    })();
  }
  const todo = db.prepare("SELECT id, grp, region, category, type FROM sources WHERE section IS NULL").all() as { id: number; grp: string | null; region: string | null; category: string | null; type: string }[];
  if (todo.length) {
    const upd = db.prepare("UPDATE sources SET section=? WHERE id=?");
    db.transaction(() => { for (const s of todo) upd.run(seedTarget(db, legacyKey(s)), s.id); })();
  }
}
