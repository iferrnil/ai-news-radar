"use client";
// 热点追踪: stories several sources reported (server-side clustering, /api/hot), sorted by heat. A topic shows its
// source count, first report and reports per hour (24 h sparkline); it expands to its items and feeds 简报 / 生成文章.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { ChevronDown, FileText, Flame, PenLine, RefreshCw, WifiOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { ItemOut } from "@/lib/types";
import { useApp } from "./app-shell";
import { ago, api, bj } from "./common";
import { EmptyState } from "./empty-state";
import { SourceAvatar } from "./source-avatar";
import { XLogo } from "./x-logo";

const BriefDialog = dynamic(() => import("./brief-dialog"), { ssr: false });
const ArticleDialog = dynamic(() => import("./article-dialog"), { ssr: false });
const isDesktop = () => typeof window !== "undefined" && !!(window as unknown as { __radarDesktop?: boolean }).__radarDesktop;

type HotSource = { id: number; name: string; first: number; count: number; x?: boolean };
type HotRow = { id: number; title: string; source: string; published: number; x?: boolean; p0?: boolean };
export type HotTopic = {
  id: number; title: string; items: number[]; rows: HotRow[]; sources: HotSource[]; reports: number;
  first: { source: string; published: number; item: number }; last: number; heat: number; spark: number[];
};

/** reports per hour, oldest → newest (24 bars) */
export function Spark({ v, className }: { v: number[]; className?: string }) {
  const max = Math.max(1, ...v), w = 3, gap = 1, h = 18;
  return (
    <svg width={v.length * (w + gap) - gap} height={h} viewBox={`0 0 ${v.length * (w + gap) - gap} ${h}`} className={cn("shrink-0 overflow-visible", className)} aria-hidden data-testid="spark">
      {v.map((n, i) => {
        const bh = n ? Math.max(3, (n / max) * h) : 1;
        return <rect key={i} x={i * (w + gap)} y={h - bh} width={w} height={bh} rx={1} className={n ? "fill-orange-500/80 dark:fill-orange-400/80" : "fill-foreground/10"} />;
      })}
    </svg>
  );
}

/** 生成文章 takes 2: the earliest report plus the first one from another source, articles before X posts */
export function articlePick(t: HotTopic): number[] {
  const rows = [...t.rows].sort((a, b) => Number(!!a.x) - Number(!!b.x) || a.published - b.published);
  const a = rows[0]; if (!a) return [];
  const b = rows.find((r) => r.source !== a.source);
  return b ? [a.id, b.id] : [a.id];
}

export default function HotTopics() {
  const { onStream, setOpen, loc, sources } = useApp();
  const [topics, setTopics] = useState<HotTopic[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [openId, setOpenId] = useState<number | null>(null);
  const [brief, setBrief] = useState<HotTopic | null>(null);
  const [article, setArticle] = useState<HotTopic | null>(null);
  const [desktop, setDesktop] = useState(false);
  useEffect(() => { setDesktop(isDesktop()); }, []);
  const load = useCallback(async () => {
    setBusy(true);
    try { const r = await api<{ topics: HotTopic[] }>("/api/hot"); setTopics(r.topics); setErr(null); }
    catch (e) { setErr((e as Error).message); setTopics((c) => c ?? []); }
    finally { setBusy(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  // new items can merge into a topic: refresh at most every 20 s while items stream in
  const t = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => onStream((ev) => { if ((ev === "item" || ev === "removed") && !t.current) t.current = setTimeout(() => { t.current = null; void load(); }, 20000); }), [onStream, load]);
  useEffect(() => () => { if (t.current) clearTimeout(t.current); }, []);
  const srcById = useMemo(() => new Map(sources.map((s) => [s.id, s])), [sources]);
  const openItem = (id: number) => api<ItemOut[]>(`/api/items?id=${id}&zh=1`).then((r) => { if (r[0]) setOpen(loc(r[0])); }).catch(() => {});

  return (
    <div className="mx-auto w-full max-w-[1100px] px-4 pb-24 pt-4 sm:px-6 xl:px-8" data-testid="hot-view">
      <div className="flex h-8 items-center gap-2 type-caption text-faint">
        <Flame className="size-3.5 text-orange-500" />多个来源报道的同一事件 · 按热度排序
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={() => void load()} className="ml-auto px-1.5 font-normal tabular-nums text-faint" aria-label="刷新" />}>
            <RefreshCw className={cn("size-3.5", busy && "animate-spin")} /><span className="font-mono">{topics?.length ?? 0}</span> 个
          </TooltipTrigger>
          <TooltipContent>热度：每个来源的首次报道计 1、后续计 0.3，按 12 小时半衰</TooltipContent>
        </Tooltip>
      </div>
      {topics == null ? (
        <div className="mt-3 space-y-3">{Array.from({ length: 5 }, (_, i) => <Skeleton key={i} className="h-[104px] rounded-xl" />)}</div>
      ) : err && !topics.length ? (
        <EmptyState icon={WifiOff} tone="error" className="mt-6 rounded-xl border border-dashed py-14" title="加载失败" hint={err}>
          <Button variant="outline" size="sm" onClick={() => void load()}><RefreshCw />重试</Button>
        </EmptyState>
      ) : !topics.length ? (
        <EmptyState icon={Flame} className="mt-6 rounded-xl border border-dashed py-14" title="暂时没有热点" hint="同一事件有 2 个以上来源报道时会出现在这里。" />
      ) : (
        <ol className="mt-3 space-y-3">
          {topics.map((tp, i) => {
            const expanded = openId === tp.id;
            return (
              <li key={tp.id} data-testid="hot-topic" className="rounded-xl bg-card ring-1 ring-border shadow-feed">
                <button type="button" onClick={() => setOpenId(expanded ? null : tp.id)} aria-expanded={expanded} className="flex w-full items-start gap-3 rounded-xl p-4 pb-2 text-left outline-none focus-visible:ring-2 focus-visible:ring-brand/50">
                  <span className="mt-0.5 w-5 shrink-0 text-right font-mono type-caption tabular-nums text-faint">{i + 1}</span>
                  <span className="min-w-0 flex-1">
                    <span className="line-clamp-2 type-title font-semibold leading-[1.5] [text-wrap:pretty]" data-testid="hot-title">{tp.title}</span>
                    <span className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 type-meta text-faint">
                      <Badge size="sm" variant="outline" className="border-orange-500/40 font-mono tabular-nums text-orange-600 dark:text-orange-300" data-testid="hot-count">{tp.sources.length} 源 · {tp.reports} 条</Badge>
                      <span title={`北京时间 ${bj(tp.first.published)}`}>首发 <span className="text-muted-foreground">{tp.first.source.replace(/^X @(\S+) \((.*)\)$/, "$2")}</span> · {ago(tp.first.published)}</span>
                      <span className="hidden sm:inline">· 最新 {ago(tp.last)}</span>
                    </span>
                  </span>
                  <span className="flex shrink-0 flex-col items-end gap-1.5">
                    <Spark v={tp.spark} />
                    <span className="type-meta text-faint">24 小时</span>
                  </span>
                </button>
                <div className="flex items-center gap-1.5 px-4 pb-3 pl-12">
                  <span className="flex -space-x-1.5">
                    {tp.sources.slice(0, 6).map((s) => {
                      const r = srcById.get(s.id);
                      return <SourceAvatar key={s.id} name={s.name} username={r?.username} homepage={r?.homepage} url={r?.url} x={!!s.x} className="size-5 ring-2 ring-card" />;
                    })}
                  </span>
                  {tp.sources.length > 6 && <span className="type-meta text-faint">+{tp.sources.length - 6}</span>}
                  <span className="ml-auto flex items-center gap-0.5">
                    <Button variant="ghost" size="sm" className="px-1.5 type-caption font-normal text-faint" onClick={() => setBrief(tp)} data-testid="hot-brief"><FileText className="size-3.5" />简报</Button>
                    {desktop && <Button variant="ghost" size="sm" className="px-1.5 type-caption font-normal text-faint" onClick={() => setArticle(tp)} data-testid="hot-article"><PenLine className="size-3.5" />生成文章</Button>}
                    <Button variant="ghost" size="icon-sm" className="text-faint" aria-label={expanded ? "收起" : "展开"} onClick={() => setOpenId(expanded ? null : tp.id)}><ChevronDown className={cn("size-3.5 transition-transform", expanded && "rotate-180")} /></Button>
                  </span>
                </div>
                {expanded && (
                  <ul className="border-t border-border/50 py-1.5" data-testid="hot-items">
                    {tp.rows.map((r) => (
                      <li key={r.id}>
                        <button type="button" onClick={() => void openItem(r.id)} className="flex w-full items-center gap-2.5 px-4 py-1.5 pl-12 text-left type-body hover:bg-secondary/60">
                          <time className="w-11 shrink-0 font-mono type-meta tabular-nums text-faint">{bj(r.published).slice(6)}</time>
                          <span className="flex w-36 shrink-0 items-center gap-1 truncate type-caption text-muted-foreground">{r.x && <XLogo className="size-2.5 shrink-0" />}<span className="truncate">{r.source.replace(/^X @(\S+) \((.*)\)$/, "$2")}</span></span>
                          <span className="min-w-0 flex-1 truncate">{r.title}</span>
                          {r.p0 && <Badge size="sm" variant="brand" className="px-1 font-mono">P0</Badge>}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            );
          })}
        </ol>
      )}
      {brief && <BriefDialog key={`hot|${brief.id}`} open onOpenChange={(o) => { if (!o) setBrief(null); }} section={null} sectionLabel="热点" ids={brief.items.slice(0, 10)} />}
      {article && <ArticleDialog key={`hot|${article.id}`} open onOpenChange={(o) => { if (!o) setArticle(null); }} section="all" ids={articlePick(article)} />}
    </div>
  );
}
