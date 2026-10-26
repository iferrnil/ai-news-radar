import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { dedupeTrendshift } from "./trendshift-dedupe.ts";
import type { ItemOut } from "./types.ts";

const base = (p: Partial<ItemOut> & Pick<ItemOut, "id" | "title" | "source" | "published">): ItemOut => ({
  url: "https://github.com/" + p.title,
  summary: "",
  is_x: 0,
  author: null,
  also_on_x: [],
  source_id: 1,
  category: null,
  region: null,
  source_type: "api",
  priority: "P1",
  homepage: "https://trendshift.io/",
  username: null,
  grp: null,
  section: "media",
  ...p,
});

describe("dedupeTrendshift", () => {
  it("keeps daily over weekly in the same section", () => {
    const daily = base({ id: 1, title: "Acme/Repo", source: "Trendshift 日榜", published: 100 });
    const weekly = base({ id: 2, title: "Acme/Repo", source: "Trendshift 周榜", published: 200 });
    const other = base({ id: 3, title: "Only/Weekly", source: "Trendshift 周榜", published: 150 });
    const news = base({ id: 4, title: "Acme/Repo", source: "Some Blog", published: 300, section: "media", source_type: "rss", homepage: null });
    const got = dedupeTrendshift([weekly, daily, other, news]);
    assert.deepEqual(got.map((x) => x.id), [1, 3, 4]);
  });
  it("keeps both when sections differ", () => {
    const daily = base({ id: 1, title: "Acme/Repo", source: "Trendshift 日榜", published: 100, section: "media" });
    const weekly = base({ id: 2, title: "Acme/Repo", source: "Trendshift 周榜", published: 100, section: "lab" });
    assert.deepEqual(dedupeTrendshift([daily, weekly]).map((x) => x.id), [1, 2]);
  });
});
