"use client";
// 设置 › 通知 (P0) and 阅读. Web: prefs + permission live in this browser; desktop: prefs live on the server,
// which the shell reads before showing a native notification.
import { useEffect, useState } from "react";
import { BellRing, BookOpenCheck, Download, Send } from "lucide-react";
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

export function NotifyCard({ desktop, server, locked, onSaved }: { desktop: boolean; server: NotifyPrefs | null; locked: boolean; onSaved: (p: NotifyPrefs) => void }) {
  const { sections } = useApp();
  const [web, setWeb] = useState<WebNotifyPrefs>({ enabled: false, sections: [] });
  const [perm, setPerm] = useState<string>("default");
  useEffect(() => { setWeb(webPrefs()); setPerm(webNotifySupported() ? Notification.permission : "unsupported"); }, []);
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
    try { await api("/api/settings/notify-test", { method: "POST" }); toast.success(desktop ? "已发送测试通知" : "已发送：切到其他标签页时会以系统通知显示"); }
    catch (e) { toast.error((e as Error).message); }
  };

  return (
    <Card className="mt-4" data-testid="notify-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><BellRing className="size-4 text-faint" />P0 通知</CardTitle>
        <CardDescription className="flex flex-wrap items-center gap-1.5">
          {desktop ? "新的 P0 动态以系统通知提醒；点击通知打开对应内容。" : "页面在后台时用浏览器通知提醒新的 P0 动态（需授权），在前台时显示为提示条。"}
          {!desktop && <Badge variant={PERM[perm]?.v ?? "secondary"} data-testid="notify-perm">{PERM[perm]?.label ?? perm}</Badge>}
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
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
      <CardFooter className="justify-end">
        <Button variant="outline" disabled={locked || !p.enabled} onClick={test}><Send />发送测试通知</Button>
      </CardFooter>
    </Card>
  );
}

export function ReadingCard() {
  const [hover, setHover] = useState(false);
  useEffect(() => { setHover(markOnHover()); }, []);
  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><BookOpenCheck className="size-4 text-faint" />阅读</CardTitle>
        <CardDescription>打开详情即标为已读；未读条目带紫色圆点，信息流顶部可「只看未读」或「全部已读」。已读记录保存在本浏览器（桌面版同时保存在本机数据库）。</CardDescription>
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

export function DesktopCard({ version }: { version: string | null }) {
  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Download className="size-4 text-faint" />应用更新</CardTitle>
        <CardDescription>
          桌面版 {version ? <span className="font-mono">v{version}</span> : null} 每天自动检查一次新版本（GitHub Releases，更新包经签名校验），也可在菜单
          <span className="font-medium text-foreground">「AI News Radar › 检查更新…」</span>或菜单栏图标中手动检查。
        </CardDescription>
      </CardHeader>
    </Card>
  );
}

