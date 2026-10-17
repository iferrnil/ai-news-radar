"use client";
// 生成文章 (desktop): the picked items → 推特 / 公众号 / 小红书 article, each written by its own Skill (设置 › Skill).
// A Skill may ask for a cover; it is made with the 多模态模型 when configured, else skipped with a note.
import { useEffect, useRef, useState } from "react";
import { Check, Copy, Download, Eye, FolderOpen, ImageOff, Loader2, PenLine, Pencil, Sparkles, Undo2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DialogFooter } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { api } from "./common";
import { Markdown } from "./md-view";

type Cover = { kind: "image" | "video"; prompt: string; status: "ok" | "skipped" | "error"; note?: string; path?: string; size?: string };
type Article = {
  markdown: string; title: string; platform: string; label: string; skill: string; count: number; verified?: boolean;
  noFullText?: { n: number; title: string; source: string; url: string; reason: string }[]; covers?: Cover[]; visionNote?: string; saved?: string;
};
export const PLATFORMS = [["twitter", "推特文章"], ["wechat", "公众号文章"], ["xhs", "小红书文章"]] as const;
type P = (typeof PLATFORMS)[number][0];

const fileUrl = (p: string) => `/api/artifacts/file?path=${encodeURIComponent(p)}`;

export default function ArticleDialog({ open, onOpenChange, section, ids }: { open: boolean; onOpenChange: (o: boolean) => void; section: string; ids: number[] }) {
  const [pf, setPf] = useState<P>("twitter");
  const [res, setRes] = useState<Partial<Record<P, Article>>>({});
  const [text, setText] = useState<Partial<Record<P, string>>>({});
  const [busy, setBusy] = useState<P | null>(null);
  const [editing, setEditing] = useState(false);
  const [copied, setCopied] = useState(false);
  const live = useRef(true);
  useEffect(() => () => { live.current = false; }, []);
  const a = res[pf], t = text[pf] ?? "";
  const edited = !!a && t !== a.markdown;

  const gen = async (p: P) => {
    setBusy(p); setEditing(false);
    try {
      const r = await api<Article>("/api/compose", { method: "POST", body: JSON.stringify({ platform: p, ids, section }) });
      if (!live.current || r.platform !== p) return; // closed meanwhile / not this request
      setRes((m) => ({ ...m, [p]: r })); setText((m) => ({ ...m, [p]: r.markdown }));
      const skipped = r.covers?.find((c) => c.status !== "ok");
      if (skipped?.note) toast(skipped.note);
    } catch (e) { if (live.current) toast.error((e as Error).message); }
    finally { if (live.current) setBusy((b) => (b === p ? null : b)); }
  };
  const copy = async () => { try { await navigator.clipboard.writeText(t); setCopied(true); toast.success("已复制 Markdown"); setTimeout(() => setCopied(false), 1500); } catch { toast.error("复制失败"); } };
  const download = () => {
    if (!a) return;
    const el = Object.assign(document.createElement("a"), { href: URL.createObjectURL(new Blob([t], { type: "text/markdown" })), download: `${a.label}-${new Date().toISOString().slice(0, 10)}.md` });
    document.body.append(el); el.click(); el.remove();
  };
  const openDir = async () => { try { await api("/api/artifacts/open", { method: "POST" }); } catch (e) { toast.error((e as Error).message); } };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" data-testid="article-dialog">
        <DialogHeader className="gap-3 border-b border-border/60 px-5 py-4">
          <DialogTitle className="flex items-center gap-2 pr-6"><PenLine className="size-4 text-brand" />生成文章</DialogTitle>
          <DialogDescription className="sr-only">已选 {ids.length} 条</DialogDescription>
          <div className="flex flex-wrap items-center gap-2">
            <div role="radiogroup" aria-label="文章类型" className="inline-flex rounded-lg bg-muted p-0.5" data-testid="article-platforms">
              {PLATFORMS.map(([k, l]) => (
                <button key={k} type="button" role="radio" aria-checked={pf === k} onClick={() => { setPf(k); setEditing(false); }} data-testid={`pf-${k}`}
                  className={cn("rounded-md px-3 py-1 type-caption transition-colors", pf === k ? "bg-background font-medium text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground")}>
                  {l}{res[k] && <span className="ml-1 inline-block size-1.5 rounded-full bg-brand align-middle" />}
                </button>
              ))}
            </div>
            <span className="type-caption text-faint">{ids.length} 条</span>
            <Button size="sm" className="ml-auto" disabled={!!busy} onClick={() => gen(pf)} data-testid="article-go">
              {busy === pf ? <Loader2 className="size-3.5 animate-spin" /> : <Sparkles className="size-3.5" />}{a ? "重新生成" : "生成"}
            </Button>
          </div>
        </DialogHeader>
        <div className="relative min-h-[30dvh] flex-1 overflow-y-auto px-5 py-4">
          {!a && busy !== pf && <p className="type-body text-muted-foreground">按「{PLATFORMS.find((x) => x[0] === pf)![1].replace("文章", "")} Skill」抓取原文并写作，只使用原文中的事实。</p>}
          {busy === pf && !a && <div className="flex items-center gap-2 type-body text-muted-foreground"><Loader2 className="size-4 animate-spin" />抓取原文并写作中…</div>}
          {a && (
            <>
              <p className="mb-3 type-meta text-faint" data-testid="article-meta">{a.skill}{a.verified ? " · 已核对事实" : ""}{edited ? " · 已编辑" : ""}{a.saved && <span title={a.saved}> · 已保存</span>}</p>
              {(a.noFullText?.length ?? 0) > 0 && (
                <details className="mb-3 rounded-md bg-muted px-3 py-2 type-caption text-muted-foreground" data-testid="article-nofull">
                  <summary className="cursor-pointer select-none">{a.noFullText!.length} 条未取到原文全文，只按标题 / 摘要写</summary>
                  <ul className="mt-1.5 space-y-0.5">{a.noFullText!.map((m) => <li key={m.n} className="truncate"><span className="font-mono text-faint">[{m.n}]</span> <a href={m.url} target="_blank" rel="noreferrer" className="hover:underline">{m.title}</a> <span className="text-faint">· {m.reason}</span></li>)}</ul>
                </details>
              )}
              {a.visionNote && <p className="mb-3 type-caption text-faint">{a.visionNote}</p>}
              {a.covers?.map((c, i) => c.status === "ok" && c.path ? (
                c.kind === "image"
                  ? <img key={i} src={fileUrl(c.path)} alt="封面" className="mb-4 max-h-64 rounded-lg ring-1 ring-border" data-testid="article-cover" />
                  : <video key={i} src={fileUrl(c.path)} controls className="mb-4 max-h-64 rounded-lg ring-1 ring-border" data-testid="article-cover" />
              ) : (
                <p key={i} className="mb-3 flex items-center gap-1.5 type-caption text-faint" data-testid="article-cover-note" title={c.prompt}><ImageOff className="size-3.5" />{c.note}</p>
              ))}
              {editing
                ? <textarea value={t} onChange={(e) => setText((m) => ({ ...m, [pf]: e.target.value }))} spellCheck={false} aria-label="编辑文章" data-testid="article-editor"
                    className="block min-h-[50dvh] w-full resize-none rounded-md border border-border bg-background p-3 font-mono text-[13px] leading-relaxed outline-none focus-visible:ring-2 focus-visible:ring-ring/40" />
                : <div data-testid="article-body" className={busy === pf ? "opacity-60" : undefined}><Markdown md={t} /></div>}
            </>
          )}
        </div>
        <Separator />
        <DialogFooter className="m-0 flex-row flex-wrap items-center gap-2 rounded-b-xl border-t-0 px-5 pt-3 pb-4 sm:justify-between">
          <div className="flex flex-wrap gap-2">
            <Button variant={editing ? "secondary" : "outline"} size="sm" disabled={!a} onClick={() => setEditing(!editing)} aria-pressed={editing}>{editing ? <Eye className="size-3.5" /> : <Pencil className="size-3.5" />}{editing ? "预览" : "编辑"}</Button>
            {edited && <Button variant="ghost" size="sm" onClick={() => a && setText((m) => ({ ...m, [pf]: a.markdown }))}><Undo2 className="size-3.5" />还原</Button>}
            <Button variant="outline" size="sm" disabled={!a} onClick={copy}>{copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}{copied ? "已复制" : "复制"}</Button>
            <Button variant="outline" size="sm" disabled={!a} onClick={download}><Download className="size-3.5" />导出</Button>
          </div>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={openDir} data-testid="article-open-dir"><FolderOpen className="size-3.5" />制品目录</Button>
            <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)}>关闭</Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
