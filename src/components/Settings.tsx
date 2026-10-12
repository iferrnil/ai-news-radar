"use client";
// 设置: DeepSeek key (write-only, masked), optional base URL / model, translation toggle; P0 通知, 阅读, desktop updates (settings-extra.tsx).
import { useEffect, useState } from "react";
import { CheckCircle2, Eye, EyeOff, KeyRound, Loader2, Lock, PlugZap, Trash2, XCircle } from "lucide-react";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { api } from "./common";
import { DesktopCard, NotifyCard, ReadingCard, type NotifyPrefs } from "./settings-extra";

type DS = { configured: boolean; source: "app" | "env" | null; baseUrl: string; model: string; baseUrlCustom: boolean; modelCustom: boolean; locked: boolean };
type TR = { available: boolean; enabled: boolean; done: number; queued: number; failed: number; lastError: string | null };
type S = { deepseek: DS; translate: TR; notify: NotifyPrefs; desktop: boolean; version?: string | null };
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
      <p className="mt-0.5 type-body text-muted-foreground">DeepSeek 翻译、P0 通知与阅读。API key 只保存在服务器端（本机数据目录，权限 0600），不会出现在任何接口返回、日志或导出文件中。</p>

      {locked && <Alert className="mt-4"><Lock /><AlertTitle>设置已锁定</AlertTitle><AlertDescription>这个部署设置了 SETTINGS_LOCKED=1，只能通过环境变量（DEEPSEEK_API_KEY 等）配置。</AlertDescription></Alert>}

      <Card className="mt-4">
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><KeyRound className="size-4 text-faint" />DeepSeek API</CardTitle>
          <CardDescription>
            {!ds ? "读取中…" : ds.configured
              ? <span className="inline-flex flex-wrap items-center gap-1.5"><Badge variant="success" data-testid="key-status">已配置</Badge>{ds.source === "env" ? "来自环境变量 DEEPSEEK_API_KEY" : "保存在应用内"}</span>
              : <span className="inline-flex items-center gap-1.5"><Badge variant="secondary" data-testid="key-status">未配置</Badge>填写后即可开启抓取时翻译</span>}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="grid gap-1.5">
            <Label htmlFor="ds-key">API key</Label>
            <InputGroup>
              <InputGroupInput id="ds-key" type={show ? "text" : "password"} autoComplete="off" spellCheck={false} disabled={locked}
                placeholder={ds?.configured ? "已配置（出于安全不显示）· 输入新 key 可替换" : "sk-…"} value={key} onChange={(e) => setKey(e.target.value)} />
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

      <Card className="mt-4">
        <CardContent>
          <label className="flex cursor-pointer items-start gap-3">
            <div className="flex-1">
              <div className="type-ui font-medium">抓取时翻译</div>
              <div className="mt-0.5 type-caption text-muted-foreground">
                {!s ? "读取中…" : !s.translate.available ? "需要先配置 API key。" : `新外文条目自动翻译标题与摘要 · 已译 ${s.translate.done} · 排队 ${s.translate.queued}${s.translate.failed ? ` · 失败 ${s.translate.failed}` : ""}`}
              </div>
              {s?.translate.lastError && <div className="mt-1 type-meta text-red-500">{s.translate.lastError}</div>}
            </div>
            <Switch aria-label="抓取时翻译" checked={!!s?.translate.enabled} disabled={!s?.translate.available || !!busy}
              onCheckedChange={(v) => patch({ translate: !!v }, "save", v ? "已开启抓取时翻译" : "已关闭抓取时翻译")} />
          </label>
        </CardContent>
      </Card>

      <NotifyCard desktop={!!s?.desktop} server={s?.notify ?? null} locked={locked} onSaved={(n) => setS((cur) => (cur ? { ...cur, notify: n } : cur))} />
      <ReadingCard />
      {s?.desktop && <DesktopCard version={s.version ?? null} />}
    </div>
  );
}
