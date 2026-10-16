"use client";
// 设置 › 通知 (P0) and 阅读. Web: prefs + permission live in this browser; desktop: prefs live on the server,
// which the shell reads before showing a native notification.
import { useEffect, useState } from "react";
import { BellRing, Download, ExternalLink, Loader2, Send, SlidersHorizontal } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Switch } from "@/components/ui/switch";
import { useApp } from "./app-shell";
import { api } from "./common";
import { SectionName } from "./section-icon";
import { askPermission, setWebPrefs, webNotifySupported, webPrefs, type WebNotifyPrefs } from "./notify-client";
import { markOnHover, setMarkOnHover } from "./read-store";

export type NotifyPrefs = { enabled: boolean; sections: string[]; system?: boolean; scope?: "p0" | "all" };
const PERM: Record<string, { label: string; v: "success" | "destructive" | "secondary" | "warning" }> = {
  granted: { label: "浏览器已允许", v: "success" }, denied: { label: "浏览器已拒绝", v: "destructive" }, default: { label: "尚未授权", v: "secondary" }, unsupported: { label: "此浏览器不支持", v: "warning" },
};

const NATIVE: Record<string, { label: string; v: "success" | "destructive" | "secondary" | "warning" }> = {
  granted: { label: "系统已允许", v: "success" }, denied: { label: "系统已拒绝", v: "destructive" }, unknown: { label: "尚未询问", v: "secondary" }, unsupported: { label: "当前运行方式不支持", v: "warning" },
};

export function NotifyCard({ desktop, server, locked, onSaved }: { desktop: boolean; server: NotifyPrefs | null; locked: boolean; onSaved: (p: NotifyPrefs) => void }) {
  const { sections } = useApp();
  const [web, setWeb] = useState<WebNotifyPrefs>({ enabled: false, sections: [] });
  const [perm, setPerm] = useState<string>("default");
  const [native, setNative] = useState<string | null>(null);
  const [testing, setTesting] = useState(false);
  useEffect(() => { setWeb(webPrefs()); setPerm(webNotifySupported() ? Notification.permission : "unsupported"); }, []);
  useEffect(() => { if (desktop) api<{ status: string }>("/api/desktop/notify").then((r) => setNative(r.status)).catch(() => {}); }, [desktop]);
  const p: NotifyPrefs = desktop ? server ?? { enabled: false, sections: [] } : web;
  const disabled = desktop ? locked || !server : perm === "unsupported";

  const save = async (n: Partial<NotifyPrefs>) => {
    if (desktop) {
      try { const r = await api<{ notify: NotifyPrefs }>("/api/settings", { method: "PATCH", body: JSON.stringify({ notify: n }) }); onSaved(r.notify); }
      catch (e) { toast.error((e as Error).message); }
    } else setWeb(setWebPrefs(n));
  };
  const toggle = async (on: boolean) => {
    if (on && !desktop) {
      const r = await askPermission(); setPerm(r);
      if (r !== "granted") { toast.error(r === "denied" ? "浏览器拒绝了通知权限，请在地址栏的站点设置里允许" : "无法获得通知权限"); if (r === "denied") return; }
    }
    await save({ enabled: on });
    toast.success(on ? "已开启 P0 通知" : "已关闭 P0 通知");
  };
  const custom = p.sections.length > 0;
  const setSec = (k: string, on: boolean) => { const next = on ? [...new Set([...p.sections, k])] : p.sections.filter((x) => x !== k); void save({ sections: next }); };
  const test = async () => {
    setTesting(true);
    try {
      const r = await api<{ ok: boolean; status?: string; error?: string }>("/api/settings/notify-test", { method: "POST" });
      if (r.status) setNative(r.status);
      if (r.ok) toast.success(desktop ? "已发送测试通知，应在屏幕右上角看到横幅" : "已发送：切到其他标签页时会以系统通知显示");
      else toast.error(r.error ?? "发送失败");
    } catch (e) { toast.error((e as Error).message); }
    finally { setTesting(false); }
  };
  const openSys = async () => { try { await api("/api/desktop/notify-open-settings", { method: "POST" }); } catch (e) { toast.error((e as Error).message); } };

  return (
    <Card className="mt-4" data-testid="notify-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><BellRing className="size-4 text-faint" />通知</CardTitle>
        <CardDescription className="flex flex-wrap items-center gap-1.5">
          {desktop ? "新的 P0 动态以 macOS 横幅提醒（App 在前台时也会显示）；点击打开对应内容。" : "页面在后台时用浏览器通知提醒新的 P0 动态（需授权），在前台时显示为提示条。"}
          {desktop
            ? native && <Badge variant={NATIVE[native]?.v ?? "secondary"} data-testid="notify-perm">{NATIVE[native]?.label ?? native}</Badge>
            : <Badge variant={PERM[perm]?.v ?? "secondary"} data-testid="notify-perm">{PERM[perm]?.label ?? perm}</Badge>}
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        {desktop && native === "denied" && (
          <div className="flex flex-wrap items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 type-caption" data-testid="notify-denied">
            <span className="flex-1">系统设置中未允许 AI News Radar 发送通知，横幅不会出现。</span>
            <Button size="sm" variant="outline" onClick={openSys} data-testid="notify-open-sys"><ExternalLink />打开「系统设置 › 通知」</Button>
          </div>
        )}
        <label className="flex cursor-pointer items-start gap-3">
          <div className="flex-1">
            <div className="type-ui font-medium">新的 P0 动态通知</div>
            <div className="mt-0.5 type-caption text-muted-foreground">合并推送（约 20 秒一批、至少间隔 1 分钟），启动时的积压和 3 小时前的内容不会通知。</div>
          </div>
          <Switch aria-label="P0 通知" checked={p.enabled} disabled={disabled} onCheckedChange={(v) => void toggle(!!v)} />
        </label>
        {desktop && (
          <>
            <label className="flex cursor-pointer items-start gap-3" data-testid="notify-system">
              <div className="flex-1">
                <div className="type-ui font-medium">桌面系统通知</div>
                <div className="mt-0.5 type-caption text-muted-foreground">在 macOS 通知中心显示（每批一条汇总，点击打开 App 和对应内容）；关闭后只在 App 内提示。</div>
              </div>
              <Switch aria-label="桌面系统通知" checked={p.system !== false} disabled={disabled || !p.enabled} onCheckedChange={(v) => void save({ system: !!v })} />
            </label>
            <div className="grid gap-2">
              <Label>系统通知内容</Label>
              <RadioGroup value={p.scope ?? "p0"} disabled={disabled || !p.enabled || p.system === false} className="flex flex-wrap gap-4" onValueChange={(v) => void save({ scope: v as "p0" | "all" })}>
                <label className="flex items-center gap-2 type-body"><RadioGroupItem value="p0" aria-label="仅 P0 动态" />仅 P0 动态</label>
                <label className="flex items-center gap-2 type-body"><RadioGroupItem value="all" aria-label="全部新内容" />全部新内容（「新增 N 条」汇总）</label>
              </RadioGroup>
            </div>
          </>
        )}
        <div className="grid gap-2">
          <Label>分区</Label>
          <RadioGroup value={custom ? "custom" : "all"} disabled={disabled || !p.enabled} className="flex flex-wrap gap-4"
            onValueChange={(v) => void save({ sections: v === "all" ? [] : sections.slice(0, 1).map((s) => s.key) })}>
            <label className="flex items-center gap-2 type-body"><RadioGroupItem value="all" aria-label="全部分区" />全部分区</label>
            <label className="flex items-center gap-2 type-body"><RadioGroupItem value="custom" aria-label="仅所选分区" />仅所选分区</label>
          </RadioGroup>
          {custom && (
            <div className="grid grid-cols-2 gap-x-4 gap-y-2 pt-1 sm:grid-cols-3" role="group" aria-label="通知的分区">
              {sections.map((s) => (
                <label key={s.key} className="flex items-center gap-2 type-body">
                  <Checkbox checked={p.sections.includes(s.key)} disabled={disabled || !p.enabled} aria-label={`通知分区 ${s.zh}`} onCheckedChange={(v) => setSec(s.key, !!v)} />
                  <SectionName k={s.key} iconClassName="size-3.5 text-faint" className="gap-1.5" />
                </label>
              ))}
            </div>
          )}
        </div>
      </CardContent>
      <CardFooter className="flex flex-wrap justify-end gap-2">
        {desktop && native !== "denied" && <Button variant="ghost" className="mr-auto text-muted-foreground" onClick={openSys}><SlidersHorizontal />系统通知设置</Button>}
        <Button variant="outline" disabled={testing || (desktop ? native === "unsupported" : locked || !p.enabled)} onClick={test} data-testid="notify-test">{testing ? <Loader2 className="animate-spin" /> : <Send />}发送测试通知</Button>
      </CardFooter>
    </Card>
  );
}

export function GeneralCard() {
  const [hover, setHover] = useState(false);
  useEffect(() => { setHover(markOnHover()); }, []);
  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><SlidersHorizontal className="size-4 text-faint" />常规</CardTitle>
        <CardDescription>打开详情即标为已读；信息流顶部可「只看未读」或「全部已读」。</CardDescription>
      </CardHeader>
      <CardContent>
        <label className="flex cursor-pointer items-start gap-3">
          <div className="flex-1">
            <div className="type-ui font-medium">悬停预览时标为已读</div>
            <div className="mt-0.5 type-caption text-muted-foreground">鼠标停留在卡片上弹出预览时也算已读。</div>
          </div>
          <Switch aria-label="悬停预览时标为已读" checked={hover} onCheckedChange={(v) => { setHover(!!v); setMarkOnHover(!!v); }} />
        </label>
      </CardContent>
    </Card>
  );
}

export function UpdateCard({ version }: { version: string | null }) {
  return (
    <Card className="mt-4" data-testid="update-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Download className="size-4 text-faint" />更新</CardTitle>
        <CardDescription>
          当前 {version ? <span className="font-mono">v{version}</span> : null}。每 12 小时自动检查新版本（GitHub Releases，更新包经签名校验），也可在菜单
          <span className="font-medium text-foreground">「AI News Radar › 检查更新…」</span>或菜单栏图标中手动检查。
        </CardDescription>
      </CardHeader>
    </Card>
  );
}

