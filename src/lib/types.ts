export type SourceType = "rss" | "api" | "x" | "x_list" | "html";
export type Priority = "P0" | "P1" | "P2";

export interface SourceRow {
  id: number; name: string; region: string | null; category: string | null; type: SourceType; url: string;
  homepage: string | null; username: string | null; enabled: number; supported: number; unsupported_reason: string | null;
  priority: Priority | null; poll_interval: number; last_fetched: number | null; last_error: string | null; last_new: number;
  origin: "seed" | "user"; grp: string | null; note: string | null; created: number;
}

export interface FetchedItem {
  uid: string; title: string; url: string; summary: string; published: number | null;
  isX: boolean; author?: string | null; links?: string[];
}

export interface ItemOut {
  id: number; title: string; url: string; summary: string; published: number; fetched?: number; is_x: number; truncated?: boolean;
  author: string | null; also_on_x: { url: string; author: string | null }[];
  source_id: number; source: string; category: string | null; region: string | null; source_type: SourceType;
  priority: Priority | null; homepage: string | null; username: string | null; grp: string | null;
}
