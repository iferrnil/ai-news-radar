"use client";
import { warmFonts } from "@/lib/font-warm";
import { memo, useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { Badge } from "@/components/ui/badge";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import { Toggle } from "@/components/ui/toggle";
import { Separator } from "@/components/ui/separator";

// hover previews only where a real pointer can hover (mobile keeps tap → drawer)
const HOVER_Q = "(hover: hover) and (pointer: fine)";
function useCanHover() {
  return useSyncExternalStore((cb) => { const m = window.matchMedia(HOVER_Q); m.addEventListener("change", cb); return () => m.removeEventListener("change", cb); }, () => window.matchMedia(HOVER_Q).matches, () => false);
}
import { ArrowUp, ArrowUpRight, Check, CheckCheck, CircleDot, FileText, Inbox, ListChecks, PenLine, RefreshCw, WifiOff, X } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { toast } from "sonner";
import { markOnHover } from "./read-store";
import { XLogo as Twitter } from "./x-logo";
import { Button } from "@/components/ui/button";
import { HoverPreviewLayer, hoverPreview } from "./hover-preview";
import { TrBadge } from "./item-preview";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { EmptyState } from "./empty-state";
import dynamic from "next/dynamic";
const BriefDialog = dynamic(() => import("./brief-dialog"), { ssr: false });
const ArticleDialog = dynamic(() => import("./article-dialog"), { ssr: false });
const PICK_MAX = { brief: 10, article: 2 } as const; // 手动简报 / 生成文章
const isDesktop = () => typeof window !== "undefined" && !!(window as unknown as { __radarDesktop?: boolean }).__radarDesktop;
import type { ItemOut } from "@/lib/types";
import { useApp } from "./app-shell";
import { sectionsReady, ago, api, bj, dekText, handleOf, sectionOf, srcName, xText } from "./common";
import { SectionName } from "./section-icon";
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
// columns follow the width of <main> (@container in app-shell), so hiding the sidebar reflows the grid
const GRID = "grid grid-cols-1 gap-3 @2xl:grid-cols-2 @2xl:gap-4 @5xl:grid-cols-3 @5xl:gap-5 @7xl:gap-6";
// content side padding (mirrored by the sticky day header's negative margin): 16 · 24 (sm) · 32 (xl) · 40 (2xl)
const PAD = "px-4 sm:px-6 xl:px-8 2xl:px-10";
const BLEED = "-mx-4 px-4 sm:-mx-6 sm:px-6 xl:-mx-8 xl:px-8 2xl:-mx-10 2xl:px-10";
const CARD_H = "h-[208px]"; // slightly denser cards without redesign

/** Equal-size card for both articles and X posts; every text block is clamped with an ellipsis. */
const ItemCard = memo(function ItemCard({ it, active, fresh, read, onOpen, hover, pick = 0 }: { it: ItemOut; active: boolean; fresh: boolean; read: boolean; onOpen: (it: ItemOut) => void; hover: boolean; pick?: 0 | 1 | 2 | 3 }) {
  // pick: 0 not selecting · 1 selectable · 2 selected · 3 selection full
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => { if (active) ref.current?.scrollIntoView({ block: "nearest", behavior: "smooth" }); }, [active]);
  const x = !!it.is_x, h = x ? handleOf(it) : null, t = x ? xText(it) : "";
  const linkOnly = x && /^https?:\/\//.test(t);
  const dek = x ? "" : dekText(it.summary);
  const card = (
    <div ref={ref} data-slot="card" data-size="sm" onClickCapture={pick ? (e) => { e.preventDefault(); e.stopPropagation(); onOpen(it); } : undefined} onClick={() => onOpen(it)} data-active={active} data-read={read} data-item={it.id} role="button" tabIndex={-1}
      data-picked={pick === 2 || undefined} aria-pressed={pick ? pick === 2 : undefined}
      className={cn(CARD_H, "feed-card group/item relative flex flex-col overflow-hidden rounded-xl bg-card text-sm text-card-foreground ring-1 cursor-pointer gap-0 py-0 shadow-feed ring-border outline-none transition-[transform,background-color,box-shadow] duration-200 ease-out",
        "hover:-translate-y-px hover:ring-foreground/[0.14] hover:shadow-raised",
        "data-[active=true]:ring-2 data-[active=true]:ring-brand/50 data-[read=true]:bg-card/55", fresh && "fresh-row",
        pick === 2 && "ring-2 ring-brand", pick === 3 && "opacity-50")}>
      {pick > 0 && (
        <span data-testid="pick-mark" role="checkbox" aria-checked={pick === 2} aria-label="选择"
          className={cn("absolute bottom-1.5 right-2 z-10 grid size-6 place-items-center rounded-full border-[1.5px] shadow-sm transition-colors duration-150 before:absolute before:-inset-2.5 before:content-['']",
            pick === 2 ? "border-brand bg-brand text-white" : "border-foreground/25 bg-background text-transparent group-hover/item:border-foreground/45")}>
          <Check className="size-3.5" strokeWidth={3} />
        </span>
      )}
      <div className="flex h-10 shrink-0 items-center gap-2 px-3.5 pt-0.5 type-caption leading-4">
        <SourceAvatar name={it.source} username={h} homepage={it.homepage} url={it.url} x={x} className="size-5" />
        <span className={cn("min-w-0 truncate font-medium", x ? "text-foreground/85" : "text-muted-foreground")}>{x ? srcName(it).replace(/^@/, "") : it.source}</span>
        {x && <span className="hidden min-w-0 shrink truncate text-faint sm:inline">@{h}</span>}
        {x && <Twitter className="size-2.5 shrink-0 text-faint" />}
        {it.priority === "P0" && <Badge size="sm" variant="brand" className="px-1 font-mono font-semibold tracking-[0.02em]">P0</Badge>}
        {it.tr && <TrBadge />}
        {!read && <span data-testid="unread-dot" aria-label="未读" title="未读" className="ml-auto size-1.5 shrink-0 rounded-full bg-brand" />}
        <time className={cn("shrink-0 font-mono type-meta tabular-nums text-faint", read && "ml-auto")} title={`北京时间 ${bj(it.published)}`}>{ago(it.published, true)}</time>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden px-3.5">
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
                className="mt-1.5 h-[88px] w-full rounded-md bg-muted object-cover ring-1 ring-border/40" onError={(e) => { e.currentTarget.style.display = "none"; }} />
            ) : dek && dek !== it.title && <p className="mt-1.5 line-clamp-3 break-words [overflow-wrap:anywhere] type-body leading-[1.6] text-muted-foreground">{dek}</p>}
          </>
        )}
      </div>
      <div className="flex h-9 shrink-0 items-center gap-1.5 border-t border-border/40 px-3.5 type-meta text-faint">
        <Badge size="sm" variant="outline" className="text-muted-foreground"><SectionName k={sectionOf(it)} iconClassName="size-3" className="gap-1" /></Badge>
        {it.also_on_x.length > 0 && <Badge size="sm" variant="muted" className="min-w-0"><Twitter />也在 X</Badge>}
        {pick ? <span className="-mr-1.5 ml-auto size-6 shrink-0" aria-hidden /> : (
          <a href={it.url} target="_blank" rel="noopener" onClick={(e) => e.stopPropagation()} aria-label="打开原文"
            className="-mr-1.5 ml-auto grid size-6 shrink-0 place-items-center rounded-md opacity-60 transition-[opacity,background-color,color] duration-150 ease-out group-hover/item:opacity-100 hover:bg-secondary hover:text-foreground">
            <ArrowUpRight className="size-3.5" />
          </a>
        )}
      </div>
    </div>
  );
  if (!hover || pick) return card;
  return <div className="min-w-0" onPointerEnter={(e) => hoverPreview.enter(it, e.currentTarget)} onPointerMove={(e) => hoverPreview.enter(it, e.currentTarget)} onPointerLeave={() => hoverPreview.leave()} onPointerDown={() => hoverPreview.close()}>{card}</div>;
});

export default function Feed() {
  const { f, setF, onStream, setOpen, open, sources, setFacet, isRead, markRead, markManyRead, markManyUnread, loc, sections } = useApp();
  const [briefOpen, setBriefOpen] = useState(false);
  // 手动简报 / 生成文章: pick up to PICK_MAX cards of this view
  const [pickMode, setPickMode] = useState<null | "brief" | "article">(null);
  const [picked, setPicked] = useState<number[]>([]);
  const [briefIds, setBriefIds] = useState<number[] | undefined>(undefined);
  const [articleOpen, setArticleOpen] = useState(false);
  const [desktop, setDesktop] = useState(false);
  useEffect(() => { setDesktop(isDesktop()); }, []);
  // brief is for a section page or 全部 (not a single-source page)
  const briefScope = f.source != null ? null : (f.section || "all");
  const briefLabel = briefScope === "all" ? "全部" : briefScope ? (sections.find((s) => s.key === briefScope)?.zh ?? briefScope) : "";
  const sp = useSearchParams(); const router = useRouter();
  // a brief / article / selection belongs to one view: switching view (sidebar, ⌘K, notification) ends them, so a
  // dialog can never show the previous section's result
  const viewId = `${briefScope}`;
  useEffect(() => { setBriefOpen(false); setArticleOpen(false); setPickMode(null); setPicked([]); }, [viewId]);
  const pickMax = PICK_MAX[pickMode ?? "brief"];
  const togglePick = useCallback((it: ItemOut) => {
    setPicked((p) => {
      if (p.includes(it.id)) return p.filter((x) => x !== it.id);
      if (p.length >= pickMax) { toast(`最多选 ${pickMax} 条`); return p; }
      return [...p, it.id];
    });
  }, [pickMax]);
  const autoBrief = () => { setBriefIds(undefined); setBriefOpen(true); };
  const startPick = (m: "brief" | "article") => { hoverPreview.close(); setPickMode((cur) => (cur === m ? null : m)); setPicked((p) => p.slice(0, PICK_MAX[m])); };
  const runPick = () => {
    if (!picked.length) return;
    if (pickMode === "brief") { setBriefIds([...picked]); setBriefOpen(true); } else setArticleOpen(true);
  };
  const secReady = sections.length > 0 || sectionsReady(); // 全部 must know which sections are excluded before rendering
  const [raw, setItems] = useState<ItemOut[]>([]);
  // hide items of sources that were unsubscribed (sidebar / sources page / another tab) without a refetch
  const enabledIds = useMemo(() => (sources.length ? new Set(sources.filter((s) => s.enabled).map((s) => s.id)) : null), [sources]);
  const items = useMemo(() => {
    const vis = enabledIds ? raw.filter((it) => enabledIds.has(it.source_id)) : raw;
    return vis.map(loc); // translation shown where its view scope (全部 / section / source) is switched on
  }, [raw, loc, enabledIds]);
  useEffect(() => { warmFonts(items.flatMap((it) => [it.title, it.summary ?? "", it.source])); }, [items]);
  const [pending, setPending] = useState<ItemOut[]>([]);
  const [fresh, setFresh] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(true);
  const [loadErr, setLoadErr] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [cursor, setCursor] = useState(-1);
  const [showTop, setShowTop] = useState(false);
  const seen = useRef(new Set<number>());
  const canHover = useCanHover();

  // 48 h retention keeps the window small: load it all once, filter client-side.
  const load = useCallback(async () => {
    let rs: ItemOut[];
    try { rs = await api<ItemOut[]>(`/api/items?limit=1000&zh=1`); } // translations are picked per item by loc()
    catch (e) { setLoadErr(e instanceof Error ? e.message : String(e)); setLoading(false); return; }
    seen.current = new Set(rs.map((r) => r.id));
    setLoadErr(null); setItems(rs); setPending([]); setLoading(false);
  }, []);
  useEffect(() => { load(); }, [load]);
  useEffect(() => { setFacet(items); }, [items, setFacet]);
  // a source was re-enabled (undo / sources page): its items aren't in the list yet -> reload once
  const prevIds = useRef<Set<number> | null>(null);
  useEffect(() => {
    if (enabledIds && prevIds.current && [...enabledIds].some((id) => !prevIds.current!.has(id))) load();
    prevIds.current = enabledIds;
  }, [enabledIds, load]);
  useEffect(() => { setCursor(-1); }, [f]);
  // batch SSE "update" (e.g. translations) so a burst of DeepSeek results doesn't re-render the list per item
  useEffect(() => {
    const pendingUpd = new Map<number, ItemOut>();
    let flushT: ReturnType<typeof setTimeout> | null = null;
    const flush = () => {
      flushT = null;
      if (!pendingUpd.size) return;
      const batch = new Map(pendingUpd); pendingUpd.clear();
      setItems((cur) => cur.map((x) => batch.get(x.id) ?? x));
      setPending((cur) => cur.map((x) => batch.get(x.id) ?? x));
    };
    return onStream((ev, d) => {
      if (ev === "item") { const it = d.item as ItemOut; if (!seen.current.has(it.id)) setPending((p) => (p.some((x) => x.id === it.id) ? p : [...p, it])); }
      else if (ev === "update") {
        pendingUpd.set((d.item as ItemOut).id, d.item as ItemOut);
        if (!flushT) flushT = setTimeout(flush, 200);
      } else if (ev === "removed") {
        pendingUpd.delete(d.id);
        setItems((c) => c.filter((x) => x.id !== d.id)); setPending((p) => p.filter((x) => x.id !== d.id));
      }
    });
  }, [onStream]);
  // one passive, rAF-coalesced scroll listener: back-to-top visibility + .feed-list[data-scrolling] (drops hover effects / pointer
  // events on cards while scrolling, so WKWebView doesn't restyle hundreds of cards and open hover cards mid-scroll)
  // The flag sits on the feed list only (one element; pointer-events inherits on Chrome/WebKit's fast path) instead of
  // html — toggling an html attribute matched by `.feed-card` selectors restyled every card at each scroll start/stop.
  const listRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let raf = 0, idle: ReturnType<typeof setTimeout> | undefined, top = false, on = false;
    const h = () => {
      if (!raf) raf = requestAnimationFrame(() => {
        raf = 0; const t = window.scrollY > 900; if (t !== top) { top = t; setShowTop(t); }
        if (!on) { on = true; listRef.current?.setAttribute("data-scrolling", ""); }
      });
      clearTimeout(idle); idle = setTimeout(() => { on = false; listRef.current?.removeAttribute("data-scrolling"); }, 140);
    };
    window.addEventListener("scroll", h, { passive: true });
    return () => { window.removeEventListener("scroll", h); cancelAnimationFrame(raf); clearTimeout(idle); };
  }, []);

  // 只看未读 hides what was read BEFORE this view was opened (snapshot), so cards read while browsing stay put until the view changes
  const [epoch, setEpoch] = useState(0); const loaded = raw.length > 0;
  // eslint-disable-next-line react-hooks/exhaustive-deps -- snapshot on purpose: only view changes / mark-all refresh it
  const snap = useMemo(() => { const s = new Set<number>(); for (const it of raw) if (isRead(it.id)) s.add(it.id); return s; }, [f, epoch, loaded]);
  const hide = useCallback((id: number) => snap.has(id), [snap]);
  const view = useMemo(() => items.filter((it) => passes(it, f, hide)), [items, f, hide]);
  const pend = useMemo(() => pending.filter((it) => passes(it, f, hide)), [pending, f, hide]);
  // the same view without 只看未读: what 全部标为已读 applies to
  const scope = useMemo(() => (f.unread ? items.filter((it) => passes(it, { ...f, unread: false })) : view), [items, f, view]);
  const unreadN = useMemo(() => scope.reduce((n, it) => n + (isRead(it.id) ? 0 : 1), 0), [scope, isRead]);
  const markAll = () => {
    const ids = scope.filter((it) => !isRead(it.id)).map((it) => it.id);
    if (!ids.length || !markManyRead(ids)) return;
    toast.success(`已将 ${ids.length} 条标为已读`, { action: { label: "撤销", onClick: () => { markManyUnread(ids); setEpoch((e) => e + 1); } } });
    if (f.unread) setEpoch((e) => e + 1);
  };
  const onPreview = useCallback((it: ItemOut) => { if (markOnHover()) markRead(it.id); }, [markRead]);
  // ?item=<id> (notification click / desktop shell): open that item, then drop the param
  const itemParam = Number(sp.get("item")) || 0;
  useEffect(() => {
    if (!itemParam || loading) return;
    const strip = () => { const q = new URLSearchParams(sp.toString()); q.delete("item"); const s = q.toString(); router.replace(s ? `/?${s}` : "/", { scroll: false }); };
    const hit = items.find((x) => x.id === itemParam);
    if (hit) { setOpen(loc(hit)); strip(); return; }
    api<ItemOut[]>(`/api/items?id=${itemParam}&zh=1`).then((r) => { if (r[0]) setOpen(loc(r[0])); else toast("这条内容已不在 48 小时窗口内"); }).catch(() => {}).finally(strip);
  }, [itemParam, loading]); // eslint-disable-line react-hooks/exhaustive-deps
  // render in pages of 30 cards (shorter commits while scrolling); an IntersectionObserver sentinel reveals more while scrolling
  const [shown, setShown] = useState(30);
  useEffect(() => { setShown(30); }, [f]);
  const sentinel = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = sentinel.current; if (!el) return;
    const io = new IntersectionObserver((es) => { if (es[0].isIntersecting) setShown((n) => n + 30); }, { rootMargin: "1400px" });
    io.observe(el); return () => io.disconnect();
  }, [loading, shown, view.length]);
  useEffect(() => { if (cursor >= shown - 3) setShown((n) => n + 60); }, [cursor, shown]);
  const groups = useMemo(() => groupByDay(view.slice(0, shown)), [view, shown]);
  const dayCount = useMemo(() => { const m = new Map<string, number>(); for (const it of view) { const d = dayLabel(it.published); m.set(d, (m.get(d) ?? 0) + 1); } return m; }, [view]);
  const idx = useMemo(() => new Map(view.map((it, i) => [it.id, i])), [view]);
  const openItem = useCallback((it: ItemOut) => { hoverPreview.close(); setCursor(idx.get(it.id) ?? -1); setOpen(it); }, [idx, setOpen]);
  const pickSet = useMemo(() => new Set(picked), [picked]);
  const onCard = pickMode ? togglePick : openItem;
  const pickOf = (id: number): 0 | 1 | 2 | 3 => (!pickMode ? 0 : pickSet.has(id) ? 2 : picked.length >= pickMax ? 3 : 1);
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
      if (e.key === "Escape" && pickMode) { setPickMode(null); e.preventDefault(); return; }
      if (e.key === "j" || e.key === "ArrowRight") setCursor((c) => Math.min(c + 1, view.length - 1));
      else if (e.key === "k" || e.key === "ArrowLeft") setCursor((c) => Math.max(c - 1, 0));
      else if ((e.key === "Enter" || e.key === "o" || (pickMode && e.key === " ")) && cursor >= 0 && view[cursor]) (pickMode ? togglePick : setOpen)(view[cursor]);
      else if (e.key === "n" && pend.length) showPending();
      else if (e.key === "g") window.scrollTo({ top: 0, behavior: "smooth" });
      else return;
      e.preventDefault();
    };
    window.addEventListener("keydown", h); return () => window.removeEventListener("keydown", h);
  });

  const p0n = countOf(items, { ...f, p0: true, unread: false });
  const lastFetch = Math.max(0, ...sources.filter((s) => s.enabled).map((s) => s.last_fetched ?? 0));

  return (
    <div className={cn("mx-auto w-full max-w-[1480px] pb-24 pt-4", PAD)}>
        {pend.length > 0 && (
          <div className="animate-in fade-in-0 zoom-in-95 slide-in-from-top-4 duration-200 ease-out pointer-events-none sticky top-[68px] z-20 -mb-8 flex justify-center">
            <Button onClick={showPending} className="pointer-events-auto h-8 rounded-full bg-[oklch(0.52_0.2_282)] px-4 type-body text-white shadow-[0_10px_30px_-8px_oklch(0.52_0.2_282/0.7)] ring-1 ring-white/15 hover:bg-[oklch(0.52_0.2_282)] hover:brightness-110">
              <ArrowUp className="size-3.5" />{pend.length} 条新内容<Kbd className="ml-1 h-4.5 bg-white/15 font-mono text-white">N</Kbd>
            </Button>
          </div>
        )}

      <div className="flex h-8 items-center gap-2">
        <Toggle variant="outline" pressed={f.p0} onPressedChange={(v) => setF({ p0: v })}
          className="gap-1.5 type-caption text-muted-foreground aria-pressed:border-brand/40 aria-pressed:bg-brand-soft aria-pressed:text-brand">
          仅 P0<Badge size="sm" variant="muted" className="px-1 font-mono tabular-nums text-faint">{p0n}</Badge>
        </Toggle>
        <Toggle variant="outline" pressed={!!f.unread} onPressedChange={(v) => setF({ unread: v })} aria-label="只看未读"
          className="gap-1.5 type-caption text-muted-foreground aria-pressed:border-brand/40 aria-pressed:bg-brand-soft aria-pressed:text-brand">
          <CircleDot className="size-3.5" /><span className="hidden sm:inline">只看未读</span><span className="sm:hidden">未读</span>
          <Badge size="sm" variant="muted" data-testid="unread-total" className="px-1 font-mono tabular-nums text-faint">{unreadN}</Badge>
        </Toggle>
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={markAll} disabled={!unreadN} aria-label="全部标为已读" className="px-1.5 type-caption font-normal text-faint" />}>
            <CheckCheck className="size-3.5" /><span className="hidden md:inline">全部已读</span>
          </TooltipTrigger>
          <TooltipContent>将当前视图（{f.source != null ? "此来源" : f.section ? "此分区" : "全部"}{f.p0 ? " · 仅 P0" : ""}）的 {unreadN} 条标为已读</TooltipContent>
        </Tooltip>
        {briefScope && (
          <div className="flex items-center gap-0.5" data-testid="brief-tools">
            <Tooltip>
              <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={autoBrief} aria-label="自动简报" data-testid="brief-btn" className="px-1.5 type-caption font-normal text-faint" />}>
                <FileText className="size-3.5" /><span className="hidden md:inline">自动简报</span>
              </TooltipTrigger>
              <TooltipContent>自动挑选 {PICK_MAX.brief} 条{briefLabel === "全部" ? "" : `「${briefLabel}」`}内容：多源报道的热点优先，再取最早与最新</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger render={<Button variant={pickMode === "brief" ? "secondary" : "ghost"} size="sm" onClick={() => startPick("brief")} aria-pressed={pickMode === "brief"} aria-label="手动简报" data-testid="brief-manual-btn" className="px-1.5 type-caption font-normal text-faint aria-pressed:text-foreground" />}>
                <ListChecks className="size-3.5" /><span className="hidden md:inline">手动简报</span>
              </TooltipTrigger>
              <TooltipContent>选择最多 {PICK_MAX.brief} 条生成简报</TooltipContent>
            </Tooltip>
            {desktop && (
              <Tooltip>
                <TooltipTrigger render={<Button variant={pickMode === "article" ? "secondary" : "ghost"} size="sm" onClick={() => startPick("article")} aria-pressed={pickMode === "article"} aria-label="生成文章" data-testid="article-btn" className="px-1.5 type-caption font-normal text-faint aria-pressed:text-foreground" />}>
                  <PenLine className="size-3.5" /><span className="hidden md:inline">生成文章</span>
                </TooltipTrigger>
                <TooltipContent>选择最多 {PICK_MAX.article} 条，写成推特 / 公众号 / 小红书文章</TooltipContent>
              </Tooltip>
            )}
          </div>
        )}
        <div className="ml-auto flex items-center gap-3 type-meta text-faint">
          <span className="hidden items-center gap-1 xl:flex">
            {[["J", "K", "移动"], ["↵", "", "打开"], ["N", "", "新内容"], ["G", "", "顶部"]].map(([a, b, l]) => (
              <span key={l} className="flex items-center gap-0.5"><KbdGroup><Kbd className="font-mono">{a}</Kbd>{b && <Kbd className="font-mono">{b}</Kbd>}</KbdGroup><span className="ml-0.5 mr-1.5">{l}</span></span>))}
          </span>
          <Tooltip>
            <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={refresh} className="px-1.5 font-normal tabular-nums text-faint" aria-label="刷新" />}>
              <RefreshCw className={cn("size-3.5", refreshing && "animate-spin")} /><span className="font-mono">{view.length}</span> 条
            </TooltipTrigger>
            <TooltipContent>{lastFetch ? `最近抓取 ${ago(lastFetch)}（北京时间 ${bj(lastFetch).slice(6)}）` : "等待首轮抓取"} · 点击刷新</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {loadErr && !items.length ? (
        <EmptyState icon={WifiOff} tone="error" className="mt-6 rounded-xl border border-dashed py-14" title="加载失败" hint={`无法获取内容：${loadErr}`}>
          <Button variant="outline" size="sm" onClick={() => { setLoading(true); load(); }}><RefreshCw />重试</Button>
        </EmptyState>
      ) : loading || !secReady ? (
        <div className={cn(GRID, "mt-4")}>{Array.from({ length: 12 }, (_, i) => <Skeleton key={i} className={cn(CARD_H, "rounded-xl")} />)}</div>
      ) : view.length === 0 ? (
        <EmptyState icon={Inbox} className="mt-6 rounded-xl border border-dashed py-14"
          title={f.unread && scope.length ? "全部读完了" : f.p0 ? "这里 48 小时内没有 P0 内容" : f.section || f.source != null ? "这里 48 小时内还没有内容" : "这里暂时很安静"}
          hint={f.unread && scope.length ? "新内容会实时推送到这里。" : f.section || f.source != null || f.p0 ? "新内容会实时推送到这里。" : "首次启动需要等待一轮抓取，新内容会实时推送到这里。"}>
          {f.unread && <Button variant="outline" size="sm" onClick={() => setF({ unread: false })}>显示已读</Button>}
          {f.p0 && <Button variant="outline" size="sm" onClick={() => setF({ p0: false })}>取消仅 P0</Button>}
          {(f.section || f.source != null) && <Button variant="ghost" size="sm" onClick={() => setF({ section: "", source: null })}>查看全部</Button>}
        </EmptyState>
      ) : (
        <div ref={listRef} className="feed-list">{groups.map((g, gi) => (
          <section key={g.day}>
            <div className={cn("sticky top-14 z-10 flex items-center gap-2 bg-background pb-3 type-caption font-medium text-muted-foreground", gi === 0 ? "pt-4" : "pt-7", BLEED)}>
              {g.day}<Badge size="sm" variant="muted" className="font-mono tabular-nums text-faint">{dayCount.get(g.day) ?? g.items.length}</Badge><Separator className="flex-1 bg-border/60" />
            </div>
            <div className={GRID}>
              {g.items.map((it) => {
                const i = idx.get(it.id)!;
                return <ItemCard key={it.id} it={it} active={i === cursor} fresh={fresh.has(it.id)} read={isRead(it.id)} onOpen={onCard} hover={canHover} pick={pickOf(it.id)} />;
              })}
            </div>
          </section>
        ))}</div>
      )}
      {!loading && shown < view.length && <div ref={sentinel} className={cn(GRID, "mt-3 @2xl:mt-4 @5xl:mt-5 @7xl:mt-6")}>{Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className={cn(CARD_H, "rounded-xl")} />)}</div>}

        {showTop && (
          <button
            onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })} aria-label="回到顶部" data-testid="back-to-top"
            className="animate-in fade-in-0 slide-in-from-bottom-2 duration-200 ease-out fixed bottom-6 right-6 z-30 grid size-9 place-items-center rounded-full bg-popover text-muted-foreground shadow-card ring-1 ring-border transition-colors duration-150 ease-out hover:text-foreground">
            <ArrowUp className="size-4" />
          </button>
        )}
        {canHover && <HoverPreviewLayer onOpen={onPreview} />}
        {pickMode && (
          <div className="animate-in fade-in-0 slide-in-from-bottom-2 duration-200 ease-out fixed inset-x-0 bottom-5 z-30 flex justify-center px-4" data-testid="pick-bar">
            <div className="flex items-center gap-2 rounded-full bg-popover py-1.5 pl-4 pr-1.5 shadow-raised ring-1 ring-border">
              <span className="type-caption text-muted-foreground">{pickMode === "brief" ? "手动简报" : "生成文章"} · 已选 <span className="font-mono tabular-nums text-foreground" data-testid="pick-count">{picked.length}/{pickMax}</span></span>
              {picked.length > 0 && <Button variant="ghost" size="sm" className="h-7 rounded-full px-2 text-faint" onClick={() => setPicked([])}>清空</Button>}
              <Button size="sm" className="h-7 rounded-full" disabled={!picked.length} onClick={runPick} data-testid="pick-go">{pickMode === "brief" ? "生成简报" : "生成文章"}</Button>
              <Button variant="ghost" size="icon-sm" className="size-7 rounded-full text-faint" aria-label="退出选择" onClick={() => setPickMode(null)}><X /></Button>
            </div>
          </div>
        )}
        {briefOpen && briefScope && <BriefDialog key={`${briefScope}|${briefIds?.join(",") ?? "auto"}`} open={briefOpen} onOpenChange={setBriefOpen} section={briefScope === "all" ? null : briefScope} sectionLabel={briefLabel} ids={briefIds} />}
        {articleOpen && briefScope && <ArticleDialog key={`${briefScope}|${picked.join(",")}`} open={articleOpen} onOpenChange={setArticleOpen} section={briefScope} ids={picked} />}
    </div>
  );
}
