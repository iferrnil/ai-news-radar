"use client";
import { memo, useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";

// hover previews only where a real pointer can hover (mobile keeps tap → drawer)
const HOVER_Q = "(hover: hover) and (pointer: fine)";
function useCanHover() {
  return useSyncExternalStore((cb) => { const m = window.matchMedia(HOVER_Q); m.addEventListener("change", cb); return () => m.removeEventListener("change", cb); }, () => window.matchMedia(HOVER_Q).matches, () => false);
}
import { AnimatePresence, LazyMotion, domAnimation, m as motion } from "framer-motion";
import { ArrowUp, ArrowUpRight, Inbox, RefreshCw } from "lucide-react";
import { XLogo as Twitter } from "./x-logo";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { HoverCard, HoverCardContent, HoverCardTrigger } from "@/components/ui/hover-card";
import { TrBadge, ItemPreview } from "./item-preview";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { ItemOut } from "@/lib/types";
import { useApp } from "./app-shell";
import { ago, api, bj, dekText, handleOf, localize, secZh, sectionOf, srcName, xText } from "./common";
import { countOf, passes } from "./filters";
import { SourceAvatar } from "./source-avatar";

const dayFmt = new Intl.DateTimeFormat("zh-CN", { timeZone: "Asia/Shanghai", year: "numeric", month: "2-digit", day: "2-digit" });
const mdFmt = new Intl.DateTimeFormat("zh-CN", { timeZone: "Asia/Shanghai", month: "long", day: "numeric", weekday: "short" });
function dayLabel(ts: number) {
  const k = dayFmt.format(new Date(ts * 1000)), md = mdFmt.format(new Date(ts * 1000));
  if (k === dayFmt.format(new Date())) return `今天 · ${md}`;
  if (k === dayFmt.format(new Date(Date.now() - 864e5))) return `昨天 · ${md}`;
  return md;
}
function groupByDay(list: ItemOut[]) {
  const out: { day: string; items: ItemOut[] }[] = [];
  for (const it of list) { const d = dayLabel(it.published); if (out.at(-1)?.day === d) out.at(-1)!.items.push(it); else out.push({ day: d, items: [it] }); }
  return out;
}

// max 3 per row; one gap value for both axes: 12px mobile · 16px md · 20px xl (1280) · 24px 2xl (1536+)
const GRID = "grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4 xl:grid-cols-3 xl:gap-5 2xl:gap-6";
// content side padding (mirrored by the sticky day header's negative margin): 16 · 24 (sm) · 32 (xl) · 40 (2xl)
const PAD = "px-4 sm:px-6 xl:px-8 2xl:px-10";
const BLEED = "-mx-4 px-4 sm:-mx-6 sm:px-6 xl:-mx-8 xl:px-8 2xl:-mx-10 2xl:px-10";
const CARD_H = "h-[204px]";

/** Equal-size card for both articles and X posts; every text block is clamped with an ellipsis. */
const ItemCard = memo(function ItemCard({ it, active, fresh, read, onOpen, hover }: { it: ItemOut; active: boolean; fresh: boolean; read: boolean; onOpen: (it: ItemOut) => void; hover: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => { if (active) ref.current?.scrollIntoView({ block: "nearest", behavior: "smooth" }); }, [active]);
  const x = !!it.is_x, h = x ? handleOf(it) : null, t = x ? xText(it) : "";
  const linkOnly = x && /^https?:\/\//.test(t);
  const dek = x ? "" : dekText(it.summary);
  const card = (
    <Card ref={ref} size="sm" onClick={() => onOpen(it)} data-active={active} data-read={read} role="button" tabIndex={-1}
      className={cn(CARD_H, "group/item cursor-pointer gap-0 py-0 shadow-card ring-border outline-none transition-[transform,background-color,box-shadow] duration-200 ease-out",
        "hover:-translate-y-px hover:ring-foreground/[0.16] hover:shadow-raised",
        "data-[active=true]:ring-2 data-[active=true]:ring-brand/50 data-[read=true]:bg-card/60", fresh && "fresh-row")}>
      <div className="flex h-11 shrink-0 items-center gap-2 px-4 pt-1 type-caption leading-4">
        <SourceAvatar name={it.source} username={h} homepage={it.homepage} url={it.url} x={x} className="size-5" />
        <span className={cn("min-w-0 truncate font-medium", x ? "text-foreground/85" : "text-muted-foreground")}>{x ? srcName(it).replace(/^@/, "") : it.source}</span>
        {x && <span className="hidden min-w-0 shrink truncate text-faint sm:inline">@{h}</span>}
        {x && <Twitter className="size-2.5 shrink-0 text-faint" />}
        {it.priority === "P0" && <span className="shrink-0 rounded-xs bg-brand-soft px-1 font-mono type-2xs font-semibold leading-4 tracking-[0.02em] text-brand shadow-[inset_0_0_0_1px_color-mix(in_oklch,var(--brand)_22%,transparent)]">P0</span>}
        {it.tr && <TrBadge />}
        <time className="ml-auto shrink-0 font-mono type-meta tabular-nums text-faint" title={`北京时间 ${bj(it.published)}`}>{ago(it.published, true)}</time>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden px-4">
        {x ? (
          <p className={cn("line-clamp-5 break-words [overflow-wrap:anywhere] type-ui leading-[1.6]", linkOnly ? "text-faint" : read ? "text-muted-foreground" : "text-foreground/90")}>
            {linkOnly ? `分享了一个链接 · ${t.replace(/^https?:\/\/(www\.)?/, "").split("/")[0]}` : t}
          </p>
        ) : (
          <>
            <h3 className={cn(it.image ? "line-clamp-1" : "line-clamp-2", "break-words [overflow-wrap:anywhere] [text-wrap:pretty] type-title font-semibold leading-[1.5]", read ? "font-medium text-muted-foreground" : "text-foreground")}>{it.title}</h3>
            {it.image ? (
              // thumbnail (memes / video covers): fixed height so cards stay equal; lazy + no-referrer for hotlink-protected CDNs
              <img src={it.image} alt="" loading="lazy" decoding="async" referrerPolicy="no-referrer"
                className="mt-2 h-[92px] w-full rounded-md bg-muted object-cover ring-1 ring-border/50" onError={(e) => { e.currentTarget.style.display = "none"; }} />
            ) : dek && dek !== it.title && <p className="mt-1.5 line-clamp-3 break-words [overflow-wrap:anywhere] type-body leading-[1.6] text-muted-foreground">{dek}</p>}
          </>
        )}
      </div>
      <div className="flex h-10 shrink-0 items-center gap-1.5 border-t border-border/50 px-4 type-meta text-faint">
        <span className="rounded-xs px-1.5 leading-[18px] shadow-hair">{secZh(sectionOf(it))}</span>
        {it.also_on_x.length > 0 && <span className="inline-flex min-w-0 items-center gap-1 truncate rounded-xs bg-secondary px-1.5 leading-[18px] text-muted-foreground"><Twitter className="size-2 shrink-0" />也在 X</span>}
        <a href={it.url} target="_blank" rel="noopener" onClick={(e) => e.stopPropagation()} aria-label="打开原文"
          className="-mr-1.5 ml-auto grid size-6 shrink-0 place-items-center rounded-md opacity-60 transition-[opacity,background-color,color] duration-150 ease-out group-hover/item:opacity-100 hover:bg-secondary hover:text-foreground">
          <ArrowUpRight className="size-3.5" strokeWidth={1.75} />
        </a>
      </div>
    </Card>
  );
  if (!hover) return card;
  return (
    <HoverCard>
      <HoverCardTrigger delay={450} closeDelay={150} render={<div className="min-w-0" />}>{card}</HoverCardTrigger>
      <HoverCardContent side="right" align="start" sideOffset={8} className="w-[min(420px,calc(100vw-24px))] overflow-hidden p-0 shadow-pop ring-border">
        <ItemPreview it={it} />
      </HoverCardContent>
    </HoverCard>
  );
});

export default function Feed() {
  const { f, setF, onStream, setOpen, open, sources, setFacet, isRead, zh } = useApp();
  const [raw, setItems] = useState<ItemOut[]>([]);
  // hide items of sources that were unsubscribed (sidebar / sources page / another tab) without a refetch
  const enabledIds = useMemo(() => (sources.length ? new Set(sources.filter((s) => s.enabled).map((s) => s.id)) : null), [sources]);
  const items = useMemo(() => {
    const vis = enabledIds ? raw.filter((it) => enabledIds.has(it.source_id)) : raw;
    return zh ? vis.map(localize) : vis;
  }, [raw, zh, enabledIds]);
  const [pending, setPending] = useState<ItemOut[]>([]);
  const [fresh, setFresh] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [cursor, setCursor] = useState(-1);
  const [showTop, setShowTop] = useState(false);
  const seen = useRef(new Set<number>());
  const canHover = useCanHover();

  // 48 h retention keeps the window small: load it all once, filter client-side.
  const load = useCallback(async () => {
    const rs = await api<ItemOut[]>(`/api/items?limit=1000${zh ? "&zh=1" : ""}`); // translations only when shown
    seen.current = new Set(rs.map((r) => r.id));
    setItems(rs); setPending([]); setLoading(false);
  }, [zh]);
  useEffect(() => { load(); }, [load]);
  useEffect(() => { setFacet(items); }, [items, setFacet]);
  // a source was re-enabled (undo / sources page): its items aren't in the list yet -> reload once
  const prevIds = useRef<Set<number> | null>(null);
  useEffect(() => {
    if (enabledIds && prevIds.current && [...enabledIds].some((id) => !prevIds.current!.has(id))) load();
    prevIds.current = enabledIds;
  }, [enabledIds, load]);
  useEffect(() => { setCursor(-1); }, [f]);
  useEffect(() => onStream((ev, d) => {
    if (ev === "item") { const it = d.item as ItemOut; if (!seen.current.has(it.id)) setPending((p) => (p.some((x) => x.id === it.id) ? p : [...p, it])); }
    else if (ev === "update") { const it = d.item as ItemOut; setItems((cur) => cur.map((x) => (x.id === it.id ? it : x))); setPending((cur) => cur.map((x) => (x.id === it.id ? it : x))); }
    else if (ev === "removed") { setItems((c) => c.filter((x) => x.id !== d.id)); setPending((p) => p.filter((x) => x.id !== d.id)); }
  }), [onStream]);
  useEffect(() => { const h = () => setShowTop(window.scrollY > 900); window.addEventListener("scroll", h, { passive: true }); return () => window.removeEventListener("scroll", h); }, []);

  const view = useMemo(() => items.filter((it) => passes(it, f)), [items, f]);
  const pend = useMemo(() => pending.filter((it) => passes(it, f)), [pending, f]);
  // render in pages of 60 cards; an IntersectionObserver sentinel reveals more while scrolling
  const [shown, setShown] = useState(60);
  useEffect(() => { setShown(60); }, [f]);
  const sentinel = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = sentinel.current; if (!el) return;
    const io = new IntersectionObserver((es) => { if (es[0].isIntersecting) setShown((n) => n + 60); }, { rootMargin: "800px" });
    io.observe(el); return () => io.disconnect();
  });
  useEffect(() => { if (cursor >= shown - 3) setShown((n) => n + 60); }, [cursor, shown]);
  const groups = useMemo(() => groupByDay(view.slice(0, shown)), [view, shown]);
  const dayCount = useMemo(() => { const m = new Map<string, number>(); for (const it of view) { const d = dayLabel(it.published); m.set(d, (m.get(d) ?? 0) + 1); } return m; }, [view]);
  const idx = useMemo(() => new Map(view.map((it, i) => [it.id, i])), [view]);
  const openItem = useCallback((it: ItemOut) => { setCursor(idx.get(it.id) ?? -1); setOpen(it); }, [idx, setOpen]);
  useEffect(() => { document.title = (pend.length ? `(${pend.length}) ` : "") + "AI Radar · 国内外 AI 情报"; }, [pend.length]);

  const showPending = () => {
    const add = pending.filter((x) => !seen.current.has(x.id));
    add.forEach((x) => seen.current.add(x.id));
    setItems((cur) => [...add, ...cur].sort((a, b) => b.published - a.published));
    setFresh(new Set(add.map((x) => x.id))); setTimeout(() => setFresh(new Set()), 3500);
    setPending([]); window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const refresh = async () => { setRefreshing(true); await load(); setRefreshing(false); };

  // j/k (and ←/→) step through cards in reading order (row by row), which is DOM order.
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (t.closest("input,textarea,[role=dialog]") || e.metaKey || e.ctrlKey || e.altKey || open) return;
      if (e.key === "j" || e.key === "ArrowRight") setCursor((c) => Math.min(c + 1, view.length - 1));
      else if (e.key === "k" || e.key === "ArrowLeft") setCursor((c) => Math.max(c - 1, 0));
      else if ((e.key === "Enter" || e.key === "o") && cursor >= 0 && view[cursor]) setOpen(view[cursor]);
      else if (e.key === "n" && pend.length) showPending();
      else if (e.key === "g") window.scrollTo({ top: 0, behavior: "smooth" });
      else return;
      e.preventDefault();
    };
    window.addEventListener("keydown", h); return () => window.removeEventListener("keydown", h);
  });

  const p0n = countOf(items, { ...f, p0: true });
  const lastFetch = Math.max(0, ...sources.filter((s) => s.enabled).map((s) => s.last_fetched ?? 0));

  return (
    <LazyMotion features={domAnimation}>
    <div className={cn("mx-auto w-full max-w-[1480px] pb-24 pt-4", PAD)}>
      <AnimatePresence>
        {pend.length > 0 && (
          <motion.div initial={{ y: -16, opacity: 0, scale: 0.96 }} animate={{ y: 0, opacity: 1, scale: 1 }} exit={{ y: -16, opacity: 0 }} transition={{ type: "spring", stiffness: 420, damping: 32 }}
            className="pointer-events-none sticky top-[68px] z-20 -mb-8 flex justify-center">
            <button onClick={showPending} className="pointer-events-auto flex items-center gap-1.5 rounded-full bg-[oklch(0.52_0.2_282)] px-3.5 py-1.5 type-body font-medium text-white shadow-[0_10px_30px_-8px_oklch(0.52_0.2_282/0.7)] ring-1 ring-white/15 transition duration-150 ease-out hover:brightness-110">
              <ArrowUp className="size-3.5" />{pend.length} 条新内容<kbd className="ml-1 rounded-xs bg-white/15 px-1 font-mono type-2xs">N</kbd>
            </button>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="flex h-8 items-center gap-2">
        <button onClick={() => setF({ p0: !f.p0 })} aria-pressed={f.p0}
          className={cn("flex h-8 items-center gap-1.5 rounded-lg px-2.5 type-caption font-medium shadow-hair transition-colors duration-150 ease-out",
            f.p0 ? "bg-brand-soft text-brand" : "text-muted-foreground hover:bg-hover hover:text-foreground")}>
          仅 P0<span className="font-mono type-2xs tabular-nums text-faint">{p0n}</span>
        </button>
        <div className="ml-auto flex items-center gap-3 type-meta text-faint">
          <span className="hidden items-center gap-1 xl:flex">
            {[["J", "K", "移动"], ["↵", "", "打开"], ["N", "", "新内容"], ["G", "", "顶部"]].map(([a, b, l]) => (
              <span key={l} className="flex items-center gap-0.5"><kbd className="rounded-xs bg-secondary px-1 font-mono type-2xs leading-4 text-muted-foreground">{a}</kbd>{b && <kbd className="rounded-xs bg-secondary px-1 font-mono type-2xs leading-4 text-muted-foreground">{b}</kbd>}<span className="ml-0.5 mr-1.5">{l}</span></span>))}
          </span>
          <Tooltip>
            <TooltipTrigger render={<button onClick={refresh} className="flex h-7 items-center gap-1.5 rounded-md px-1.5 tabular-nums transition-colors duration-150 ease-out hover:bg-hover hover:text-foreground" aria-label="刷新" />}>
              <RefreshCw className={cn("size-3.5", refreshing && "animate-spin")} strokeWidth={1.75} /><span className="font-mono">{view.length}</span> 条
            </TooltipTrigger>
            <TooltipContent>{lastFetch ? `最近抓取 ${ago(lastFetch)}（北京时间 ${bj(lastFetch).slice(6)}）` : "等待首轮抓取"} · 点击刷新</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {loading ? (
        <div className={cn(GRID, "mt-4")}>{Array.from({ length: 12 }, (_, i) => <Skeleton key={i} className={cn(CARD_H, "rounded-xl")} />)}</div>
      ) : view.length === 0 ? (
        <div className="mt-6 flex flex-col items-center rounded-xl border border-dashed px-6 py-14 text-center">
          <div className="grid size-10 place-items-center rounded-lg border bg-card"><Inbox className="size-4 text-muted-foreground" strokeWidth={1.75} /></div>
          <div className="mt-4 type-ui font-semibold">{f.p0 ? "这里 48 小时内没有 P0 内容" : f.section || f.source != null ? "这里 48 小时内还没有内容" : "这里暂时很安静"}</div>
          <p className="mt-1 max-w-sm type-body leading-5 text-muted-foreground">{f.section || f.source != null || f.p0 ? "新内容会实时推送到这里。" : "首次启动需要等待一轮抓取，新内容会实时推送到这里。"}</p>
          <div className="mt-4 flex gap-2">
            {f.p0 && <Button variant="outline" size="sm" onClick={() => setF({ p0: false })}>取消仅 P0</Button>}
            {(f.section || f.source != null) && <Button variant="ghost" size="sm" onClick={() => setF({ section: "", source: null })}>查看全部</Button>}
          </div>
        </div>
      ) : (
        groups.map((g, gi) => (
          <section key={g.day}>
            <div className={cn("sticky top-14 z-10 flex items-center gap-2 bg-background/85 pb-3 type-caption font-medium text-muted-foreground backdrop-blur-xl backdrop-saturate-150", gi === 0 ? "pt-4" : "pt-7", BLEED)}>
              {g.day}<span className="rounded-xs bg-secondary px-1.5 font-mono type-2xs leading-4 tabular-nums text-faint">{dayCount.get(g.day) ?? g.items.length}</span><span className="h-px flex-1 bg-border/60" />
            </div>
            <div className={GRID}>
              {g.items.map((it) => {
                const i = idx.get(it.id)!;
                return <ItemCard key={it.id} it={it} active={i === cursor} fresh={fresh.has(it.id)} read={isRead(it.id)} onOpen={openItem} hover={canHover && !open} />;
              })}
            </div>
          </section>
        ))
      )}
      {!loading && shown < view.length && <div ref={sentinel} className={cn(GRID, "mt-3 md:mt-4 xl:mt-5 2xl:mt-6")}>{Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className={cn(CARD_H, "rounded-xl")} />)}</div>}

      <AnimatePresence>
        {showTop && (
          <motion.button initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 8 }} transition={{ duration: 0.18, ease: "easeOut" }}
            onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })} aria-label="回到顶部"
            className="fixed bottom-6 right-6 z-30 grid size-9 place-items-center rounded-full bg-popover text-muted-foreground shadow-card ring-1 ring-border transition-colors duration-150 ease-out hover:text-foreground">
            <ArrowUp className="size-4" strokeWidth={1.75} />
          </motion.button>
        )}
      </AnimatePresence>
    </div>
    </LazyMotion>
  );
}
