"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Languages, SlidersHorizontal } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { useApp } from "./app-shell";
import { api } from "./common";

// Code-split: app-shell renders a placeholder button and loads this on first click.
type TrStatus = { available: boolean; enabled: boolean; queued: number; done: number; cached: number; failed: number; lastError: string | null; model: string };
/** Top-bar popover: client display toggle (localStorage) + server fetch-time translation toggle (DB setting). */
export default function TranslateMenu({ defaultOpen = false }: { defaultOpen?: boolean }) {
  const { zh, setZh } = useApp();
  const [st, setSt] = useState<TrStatus | null>(null);
  const [open, setOpen] = useState(defaultOpen);
  useEffect(() => {
    if (!open) return;
    const get = () => api<{ translate: TrStatus }>("/api/settings").then((r) => setSt(r.translate)).catch(() => {});
    get(); const t = setInterval(get, 3000); return () => clearInterval(t);
  }, [open]);
  const setServer = async (on: boolean) => { const r = await api<{ translate: TrStatus }>("/api/settings", { method: "PATCH", body: JSON.stringify({ translate: on }) }); setSt(r.translate); toast.success(on ? "已开启抓取时翻译" : "已关闭抓取时翻译"); };
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger render={<Button variant="ghost" size="icon" aria-label="翻译设置" className={zh ? "text-brand" : ""} />}><Languages className="size-4" /></PopoverTrigger>
      <PopoverContent align="end" className="w-[300px] gap-0 p-0">
        <label className="flex cursor-pointer items-start gap-3 p-4">
          <div className="flex-1"><div className="type-body font-medium">翻译成中文</div><div className="mt-0.5 type-caption leading-snug text-muted-foreground">显示外文内容的中文译文（标「译」），悬停卡片和详情里可看原文。</div></div>
          <Switch checked={zh} onCheckedChange={(v) => setZh(!!v)} aria-label="翻译成中文" />
        </label>
        <Separator className="bg-border/50" />
        <label className="flex cursor-pointer items-start gap-3 p-4">
          <div className="flex-1">
            <div className="type-body font-medium">抓取时翻译（DeepSeek）</div>
            <div className="mt-0.5 type-caption leading-snug text-muted-foreground">
              {!st ? "读取中…" : !st.available ? "未配置 DeepSeek API key，翻译不可用。" : `新外文条目自动翻译标题与摘要 · 已译 ${st.done}（缓存 ${st.cached}）· 排队 ${st.queued}${st.failed ? ` · 失败 ${st.failed}` : ""}`}
            </div>
            {st?.lastError && <div className="mt-1 type-meta text-red-500">{st.lastError}</div>}
          </div>
          <Switch checked={!!st?.enabled} disabled={!st?.available} onCheckedChange={(v) => setServer(!!v)} aria-label="抓取时翻译" />
        </label>
        <Separator className="bg-border/50" />
        <Link href="/settings" onClick={() => setOpen(false)} className="flex items-center gap-2 px-4 py-2.5 type-caption text-muted-foreground hover:bg-hover hover:text-foreground"><SlidersHorizontal className="size-3.5" />DeepSeek 设置…</Link>
      </PopoverContent>
    </Popover>
  );
}

