"use client";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { ThemeProvider, useTheme } from "next-themes";
import { Suspense, createContext, memo, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import {
  Activity, BookOpen, ChevronRight, Command as CommandIcon, Database, Inbox, Menu, Moon,
  PanelLeft, Radio, Search, Settings, Settings2, Sun,
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
import { type Filters, countOf, p0For, parseFilters, setP0For, setUnreadPref, toQuery, unreadPref, viewKey } from "./filters";
import * as rs from "./read-store";
import { initReadSync, useReadVersion } from "./read-store";
import { handleBatch } from "./notify-client";
import { cn } from "@/lib/utils";
import type { ItemOut } from "@/lib/types";
import { secIcon } from "./section-icon";
import { HealthDot, healthOf, healthText } from "./source-health";
import { type SectionApiRow, type Section, type SourceRow, type TrInfo, allSections, api, inAll, localize, secZh, sectionOf, setSections, trOnFor } from "./common";
import { Flame, Languages, MinusCircle, Star } from "lucide-react";
import dynamic from "next/dynamic";
// ⌘K palette (cmdk) is code-split and only mounted after first open
const Palette = dynamic(() => import("./palette"), { ssr: false });
// drawer, mobile nav sheet and translate popover are split the same way (Dialog/Popover/Switch stay out of first load)
const ItemSheet = dynamic(() => import("./item-sheet"), { ssr: false });
const MobileNav = dynamic(() => import("./mobile-nav"), { ssr: false });
const TranslateMenu = dynamic(() => import("./translate-menu"), { ssr: false, loading: () => <TrButton /> });
/** Placeholder with the same look as the translate popover trigger; first click loads the real menu (opened). */
function TrButton({ onClick }: { onClick?: () => void }) {
  const { trView } = useApp();
  return <Button variant="ghost" size="icon" aria-label="翻译" aria-pressed={trView} data-testid="tr-toggle" className={trView ? "text-brand" : ""} onClick={onClick}><Languages className="size-4" /></Button>;
}

export const SEC_ICON: Record<string, React.ElementType> = { all: Inbox };
export { secIcon };

type Ctx = {
  f: Filters; setF: (p: Partial<Filters>) => void;
  /** Switch the feed view (section / source), keeping the P0 toggle. */
  navigate: (p: Partial<Filters>) => void;
  facet: ItemOut[]; setFacet: (i: ItemOut[]) => void; isRead: (id: number) => boolean; markRead: (id: number) => void; markManyRead: (ids: number[]) => boolean; markManyUnread: (ids: number[]) => void; readVer: number; sources: SourceRow[]; reloadSources: () => Promise<void>;
  live: boolean; setLive: (b: boolean) => void; open: ItemOut | null; setOpen: (i: ItemOut | null) => void;
  sections: Section[]; // built-in + user-created (re-render trigger; read via allSections()/secZh too)
  setPalette: (b: boolean) => void; addOpen: boolean; setAddOpen: (b: boolean) => void;
  onStream: (fn: (ev: string, data: any) => void) => () => void;
  /** translation scopes + availability (null until loaded); trView = the current view's scope is translated; loc = show translation if its scope is on */
  tr: TrInfo | null; settingsLocked: boolean; reloadTr: () => Promise<void>; trView: boolean; loc: (it: ItemOut) => ItemOut;
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
      <button onClick={onGo} title={[s.name, healthText(s), s.last_error].filter(Boolean).join("\n")} data-health={healthOf(s).level} className="flex h-full min-w-0 flex-1 items-center gap-2 rounded-md px-2 type-caption outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
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

export function StatusDot({ s }: { s: SourceRow }) {
  const h = healthOf(s);
  return <HealthDot level={h.level === "unknown" && s.last_fetched && !s.last_error ? "ok" : h.level} />;
}

function NavItem({ active, icon: Icon, label, count, unread, onClick, className }: { active: boolean; icon: React.ElementType; label: string; count?: number; unread?: number; onClick: () => void; className?: string }) {
  return (
    <button onClick={onClick} className={cn("group relative flex h-8 w-full items-center gap-2.5 rounded-lg px-2.5 type-body font-medium tracking-[-0.005em] transition-colors duration-150 ease-out outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
      active ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]" : "text-muted-foreground hover:bg-hover hover:text-foreground", !active && count === 0 && "opacity-60", className)}>
      {active && <span className="absolute -left-2.5 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r-full bg-brand" />}
      <Icon className={cn("size-4 transition-colors duration-150 ease-out", active ? "text-brand" : "text-faint group-hover:text-foreground")} />
      <span className="min-w-0 flex-1 truncate text-left">{label}</span>
      {/* unread count (brand) while there is something unread, else the total */}
      {count != null && (unread ? <span data-testid="unread-count" title={`未读 ${unread} / 共 ${count}`} className="font-mono type-meta font-medium tabular-nums text-brand">{unread}</span>
        : <span title={unread === 0 && count ? `全部已读 · 共 ${count}` : undefined} className={cn("font-mono type-meta tabular-nums", count === 0 ? "text-faint/60" : "text-faint")}>{count}</span>)}
    </button>
  );
}

/** Sidebar section row: a plain filter row; every section gets a hover/focus ⚙ that opens 管理分区 on /sources. */
function SecNav({ s, active, count, unread, onClick, onManage }: { s: Section; active: boolean; count: number; unread?: number; onClick: () => void; onManage: () => void }) {
  return (
    <div className="group/sec relative">
      <NavItem active={active} icon={secIcon(s)} label={s.zh} count={count} unread={unread} onClick={onClick} />
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
    <Collapsible open={open} onOpenChange={onToggle}>
      <CollapsibleTrigger aria-label={`文件夹：${s.zh}`} className="group flex h-7 w-full items-center gap-1.5 rounded-md px-2 type-caption text-muted-foreground outline-none transition-colors duration-150 ease-out hover:bg-hover hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50">
        <ChevronRight className="size-3.5 shrink-0 transition-transform duration-200 ease-out group-data-[panel-open]:rotate-90" />
        <Icon className="size-3.5 shrink-0 opacity-80" />
        <span className="min-w-0 flex-1 truncate text-left">{s.zh}</span>
        <span className="font-mono tabular-nums" title={`${list.length} 个来源`}>{list.length}</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div role="group" aria-label={`${s.zh} 的订阅源`} className="relative ml-[13px] space-y-px border-l border-border pb-0.5 pl-1.5">{children}</div>
      </CollapsibleContent>
    </Collapsible>
  );
}

function SectionLabel({ children, first }: { children: React.ReactNode; first?: boolean }) {
  return <div className={cn("flex h-6 items-center px-2 type-meta font-medium tracking-[0.04em] text-faint", first ? "mt-1" : "mt-4")}>{children}</div>;
}

const DOCS_KEY = "sidebar.docs.open";
export const SIDEBAR_KEY = "sidebar.collapsed";
const MOD = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘" : "Ctrl+";
const SOURCES_KEY = "sidebar.sources.open";
/** Collapsible sidebar group (文档, 数据源): header row + chevron; open state persisted in localStorage `storageKey`.
 *  `onHeader` (optional) makes the label navigate (文档 → /docs); without it the whole header toggles. */
function NavGroup({ icon: Icon, label, count, storageKey, defaultOpen = true, current, inside, onHeader, children }: {
  icon: React.ElementType; label: string; count?: number; storageKey: string; defaultOpen?: boolean; current?: boolean; inside?: boolean; onHeader?: () => void; children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  useEffect(() => { const v = localStorage.getItem(storageKey); if (v != null) setOpen(v === "1"); }, [storageKey]);
  const toggle = (b: boolean) => { setOpen(b); localStorage.setItem(storageKey, b ? "1" : "0"); };
  return (
    <Collapsible open={open} onOpenChange={toggle}>
      <div className={cn("group relative flex h-8 w-full items-center rounded-lg type-body font-medium tracking-[-0.005em] transition-colors duration-150 ease-out",
        current ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]" : inside ? "text-foreground" : "text-muted-foreground hover:bg-hover hover:text-foreground")}>
        {current && <span className="absolute -left-2.5 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r-full bg-brand" />}
        <button onClick={() => { if (onHeader) { onHeader(); if (!open) toggle(true); } else toggle(!open); }} aria-expanded={onHeader ? undefined : open}
          className="flex h-full min-w-0 flex-1 items-center gap-2.5 rounded-lg pl-2.5 outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <Icon className={cn("size-4 transition-colors duration-150 ease-out", inside ? "text-brand" : "text-faint group-hover:text-foreground")} />
          <span className="flex-1 truncate text-left">{label}</span>
          {count != null && <span className="font-mono type-meta tabular-nums text-faint">{count}</span>}
        </button>
        <CollapsibleTrigger aria-label={open ? `收起${label}` : `展开${label}`} className="mr-1 grid size-6 place-items-center rounded-md text-faint outline-none transition-colors duration-150 ease-out hover:bg-hover hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50">
          <ChevronRight className={cn("size-3.5 transition-transform duration-200 ease-out", open && "rotate-90")} />
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent>
        <div className="relative ml-[17px] mt-px space-y-px border-l border-border py-0.5 pl-2">{children}</div>
      </CollapsibleContent>
    </Collapsible>
  );
}
/** child row inside a NavGroup (doc pages, 管理数据源) */
function GroupLink({ on, icon: Icon, children, onClick }: { on: boolean; icon?: React.ElementType; children: React.ReactNode; onClick: () => void }) {
  return (
    <button onClick={onClick}
      className={cn("relative flex h-7 w-full items-center gap-2 rounded-md px-2 type-caption transition-colors duration-150 ease-out outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
        on ? "bg-sidebar-accent font-medium text-foreground" : "text-muted-foreground hover:bg-hover hover:text-foreground")}>
      {on && <span className="absolute -left-[9.5px] top-1/2 h-3.5 w-px -translate-y-1/2 bg-brand" />}
      {Icon && <Icon className="size-3.5 shrink-0 text-faint" />}
      <span className="truncate">{children}</span>
    </button>
  );
}

// memo: toggling the sidebar re-renders AppShell; the (large) sidebar tree itself does not need to re-render
export const SidebarBody = memo(function SidebarBody({ onNavigate }: { onNavigate?: () => void }) {
  const { f, navigate, sources, facet, sections, isRead } = useApp();
  const router = useRouter(); const path = usePathname();
  const active = sources.filter((s) => s.enabled && s.supported);
  const feed = path === "/";
  const count = (section: Filters["section"]) => facet.length ? countOf(facet, { section, source: null, p0: p0For(section || "all") })
    : active.filter((s) => { const k = sectionOf({ ...s, is_x: s.type.startsWith("x") }); return section ? k === section : inAll(k); }).reduce((a, s) => a + s.item_count, 0);
  // unread per view (same filter as the feed, incl. that view's 仅 P0); null until items are loaded
  const unread = (section: Filters["section"]) => facet.length ? countOf(facet, { section, source: null, p0: p0For(section || "all"), unread: true }, isRead) : undefined;
  const go = (p: Partial<Filters>) => { navigate({ section: "", source: null, view: "", ...p }); onNavigate?.(); };
  const on = (section: Filters["section"]) => feed && !f.view && f.source == null && f.section === section;
  const errs = active.filter((s) => s.last_error).length;
  const weak = active.filter((s) => { const l = healthOf(s).level; return l === "degraded" || l === "open"; }).length;
  const manage = () => { router.push("/sources?sections=1"); onNavigate?.(); };
  const [closed, toggleFolder] = useClosedFolders();
  const listOf = (k: string) => active.filter((s) => sectionOf({ ...s, is_x: s.type.startsWith("x") }) === k);
  const sec = (s: Section) => <SecNav key={s.key} s={s} active={on(s.key)} count={count(s.key)} unread={unread(s.key)} onClick={() => go({ section: s.key })} onManage={manage} />;
  const outside = sections.filter((s) => !inAll(s.key));
  return (
    <div className="flex h-full flex-col">
      <div className="app-brand flex h-14 items-center gap-2.5 px-4">
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
          <NavItem active={on("")} icon={Inbox} label="全部" count={count("")} unread={unread("")} onClick={() => go({})} />
          {sections.filter((s) => inAll(s.key)).map(sec)}
          {/* sections outside 全部 (e.g. 娱乐) sit below a divider */}
          {outside.length > 0 && <div className="mx-2 my-2 h-px bg-border" role="separator" title="不计入「全部」" />}
          {outside.map(sec)}
          <SectionLabel>选题</SectionLabel>
          <NavItem active={feed && f.view === "hot"} icon={Flame} label="热点" onClick={() => go({ view: "hot" })} />
          <NavItem active={feed && f.view === "fav"} icon={Star} label="收藏" onClick={() => go({ view: "fav" })} />
          <SectionLabel>管理</SectionLabel>
          <NavGroup icon={Database} label="数据源" count={active.length} storageKey={SOURCES_KEY} defaultOpen={false} inside={path === "/sources"}>
            {sections.map((sec) => {
              const list = listOf(sec.key);
              if (!list.length) return null;
              return (
                <SourceFolder key={sec.key} s={sec} icon={secIcon(sec)} list={list} open={!closed.has(sec.key)} onToggle={() => toggleFolder(sec.key)}>
                  {list.map((s) => <SourceRowNav key={s.id} s={s} active={f.source === s.id && feed} onGo={() => go({ source: s.id })} />)}
                </SourceFolder>
              );
            })}
          </NavGroup>
          <NavGroup icon={BookOpen} label="文档" storageKey={DOCS_KEY} current={path === "/docs"} inside={path.startsWith("/docs")} onHeader={() => { router.push("/docs"); onNavigate?.(); }}>
            {DOCS.map((d) => <GroupLink key={d.slug} on={path === docHref(d)} onClick={() => { router.push(docHref(d)); onNavigate?.(); }}>{d.title}</GroupLink>)}
          </NavGroup>
          <NavItem active={path === "/settings"} icon={Settings} label="设置" onClick={() => { router.push("/settings"); onNavigate?.(); }} />
        </div>
      </ScrollArea>
      <div className="border-t border-sidebar-border p-3 type-meta text-faint">
        <div className="flex h-5 items-center gap-2 tabular-nums"><Activity className="size-3.5" />{active.length} 个源在线{errs ? <span className="text-red-500">· {errs} 报错</span> : null}{weak ? <span className="text-amber-500" title="成功率低于 90% 或已熔断">· {weak} 不稳定</span> : null}</div>
      </div>
    </div>
  );
});

function Shell({ children }: { children: React.ReactNode }) {
  const sp = useSearchParams();
  const router = useRouter();
  const path = usePathname();
  const f = useMemo(() => parseFilters(new URLSearchParams(sp.toString())), [sp]);
  const [facet, setFacet] = useState<ItemOut[]>([]);
  // read state: see read-store.ts (localStorage + optional server mirror); readVer bumps on every change
  const readVer = useReadVersion();
  const markRead = useCallback((id: number) => { rs.markRead([id]); }, []);
  // eslint-disable-next-line react-hooks/exhaustive-deps -- new identity per read change, so memoized consumers re-render
  const isRead = useCallback((id: number) => rs.isRead(id), [readVer]);
  const [sources, setSources] = useState<SourceRow[]>([]);
  const [live, setLive] = useState(false);
  const [tr, setTr] = useState<TrInfo | null>(null);
  const [settingsLocked, setSettingsLocked] = useState(false);
  const reloadTr = useCallback(async () => {
    try { const r = await api<{ translate: TrInfo; deepseek?: { locked?: boolean } }>("/api/settings"); setTr(r.translate); setSettingsLocked(!!r.deepseek?.locked); } catch {}
  }, []);
  useEffect(() => { void reloadTr(); }, [reloadTr]);
  const scopes = tr?.scopes;
  const loc = useCallback((it: ItemOut) => (trOnFor(scopes, it.source_id, it.section) ? localize(it) : it), [scopes]);
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
  const reloadTrRef = useRef(reloadTr);
  useEffect(() => { reloadTrRef.current = reloadTr; }, [reloadTr]);
  const listeners = useRef(new Set<(ev: string, data: any) => void>());
  const onStream = useCallback((fn: (ev: string, data: any) => void) => { listeners.current.add(fn); return () => { listeners.current.delete(fn); }; }, []);
  useEffect(() => {
    const es = new EventSource("/api/stream");
    es.onopen = () => setLive(true);
    es.onerror = () => setLive(false);
    for (const ev of ["item", "update", "removed", "sources", "notify", "translate", "fav", "watch", "keyword"]) es.addEventListener(ev, (e) => {
      const d = JSON.parse((e as MessageEvent).data); listeners.current.forEach((fn) => fn(ev, d));
      if ((ev === "notify" || ev === "keyword") && d.batch) handleBatch(d.batch, (id) => openByIdRef.current(id), ev === "keyword");
      if (ev === "item") reloadSourcesRef.current?.();
      if (ev === "sources") reloadNowRef.current?.();
      if (ev === "translate") reloadTrRef.current();
    });
    return () => es.close();
  }, []);
  const fRef = useRef(f);
  useEffect(() => { fRef.current = f; }, [f]);
  // open an item by id from anywhere (notification click, desktop shell via window.__radarOpenItem): the Feed reads ?item=
  const openByIdRef = useRef((id: number) => { const q = toQuery({ ...fRef.current }); router.push(`${q}${q.includes("?") ? "&" : "?"}item=${id}`, { scroll: false }); });
  useEffect(() => {
    const w = window as unknown as { __radarOpenItem?: (id: number) => void };
    w.__radarOpenItem = (id) => openByIdRef.current(Number(id));
    const t = setTimeout(() => { void initReadSync(); }, 1500); // off the critical path
    return () => { clearTimeout(t); delete w.__radarOpenItem; };
  }, []);
  // switching view restores that view's own 仅 P0; toggling P0 stores it for the current view only
  const navigate = useCallback((p: Partial<Filters>) => {
    const next = { ...fRef.current, ...p };
    if (("section" in p || "source" in p) && !("view" in p)) next.view = ""; // picking a section / source leaves 热点 / 收藏
    if ("p0" in p) setP0For(viewKey(next), !!p.p0); else next.p0 = p0For(viewKey(next));
    if ("unread" in p) setUnreadPref(!!p.unread);
    router.push(toQuery(next), { scroll: false });
  }, [router]);
  const setF = navigate;
  // first load without ?p0: apply the stored value for this view
  useEffect(() => {
    const qs = new URLSearchParams(window.location.search);
    if (path !== "/" || qs.has("item")) return;
    const p0 = qs.has("p0") ? fRef.current.p0 : p0For(viewKey(fRef.current)), unread = qs.has("unread") ? !!fRef.current.unread : unreadPref();
    if (p0 !== fRef.current.p0 || unread !== !!fRef.current.unread) router.replace(toQuery({ ...fRef.current, p0, unread }), { scroll: false });
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
  // sidebar collapse (≥lg only; mobile keeps its sheet). The <html data-sidebar> attribute is set before paint by the
  // inline script in layout.tsx, so a collapsed sidebar never flashes open; CSS in globals.css does the animation.
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => { setCollapsed(document.documentElement.dataset.sidebar === "collapsed"); }, []);
  const lastToggle = useRef(-Infinity); // not 0: performance.now() starts near 0 on every page load
  const toggleSidebar = useCallback(() => {
    // the desktop menu accelerator and the page keydown may both fire for one ⌘B: count it once
    const now = performance.now(); if (now - lastToggle.current < 250) return; lastToggle.current = now;
    setCollapsed((c) => {
      const n = !c;
      document.documentElement.dataset.sidebar = n ? "collapsed" : "expanded";
      try { localStorage.setItem(SIDEBAR_KEY, n ? "1" : "0"); } catch {}
      return n;
    });
  }, []);
  const desktopCmd = useCallback((cmd: string) => {
    if (cmd === "sidebar") { if (window.matchMedia("(min-width: 1024px)").matches) toggleSidebar(); else setMobileNav((v) => !v); }
    else if (cmd === "search") setPalette(true);
    else if (cmd === "settings") router.push("/settings");
  }, [toggleSidebar, router]);
  useEffect(() => {
    const w = window as unknown as { __radarCmd?: (c: string) => void; __radarDesktop?: boolean };
    w.__radarCmd = desktopCmd;
    const h = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey) || e.altKey) return;
      const k = e.key.toLowerCase();
      if (k === "k") { e.preventDefault(); setPalette((p) => !p); }
      else if (k === "b" && !e.shiftKey && !(e.target as HTMLElement)?.isContentEditable) { e.preventDefault(); desktopCmd("sidebar"); }
      // desktop app only (browsers keep their own ⌘F / ⌘,)
      else if (w.__radarDesktop && k === "f") { e.preventDefault(); desktopCmd("search"); }
      else if (w.__radarDesktop && k === ",") { e.preventDefault(); desktopCmd("settings"); }
    };
    window.addEventListener("keydown", h); return () => { window.removeEventListener("keydown", h); delete w.__radarCmd; };
  }, [desktopCmd]);
  const markManyRead = useCallback((ids: number[]) => rs.markRead(ids), []);
  const markManyUnread = useCallback((ids: number[]) => rs.markUnread(ids), []);
  const trView = !!tr && trOnFor(tr.scopes, f.source, f.source != null ? sources.find((s) => s.id === f.source)?.section ?? null : f.section || null);
  const ctx = useMemo(() => ({ f, setF, navigate, facet, setFacet, isRead, markRead, markManyRead, markManyUnread, readVer, sources, reloadSources, sections, live, setLive, open, setOpen, setPalette, addOpen, setAddOpen, onStream, tr, settingsLocked, reloadTr, trView, loc }),
    [f, setF, navigate, facet, isRead, markRead, markManyRead, markManyUnread, readVer, sources, reloadSources, sections, live, open, setOpen, addOpen, onStream, tr, settingsLocked, reloadTr, trView, loc]);
  const title = path === "/sources" ? "数据源" : path === "/settings" ? "设置" : path.startsWith("/docs") ? ["文档", DOCS.find((d) => path === docHref(d))?.title].filter(Boolean).join(" / ")
    : f.view === "hot" ? "热点" : f.view === "fav" ? "收藏" : f.source != null ? sources.find((s) => s.id === f.source)?.name ?? "来源" : f.section ? secZh(f.section) : "全部动态";

  return (
    <AppCtx.Provider value={ctx}>
      <div className="flex min-h-screen">
        <aside id="app-sidebar" aria-label="侧边栏" inert={collapsed} className="app-sidebar sticky top-0 hidden h-screen w-[244px] shrink-0 overflow-hidden border-r border-sidebar-border bg-sidebar lg:block">
          <div className="h-full w-[244px]"><SidebarBody /></div>
        </aside>
        {navUsed && <MobileNav open={mobileNav} onOpenChange={setMobileNav} />}
        <div className="glow flex min-w-0 flex-1 flex-col">
          <header className="app-header sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background/95 px-3 sm:px-5">
            <Button variant="ghost" size="icon" className="lg:hidden" onClick={() => setMobileNav(true)} aria-label="菜单"><Menu className="size-4" /></Button>
            <Tooltip>
              <TooltipTrigger render={<Button variant="ghost" size="icon" className="-ml-1 hidden lg:inline-flex" onClick={toggleSidebar} aria-label={collapsed ? "显示侧边栏" : "隐藏侧边栏"} aria-controls="app-sidebar" data-state={collapsed ? "collapsed" : "expanded"} data-testid="sidebar-toggle" />}>
                <PanelLeft className="size-4" />
              </TooltipTrigger>
              <TooltipContent className="flex items-center gap-2">{collapsed ? "显示侧边栏" : "隐藏侧边栏"}<Kbd className="font-mono">{MOD}B</Kbd></TooltipContent>
            </Tooltip>
            <div className="hidden min-w-0 truncate type-ui font-semibold tracking-tight sm:block">{title}</div>
            <Button variant="outline" onClick={() => setPalette(true)}
              className="ml-auto w-full min-w-0 max-w-[360px] shrink justify-start gap-2 bg-muted px-2.5 type-body font-normal text-faint hover:text-muted-foreground dark:bg-muted sm:ml-6 sm:mr-auto">
              <Search className="size-3.5" /><span className="truncate">搜索新闻、来源、命令…</span>
              <Kbd className="ml-auto hidden bg-background/80 font-mono shadow-hair sm:inline-flex"><CommandIcon className="size-2.5" />K</Kbd>
            </Button>
            <Tooltip>
              <TooltipTrigger render={<Badge variant="outline" className="hidden h-6 gap-1.5 px-2.5 text-muted-foreground sm:inline-flex" />}>
                <span className="relative flex size-1.5">
                  {/* pings 3× after (re)connecting, then rests: an endless animation kept the desktop app repainting (~17% CPU idle) */}
                  {live && <span className="absolute inline-flex size-full animate-[ping_1s_cubic-bezier(0,0,0.2,1)_3_forwards] rounded-full bg-emerald-400 opacity-75" />}
                  <span className={cn("relative inline-flex size-1.5 rounded-full", live ? "bg-emerald-500" : "bg-amber-500")} />
                </span>{live ? "Live" : "重连中"}
              </TooltipTrigger>
              <TooltipContent>{live ? "实时推送已连接（SSE）" : "实时推送已断开，正在自动重连；内容仍可浏览"}</TooltipContent>
            </Tooltip>
            {trUsed ? <TranslateMenu defaultOpen /> : <TrButton onClick={() => setTrUsed(true)} />}
            <ThemeToggle />
            <Button variant="ghost" size="icon" render={<Link href="/sources" aria-label="数据源" />} className="hidden sm:inline-flex"><Database className="size-4" /></Button>
          </header>
          <main className="@container min-w-0 flex-1">{children}</main>
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
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem disableTransitionOnChange>
      <TooltipProvider delay={300}><Suspense><Shell>{children}</Shell></Suspense></TooltipProvider>
    </ThemeProvider>
  );
}
