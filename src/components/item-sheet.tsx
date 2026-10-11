"use client";
import { ExternalLink } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useApp } from "./app-shell";
import { ago, bj, bjFull, dekText, handleOf, secZh, sectionOf, srcName } from "./common";
import { Original, useFullItem } from "./item-preview";
import { SourceAvatar } from "./source-avatar";

// Item drawer, code-split: mounted after the first item is opened.
export default function ItemSheet() {
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
                  <div className="truncate type-body font-medium">{it.is_x ? `@${handleOf(it)}` : it.source}</div>
                  <div className="truncate type-caption tabular-nums text-muted-foreground">{bjFull(it.published)} · {ago(it.published)}</div>
                </div>
              </div>
              <SheetTitle className="type-h3 font-semibold leading-snug [text-wrap:pretty] pr-6">{it.is_x ? `${srcName(it).replace(/^@/, "")} 在 X 上发帖` : it.title}</SheetTitle>
              <SheetDescription className="sr-only">内容详情</SheetDescription>
              <div className="flex flex-wrap gap-1.5">
                {it.priority && <Badge variant={it.priority === "P0" ? "brand" : "outline"}>{it.priority}</Badge>}
                <Badge variant="secondary">{secZh(sectionOf(it))}</Badge>
                {it.tr && <Badge variant="secondary" title="DeepSeek 机器翻译，原文见下方">译</Badge>}
              </div>
            </SheetHeader>
            <ScrollArea className="min-h-0 flex-1">
              <div className="space-y-5 p-5">
                <p className="whitespace-pre-wrap type-ui leading-7 text-foreground/85">{it.is_x ? it.summary || it.title : dekText(it.summary) || "（该来源未提供摘要）"}</p>
                {it.image && <img src={it.image} alt="" referrerPolicy="no-referrer" className="mt-3 max-h-[420px] w-full rounded-lg bg-muted object-contain" />}
                {it.orig && <Original o={it.orig} x={!!it.is_x} />}
                {it.also_on_x.length > 0 && (
                  <div className="rounded-lg border bg-muted/40 p-3">
                    <div className="mb-2 type-caption font-medium text-muted-foreground">也在 X 上发布</div>
                    {it.also_on_x.map((x) => (
                      <a key={x.url} href={x.url} target="_blank" className="flex items-center gap-2 py-1 type-body hover:text-brand">
                        <SourceAvatar name={x.author ?? "x"} username={x.author} x className="size-5" />@{x.author}<ExternalLink className="ml-auto size-3.5 opacity-50" />
                      </a>))}
                  </div>
                )}
                <div className="type-caption text-muted-foreground">来源：{it.source} · 北京时间 {bj(it.published)}</div>
              </div>
            </ScrollArea>
            <div className="flex gap-2 border-t p-4">
              <Button className="flex-1" render={<a href={it.url} target="_blank" rel="noopener" />}>打开原文<ExternalLink /></Button>
              <Button variant="outline" onClick={() => { navigator.clipboard?.writeText(it.url); toast.success("已复制链接"); }}>复制链接</Button>
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

