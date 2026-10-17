"use client";
// 设置 › 抓取 API (desktop only): keys for Jina Reader / Firecrawl / crawl4ai, used in that order by 生成文章.
import { useEffect, useState } from "react";
import { Eye, EyeOff, Globe, Loader2, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group";
import { Label } from "@/components/ui/label";
import { api } from "./common";

type Quota = { ok: boolean; remaining?: number; total?: number; allowance?: number; unit?: string; resetAt?: string; at: number; error?: string };
type Prov = { id: "jina" | "firecrawl" | "crawl4ai"; label: string; configured: boolean; status: "off" | "ok" | "exhausted" | "cooling"; why?: string; quota?: Quota; baseUrl?: string; baseUrlDefault?: string };
type FA = { providers: Prov[]; locked: boolean };

const STATUS: Record<Prov["status"], [string, "outline" | "secondary" | "destructive" | "muted"]> = {
  off: ["未配置", "muted"], ok: ["可用", "secondary"], exhausted: ["额度用尽", "destructive"], cooling: ["冷却中", "outline"],
};
// one number style everywhere: comma groups, up to 2 decimals (no 万 / compact notation)
const n = (v: number) => new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(v);
const day = (s: string) => { const d = new Date(/^\d{4}-\d{2}-\d{2}$/.test(s) ? `${s}T00:00:00+08:00` : s); return isNaN(+d) ? s : d.toLocaleDateString("zh-CN", { timeZone: "Asia/Shanghai", month: "numeric", day: "numeric" }); };

// 剩余 is what can be spent now (it can exceed the plan: coupons / top-ups); the plan amount and a monthly cap
// below the balance are shown as their own parts, never as "remaining / plan".
export function quotaText(q?: Quota) {
  if (!q) return "";
  if (!q.ok || (q.remaining == null && q.allowance == null)) return "额度 不可用";
  const unit = q.unit ? ` ${q.unit}` : "";
  const parts = [];
  if (q.remaining != null) parts.push(`剩余 ${n(q.remaining)}${unit}`);
  if (q.total != null && q.total > 0) parts.push(`套餐 ${n(q.total)}`);
  if (q.allowance != null && (q.remaining == null || q.allowance < q.remaining)) parts.push(`本月可用 ${n(q.allowance)}`);
  if (q.resetAt) parts.push(`${day(q.resetAt)}重置`);
  return parts.join(" · ");
}

export function FetchApiCard() {
  const [fa, setFa] = useState<FA | null>(null);
  const [keys, setKeys] = useState<Record<string, string>>({});
  const [base, setBase] = useState("");
  const [show, setShow] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState("");
  const fill = (r: FA) => { setFa(r); setKeys({}); setBase(r.providers.find((p) => p.id === "jina")?.baseUrl ?? ""); };
  useEffect(() => { api<FA>("/api/fetchapi").then(fill).catch(() => {}); }, []);
  const call = async (what: string, path: string, init: RequestInit, ok?: string) => {
    setBusy(what);
    try { fill(await api<FA>(path, init)); if (ok) toast.success(ok); } catch (e) { toast.error((e as Error).message); } finally { setBusy(""); }
  };
  const jina = fa?.providers.find((p) => p.id === "jina");
  const dirty = Object.values(keys).some((k) => k.trim()) || base.trim() !== (jina?.baseUrl ?? "");
  const save = () => {
    const body: Record<string, object> = {};
    for (const p of fa?.providers ?? []) {
      const b: Record<string, string> = {};
      if (keys[p.id]?.trim()) b.key = keys[p.id].trim();
      if (p.id === "jina" && base.trim() !== (p.baseUrl ?? "")) b.baseUrl = base.trim();
      if (Object.keys(b).length) body[p.id] = b;
    }
    void call("save", "/api/fetchapi", { method: "PATCH", body: JSON.stringify(body) }, "已保存");
  };
  const locked = !!fa?.locked;
  return (
    <Card className="mt-4" data-testid="fetchapi-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Globe className="size-4 text-faint" />抓取 API</CardTitle>
        <CardDescription>生成文章依次使用，简报自抓失败时补抓；额度用尽自动切换。</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        {fa?.providers.map((p) => {
          const [st, variant] = STATUS[p.status];
          const qt = quotaText(p.quota);
          return (
            <div key={p.id} className="grid gap-1.5" data-testid={`fetchapi-${p.id}`}>
              <div className="flex min-w-0 items-center gap-2">
                <Label htmlFor={`fa-${p.id}`}>{p.label}</Label>
                <Badge size="sm" variant={variant} title={p.why} data-testid={`fetchapi-${p.id}-status`}>{st}</Badge>
                {qt && <span className="ml-auto truncate font-mono type-meta tabular-nums text-faint" title={p.quota?.error ?? qt} data-testid={`fetchapi-${p.id}-quota`}>{qt}</span>}
              </div>
              <InputGroup>
                <InputGroupInput id={`fa-${p.id}`} type={show[p.id] ? "text" : "password"} autoComplete="off" spellCheck={false} disabled={locked}
                  placeholder={p.configured ? "已配置 · 输入新 key 替换" : "API key"} value={keys[p.id] ?? ""} onChange={(e) => setKeys({ ...keys, [p.id]: e.target.value })} />
                <InputGroupAddon align="inline-end">
                  <InputGroupButton size="icon-xs" aria-label={show[p.id] ? "隐藏" : "显示"} disabled={!keys[p.id]} onClick={() => setShow({ ...show, [p.id]: !show[p.id] })}>{show[p.id] ? <EyeOff /> : <Eye />}</InputGroupButton>
                  {p.configured && <InputGroupButton size="icon-xs" aria-label={`清除 ${p.label} key`} disabled={locked || !!busy} onClick={() => call("clear", "/api/fetchapi", { method: "PATCH", body: JSON.stringify({ [p.id]: { clearKey: true } }) }, "已清除 key")}><Trash2 /></InputGroupButton>}
                </InputGroupAddon>
              </InputGroup>
              {p.id === "jina" && (
                <Input aria-label="Jina Base URL" data-testid="fetchapi-jina-base" className="font-mono text-[12px]" disabled={locked} placeholder={p.baseUrlDefault} value={base} onChange={(e) => setBase(e.target.value)} />
              )}
            </div>
          );
        })}
      </CardContent>
      <CardFooter className="flex justify-end gap-2">
        <Button variant="outline" className="mr-auto" disabled={!fa?.providers.some((p) => p.configured) || !!busy} onClick={() => call("quota", "/api/fetchapi/quota", { method: "POST" })} data-testid="fetchapi-refresh">
          {busy === "quota" ? <Loader2 className="animate-spin" /> : <RefreshCw />}刷新额度
        </Button>
        <Button disabled={locked || !!busy || !dirty} onClick={save} data-testid="fetchapi-save">{busy === "save" && <Loader2 className="animate-spin" />}保存</Button>
      </CardFooter>
    </Card>
  );
}
