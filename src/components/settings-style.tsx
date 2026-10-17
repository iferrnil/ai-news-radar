"use client";
// 设置 › 写作风格 (desktop): a few of your own past articles per platform (paste / .md .txt / URL via the fetcher) and a
// style profile distilled from them by the model (editable). 生成文章 imitates the voice, never the samples' facts.
import { useEffect, useRef, useState } from "react";
import { FileUp, Link2, Loader2, PenLine, Plus, Sparkles, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { api } from "./common";

type Sample = { id: string; title: string; text: string; source: "paste" | "file" | "url"; url?: string; added: number };
type Style = { kind: string; samples: Sample[]; profile: string; profileAt?: number; enabled: boolean };
const KINDS = [["twitter", "推特"], ["wechat", "公众号"], ["xhs", "小红书"]] as const;
const SRC = { paste: "粘贴", file: "文件", url: "网址" } as const;

export default function StyleCard() {
  const [kind, setKind] = useState<(typeof KINDS)[number][0]>("wechat");
  const [st, setSt] = useState<Style | null>(null);
  const [paste, setPaste] = useState(""); const [url, setUrl] = useState("");
  const [profile, setProfile] = useState(""); const [busy, setBusy] = useState("");
  const file = useRef<HTMLInputElement>(null);
  useEffect(() => { setSt(null); api<Style>(`/api/style?kind=${kind}`).then((r) => { setSt(r); setProfile(r.profile); }).catch(() => setSt(null)); }, [kind]);
  const run = async (what: string, fn: () => Promise<Style>, ok?: string) => {
    setBusy(what);
    try { const r = await fn(); setSt(r); setProfile(r.profile); if (ok) toast.success(ok); return true; }
    catch (e) { toast.error((e as Error).message); return false; }
    finally { setBusy(""); }
  };
  const patch = (body: object) => api<Style>("/api/style", { method: "PATCH", body: JSON.stringify({ kind, ...body }) });
  const addPaste = async () => { if (await run("paste", () => patch({ add: { text: paste, source: "paste" } }), "已添加样本")) setPaste(""); };
  const addUrl = async () => { if (await run("url", () => api<Style>("/api/style/fetch", { method: "POST", body: JSON.stringify({ kind, url: url.trim() }) }), "已抓取并添加")) setUrl(""); };
  const addFiles = async (fs: FileList | null) => {
    for (const f of Array.from(fs ?? [])) {
      if (!/\.(md|markdown|txt)$/i.test(f.name) || f.size > 512 * 1024) { toast.error(`${f.name}：只支持 .md / .txt（≤512 KB）`); continue; }
      const text = await f.text();
      await run("file", () => patch({ add: { title: f.name.replace(/\.(md|markdown|txt)$/i, ""), text, source: "file" } }), `已添加 ${f.name}`);
    }
    if (file.current) file.current.value = "";
  };
  const n = st?.samples.length ?? 0;
  return (
    <Card className="mt-4" data-testid="style-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><PenLine className="size-4 text-faint" />写作风格</CardTitle>
        <CardDescription>导入几篇自己的旧文，生成文章时模仿你的语气（不会沿用旧文里的事实）</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <div className="flex items-center gap-1" role="tablist">
          {KINDS.map(([k, l]) => (
            <Button key={k} role="tab" aria-selected={kind === k} variant={kind === k ? "secondary" : "ghost"} size="sm" onClick={() => setKind(k)} data-testid={`style-tab-${k}`}>{l}</Button>
          ))}
          {st && (
            <Label className="ml-auto gap-2 type-caption font-normal text-muted-foreground">
              <Switch size="sm" checked={st.enabled} disabled={!!busy} onCheckedChange={(v) => void run("on", () => patch({ enabled: !!v }))} data-testid="style-enabled" />启用
            </Label>
          )}
        </div>
        {st && n > 0 && (
          <ul className="divide-y rounded-lg border" data-testid="style-samples">
            {st.samples.map((x) => (
              <li key={x.id} className="flex items-center gap-2 px-3 py-2 type-body">
                <Badge size="sm" variant="muted">{SRC[x.source]}</Badge>
                <span className="min-w-0 flex-1 truncate" title={x.url ?? x.title}>{x.title}</span>
                <span className="shrink-0 font-mono type-meta tabular-nums text-faint">{x.text.length} 字</span>
                <Button variant="ghost" size="icon-xs" aria-label={`删除 ${x.title}`} disabled={!!busy} onClick={() => void run("rm", () => patch({ remove: x.id }))}><Trash2 /></Button>
              </li>
            ))}
          </ul>
        )}
        {st && n < 6 && (
          <div className="grid gap-2">
            <Textarea value={paste} onChange={(e) => setPaste(e.target.value)} placeholder="粘贴一篇自己写的文章（至少 80 字）" className="min-h-24" data-testid="style-paste" />
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" disabled={!!busy || paste.trim().length < 80} onClick={() => void addPaste()} data-testid="style-add">{busy === "paste" ? <Loader2 className="animate-spin" /> : <Plus />}添加</Button>
              <Button variant="outline" size="sm" disabled={!!busy} onClick={() => file.current?.click()}>{busy === "file" ? <Loader2 className="animate-spin" /> : <FileUp />}.md / .txt</Button>
              <input ref={file} type="file" accept=".md,.markdown,.txt,text/markdown,text/plain" multiple hidden onChange={(e) => void addFiles(e.target.files)} />
              <div className="flex min-w-[220px] flex-1 gap-2">
                <Input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://…" aria-label="文章网址" className="h-8" />
                <Button variant="outline" size="sm" disabled={!!busy || !/^https?:\/\//i.test(url.trim())} onClick={() => void addUrl()}>{busy === "url" ? <Loader2 className="animate-spin" /> : <Link2 />}抓取</Button>
              </div>
            </div>
          </div>
        )}
        {st && (
          <div className="grid gap-2">
            <div className="flex items-center gap-2">
              <Label htmlFor="style-profile">风格说明</Label>
              <Button variant="ghost" size="sm" className="ml-auto" disabled={!!busy || !n} onClick={() => void run("gen", () => api<Style>("/api/style/profile", { method: "POST", body: JSON.stringify({ kind }) }), "已生成")} data-testid="style-gen">
                {busy === "gen" ? <Loader2 className="animate-spin" /> : <Sparkles />}{st.profile ? "重新生成" : "从样本生成"}
              </Button>
            </div>
            <Textarea id="style-profile" value={profile} onChange={(e) => setProfile(e.target.value)} placeholder={n ? "点「从样本生成」，或直接写" : "先添加样本，或直接写下你的风格要求"} className={cn("min-h-32 font-mono text-[12.5px]")} data-testid="style-profile" />
            <div className="flex justify-end">
              <Button size="sm" disabled={!!busy || profile === st.profile} onClick={() => void run("save", () => patch({ profile }), "已保存")}>保存</Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
