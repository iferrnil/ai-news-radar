"use client";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { ThemeProvider, useTheme } from "next-themes";
import { Suspense, createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import {
  Activity, BookOpen, ChevronRight, Clapperboard, FileText, Command as CommandIcon, Database, ExternalLink, FlaskConical, Globe2, Inbox, Menu, Moon, Newspaper,
  Plus, Radio, Search, Sun,
} from "lucide-react";
import { toast } from "sonner";
import { XLogo as Twitter } from "./x-logo";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { CommandDialog, Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator, CommandShortcut } from "@/components/ui/command";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Toaster } from "@/components/ui/sonner";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { DOCS, docHref } from "@/lib/docs-registry";
import { type Filters, countOf, parseFilters, toQuery } from "./filters";
import { cn } from "@/lib/utils";
import type { ItemOut } from "@/lib/types";
import { SECTIONS, inAll, type SectionKey, type SourceRow, ago, api, bj, bjFull, dekText, handleOf, secZh, sectionOf, srcName, xText, REG, CAT } from "./common";
import { SourceAvatar } from "./source-avatar";
import { useFullItem } from "./item-preview";

export const SEC_ICON: Record<SectionKey | "all", React.ElementType> = { all: Inbox, lab: FlaskConical, cn: Globe2, media: Newspaper, x: Twitter, ent: Clapperboard };

type Ctx = {
  f: Filters; setF: (p: Partial<Filters>) => void;
  /** Switch the feed view (section / source), keeping the P0 toggle. */
  navigate: (p: Partial<Filters>) => void;
  facet: ItemOut[]; setFacet: (i: ItemOut[]) => void; isRead: (id: number) => boolean; markRead: (id: number) => void; sources: SourceRow[]; reloadSources: () => Promise<void>;
  live: boolean; setLive: (b: boolean) => void; open: ItemOut | null; setOpen: (i: ItemOut | null) => void;
  setPalette: (b: boolean) => void; addOpen: boolean; setAddOpen: (b: boolean) => void;
  onStream: (fn: (ev: string, data: any) => void) => () => void;
};
const AppCtx = createContext<Ctx>(null as unknown as Ctx);
export const useApp = () => useContext(AppCtx);

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

function NavItem({ active, icon: Icon, label, count, onClick }: { active: boolean; icon: React.ElementType; label: string; count?: number; onClick: () => void }) {
  return (
    <button onClick={onClick} className={cn("group relative flex h-8 w-full items-center gap-2.5 rounded-lg px-2.5 text-[13px] font-medium tracking-[-0.005em] transition-colors duration-150 ease-out outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
      active ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]" : "text-muted-foreground hover:bg-hover hover:text-foreground", !active && count === 0 && "opacity-60")}>
      {active && <span className="absolute -left-2.5 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r-full bg-brand" />}
      <Icon strokeWidth={1.75} className={cn("size-4 transition-colors duration-150 ease-out", active ? "text-brand" : "text-faint group-hover:text-foreground")} />
      <span className="flex-1 text-left">{label}</span>
      {count != null && <span className={cn("font-mono text-[11px] tabular-nums", count === 0 ? "text-faint/60" : "text-faint")}>{count}</span>}
    </button>
  );
}

function SectionLabel({ children, first }: { children: React.ReactNode; first?: boolean }) {
  return <div className={cn("flex h-6 items-center px-2 text-[11px] font-medium tracking-[0.04em] text-faint", first ? "mt-1" : "mt-4")}>{children}</div>;
}

const DOCS_KEY = "sidebar.docs.open";
function DocsGroup({ path, onGo }: { path: string; onGo: (href: string) => void }) {
  const inDocs = path.startsWith("/docs");
  const [open, setOpen] = useState(true);
  useEffect(() => { const v = localStorage.getItem(DOCS_KEY); if (v != null) setOpen(v === "1"); }, []);
  const toggle = (b: boolean) => { setOpen(b); localStorage.setItem(DOCS_KEY, b ? "1" : "0"); };
  return (
    <Collapsible open={open} onOpenChange={toggle}>
      <div className={cn("group relative flex h-8 w-full items-center rounded-lg text-[13px] font-medium tracking-[-0.005em] transition-colors duration-150 ease-out",
        path === "/docs" ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]" : inDocs ? "text-foreground" : "text-muted-foreground hover:bg-hover hover:text-foreground")}>
        {path === "/docs" && <span className="absolute -left-2.5 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-r-full bg-brand" />}
        <button onClick={() => { onGo("/docs"); if (!open) toggle(true); }} className="flex h-full flex-1 items-center gap-2.5 rounded-lg pl-2.5 outline-none focus-visible:ring-2 focus-visible:ring-ring/50">
          <BookOpen strokeWidth={1.75} className={cn("size-4 transition-colors duration-150 ease-out", inDocs ? "text-brand" : "text-faint group-hover:text-foreground")} />
          <span className="flex-1 text-left">文档</span>
        </button>
        <CollapsibleTrigger aria-label={open ? "收起文档" : "展开文档"} className="mr-1 grid size-6 place-items-center rounded-md text-faint outline-none transition-colors duration-150 ease-out hover:bg-hover hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50">
          <ChevronRight strokeWidth={2} className={cn("size-3.5 transition-transform duration-200 ease-out", open && "rotate-90")} />
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent>
        <div className="relative ml-[17px] mt-px space-y-px border-l border-border py-0.5 pl-2">
          {DOCS.map((d) => {
            const on = path === docHref(d);
            return (
              <button key={d.slug} onClick={() => onGo(docHref(d))}
                className={cn("relative flex h-7 w-full items-center gap-2 rounded-md px-2 text-[12.5px] transition-colors duration-150 ease-out outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
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

function SidebarBody({ onNavigate }: { onNavigate?: () => void }) {
  const { f, navigate, sources, facet } = useApp();
  const router = useRouter(); const path = usePathname();
  const active = sources.filter((s) => s.enabled && s.supported);
  const feed = path === "/";
  const count = (section: Filters["section"]) => facet.length ? countOf(facet, { section, source: null, p0: f.p0 })
    : active.filter((s) => { const k = sectionOf({ ...s, is_x: s.type.startsWith("x") }); return section ? k === section : inAll(k); }).reduce((a, s) => a + s.item_count, 0);
  const go = (p: Partial<Filters>) => { navigate({ section: "", source: null, ...p }); onNavigate?.(); };
  const on = (section: Filters["section"]) => feed && f.source == null && f.section === section;
  const errs = active.filter((s) => s.last_error).length;
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-14 items-center gap-2.5 px-4">
        <div className="grid size-7 place-items-center rounded-lg bg-gradient-to-b from-violet-500 to-indigo-600 text-white shadow-[0_0_0_1px_rgb(255_255_255/0.15)_inset,0_4px_12px_-2px_oklch(0.56_0.22_285/0.5)]">
          <Radio className="size-4" />
        </div>
        <div className="leading-tight">
          <div className="text-[13.5px] font-semibold tracking-tight">AI Radar</div>
          <div className="text-[11px] text-muted-foreground">国内外 AI 情报</div>
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-0.5 px-2.5 pb-3">
          <SectionLabel first>信息流</SectionLabel>
          <NavItem active={on("")} icon={Inbox} label="全部" count={count("")} onClick={() => go({})} />
          {SECTIONS.filter((s) => inAll(s.key)).map((s) => <NavItem key={s.key} active={on(s.key)} icon={SEC_ICON[s.key]} label={s.zh} count={count(s.key)} onClick={() => go({ section: s.key })} />)}
          {/* sections outside 全部 (e.g. 娱乐) sit below a divider */}
          <div className="mx-2 my-2 h-px bg-border" role="separator" title="不计入「全部」" />
          {SECTIONS.filter((s) => !inAll(s.key)).map((s) => <NavItem key={s.key} active={on(s.key)} icon={SEC_ICON[s.key]} label={s.zh} count={count(s.key)} onClick={() => go({ section: s.key })} />)}
          <SectionLabel>管理</SectionLabel>
          <NavItem active={path === "/sources"} icon={Database} label="数据源" count={active.length} onClick={() => { router.push("/sources"); onNavigate?.(); }} />
          <DocsGroup path={path} onGo={(h) => { router.push(h); onNavigate?.(); }} />
          <Collapsible defaultOpen className="pt-4">
            <CollapsibleTrigger className="group flex h-6 w-full items-center gap-1 px-2 text-[11px] font-medium tracking-[0.04em] text-faint transition-colors duration-150 ease-out hover:text-foreground">
              <ChevronRight strokeWidth={2} className="size-3 transition-transform duration-200 ease-out group-data-[panel-open]:rotate-90" />订阅源<span className="ml-auto font-mono tabular-nums">{active.length}</span>
            </CollapsibleTrigger>
            <CollapsibleContent className="space-y-px">
              {SECTIONS.map((sec) => {
                const list = active.filter((s) => sectionOf({ ...s, is_x: s.type.startsWith("x") }) === sec.key);
                if (!list.length) return null;
                return (
                  <div key={sec.key} className="pt-2">
                    <div className="flex h-6 items-center px-2 text-[11px] text-faint">{sec.zh}</div>
                    <div className="relative ml-[11px] space-y-px border-l border-border pl-1.5">
                    {list.map((s) => (
                      <button key={s.id} onClick={() => go({ source: s.id })} title={s.last_error ?? s.name}
                        className={cn("flex h-7 w-full items-center gap-2 rounded-md px-2 text-[12.5px] transition-colors duration-150 ease-out",
                          f.source === s.id && feed ? "bg-sidebar-accent text-foreground" : "text-muted-foreground hover:bg-hover hover:text-foreground")}>
                        <StatusDot s={s} />
                        <span className="flex-1 truncate text-left">{s.name.replace(/^X @(\S+).*$/, "@$1")}</span>
                        <span className="font-mono text-[11px] tabular-nums text-faint">{s.item_count || ""}</span>
                      </button>
                    ))}
                    </div>
                  </div>
                );
              })}
            </CollapsibleContent>
          </Collapsible>
        </div>
      </ScrollArea>
      <div className="border-t border-sidebar-border p-3 text-[11.5px] text-faint">
        <div className="flex h-5 items-center gap-2 tabular-nums"><Activity className="size-3.5" strokeWidth={1.75} />{active.length} 个源在线{errs ? <span className="text-red-500">· {errs} 报错</span> : null}</div>
      </div>
    </div>
  );
}

function Palette({ open, onOpenChange }: { open: boolean; onOpenChange: (b: boolean) => void }) {
  const { navigate, sources, setOpen, setAddOpen } = useApp();
  const { resolvedTheme, setTheme } = useTheme();
  const router = useRouter();
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<ItemOut[]>([]);
  useEffect(() => {
    if (!open) return;
    const t = setTimeout(async () => {
      // "最新" (no query) mirrors 全部 and skips 娱乐; a real search may still hit 娱乐 items (labelled)
      const r = await api<ItemOut[]>(`/api/items?limit=${q.trim() ? 12 : 40}${q.trim() ? `&q=${encodeURIComponent(q.trim())}` : ""}`);
      setHits(q.trim() ? r : r.filter((it) => inAll(sectionOf(it))).slice(0, 12));
    }, 150);
    return () => clearTimeout(t);
  }, [q, open]);
  const run = (fn: () => void) => { onOpenChange(false); setQ(""); fn(); };
  const goFeed = (p: Partial<Filters>) => run(() => navigate({ section: "", source: null, ...p }));
  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} title="命令面板" description="搜索内容、来源与操作" className="sm:max-w-xl">
      <Command shouldFilter={false} className="rounded-xl">
        <CommandInput placeholder="搜索新闻、来源、操作…" value={q} onValueChange={setQ} />
        <CommandList className="max-h-[420px]">
          <CommandEmpty>没有结果</CommandEmpty>
          {hits.length > 0 && (
            <CommandGroup heading={q ? "内容" : "最新"}>
              {hits.map((it) => (
                <CommandItem key={it.id} value={`item-${it.id}`} onSelect={() => run(() => setOpen(it))}>
                  <SourceAvatar name={it.source} username={it.is_x ? handleOf(it) : null} homepage={it.homepage} url={it.url} x={!!it.is_x} className="size-5" />
                  <span className="truncate">{it.is_x ? xText(it) : it.title}</span>
                  {!inAll(sectionOf(it)) && <span className="shrink-0 rounded-[4px] px-1.5 text-[11px] leading-[18px] text-muted-foreground shadow-[inset_0_0_0_1px_var(--border)]">{secZh(sectionOf(it))}</span>}
                  <CommandShortcut className="whitespace-nowrap tracking-normal">{ago(it.published, true)}</CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          <CommandSeparator />
          <CommandGroup heading="跳转">
            {SECTIONS.filter((s) => !q || s.zh.includes(q)).map((s) => { const I = SEC_ICON[s.key]; return (
              <CommandItem key={s.key} value={`sec-${s.key}`} onSelect={() => goFeed({ section: s.key })}><I />{s.zh}</CommandItem>); })}
            {sources.filter((s) => s.enabled && q && s.name.toLowerCase().includes(q.toLowerCase())).slice(0, 6).map((s) => (
              <CommandItem key={s.id} value={`src-${s.id}`} onSelect={() => goFeed({ source: s.id })}>
                <SourceAvatar name={s.name} username={s.username} homepage={s.homepage} url={s.url} x={s.type.startsWith("x")} className="size-5" />{s.name}
              </CommandItem>))}
          </CommandGroup>
          <CommandSeparator />
          <CommandGroup heading="操作">
            <CommandItem value="add" onSelect={() => run(() => { router.push("/sources"); setAddOpen(true); })}><Plus />添加数据源</CommandItem>
            <CommandItem value="sources" onSelect={() => run(() => router.push("/sources"))}><Database />管理数据源</CommandItem>
            <CommandItem value="docs 文档" onSelect={() => run(() => router.push("/docs"))}><BookOpen />文档</CommandItem>
            {DOCS.map((d) => <CommandItem key={d.slug} value={`docs ${d.slug} 文档 ${d.title}`} onSelect={() => run(() => router.push(docHref(d)))}><FileText />文档 / {d.title}</CommandItem>)}
            <CommandItem value="theme" onSelect={() => run(() => setTheme(resolvedTheme === "dark" ? "light" : "dark"))}><Moon />切换深色/浅色</CommandItem>
          </CommandGroup>
        </CommandList>
      </Command>
    </CommandDialog>
  );
}

function ItemSheet() {
  const { open, setOpen } = useApp();
  const { item: it } = useFullItem(open); // shared cache with the hover preview
  return (
    <Sheet open={!!it} onOpenChange={(b) => !b && setOpen(null)}>
      <SheetContent initialFocus={false} className="w-full gap-0 border-border bg-popover p-0 sm:max-w-[520px]">
        {it && (
          <>
            <SheetHeader className="gap-3 border-b p-5 pr-12">
              <div className="flex items-center gap-2.5">
                <SourceAvatar name={it.source} username={it.is_x ? handleOf(it) : null} homepage={it.homepage} url={it.url} x={!!it.is_x} />
                <div className="min-w-0 leading-tight">
                  <div className="truncate text-[13px] font-medium">{it.is_x ? `@${handleOf(it)}` : it.source}</div>
                  <div className="text-[12px] text-muted-foreground">{bjFull(it.published)} · {ago(it.published)}</div>
                </div>
              </div>
              <SheetTitle className="text-[19px] font-semibold leading-snug tracking-tight">{it.is_x ? `${srcName(it).replace(/^@/, "")} 在 X 上发帖` : it.title}</SheetTitle>
              <SheetDescription className="sr-only">内容详情</SheetDescription>
              <div className="flex flex-wrap gap-1.5">
                {it.priority && <Badge variant="outline" className={cn(it.priority === "P0" && "border-brand/40 bg-brand-soft text-brand")}>{it.priority}</Badge>}
                <Badge variant="secondary">{secZh(sectionOf(it))}</Badge>
                {it.category && <Badge variant="secondary">{CAT[it.category] ?? it.category}</Badge>}
                {it.region && <Badge variant="secondary">{REG[it.region]}</Badge>}
              </div>
            </SheetHeader>
            <ScrollArea className="min-h-0 flex-1">
              <div className="space-y-5 p-5">
                <p className="whitespace-pre-wrap text-[14.5px] leading-7 text-foreground/85">{it.is_x ? it.summary || it.title : dekText(it.summary) || "（该来源未提供摘要）"}</p>
                {it.also_on_x.length > 0 && (
                  <div className="rounded-lg border bg-muted/40 p-3">
                    <div className="mb-2 text-[12px] font-medium text-muted-foreground">也在 X 上发布</div>
                    {it.also_on_x.map((x) => (
                      <a key={x.url} href={x.url} target="_blank" className="flex items-center gap-2 py-1 text-[13px] hover:text-brand">
                        <SourceAvatar name={x.author ?? "x"} username={x.author} x className="size-5" />@{x.author}<ExternalLink className="ml-auto size-3.5 opacity-50" />
                      </a>))}
                  </div>
                )}
                <div className="text-[12px] text-muted-foreground">来源：{it.source} · 北京时间 {bj(it.published)}</div>
              </div>
            </ScrollArea>
            <div className="flex gap-2 border-t p-4">
              <Button className="flex-1" render={<a href={it.url} target="_blank" rel="noopener" />}>打开原文<ExternalLink /></Button>
              <Button variant="outline" onClick={() => { navigator.clipboard?.writeText(it.url); toast.success("链接已复制"); }}>复制链接</Button>
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
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
  const [open, setOpenState] = useState<ItemOut | null>(null);
  const setOpen = useCallback((i: ItemOut | null) => { setOpenState(i); if (i) markRead(i.id); }, [markRead]);
  const [palette, setPalette] = useState(false);
  const [mobileNav, setMobileNav] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const reloadSourcesRef = useRef<(() => void) | null>(null);
  const listeners = useRef(new Set<(ev: string, data: any) => void>());
  const onStream = useCallback((fn: (ev: string, data: any) => void) => { listeners.current.add(fn); return () => { listeners.current.delete(fn); }; }, []);
  useEffect(() => {
    const es = new EventSource("/api/stream");
    es.onopen = () => setLive(true);
    es.onerror = () => setLive(false);
    for (const ev of ["item", "update", "removed"]) es.addEventListener(ev, (e) => {
      const d = JSON.parse((e as MessageEvent).data); listeners.current.forEach((fn) => fn(ev, d));
      if (ev === "item") reloadSourcesRef.current?.();
    });
    return () => es.close();
  }, []);
  const fRef = useRef(f);
  useEffect(() => { fRef.current = f; }, [f]);
  const setF = useCallback((p: Partial<Filters>) => router.push(toQuery({ ...fRef.current, ...p }), { scroll: false }), [router]);
  const navigate = useCallback((p: Partial<Filters>) => router.push(toQuery({ ...fRef.current, ...p }), { scroll: false }), [router]);
  const reloadSources = useCallback(async () => setSources(await api<SourceRow[]>("/api/sources")), []);
  useEffect(() => { let t: ReturnType<typeof setTimeout> | null = null; reloadSourcesRef.current = () => { if (!t) t = setTimeout(() => { t = null; reloadSources(); }, 3000); }; }, [reloadSources]);
  useEffect(() => { reloadSources(); const t = setInterval(reloadSources, 20000); return () => clearInterval(t); }, [reloadSources]);
  useEffect(() => {
    const h = (e: KeyboardEvent) => { if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); setPalette((p) => !p); } };
    window.addEventListener("keydown", h); return () => window.removeEventListener("keydown", h);
  }, []);
  const ctx = useMemo(() => ({ f, setF, navigate, facet, setFacet, isRead, markRead, sources, reloadSources, live, setLive, open, setOpen, setPalette, addOpen, setAddOpen, onStream }),
    [f, setF, navigate, facet, isRead, markRead, sources, reloadSources, live, open, setOpen, addOpen, onStream]);
  const title = path === "/sources" ? "数据源" : path.startsWith("/docs") ? ["文档", DOCS.find((d) => path === docHref(d))?.title].filter(Boolean).join(" / ")
    : f.source != null ? sources.find((s) => s.id === f.source)?.name ?? "来源" : f.section ? secZh(f.section) : "全部动态";

  return (
    <AppCtx.Provider value={ctx}>
      <div className="flex min-h-screen">
        <aside className="sticky top-0 hidden h-screen w-[244px] shrink-0 border-r border-sidebar-border bg-sidebar lg:block"><SidebarBody /></aside>
        <Sheet open={mobileNav} onOpenChange={setMobileNav}>
          <SheetContent side="left" className="w-[270px] bg-sidebar p-0" showCloseButton={false}>
            <SheetTitle className="sr-only">导航</SheetTitle><SidebarBody onNavigate={() => setMobileNav(false)} />
          </SheetContent>
        </Sheet>
        <div className="glow flex min-w-0 flex-1 flex-col">
          <header className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border/80 bg-background/70 px-3 backdrop-blur-xl backdrop-saturate-150 sm:px-5">
            <Button variant="ghost" size="icon" className="lg:hidden" onClick={() => setMobileNav(true)} aria-label="菜单"><Menu className="size-4" /></Button>
            <div className="hidden min-w-0 truncate text-[14px] font-semibold tracking-tight sm:block">{title}</div>
            <button onClick={() => setPalette(true)}
              className="ml-auto flex h-8 w-full max-w-[360px] items-center gap-2 rounded-lg bg-muted px-2.5 text-[13px] text-faint shadow-[inset_0_0_0_1px_var(--border)] transition hover:text-muted-foreground hover:shadow-[inset_0_0_0_1px_var(--input)] sm:ml-6 sm:mr-auto">
              <Search className="size-3.5" /><span className="truncate">搜索新闻、来源、命令…</span>
              <kbd className="ml-auto hidden items-center gap-0.5 rounded-[4px] bg-background/80 px-1.5 font-mono text-[10.5px] text-muted-foreground shadow-[inset_0_0_0_1px_var(--border)] sm:flex"><CommandIcon className="size-2.5" />K</kbd>
            </button>
            <Tooltip>
              <TooltipTrigger render={<span className="hidden items-center gap-1.5 rounded-full px-2.5 py-1 text-[11.5px] font-medium text-muted-foreground shadow-[inset_0_0_0_1px_var(--border)] sm:flex" />}>
                <span className="relative flex size-1.5">
                  {live && <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-75" />}
                  <span className={cn("relative inline-flex size-1.5 rounded-full", live ? "bg-emerald-500" : "bg-muted-foreground")} />
                </span>{live ? "Live" : "离线"}
              </TooltipTrigger>
              <TooltipContent>{live ? "实时推送已连接（SSE）" : "正在重连实时推送"}</TooltipContent>
            </Tooltip>
            <ThemeToggle />
            <Button variant="ghost" size="icon" render={<Link href="/sources" aria-label="数据源" />} className="hidden sm:inline-flex"><Database className="size-4" /></Button>
          </header>
          <main className="min-w-0 flex-1">{children}</main>
        </div>
      </div>
      <Palette open={palette} onOpenChange={setPalette} />
      <ItemSheet />
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
