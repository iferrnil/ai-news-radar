"use client";
// 设置 › 简报 Skill (desktop only). Agent Skills format: a folder with SKILL.md (front matter name + description)
// and optional references/ · scripts/ · assets/. 「AI 润色」 uses the active one; scripts are never run.
import { useEffect, useRef, useState } from "react";
import { Check, Eye, FileUp, FolderUp, Link2, Loader2, Sparkles, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { api } from "./common";

type SFile = { path: string; size: number; kind: "skill" | "reference" | "script" | "asset" | "other" };
type Skill = { name: string; slug: string; description: string; license?: string; source?: string; installedAt?: number; size: number; files: SFile[] | null; builtin?: boolean; active: boolean };
type List = { skills: Skill[]; installed?: string };
type Preview = { skill: Skill; file: string; content: string };

const kb = (n: number) => `${(n / 1024).toFixed(n < 10240 ? 1 : 0)} KB`;
const counts = (s: Skill) => {
  const f = s.files ?? [];
  const n = (k: SFile["kind"]) => f.filter((x) => x.kind === k).length;
  return [n("reference") && `${n("reference")} 个参考`, n("asset") && `${n("asset")} 个资源`, n("script") && `${n("script")} 个脚本（不执行）`].filter(Boolean).join(" · ");
};

async function upload(fd: FormData): Promise<List> {
  const r = await fetch("/api/skills/upload", { method: "POST", body: fd });
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(j.detail ?? `HTTP ${r.status}`);
  return j as List;
}

export default function SkillCard() {
  const [list, setList] = useState<Skill[] | null>(null);
  const [url, setUrl] = useState("");
  const [busy, setBusy] = useState("");
  const [pv, setPv] = useState<Preview | null>(null);
  const file = useRef<HTMLInputElement>(null);
  const folder = useRef<HTMLInputElement>(null);
  useEffect(() => { api<List>("/api/skills").then((r) => setList(r.skills)).catch(() => setList([])); }, []);
  useEffect(() => { folder.current?.setAttribute("webkitdirectory", ""); }, []);

  const run = async (what: string, fn: () => Promise<List>, ok?: (r: List) => string) => {
    setBusy(what);
    try { const r = await fn(); setList(r.skills); if (ok) toast.success(ok(r)); return r; }
    catch (e) { toast.error((e as Error).message); }
    finally { setBusy(""); }
  };
  const installed = (r: List) => `已安装 ${r.installed ?? "Skill"}`;
  const install = async () => {
    const u = url.trim();
    if (!/^https:\/\//i.test(u)) { toast.error("只支持 https:// 地址"); return; }
    if (await run("url", () => api<List>("/api/skills/install", { method: "POST", body: JSON.stringify({ url: u }) }), installed)) setUrl("");
  };
  const upFile = async (f?: File) => {
    if (!f) return;
    const fd = new FormData(); fd.append("file", f);
    await run("file", () => upload(fd), installed);
    if (file.current) file.current.value = "";
  };
  const upFolder = async (fl: FileList | null) => {
    if (!fl?.length) return;
    const fd = new FormData();
    for (const f of Array.from(fl)) { const rel = (f as File & { webkitRelativePath?: string }).webkitRelativePath || f.name; fd.append("files", f, rel); fd.append("paths", rel); }
    await run("folder", () => upload(fd), installed);
    if (folder.current) folder.current.value = "";
  };
  const activate = (s: Skill) => run("act", () => api<List>("/api/skills/active", { method: "PATCH", body: JSON.stringify({ slug: s.slug }) }), () => `AI 润色将使用 ${s.name}`);
  const remove = (s: Skill) => run("del:" + s.slug, () => api<List>(`/api/skills/${encodeURIComponent(s.slug)}`, { method: "DELETE" }), () => `已删除 ${s.name}`);
  const show = async (s: Skill, f = "SKILL.md") => {
    try { const r = await api<{ content: string; file: string }>(`/api/skills/${s.builtin ? "default" : encodeURIComponent(s.slug)}?file=${encodeURIComponent(f)}`); setPv({ skill: s, file: r.file, content: r.content }); }
    catch (e) { toast.error((e as Error).message); }
  };

  return (
    <Card className="mt-4" data-testid="skill-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Sparkles className="size-4 text-faint" />简报 Skill</CardTitle>
        <CardDescription>「AI 润色」使用的 Agent Skill。脚本不会执行。</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <ul className="divide-y divide-border/60 overflow-hidden rounded-lg border border-border/60" data-testid="skill-list">
          {!list && <li className="flex items-center gap-2 px-3 py-3 type-caption text-muted-foreground"><Loader2 className="size-3.5 animate-spin" />读取中…</li>}
          {list?.map((s) => (
            <li key={s.slug || "builtin"} className="flex items-center gap-3 px-3 py-2.5" data-testid="skill-row">
              <button type="button" onClick={() => !s.active && activate(s)} aria-label={`使用 ${s.name}`} aria-pressed={s.active} disabled={!!busy}
                className={cn("grid size-4 shrink-0 place-items-center rounded-full border transition-colors", s.active ? "border-brand bg-brand text-white" : "border-border hover:border-foreground/40")}>
                {s.active && <Check className="size-2.5" />}
              </button>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-1.5 type-body font-medium"><span className="font-mono text-[13px]">{s.name}</span>{s.builtin && <Badge variant="secondary">内置</Badge>}{s.active && <Badge variant="success" data-testid="skill-active">使用中</Badge>}</div>
                <div className="line-clamp-1 type-meta text-faint" title={s.description}>{s.description}</div>
                <div className="type-meta text-faint" data-testid="skill-counts">{[counts(s), kb(s.size)].filter(Boolean).join(" · ")}</div>
              </div>
              <Button variant="ghost" size="icon-sm" aria-label={`预览 ${s.name}`} onClick={() => show(s)}><Eye /></Button>
              {!s.builtin && <Button variant="ghost" size="icon-sm" aria-label={`删除 ${s.name}`} className="text-muted-foreground hover:text-destructive" disabled={busy === "del:" + s.slug} onClick={() => remove(s)}><Trash2 /></Button>}
            </li>
          ))}
        </ul>
        <div className="flex gap-2">
          <Input placeholder="https://github.com/…/tree/main/skill-name、.zip、.tar.gz 或 SKILL.md" value={url} onChange={(e) => setUrl(e.target.value)} onKeyDown={(e) => e.key === "Enter" && install()} data-testid="skill-url" />
          <Button variant="outline" className="shrink-0" disabled={!url.trim() || !!busy} onClick={install} data-testid="skill-install">{busy === "url" ? <Loader2 className="animate-spin" /> : <Link2 />}安装</Button>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" disabled={!!busy} onClick={() => folder.current?.click()}>{busy === "folder" ? <Loader2 className="animate-spin" /> : <FolderUp />}上传文件夹…</Button>
          <Button variant="outline" size="sm" disabled={!!busy} onClick={() => file.current?.click()}>{busy === "file" ? <Loader2 className="animate-spin" /> : <FileUp />}上传 zip / SKILL.md…</Button>
          <input ref={folder} type="file" hidden multiple onChange={(e) => upFolder(e.target.files)} data-testid="skill-folder" />
          <input ref={file} type="file" hidden accept=".md,.zip,.gz,.tgz,.tar" onChange={(e) => upFile(e.target.files?.[0])} data-testid="skill-file" />
        </div>
      </CardContent>
      <Dialog open={!!pv} onOpenChange={(o) => !o && setPv(null)}>
        <DialogContent className="flex max-h-[82dvh] flex-col gap-3 sm:max-w-3xl" data-testid="skill-preview">
          <DialogHeader><DialogTitle className="font-mono">{pv?.skill.name}</DialogTitle><DialogDescription className="line-clamp-2">{pv?.skill.description}</DialogDescription></DialogHeader>
          <div className="grid min-h-0 flex-1 gap-3 sm:grid-cols-[180px_1fr]">
            <ul className="max-h-[60dvh] space-y-0.5 overflow-auto" data-testid="skill-files">
              {pv?.skill.files?.map((f) => (
                <li key={f.path}>
                  <button type="button" onClick={() => f.kind !== "script" && pv && show(pv.skill, f.path)} disabled={f.kind === "script"}
                    className={cn("flex w-full items-center gap-1.5 rounded px-2 py-1 text-left font-mono text-[12px]", f.path === pv.file ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-hover", f.kind === "script" && "cursor-not-allowed opacity-60")}>
                    <span className="min-w-0 flex-1 truncate" title={f.path}>{f.path}</span>{f.kind === "script" && <span className="shrink-0 font-sans text-[10px]">不执行</span>}
                  </button>
                </li>
              ))}
            </ul>
            <pre className="min-h-0 overflow-auto whitespace-pre-wrap rounded-md bg-muted p-3 font-mono text-[12px] leading-relaxed">{pv?.content}</pre>
          </div>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
