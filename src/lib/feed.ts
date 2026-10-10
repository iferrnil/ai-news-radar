import { XMLParser } from "fast-xml-parser";
import { clean, normUrl, parseDate } from "./util";
import type { FetchedItem } from "./types";

const parser = new XMLParser({
  ignoreAttributes: false, attributeNamePrefix: "@_", textNodeName: "#text",
  processEntities: true, htmlEntities: true, trimValues: true, parseTagValue: false,
});

const arr = <T,>(v: T | T[] | undefined | null): T[] => (v == null ? [] : Array.isArray(v) ? v : [v]);
function txt(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string" || typeof v === "number") return String(v);
  if (Array.isArray(v)) return txt(v[0]);
  if (typeof v === "object") {
    const o = v as Record<string, unknown>;
    return txt(o["#text"] ?? o["__cdata"] ?? "");
  }
  return "";
}
function atomLink(l: unknown): string {
  const ls = arr(l as Record<string, string> | Record<string, string>[]);
  const alt = ls.find((x) => typeof x === "object" && (!x["@_rel"] || x["@_rel"] === "alternate")) ?? ls[0];
  if (typeof alt === "string") return alt;
  return (alt as Record<string, string> | undefined)?.["@_href"] ?? "";
}

export interface ParsedFeed { title: string; link: string; items: FetchedItem[] }

/** Parse RSS 2.0 / Atom / RSS 1.0 (RDF). Returns null if it isn't a feed. */
export function parseFeed(xml: string): ParsedFeed | null {
  if (!/<(rss|feed|rdf:RDF)[\s>]/i.test(xml.slice(0, 5000))) return null;
  let d: Record<string, any>;
  try { d = parser.parse(xml); } catch { return null; }
  let title = "", link = "";
  let raw: Record<string, any>[] = [];
  if (d.rss) {
    const ch = d.rss.channel ?? {};
    title = txt(ch.title); link = txt(ch.link); raw = arr(ch.item);
  } else if (d.feed) {
    title = txt(d.feed.title); link = atomLink(d.feed.link); raw = arr(d.feed.entry);
  } else if (d["rdf:RDF"]) {
    title = txt(d["rdf:RDF"].channel?.title); link = txt(d["rdf:RDF"].channel?.link); raw = arr(d["rdf:RDF"].item);
  } else return null;
  const items: FetchedItem[] = [];
  for (const e of raw.slice(0, 60)) {
    const l = d.feed ? atomLink(e.link) : txt(e.link) || (e.guid && /^https?:/.test(txt(e.guid)) ? txt(e.guid) : "");
    if (!l) continue;
    const t = clean(txt(e.title), 300) || l;
    const body = txt(e.description) || txt(e.summary) || txt(e["content:encoded"]) || txt(e.content);
    items.push({
      uid: normUrl(l), title: t, url: l, summary: clean(body),
      published: parseDate(txt(e.pubDate) || txt(e.published) || txt(e.updated) || txt(e["dc:date"])),
      isX: false, author: txt(e.author?.name ?? e.author ?? e["dc:creator"]) || null,
    });
  }
  return { title: clean(title, 100), link, items };
}

/** <link rel="alternate" type="application/rss+xml|atom+xml" href="..."> */
export function discoverFeeds(html: string, base: string): string[] {
  const out: string[] = [];
  for (const m of html.matchAll(/<link\b[^>]*>/gi)) {
    const tag = m[0];
    if (!/rel=["']?[^"'>]*alternate/i.test(tag)) continue;
    if (!/type=["']?application\/(rss|atom)\+xml|type=["']?application\/feed\+json|type=["']?(text|application)\/xml/i.test(tag)) continue;
    const h = tag.match(/href=["']([^"']+)["']/i)?.[1];
    if (h) { try { out.push(new URL(h.replace(/&amp;/g, "&"), base).toString()); } catch { /* skip */ } }
  }
  return out;
}
