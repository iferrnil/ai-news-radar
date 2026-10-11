"use client";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { ThemeProvider, useTheme } from "next-themes";
import { Suspense, createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import {
  Activity, BookOpen, ChevronRight, Command as CommandIcon, Database, Inbox, Menu, Moon,
  Radio, Search, Settings2, Sun,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Kbd } from "@/components/ui/kbd";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Toaster } from "@/components/ui/sonner";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { DOCS, docHref } from "@/lib/docs-registry";
import { type Filters, countOf, p0For, parseFilters, setP0For, toQuery, viewKey } from "./filters";
import { cn } from "@/lib/utils";
import type { ItemOut } from "@/lib/types";
import { secIcon } from "./section-icon";
import { type SectionApiRow, type Section, type SourceRow, allSections, api, inAll, secZh, sectionOf, setSections } from "./common";
import { Languages, MinusCircle } from "lucide-react";
import dynamic from "next/dynamic";
// ⌘K palette (cmdk) is code-split and only mounted after first open
const Palette = dynamic(() => import("./palette"), { ssr: false });
// drawer, mobile nav sheet and translate popover are split the same way (Dialog/Popover/Switch stay out of first load)
const ItemSheet = dynamic(() => import("./item-sheet"), { ssr: false });
const MobileNav = dynamic(() => import("./mobile-nav"), { ssr: false });
const TranslateMenu = dynamic(() => import("./translate-menu"), { ssr: false, loading: () => <TrButton /> });
/** Placeholder with the same look as the translate popover trigger; first click loads the real menu (opened). */
function TrButton({ onClick }: { onClick?: () => void }) {
  const { zh } = useApp();
  return <Button variant="ghost" size="icon" aria-label="翻译设置" className={zh ? "text-brand" : ""} onClick={onClick}><Languages className="size-4" /></Button>;
}

export const SEC_ICON: Record<string, React.ElementType> = { all: Inbox };
export { secIcon };

type Ctx = {
  f: Filters; setF: (p: Partial<Filters>) => void;
  /** Switch the feed view (section / source), keeping the P0 toggle. */
  navigate: (p: Partial<Filters>) => void;
  facet: ItemOut[]; setFacet: (i: ItemOut[]) => void; isRead: (id: number) => boolean; markRead: (id: number) => void; sources: SourceRow[]; reloadSources: () => Promise<void>;
  live: boolean; setLive: (b: boolean) => void; open: ItemOut | null; setOpen: (i: ItemOut | null) => void;
  sections: Section[]; // built-in + user-created (re-render trigger; read via allSections()/secZh too)
  setPalette: (b: boolean) => void; addOpen: boolean; setAddOpen: (b: boolean) => void;
  onStream: (fn: (ev: string, data: any) => void) => () => void;
  zh: boolean; setZh: (b: boolean) => void;
};
const AppCtx = createContext<Ctx>(null as unknown as Ctx);
export const useApp = () => useContext(AppCtx);

/** Sidebar source row: click to filter; hover/focus (always on touch) reveals 取消订阅 with a small confirm + undo toast. */
function SourceRowNav({ s, active, onGo }: { s: SourceRow; active: boolean; onGo: () => void }) {
  const { reloadSources, f, navigate } = useApp();
  const [confirm, setConfirm] = useState(false);
  const label = s.name.replace(/^X @(\S+).*$/, "@$1");
  const setEnabled = (enabled: boolean) => api(`/api/sources/${s.id}`, { method: "PATCH", body: JSON.stringify({ enabled }) });
  const unsubscribe = async () => {
    setConfirm(false);
    try {
      await setEnabled(false);
      if (f.source === s.id) navigate({ source: null, section: "" });
      await reloadSources();
      toast(`已取消订阅「${label}」`, { description: "可在「数据源 › 停用」中重新启用", action: { label: "撤销", onClick: async () => { await setEnabled(true); await reloadSources(); toast.success(`已恢复订阅「${label}」`); } } });
    } catch (e) { toast.error("操作失败", { description: (e as Error).message }); }
  };
  return (
    <>
    <div className={cn("group/src relative flex h-7 items-center rounded-md transition-colors duration-150 ease-out", active ? "bg-sidebar-accent text-foreground" : "text-muted-foreground hover:bg-hover hover:text-foreground", confirm && "bg-hover text-foreground")}>
      <button onClick={onGo} title={s.last_error ?? s.name} className="flex h-full min-w-0 flex-1 items-center gap-2 rounded-md px-2 type-caption outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        <StatusDot s={s} />
        <span className="flex-1 truncate text-left">{label}</span>
        <span className={cn("font-mono type-meta tabular-nums text-faint transition-opacity duration-150 group-hover/src:opacity-0 group-focus-within/src:opacity-0 [@media(hover:none)]:hidden", confirm && "opacity-0")}>{s.item_count || ""}</span>
      </button>
      <Button variant="ghost" size="icon-xs" aria-label={`取消订阅 ${label}`} title="取消订阅" aria-expanded={confirm} onClick={() => setConfirm((v) => !v)}
        className={cn("absolute right-1 size-5 rounded-xs text-faint opacity-0 hover:bg-red-500/10 hover:text-red-500 focus-visible:opacity-100 group-hover/src:opacity-100 aria-expanded:bg-transparent [@media(hover:none)]:static [@media(hover:none)]:mr-1 [@media(hover:none)]:opacity-100", confirm && "text-red-500 opacity-100")}>
        <MinusCircle className="size-3.5" />
      </Button>
    </div>
    {/* inline confirm inside the sidebar, so nothing covers the feed */}
    {confirm && (
      <div role="alertdialog" aria-label={`确认取消订阅 ${label}`} onKeyDown={(e) => e.key === "Escape" && setConfirm(false)}
        className="mx-0.5 mb-1 mt-0.5 rounded-lg bg-popover p-3 type-caption leading-snug shadow-card ring-1 ring-border animate-in fade-in-0 slide-in-from-top-1 duration-150">
        <div className="font-medium text-foreground">取消订阅「{label}」？</div>
        <div className="mt-1 text-muted-foreground">停止抓取并隐藏内容，可在「数据源」页重新启用。</div>
        <div className="mt-2 flex justify-end gap-1.5">
          <Button size="xs" variant="ghost" onClick={() => setConfirm(false)}>保留</Button>
          <Button size="xs" variant="destructive" autoFocus onClick={unsubscribe}>取消订阅</Button>
        </div>
      </div>
    )}
    </>
  );
}

function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  return (
    <Tooltip>
      <TooltipTrigger render={<Button variant="ghost" size="icon" aria-label="切换主题" onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")} />}>
        <Sun className="hidden size-4 dark:block" /><Moon className="size-4 dark:hidden" />
      </TooltipTrigger>
      <TooltipContent>切换{resolvedTheme === "dark" ? "浅色" : "深色"}模式</TooltipContent>
    </Tooltip>
  );
}

export function StatusDot({ s }: { s: Pick<SourceRow, "enabled" | "supported" | "last_error" | "last_fetched"> }) {
  const c = !s.supported || !s.enabled ? "bg-faint/50" : s.last_error ? "bg-red-500 shadow-[0_0_6px_var(--color-red-500)]" : s.last_fetched ? "bg-emerald-500/90" : "bg-amber-400";
  return <span className={cn("inline-block size-1.5 shrink-0 rounded-full", c)} />;
}

function NavItem({ active, icon: Icon, label, count, onClick, className }: { active: boolean; icon: React.ElementType; label: string; count?: number; onClick: () => void; className?: string }) {
  return (
    <button onClick={onClick} className={cn("group relative flex h-8 w-full items-center gap-2.5 rounded-lg px-2.5 type-body font-medium tracking-[-0.005em] transition-colors duration-150 ease-out outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
      active ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]" : "text-muted-foreground hover:bg-hover hover:text-foreground", !active && count === 0 && "opacity-60", className)}>
      {active && <span className="absolute -left-2.5 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r-full bg-brand" />}
      <Icon className={cn("size-4 transition-colors duration-150 ease-out", active ? "text-brand" : "text-faint group-hover:text-foreground")} />
      <span className="min-w-0 flex-1 truncate text-left">{label}</span>
      {count != null && <span className={cn("font-mono type-meta tabular-nums", count === 0 ? "text-faint/60" : "text-faint")}>{count}</span>}
    </button>
  );
}

/** Sidebar section row: a plain filter row; every section gets a hover/focus ⚙ that opens 管理分区 on /sources. */
function SecNav({ s, active, count, onClick, onManage }: { s: Section; active: boolean; count: number; onClick: () => void; onManage: () => void }) {
  return (
    <div className="group/sec relative">
      <NavItem active={active} icon={secIcon(s)} label={s.zh} count={count} onClick={onClick} />
      <Button variant="ghost" size="icon-xs" aria-label={`管理分区「${s.zh}」`} onClick={onManage}
        className="absolute right-1.5 top-1 z-10 bg-sidebar text-faint opacity-0 transition-opacity duration-150 hover:text-foreground group-hover/sec:opacity-100 focus-visible:opacity-100">
        <Settings2 />
      </Button>
    </div>
  );
}

/** 订阅源 › one collapsible folder per section listing its sources. Open by default; collapsed folders persist. */
const FOLDERS_KEY = "sidebar.folders.closed";
function useClosedFolders() {
  const [closed, setClosed] = useState<Set<string>>(new Set());
  useEffect(() => { try { setClosed(new Set(JSON.parse(localStorage.getItem(FOLDERS_KEY) || "[]"))); } catch {} }, []);
  const toggle = useCallback((k: string) => setClosed((cur) => {
    const n = new Set(cur); if (n.has(k)) n.delete(k); else n.add(k);
    try { localStorage.setItem(FOLDERS_KEY, JSON.stringify([...n])); } catch {}
    return n;
  }), []);
  return [closed, toggle] as const;
}
function SourceFolder({ s, icon: Icon, list, open, onToggle, children }: { s: Section; icon: React.ElementType; list: SourceRow[]; open: boolean; onToggle: () => void; children: React.ReactNode }) {
  return (
    <Collapsible open={open} onOpenChange={onToggle} className="pt-1">
      <CollapsibleTrigger aria-label={`文件夹：${s.zh}`} className="group flex h-7 w-full items-center gap-1.5 rounded-md px-2 type-meta text-faint outline-none transition-colors duration-150 ease-out hover:bg-hover hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50">
        <ChevronRight className="size-3.5 shrink-0 transition-transform duration-200 ease-out group-data-[panel-open]:rotate-90" />
        <Icon className="size-3.5 shrink-0 opacity-80" />
        <span className="min-w-0 flex-1 truncate text-left">{s.zh}</span>
        <span className="font-mono tabular-nums" title={`${list.length} 个来源`}>{list.length}</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div role="group" aria-label={`${s.zh} 的订阅源`} className="relative ml-[13px] space-y-px border-l border-border pl-1.5">{children}</div>
      </CollapsibleContent>
    </Collapsible>
  );
}

function SectionLabel({ children, first }: { children: React.ReactNode; first?: boolean }) {
  return <div className={cn("flex h-6 items-center px-2 type-meta font-medium tracking-[0.04em] text-faint", first ? "mt-1" : "mt-4")}>{children}</div>;
}

const DOCS_KEY = "sidebar.docs.open";
function DocsGroup({ path, onGo }: { path: string; onGo: (href: string) => void }) {
  const inDocs = path.startsWith("/docs");
  const [open, setOpen] = useState(true);
  useEffect(() => { const v = localStorage.getItem(DOCS_KEY); if (v != null) setOpen(v === "1"); }, []);
  const toggle = (b: boolean) => { setOpen(b); localStorage.setItem(DOCS_KEY, b ? "1" : "0"); };
  return (
    <Collapsible open={open} onOpenChange={toggle}>
      <div className={cn("group relative flex h-8 w-full items-center rounded-lg type-body font-medium tracking-[-0.005em] transition-colors duration-150 ease-out",
        path === "/docs" ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]" : inDocs ? "text-foreground" : "text-muted-foreground hover:bg-hover hover:text-foreground")}>
        {path === "/docs" && <span className="absolute -left-2.5 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r-full bg-brand" />}
        <button onClick={() => { onGo("/docs"); if (!open) toggle(true); }} className="flex h-full flex-1 items-center gap-2.5 rounded-lg pl-2.5 outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <BookOpen className={cn("size-4 transition-colors duration-150 ease-out", inDocs ? "text-brand" : "text-faint group-hover:text-foreground")} />
          <span className="flex-1 text-left">文档</span>
        </button>
        <CollapsibleTrigger aria-label={open ? "收起文档" : "展开文档"} className="mr-1 grid size-6 place-items-center rounded-md text-faint outline-none transition-colors duration-150 ease-out hover:bg-hover hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50">
          <ChevronRight className={cn("size-3.5 transition-transform duration-200 ease-out", open && "rotate-90")} />
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent>
        <div className="relative ml-[17px] mt-px space-y-px border-l border-border py-0.5 pl-2">
          {DOCS.map((d) => {
            const on = path === docHref(d);
            return (
              <button key={d.slug} onClick={() => onGo(docHref(d))}
                className={cn("relative flex h-7 w-full items-center gap-2 rounded-md px-2 type-caption transition-colors duration-150 ease-out outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                  on ? "bg-sidebar-accent font-medium text-foreground" : "text-muted-foreground hover:bg-hover hover:text-foreground")}>
                {on && <span className="absolute -left-[9.5px] top-1/2 h-3.5 w-px -translate-y-1/2 bg-brand" />}
                <span className="truncate">{d.title}</span>
              </button>
            );
          })}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

export function SidebarBody({ onNavigate }: { onNavigate?: () => void }) {
  const { f, navigate, sources, facet, sections } = useApp();
  const router = useRouter(); const path = usePathname();
  const active = sources.filter((s) => s.enabled && s.supported);
  const feed = path === "/";
  const count = (section: Filters["section"]) => facet.length ? countOf(facet, { section, source: null, p0: p0For(section || "all") })
    : active.filter((s) => { const k = sectionOf({ ...s, is_x: s.type.startsWith("x") }); return section ? k === section : inAll(k); }).reduce((a, s) => a + s.item_count, 0);
  const go = (p: Partial<Filters>) => { navigate({ section: "", source: null, ...p }); onNavigate?.(); };
  const on = (section: Filters["section"]) => feed && f.source == null && f.section === section;
  const errs = active.filter((s) => s.last_error).length;
  const manage = () => { router.push("/sources?sections=1"); onNavigate?.(); };
  const [closed, toggleFolder] = useClosedFolders();
  const listOf = (k: string) => active.filter((s) => sectionOf({ ...s, is_x: s.type.startsWith("x") }) === k);
  const sec = (s: Section) => <SecNav key={s.key} s={s} active={on(s.key)} count={count(s.key)} onClick={() => go({ section: s.key })} onManage={manage} />;
  const outside = sections.filter((s) => !inAll(s.key));
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-14 items-center gap-2.5 px-4">
        <div className="grid size-7 place-items-center rounded-lg bg-gradient-to-b from-violet-500 to-indigo-600 text-white shadow-[0_0_0_1px_rgb(255_255_255/0.15)_inset,0_4px_12px_-2px_oklch(0.56_0.22_285/0.5)]">
          <Radio className="size-4" />
        </div>
        <div className="leading-tight">
          <div className="type-body font-semibold tracking-tight">AI Radar</div>
          <div className="type-meta text-muted-foreground">国内外 AI 情报</div>
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-0.5 px-2.5 pb-3">
          <SectionLabel first>信息流</SectionLabel>
          <NavItem active={on("")} icon={Inbox} label="全部" count={count("")} onClick={() => go({})} />
          {sections.filter((s) => inAll(s.key)).map(sec)}
          {/* sections outside 全部 (e.g. 娱乐) sit below a divider */}
          {outside.length > 0 && <div className="mx-2 my-2 h-px bg-border" role="separator" title="不计入「全部」" />}
          {outside.map(sec)}
          <SectionLabel>管理</SectionLabel>
          <NavItem active={path === "/sources"} icon={Database} label="数据源" count={active.length} onClick={() => { router.push("/sources"); onNavigate?.(); }} />
          <DocsGroup path={path} onGo={(h) => { router.push(h); onNavigate?.(); }} />
          <Collapsible defaultOpen className="pt-4">
            <CollapsibleTrigger className="group flex h-6 w-full items-center gap-1 px-2 type-meta font-medium tracking-[0.04em] text-faint transition-colors duration-150 ease-out hover:text-foreground">
              <ChevronRight className="size-3.5 transition-transform duration-200 ease-out group-data-[panel-open]:rotate-90" />订阅源<span className="ml-auto font-mono tabular-nums">{active.length}</span>
            </CollapsibleTrigger>
            <CollapsibleContent className="space-y-px">
              {sections.map((sec) => {
                const list = listOf(sec.key);
                if (!list.length) return null;
                return (
                  <SourceFolder key={sec.key} s={sec} icon={secIcon(sec)} list={list} open={!closed.has(sec.key)} onToggle={() => toggleFolder(sec.key)}>
                    {list.map((s) => <SourceRowNav key={s.id} s={s} active={f.source === s.id && feed} onGo={() => go({ source: s.id })} />)}
                  </SourceFolder>
                );
              })}
            </CollapsibleContent>
          </Collapsible>
        </div>
      </ScrollArea>
      <div className="border-t border-sidebar-border p-3 type-meta text-faint">
        <div className="flex h-5 items-center gap-2 tabular-nums"><Activity className="size-3.5" />{active.length} 个源在线{errs ? <span className="text-red-500">· {errs} 报错</span> : null}</div>
      </div>
    </div>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  const sp = useSearchParams();
  const router = useRouter();
  const path = usePathname();
  const f = useMemo(() => parseFilters(new URLSearchParams(sp.toString())), [sp]);
  const [facet, setFacet] = useState<ItemOut[]>([]);
  const [read, setRead] = useState<Set<number>>(new Set());
  useEffect(() => { try { setRead(new Set(JSON.parse(localStorage.getItem("read.ids") ?? "[]"))); } catch {} }, []);
  const markRead = useCallback((id: number) => setRead((cur) => {
    if (cur.has(id)) return cur;
    const n = new Set(cur); n.add(id);
    localStorage.setItem("read.ids", JSON.stringify([...n].slice(-3000)));
    return n;
  }), []);
  const isRead = useCallback((id: number) => read.has(id), [read]);
  const [sources, setSources] = useState<SourceRow[]>([]);
  const [live, setLive] = useState(false);
  const [zh, setZhState] = useState(true);
  useEffect(() => { setZhState(localStorage.getItem("zh") !== "0"); }, []);
  const setZh = useCallback((b: boolean) => { setZhState(b); localStorage.setItem("zh", b ? "1" : "0"); }, []);
  const [open, setOpenState] = useState<ItemOut | null>(null);
  const setOpen = useCallback((i: ItemOut | null) => { setOpenState(i); if (i) markRead(i.id); }, [markRead]);
  const [palette, setPalette] = useState(false);
  const [paletteUsed, setPaletteUsed] = useState(false);
  useEffect(() => { if (palette) setPaletteUsed(true); }, [palette]);
  const [mobileNav, setMobileNav] = useState(false);
  const [navUsed, setNavUsed] = useState(false);
  useEffect(() => { if (mobileNav) setNavUsed(true); }, [mobileNav]);
  const [drawerUsed, setDrawerUsed] = useState(false);
  useEffect(() => { if (open) setDrawerUsed(true); }, [open]);
  const [trUsed, setTrUsed] = useState(false);
  // warm the split chunks once the page is idle so the first drawer / ⌘K open has no load delay
  useEffect(() => {
    const warm = () => { import("./item-sheet"); import("./palette"); };
    if ("requestIdleCallback" in window) { const id = requestIdleCallback(warm, { timeout: 4000 }); return () => cancelIdleCallback(id); }
    const t = setTimeout(warm, 2500); return () => clearTimeout(t);
  }, []);
  const [addOpen, setAddOpen] = useState(false);
  const reloadSourcesRef = useRef<(() => void) | null>(null);
  const reloadNowRef = useRef<(() => void) | null>(null);
  const listeners = useRef(new Set<(ev: string, data: any) => void>());
  const onStream = useCallback((fn: (ev: string, data: any) => void) => { listeners.current.add(fn); return () => { listeners.current.delete(fn); }; }, []);
  useEffect(() => {
    const es = new EventSource("/api/stream");
    es.onopen = () => setLive(true);
    es.onerror = () => setLive(false);
    for (const ev of ["item", "update", "removed", "sources"]) es.addEventListener(ev, (e) => {
      const d = JSON.parse((e as MessageEvent).data); listeners.current.forEach((fn) => fn(ev, d));
      if (ev === "item") reloadSourcesRef.current?.();
      if (ev === "sources") reloadNowRef.current?.();
    });
    return () => es.close();
  }, []);
  const fRef = useRef(f);
  useEffect(() => { fRef.current = f; }, [f]);
  // switching view restores that view's own 仅 P0; toggling P0 stores it for the current view only
  const navigate = useCallback((p: Partial<Filters>) => {
    const next = { ...fRef.current, ...p };
    if ("p0" in p) setP0For(viewKey(next), !!p.p0); else next.p0 = p0For(viewKey(next));
    router.push(toQuery(next), { scroll: false });
  }, [router]);
  const setF = navigate;
  // first load without ?p0: apply the stored value for this view
  useEffect(() => {
    if (path !== "/" || new URLSearchParams(window.location.search).has("p0")) return;
    if (p0For(viewKey(fRef.current))) router.replace(toQuery({ ...fRef.current, p0: true }), { scroll: false });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const [sections, setSectionList] = useState<Section[]>(() => allSections());
  const reloadSources = useCallback(async () => {
    const [src, secs] = await Promise.all([api<SourceRow[]>("/api/sources"), api<SectionApiRow[]>("/api/sections").catch(() => null)]);
    if (secs) { setSections(secs); setSectionList(allSections()); }
    setSources(src);
  }, []);
  useEffect(() => { let t: ReturnType<typeof setTimeout> | null = null; reloadSourcesRef.current = () => { if (!t) t = setTimeout(() => { t = null; reloadSources(); }, 3000); }; }, [reloadSources]);
  useEffect(() => { reloadNowRef.current = () => { reloadSources(); }; }, [reloadSources]);
  // structural changes arrive via the SSE "sources" event; this slow poll only refreshes fetch status dots, and pauses in hidden tabs
  useEffect(() => { reloadSources(); const t = setInterval(() => { if (!document.hidden) reloadSources(); }, 60000); return () => clearInterval(t); }, [reloadSources]);
  useEffect(() => {
    const h = (e: KeyboardEvent) => { if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); setPalette((p) => !p); } };
    window.addEventListener("keydown", h); return () => window.removeEventListener("keydown", h);
  }, []);
  const ctx = useMemo(() => ({ f, setF, navigate, facet, setFacet, isRead, markRead, sources, reloadSources, sections, live, setLive, open, setOpen, setPalette, addOpen, setAddOpen, onStream, zh, setZh }),
    [f, setF, navigate, facet, isRead, markRead, sources, reloadSources, sections, live, open, setOpen, addOpen, onStream, zh, setZh]);
  const title = path === "/sources" ? "数据源" : path.startsWith("/docs") ? ["文档", DOCS.find((d) => path === docHref(d))?.title].filter(Boolean).join(" / ")
    : f.source != null ? sources.find((s) => s.id === f.source)?.name ?? "来源" : f.section ? secZh(f.section) : "全部动态";

  return (
    <AppCtx.Provider value={ctx}>
      <div className="flex min-h-screen">
        <aside className="sticky top-0 hidden h-screen w-[244px] shrink-0 border-r border-sidebar-border bg-sidebar lg:block"><SidebarBody /></aside>
        {navUsed && <MobileNav open={mobileNav} onOpenChange={setMobileNav} />}
        <div className="glow flex min-w-0 flex-1 flex-col">
          <header className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background/70 px-3 backdrop-blur-xl backdrop-saturate-150 sm:px-5">
            <Button variant="ghost" size="icon" className="lg:hidden" onClick={() => setMobileNav(true)} aria-label="菜单"><Menu className="size-4" /></Button>
            <div className="hidden min-w-0 truncate type-ui font-semibold tracking-tight sm:block">{title}</div>
            <Button variant="outline" onClick={() => setPalette(true)}
              className="ml-auto w-full min-w-0 max-w-[360px] shrink justify-start gap-2 bg-muted px-2.5 type-body font-normal text-faint hover:text-muted-foreground dark:bg-muted sm:ml-6 sm:mr-auto">
              <Search className="size-3.5" /><span className="truncate">搜索新闻、来源、命令…</span>
              <Kbd className="ml-auto hidden bg-background/80 font-mono shadow-hair sm:inline-flex"><CommandIcon className="size-2.5" />K</Kbd>
            </Button>
            <Tooltip>
              <TooltipTrigger render={<Badge variant="outline" className="hidden h-6 gap-1.5 px-2.5 text-muted-foreground sm:inline-flex" />}>
                <span className="relative flex size-1.5">
                  {live && <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-75" />}
                  <span className={cn("relative inline-flex size-1.5 rounded-full", live ? "bg-emerald-500" : "bg-amber-500")} />
                </span>{live ? "Live" : "重连中"}
              </TooltipTrigger>
              <TooltipContent>{live ? "实时推送已连接（SSE）" : "实时推送已断开，正在自动重连；内容仍可浏览"}</TooltipContent>
            </Tooltip>
            {trUsed ? <TranslateMenu defaultOpen /> : <TrButton onClick={() => setTrUsed(true)} />}
            <ThemeToggle />
            <Button variant="ghost" size="icon" render={<Link href="/sources" aria-label="数据源" />} className="hidden sm:inline-flex"><Database className="size-4" /></Button>
          </header>
          <main className="min-w-0 flex-1">{children}</main>
        </div>
      </div>
      {paletteUsed && <Palette open={palette} onOpenChange={setPalette} />}
      {drawerUsed && <ItemSheet />}
      <Toaster position="bottom-right" />
    </AppCtx.Provider>
  );
}

export default function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false} disableTransitionOnChange>
      <TooltipProvider delay={300}><Suspense><Shell>{children}</Shell></Suspense></TooltipProvider>
    </ThemeProvider>
  );
}
