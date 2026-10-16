"use client";
// 设置 › 简报 Skill (desktop only): the instructions 「AI 润色」 sends as its system prompt.
import { useEffect, useRef, useState } from "react";
import { Check, Eye, FileUp, Link2, Loader2, Sparkles, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { api } from "./common";

type Skill = { name: string; slug: string; description: string; source?: string; installedAt?: number; size: number; builtin?: boolean; active: boolean };
type List = { skills: Skill[]; installed?: string };

export default function SkillCard() {
  const [list, setList] = useState<Skill[] | null>(null);
  const [url, setUrl] = useState("");
  const [busy, setBusy] = useState<string>("");
  const [preview, setPreview] = useState<{ name: string; content: string } | null>(null);
  const file = useRef<HTMLInputElement>(null);
  useEffect(() => { api<List>("/api/skills").then((r) => setList(r.skills)).catch(() => setList([])); }, []);

  const run = async (what: string, fn: () => Promise<List>, ok?: string) => {
    setBusy(what);
    try { const r = await fn(); setList(r.skills); if (ok) toast.success(ok); return r; }
    catch (e) { toast.error((e as Error).message); }
    finally { setBusy(""); }
  };
  const install = async () => {
    const u = url.trim();
    if (!/^https:\/\//i.test(u)) { toast.error("只支持 https:// 地址"); return; }
    if (await run("url", () => api<List>("/api/skills/install", { method: "POST", body: JSON.stringify({ url: u }) }), "已安装 Skill")) setUrl("");
  };
  const upload = async (f: File | undefined) => {
    if (!f) return;
    if (f.size > 4 * 1024 * 1024) { toast.error("文件过大（上限 4 MB）"); return; }
    const fd = new FormData(); fd.append("file", f);
    await run("file", async () => {
      const r = await fetch("/api/skills/upload", { method: "POST", body: fd });
      const j = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(j.detail ?? `HTTP ${r.status}`);
      return j as List;
    }, `已安装 ${f.name}`);
    if (file.current) file.current.value = "";
  };
  const activate = (s: Skill) => run("act:" + s.slug, () => api<List>("/api/skills/active", { method: "PATCH", body: JSON.stringify({ slug: s.slug }) }), `AI 润色将使用「${s.name}」`);
  const remove = (s: Skill) => run("del:" + s.slug, () => api<List>(`/api/skills/${encodeURIComponent(s.slug)}`, { method: "DELETE" }), `已删除「${s.name}」`);
  const show = async (s: Skill) => {
    try { const r = await api<{ name: string; content: string }>(`/api/skills/${s.builtin ? "default" : encodeURIComponent(s.slug)}`); setPreview(r); }
    catch (e) { toast.error((e as Error).message); }
  };

  return (
    <Card className="mt-4" data-testid="skill-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Sparkles className="size-4 text-faint" />简报 Skill</CardTitle>
        <CardDescription>「AI 润色」按选中的 Skill（SKILL.md）改写简报：它会作为系统提示词发送给 DeepSeek。可从 https 地址或本地文件安装第三方 Skill。</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <ul className="divide-y divide-border/60 overflow-hidden rounded-lg border border-border/60" data-testid="skill-list">
          {!list && <li className="flex items-center gap-2 px-3 py-3 type-caption text-muted-foreground"><Loader2 className="size-3.5 animate-spin" />读取中…</li>}
          {list?.map((s) => (
            <li key={s.slug || "default"} className="flex items-center gap-3 px-3 py-2.5" data-testid="skill-row">
              <button type="button" onClick={() => !s.active && activate(s)} aria-label={`使用 ${s.name}`} aria-pressed={s.active}
                className={`grid size-4 shrink-0 place-items-center rounded-full border ${s.active ? "border-brand bg-brand text-white" : "border-border hover:border-foreground/40"}`}>
                {s.active && <Check className="size-2.5" />}
              </button>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-1.5 type-body font-medium">{s.name}{s.builtin && <Badge variant="secondary">内置</Badge>}{s.active && <Badge variant="success" data-testid="skill-active">使用中</Badge>}</div>
                <div className="truncate type-meta text-faint">{s.description || s.source || ""}{s.builtin ? "" : ` · ${(s.size / 1024).toFixed(1)} KB`}</div>
              </div>
              <Button variant="ghost" size="icon-sm" aria-label={`预览 ${s.name}`} onClick={() => show(s)}><Eye /></Button>
              {!s.builtin && <Button variant="ghost" size="icon-sm" aria-label={`删除 ${s.name}`} className="text-muted-foreground hover:text-destructive" disabled={busy === "del:" + s.slug} onClick={() => remove(s)}><Trash2 /></Button>}
            </li>
          ))}
        </ul>
        <div className="grid gap-2 sm:grid-cols-[1fr_auto_auto]">
          <Input placeholder="https://…/SKILL.md 或 .zip / .tar.gz" value={url} onChange={(e) => setUrl(e.target.value)} onKeyDown={(e) => e.key === "Enter" && install()} data-testid="skill-url" />
          <Button variant="outline" disabled={!url.trim() || !!busy} onClick={install} data-testid="skill-install">{busy === "url" ? <Loader2 className="animate-spin" /> : <Link2 />}从 URL 安装</Button>
          <Button variant="outline" disabled={!!busy} onClick={() => file.current?.click()}>{busy === "file" ? <Loader2 className="animate-spin" /> : <FileUp />}上传文件…</Button>
          <input ref={file} type="file" hidden accept=".md,.zip,.gz,.tgz,.tar,text/markdown,application/zip,application/gzip" onChange={(e) => upload(e.target.files?.[0])} data-testid="skill-file" />
        </div>
        <p className="type-meta text-faint">支持单个 SKILL.md，或包含 SKILL.md 的 zip / tar.gz（文件夹请先压缩）；上限 4 MB，SKILL.md 不超过 256 KB。保存在本机数据目录的 skills 文件夹。</p>
      </CardContent>
      <Dialog open={!!preview} onOpenChange={(o) => !o && setPreview(null)}>
        <DialogContent className="flex max-h-[80dvh] flex-col sm:max-w-2xl" data-testid="skill-preview">
          <DialogHeader><DialogTitle>{preview?.name}</DialogTitle><DialogDescription>SKILL.md 预览</DialogDescription></DialogHeader>
          <pre className="min-h-0 flex-1 overflow-auto whitespace-pre-wrap rounded-md bg-muted p-3 font-mono text-[12px] leading-relaxed">{preview?.content}</pre>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
