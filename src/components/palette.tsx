"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { BookOpen, Database, FileText, Moon, Plus } from "lucide-react";
import { CommandDialog, Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator, CommandShortcut } from "@/components/ui/command";
import { DOCS, docHref } from "@/lib/docs-registry";
import type { ItemOut } from "@/lib/types";
import type { Filters } from "./filters";
import { SECTIONS, inAll, ago, api, handleOf, secZh, sectionOf, xText, localize } from "./common";
import { SourceAvatar } from "./source-avatar";
import { SEC_ICON, useApp } from "./app-shell";

export default function Palette({ open, onOpenChange }: { open: boolean; onOpenChange: (b: boolean) => void }) {
  const { navigate, sources, setOpen, setAddOpen, zh } = useApp();
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
                  {!inAll(sectionOf(it)) && <span className="shrink-0 rounded-[4px] px-1.5 type-meta leading-[18px] text-muted-foreground shadow-[inset_0_0_0_1px_var(--border)]">{secZh(sectionOf(it))}</span>}
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

