"use client";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { BookOpen, Database, FileText, Moon, Plus, SearchX } from "lucide-react";
import { EmptyState } from "./empty-state";
import { CommandDialog, Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator, CommandShortcut } from "@/components/ui/command";
import { DOCS, docHref } from "@/lib/docs-registry";
import type { ItemOut } from "@/lib/types";
import type { Filters } from "./filters";
import { inAll, ago, api, handleOf, secZh, sectionOf, xText, localize } from "./common";
import { SourceAvatar } from "./source-avatar";
import { secIcon, useApp } from "./app-shell";

export default function Palette({ open, onOpenChange }: { open: boolean; onOpenChange: (b: boolean) => void }) {
  const { navigate, sources, sections, setOpen, setAddOpen, zh } = useApp();
  const { resolvedTheme, setTheme } = useTheme();
  const router = useRouter();
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<ItemOut[]>([]);
  useEffect(() => {
    if (!open) return;
    const t = setTimeout(async () => {
      // "最新" (no query) mirrors 全部 and skips 娱乐; a real search may still hit 娱乐 items (labelled)
      const r = (await api<ItemOut[]>(`/api/items?limit=${q.trim() ? 12 : 40}${zh ? "&zh=1" : ""}${q.trim() ? `&q=${encodeURIComponent(q.trim())}` : ""}`)).map((it) => (zh ? localize(it) : it));
      setHits(q.trim() ? r : r.filter((it) => inAll(sectionOf(it))).slice(0, 12));
    }, 150);
    return () => clearTimeout(t);
  }, [q, open, zh]);
  const run = (fn: () => void) => { onOpenChange(false); setQ(""); fn(); };
  const goFeed = (p: Partial<Filters>) => run(() => navigate({ section: "", source: null, ...p }));
  const ql = q.trim().toLowerCase();
  const navSec = sections.filter((s) => !ql || s.zh.includes(q.trim()));
  const navSrc = ql ? sources.filter((s) => s.enabled && s.name.toLowerCase().includes(ql)).slice(0, 6) : [];
  const acts = [
    { k: "add", label: "添加数据源", icon: Plus, go: () => { router.push("/sources"); setAddOpen(true); } },
    { k: "sources", label: "管理数据源", icon: Database, go: () => router.push("/sources") },
    { k: "docs", label: "文档", icon: BookOpen, go: () => router.push("/docs") },
    ...DOCS.map((d) => ({ k: `docs-${d.slug}`, label: `文档 / ${d.title}`, icon: FileText, go: () => router.push(docHref(d)) })),
    { k: "theme", label: "切换深色/浅色", icon: Moon, go: () => setTheme(resolvedTheme === "dark" ? "light" : "dark") },
  ].filter((a) => !ql || a.label.toLowerCase().includes(ql) || a.k.includes(ql));
  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} title="命令面板" description="搜索内容、来源与操作" className="sm:max-w-xl">
      <Command shouldFilter={false} className="rounded-xl">
        <CommandInput placeholder="搜索新闻、来源、操作…" value={q} onValueChange={setQ} />
        <CommandList className="max-h-[420px]">
          <CommandEmpty className="py-0"><EmptyState icon={SearchX} className="py-8" title="没有结果" hint="换个关键词试试；搜索标题、摘要与来源名称。" /></CommandEmpty>
          {hits.length > 0 && (
            <CommandGroup heading={q ? "内容" : "最新"}>
              {hits.map((it) => (
                <CommandItem key={it.id} value={`item-${it.id}`} onSelect={() => run(() => setOpen(it))}>
                  <SourceAvatar name={it.source} username={it.is_x ? handleOf(it) : null} homepage={it.homepage} url={it.url} x={!!it.is_x} className="size-5" />
                  <span className="truncate">{it.is_x ? xText(it) : it.title}</span>
                  {!inAll(sectionOf(it)) && <Badge size="sm" variant="outline" className="text-muted-foreground">{secZh(sectionOf(it))}</Badge>}
                  <CommandShortcut className="whitespace-nowrap tracking-normal">{ago(it.published, true)}</CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          {navSec.length + navSrc.length > 0 && <><CommandSeparator /><CommandGroup heading="跳转">
            {navSec.map((s) => { const I = secIcon(s); return (
              <CommandItem key={s.key} value={`sec-${s.key}`} onSelect={() => goFeed({ section: s.key })}><I />{s.zh}</CommandItem>); })}
            {navSrc.map((s) => (
              <CommandItem key={s.id} value={`src-${s.id}`} onSelect={() => goFeed({ source: s.id })}>
                <SourceAvatar name={s.name} username={s.username} homepage={s.homepage} url={s.url} x={s.type.startsWith("x")} className="size-5" />{s.name}
              </CommandItem>))}
          </CommandGroup></>}
          {acts.length > 0 && <><CommandSeparator /><CommandGroup heading="操作">
            {acts.map((a) => { const I = a.icon; return <CommandItem key={a.k} value={a.k} onSelect={() => run(a.go)}><I />{a.label}</CommandItem>; })}
          </CommandGroup></>}
        </CommandList>
      </Command>
    </CommandDialog>
  );
}

