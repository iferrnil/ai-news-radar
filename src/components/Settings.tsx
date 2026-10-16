"use client";
// 设置, grouped cards: 常规 · 通知 · 翻译 / AI (DeepSeek key, base URL, model, test) · 简报 Skill (desktop) · 更新 (desktop) · 数据 · 关于.
import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { BookOpen, CheckCircle2, Database, Eye, EyeOff, FolderOpen, ExternalLink, Info, Languages, Loader2, Lock, PlugZap, Trash2, XCircle } from "lucide-react";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { type TrScopes, api, secZh } from "./common";
import { GeneralCard, NotifyCard, UpdateCard, type NotifyPrefs } from "./settings-extra";
const SkillCard = dynamic(() => import("./settings-skill"), { ssr: false });
const MultimodalCard = dynamic(() => import("./settings-media").then((m) => m.MultimodalCard), { ssr: false });
const ArtifactsCard = dynamic(() => import("./settings-media").then((m) => m.ArtifactsCard), { ssr: false });

type DS = { configured: boolean; source: "app" | "env" | null; baseUrl: string; model: string; baseUrlCustom: boolean; modelCustom: boolean; locked: boolean };
type TR = { available: boolean; reason: string | null; scopes: TrScopes; done: number; queued: number; failed: number; lastError: string | null };
const trScopeText = (sc: TrScopes) => [sc.all && "全部", ...sc.sections.map((k) => `分区「${secZh(k)}」`), sc.sources.length && `${sc.sources.length} 个来源`].filter(Boolean).join("、") || "无";
type S = { deepseek: DS; translate: TR; notify: NotifyPrefs; desktop: boolean; version?: string | null; dataDir?: string };
type Test = { ok: boolean; error?: string } | null;

export default function Settings() {
  const [s, setS] = useState<S | null>(null);
  const [key, setKey] = useState(""); const [show, setShow] = useState(false);
  const [base, setBase] = useState(""); const [model, setModel] = useState("");
  const [busy, setBusy] = useState<"" | "save" | "test" | "clear">(""); const [test, setTest] = useState<Test>(null);
  const load = () => api<S>("/api/settings").then((r) => { setS(r); setBase(r.deepseek.baseUrlCustom ? r.deepseek.baseUrl : ""); setModel(r.deepseek.modelCustom ? r.deepseek.model : ""); });
  useEffect(() => { load().catch(() => {}); }, []);
  const ds = s?.deepseek; const locked = !!ds?.locked;
  const patch = async (body: object, what: typeof busy, msg: string) => {
    setBusy(what);
    try { const r = await api<S>("/api/settings", { method: "PATCH", body: JSON.stringify(body) }); setS(r); toast.success(msg); return true; }
    catch (e) { toast.error((e as Error).message); return false; }
    finally { setBusy(""); }
  };
  const save = async () => { if (await patch({ deepseek: { ...(key.trim() ? { key: key.trim() } : {}), baseUrl: base, model } }, "save", key.trim() ? "已保存 API key" : "已保存")) { setKey(""); setShow(false); setTest(null); } };
  const clear = async () => { if (await patch({ deepseek: { clearKey: true } }, "clear", "已清除应用内保存的 API key")) setTest(null); };
  const runTest = async () => {
    setBusy("test"); setTest(null);
    try { setTest(await api<Test>("/api/settings/test", { method: "POST", body: JSON.stringify(locked ? {} : { key: key.trim() || undefined, baseUrl: base || undefined }) })); }
    catch (e) { setTest({ ok: false, error: (e as Error).message }); }
    finally { setBusy(""); }
  };

  return (
    <div className="mx-auto w-full max-w-[760px] px-3 pb-20 pt-6 sm:px-6">
      <h1 className="type-h1 font-semibold tracking-tight">设置</h1>

      <GeneralCard />
      <NotifyCard desktop={!!s?.desktop} server={s?.notify ?? null} locked={locked} onSaved={(n) => setS((cur) => (cur ? { ...cur, notify: n } : cur))} />

      <Card className="mt-4" data-testid="ai-card">
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><Languages className="size-4 text-faint" />翻译 / AI</CardTitle>
          <CardDescription>
            {!ds ? "读取中…" : ds.configured
              ? <span className="inline-flex flex-wrap items-center gap-1.5"><Badge variant="success" data-testid="key-status">已配置</Badge>{ds.source === "env" ? "环境变量" : "应用内"}</span>
              : <span className="inline-flex items-center gap-1.5"><Badge variant="secondary" data-testid="key-status">未配置</Badge></span>}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          {locked && <Alert><Lock /><AlertTitle>密钥由部署方配置</AlertTitle><AlertDescription>SETTINGS_LOCKED=1</AlertDescription></Alert>}
          <div className="type-caption text-muted-foreground" data-testid="settings-translate">
            翻译开关在页面右上角 <Languages className="inline size-3 align-[-1px]" />。
            {s && (s.translate.available ? ` 已开启：${trScopeText(s.translate.scopes)}` : ` 不可用：${s.translate.reason ?? "未配置"}`)}
            {s?.translate.lastError && <div className="mt-1 type-meta text-red-500">{s.translate.lastError}</div>}
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="ds-key">API key</Label>
            <InputGroup>
              <InputGroupInput id="ds-key" type={show ? "text" : "password"} autoComplete="off" spellCheck={false} disabled={locked}
                placeholder={ds?.configured ? "已配置 · 输入新 key 替换" : "sk-…"} value={key} onChange={(e) => setKey(e.target.value)} />
              <InputGroupAddon align="inline-end">
                <InputGroupButton size="icon-xs" aria-label={show ? "隐藏" : "显示"} disabled={!key} onClick={() => setShow(!show)}>{show ? <EyeOff /> : <Eye />}</InputGroupButton>
              </InputGroupAddon>
            </InputGroup>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5"><Label htmlFor="ds-base">Base URL（可选）</Label>
              <Input id="ds-base" disabled={locked} placeholder={ds?.baseUrlCustom ? "" : ds?.baseUrl ?? "https://api.deepseek.com"} value={base} onChange={(e) => setBase(e.target.value)} /></div>
            <div className="grid gap-1.5"><Label htmlFor="ds-model">模型（可选）</Label>
              <Input id="ds-model" disabled={locked} placeholder={ds?.modelCustom ? "" : ds?.model ?? "deepseek-chat"} value={model} onChange={(e) => setModel(e.target.value)} /></div>
          </div>
          {test && (
            <Alert variant={test.ok ? "default" : "destructive"} data-testid="test-result">
              {test.ok ? <CheckCircle2 className="text-emerald-500" /> : <XCircle />}
              <AlertTitle>{test.ok ? "连接成功" : "连接失败"}</AlertTitle>
              {test.error && <AlertDescription>{test.error}</AlertDescription>}
            </Alert>
          )}
        </CardContent>
        <CardFooter className="flex flex-wrap justify-end gap-2">
          {ds?.source === "app" && <Button variant="ghost" className="mr-auto text-muted-foreground hover:text-destructive" disabled={locked || !!busy} onClick={clear}>{busy === "clear" ? <Loader2 className="animate-spin" /> : <Trash2 />}清除 key</Button>}
          <Button variant="outline" disabled={!!busy || (!ds?.configured && !key.trim())} onClick={runTest}>{busy === "test" ? <Loader2 className="animate-spin" /> : <PlugZap />}测试连接</Button>
          <Button disabled={locked || !!busy} onClick={save}>{busy === "save" && <Loader2 className="animate-spin" />}保存</Button>
        </CardFooter>
      </Card>

      {s?.desktop && <><SkillCard /><SkillCard kind="twitter" /><SkillCard kind="wechat" /><SkillCard kind="xhs" /><MultimodalCard /><ArtifactsCard /></>}
      {s?.desktop && <UpdateCard version={s.version ?? null} />}
      {s?.dataDir && <DataCard dataDir={s.dataDir} />}
      <AboutCard version={s?.version ?? null} desktop={!!s?.desktop} />
    </div>
  );
}

function DataCard({ dataDir }: { dataDir: string }) {
  const openDir = async () => { try { await api("/api/desktop/open-data", { method: "POST" }); } catch (e) { toast.error((e as Error).message); } };
  return (
    <Card className="mt-4" data-testid="data-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Database className="size-4 text-faint" />数据</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-wrap items-center gap-2">
        <div className="min-w-0 flex-1 truncate font-mono text-[12px] text-muted-foreground" title={dataDir}>{dataDir}</div>
        <Button size="sm" variant="outline" onClick={openDir}><FolderOpen />打开数据目录</Button>
      </CardContent>
    </Card>
  );
}

function AboutCard({ version, desktop }: { version: string | null; desktop: boolean }) {
  return (
    <Card className="mt-4" data-testid="about-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Info className="size-4 text-faint" />关于</CardTitle>
        <CardDescription>AI News Radar{version ? <span className="font-mono"> v{version}</span> : null}{desktop ? " · 桌面版" : ""}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" render={<Link href="/docs" />}><BookOpen />使用文档</Button>
        <Button variant="outline" size="sm" render={<a href="https://github.com/iferrnil/ai-news-radar" target="_blank" rel="noreferrer" />}><ExternalLink />GitHub</Button>
      </CardContent>
    </Card>
  );
}
