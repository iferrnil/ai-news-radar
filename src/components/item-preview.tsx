"use client";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { ArrowUpRight, MousePointerClick } from "lucide-react";
import { XLogo as Twitter } from "./x-logo";
import { Skeleton } from "@/components/ui/skeleton";
import type { ItemOut } from "@/lib/types";
import { ago, api, bjFull, dekText, handleOf, localize, sectionOf, srcName } from "./common";
import { SectionName } from "./section-icon";
import { SourceAvatar } from "./source-avatar";

// One shared cache for full item bodies (list payload only carries a 420-char preview).
const cache = new Map<number, Promise<ItemOut | null>>();
export function fetchFull(id: number) {
  let p = cache.get(id);
  if (!p) {
    p = api<ItemOut[]>(`/api/items?id=${id}&limit=1&zh=1`).then((r) => r[0] ?? null).catch(() => { cache.delete(id); return null; });
    cache.set(id, p);
  }
  return p;
}
export function useFullItem(it: ItemOut | null) {
  const [full, setFull] = useState<ItemOut | null>(null);
  useEffect(() => {
    if (!it?.truncated) return;
    let alive = true;
    fetchFull(it.id).then((r) => { if (alive && r) setFull(r); });
    return () => { alive = false; };
  }, [it]);
  if (!it) return { item: null, loading: false };
  if (!it.truncated) return { item: it, loading: false };
  // keep the language the card was shown in: translated card -> translated full text (original kept in .orig)
  return full?.id === it.id ? { item: it.tr ? localize(full) : full, loading: false } : { item: it, loading: true };
}

/** Body of the feed-card hover preview. Mounted only while the hover card is open. */
export function ItemPreview({ it: base }: { it: ItemOut }) {
  const { item: it, loading } = useFullItem(base);
  const x = !!it!.is_x, h = x ? handleOf(it!) : null;
  const body = x ? it!.summary || it!.title : dekText(it!.summary);
  return (
    <div className="flex max-h-[min(460px,70vh)] flex-col">
      <div className="flex items-center gap-2.5 border-b border-border/50 px-4 py-3">
        <SourceAvatar name={it!.source} username={h} homepage={it!.homepage} url={it!.url} x={x} className="size-7" />
        <div className="min-w-0 flex-1 leading-tight">
          <div className="flex items-center gap-1.5 type-body font-medium">
            <span className="truncate">{x ? srcName(it!).replace(/^@/, "") : it!.source}</span>
            {x && <Twitter className="size-2.5 shrink-0 text-faint" />}
            {it!.priority === "P0" && <Badge size="sm" variant="brand" className="px-1 font-mono font-semibold tracking-[0.02em]">P0</Badge>}
          </div>
          <div className="mt-0.5 truncate type-meta tabular-nums text-faint">{x ? `@${h} · ` : ""}{bjFull(it!.published)} · {ago(it!.published)}</div>
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
        {!x && <h4 className="break-words type-ui font-semibold leading-[1.5] tracking-[-0.008em] [overflow-wrap:anywhere]">{it!.title}</h4>}
        {it!.image && <img src={it!.image} alt="" referrerPolicy="no-referrer" className="mt-2 max-h-[300px] w-full rounded-md bg-muted object-contain" />}
        {body && <p className={x ? "whitespace-pre-wrap break-words type-body leading-[1.7] text-foreground/90 [overflow-wrap:anywhere]" : "mt-1.5 whitespace-pre-wrap break-words type-body leading-[1.7] text-muted-foreground [overflow-wrap:anywhere]"}>{body}</p>}
        {loading && <div className="mt-2 space-y-1.5"><Skeleton className="h-3 w-full" /><Skeleton className="h-3 w-11/12" /><Skeleton className="h-3 w-2/3" /></div>}
        {!body && !loading && !x && <p className="mt-1.5 type-caption text-faint">该来源未提供摘要</p>}
        {it!.orig && <Original o={it!.orig} x={x} />}
        {it!.also_on_x.length > 0 && (
          <div className="mt-3 flex flex-wrap items-center gap-1.5 type-meta text-muted-foreground">
            <Badge size="sm" variant="muted"><Twitter />也在 X</Badge>
            {it!.also_on_x.slice(0, 4).map((a) => <span key={a.url}>@{a.author}</span>)}
          </div>
        )}
      </div>
      <div className="flex items-center gap-2 border-t border-border/50 px-4 py-3 type-meta text-faint">
        <Badge size="sm" variant="outline" className="text-muted-foreground"><SectionName k={sectionOf(it!)} iconClassName="size-3" className="gap-1" /></Badge>
        <span className="flex items-center gap-1"><MousePointerClick className="size-3.5" />点击查看详情</span>
        <a href={it!.url} target="_blank" rel="noopener" className="ml-auto flex items-center gap-0.5 font-medium text-brand hover:underline">原文<ArrowUpRight className="size-3.5" /></a>
      </div>
    </div>
  );
}

export function TrBadge({ className = "" }: { className?: string }) {
  return <Badge size="sm" variant="muted" title="DeepSeek 机器翻译" className={`px-1 font-medium ${className}`}>译</Badge>;
}
/** Original (untranslated) text, shown under a translated item in the hover card and drawer. */
export function Original({ o, x }: { o: { title: string; summary: string }; x: boolean }) {
  const body = x ? o.summary || o.title : dekText(o.summary);
  return (
    <div className="mt-3 border-t border-dashed border-border pt-3">
      <div className="mb-1 type-meta font-medium text-faint">原文</div>
      {!x && <div className="break-words type-body font-medium leading-[1.5] text-foreground/80 [overflow-wrap:anywhere]">{o.title}</div>}
      {body && <p className="mt-1 whitespace-pre-wrap break-words type-caption leading-[1.65] text-muted-foreground [overflow-wrap:anywhere]">{body}</p>}
    </div>
  );
}
