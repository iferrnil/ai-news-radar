export type SourceType = "rss" | "api" | "x" | "x_list" | "html";
export type Priority = "P0" | "P1" | "P2";

export interface SourceRow {
  id: number; name: string; region: string | null; category: string | null; type: SourceType; url: string;
  homepage: string | null; username: string | null; enabled: number; supported: number; unsupported_reason: string | null;
  priority: Priority | null; poll_interval: number; last_fetched: number | null; last_error: string | null; last_new: number;
  origin: "seed" | "user"; grp: string | null; section: string | null; note: string | null; created: number;
  f_include?: string | null; f_exclude?: string | null; f_scope?: "title" | "all" | null; f_cs?: number;
  tr?: number; // 1 = translation switched on for this source view (see lib/translate.ts)
}

export interface FetchedItem {
  uid: string; title: string; url: string; summary: string; published: number | null;
  isX: boolean; author?: string | null; links?: string[]; image?: string | null;
}

export interface ItemOut {
  id: number; title: string; url: string; summary: string; published: number; fetched?: number; is_x: number; truncated?: boolean;
  author: string | null; also_on_x: { url: string; author: string | null }[];
  source_id: number; source: string; category: string | null; region: string | null; source_type: SourceType;
  priority: Priority | null; homepage: string | null; username: string | null; grp: string | null; section: string | null;
  image?: string; // thumbnail (meme/video cover), only some API sources
  title_zh?: string; summary_zh?: string;
  fav?: boolean; kw?: string[]; // 选题收藏夹 · matched 关键词 (tags / filter)
  tr?: boolean; orig?: { title: string; summary: string }; // client-side: set by localize() when showing the translation // only when requested with zh=1 and a translation exists
}
