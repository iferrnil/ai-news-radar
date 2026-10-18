"use client";
// 设置 › 多模态模型 and 工作目录 (desktop only).
import { useEffect, useState } from "react";
import { Eye, EyeOff, FolderOpen, ImageIcon, Loader2, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group";
import { Label } from "@/components/ui/label";
import { api } from "./common";

type MM = { baseUrl: string; configured: boolean; imageModel: string; visionModel: string; image: boolean; vision: boolean; locked: boolean };

export function MultimodalCard() {
  const [m, setM] = useState<MM | null>(null);
  const [f, setF] = useState({ baseUrl: "", key: "", imageModel: "", visionModel: "" });
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState("");
  const fill = (r: MM) => { setM(r); setF({ baseUrl: r.baseUrl, key: "", imageModel: r.imageModel ?? "", visionModel: r.visionModel ?? "" }); };
  useEffect(() => { api<MM>("/api/multimodal").then(fill).catch(() => {}); }, []);
  const patch = async (what: string, body: object, ok: string) => {
    setBusy(what);
    try { fill(await api<MM>("/api/multimodal", { method: "PATCH", body: JSON.stringify(body) })); toast.success(ok); }
    catch (e) { toast.error((e as Error).message); } finally { setBusy(""); }
  };
  const save = () => { const { key, ...rest } = f; void patch("save", key.trim() ? { ...rest, key: key.trim() } : rest, "已保存"); };
  const locked = !!m?.locked;
  const field = (k: keyof typeof f, label: string, ph: string) => (
    <div className="grid gap-1.5"><Label htmlFor={`mm-${k}`}>{label}</Label>
      <Input id={`mm-${k}`} disabled={locked} placeholder={ph} value={f[k]} onChange={(e) => setF({ ...f, [k]: e.target.value })} /></div>
  );
  const on = m ? [m.image && "封面", m.vision && "看图"].filter(Boolean).join(" · ") : "";
  return (
    <Card className="mt-4" data-testid="mm-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><ImageIcon className="size-4 text-faint" />多模态模型</CardTitle>
        <CardDescription>OpenAI 兼容接口，用于文章封面和看图；未配置则跳过。{on && <span data-testid="mm-on"> 已启用：{on}</span>}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        {field("baseUrl", "Base URL", "https://…/v1")}
        <div className="grid gap-1.5">
          <Label htmlFor="mm-key">API key</Label>
          <InputGroup>
            <InputGroupInput id="mm-key" type={show ? "text" : "password"} autoComplete="off" spellCheck={false} disabled={locked}
              placeholder={m?.configured ? "已配置 · 输入新 key 替换" : "sk-…"} value={f.key} onChange={(e) => setF({ ...f, key: e.target.value })} />
            <InputGroupAddon align="inline-end"><InputGroupButton size="icon-xs" aria-label={show ? "隐藏" : "显示"} disabled={!f.key} onClick={() => setShow(!show)}>{show ? <EyeOff /> : <Eye />}</InputGroupButton></InputGroupAddon>
          </InputGroup>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          {field("imageModel", "图片模型", "如 gpt-image-1")}
          {field("visionModel", "看图模型（可选）", "")}
        </div>
      </CardContent>
      <CardFooter className="flex justify-end gap-2">
        {m?.configured && <Button variant="ghost" className="mr-auto text-muted-foreground hover:text-destructive" disabled={locked || !!busy} onClick={() => patch("clear", { clearKey: true }, "已清除 key")}>{busy === "clear" ? <Loader2 className="animate-spin" /> : <Trash2 />}清除 key</Button>}
        <Button disabled={locked || !!busy} onClick={save} data-testid="mm-save">{busy === "save" && <Loader2 className="animate-spin" />}保存</Button>
      </CardFooter>
    </Card>
  );
}

type Art = { dir: string; default: string; custom: boolean; canOpen: boolean };

export function ArtifactsCard() {
  const [a, setA] = useState<Art | null>(null);
  const [dir, setDir] = useState("");
  useEffect(() => { api<Art>("/api/artifacts").then((r) => { setA(r); setDir(r.custom ? r.dir : ""); }).catch(() => {}); }, []);
  const save = async (d: string) => {
    try { const r = await api<Art>("/api/artifacts", { method: "PATCH", body: JSON.stringify({ dir: d }) }); setA(r); setDir(r.custom ? r.dir : ""); toast.success("已保存"); }
    catch (e) { toast.error((e as Error).message); }
  };
  const open = async () => { try { await api("/api/artifacts/open", { method: "POST" }); } catch (e) { toast.error((e as Error).message); } };
  return (
    <Card className="mt-4" data-testid="artifacts-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><FolderOpen className="size-4 text-faint" />工作目录</CardTitle>
        <CardDescription>导出的简报和文章存在这里；抓取的原文和生成的封面在「缓存」中保留 24 小时。</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        <Input className="min-w-0 flex-1 font-mono type-2xs" placeholder={a?.default ?? ""} value={dir} onChange={(e) => setDir(e.target.value)} onKeyDown={(e) => e.key === "Enter" && save(dir.trim())} aria-label="工作目录" data-testid="artifacts-dir" />
        <Button variant="outline" size="sm" className="h-9" disabled={!a || dir.trim() === (a.custom ? a.dir : "")} onClick={() => save(dir.trim())}>保存</Button>
        {a?.canOpen && <Button variant="outline" size="sm" className="h-9" onClick={open} data-testid="artifacts-open"><FolderOpen />打开</Button>}
      </CardContent>
    </Card>
  );
}
