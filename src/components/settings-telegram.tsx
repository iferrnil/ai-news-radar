"use client";
// 设置 › Telegram 定时简报（桌面版）：Bot Token / Chat ID 存在 secrets；定时在本机 Asia/Shanghai 触发，需 App 在运行。
import { useEffect, useState } from "react";
import { Eye, EyeOff, Loader2, Send, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { useApp } from "./app-shell";
import { api } from "./common";

export type TelegramPrefs = {
  enabled: boolean;
  chatId: string;
  schedule: string;
  section: string;
  hasToken: boolean;
  desktop?: boolean;
  lastAt?: number;
  lastError?: string;
  lastOk?: boolean;
};

export function TelegramCard({
  initial, locked, onSaved,
}: {
  initial: TelegramPrefs | null;
  locked: boolean;
  onSaved: (t: TelegramPrefs) => void;
}) {
  const { sections } = useApp();
  const [p, setP] = useState<TelegramPrefs>(initial ?? { enabled: false, chatId: "", schedule: "08:00", section: "all", hasToken: false });
  const [token, setToken] = useState("");
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  useEffect(() => {
    if (initial) setP(initial);
  }, [initial]);

  const patch = async (body: Record<string, unknown>, okMsg?: string) => {
    setBusy("save");
    try {
      const r = await api<{ telegram: TelegramPrefs }>("/api/settings", { method: "PATCH", body: JSON.stringify({ telegram: body }) });
      onSaved(r.telegram);
      setP(r.telegram);
      if (okMsg) toast.success(okMsg);
      return true;
    } catch (e) {
      toast.error((e as Error).message);
      return false;
    } finally {
      setBusy(null);
    }
  };

  const save = async () => {
    const body: Record<string, unknown> = {
      enabled: p.enabled,
      chatId: p.chatId.trim(),
      schedule: p.schedule.trim() || "08:00",
      section: p.section || "all",
    };
    if (token.trim()) body.token = token.trim();
    if (await patch(body, "已保存 Telegram 设置")) {
      setToken("");
      setShow(false);
    }
  };

  const clearToken = async () => {
    if (await patch({ clearToken: true }, "已清除 Bot Token")) setToken("");
  };

  const test = async () => {
    setBusy("test");
    try {
      const r = await api<{ ok: boolean; error?: string; telegram?: TelegramPrefs }>("/api/settings/telegram-test", { method: "POST" });
      if (r.telegram) { onSaved(r.telegram); setP(r.telegram); }
      if (r.ok) toast.success("已发送测试简报到 Telegram");
      else toast.error(r.error ?? "发送失败");
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(null);
    }
  };

  return (
    <Card className="mt-4" data-testid="telegram-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Send className="size-4 text-faint" />Telegram 定时简报
        </CardTitle>
        <CardDescription>
          按北京时间每天发送当前分区的自动简报。需要桌面版保持运行；密钥只保存在本机。
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <Alert>
          <AlertTitle>如何获取 Bot Token / Chat ID</AlertTitle>
          <AlertDescription className="type-caption text-muted-foreground">
            在 Telegram 打开 @BotFather → /newbot 创建机器人，复制 Token。Chat ID：私聊可用 @userinfobot 查看数字 ID；频道 / 群组填 <span className="font-mono">@频道用户名</span> 或负数 ID，并先把机器人加进去。
          </AlertDescription>
        </Alert>
        <label className="flex cursor-pointer items-start gap-3">
          <div className="flex-1">
            <div className="type-ui font-medium">启用定时发送</div>
            <p className="type-meta text-faint">到点自动生成并推送；关闭即停。</p>
          </div>
          <Switch
            aria-label="启用 Telegram 定时简报"
            checked={p.enabled}
            disabled={locked || !!busy}
            onCheckedChange={(v) => { setP((c) => ({ ...c, enabled: !!v })); void patch({ enabled: !!v }, v ? "已开启" : "已关闭"); }}
          />
        </label>
        <div className="grid gap-2">
          <Label htmlFor="tg-token">Bot Token</Label>
          <div className="flex gap-2">
            <Input
              id="tg-token"
              type={show ? "text" : "password"}
              autoComplete="off"
              disabled={locked}
              placeholder={p.hasToken ? "已配置（输入新值可覆盖）" : "123456:AA…"}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              data-testid="tg-token"
            />
            <Button type="button" variant="outline" size="icon" aria-label={show ? "隐藏" : "显示"} onClick={() => setShow((s) => !s)}>
              {show ? <EyeOff /> : <Eye />}
            </Button>
          </div>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="tg-chat">Chat ID</Label>
          <Input
            id="tg-chat"
            disabled={locked}
            placeholder="@channel 或 -100…"
            value={p.chatId}
            onChange={(e) => setP((c) => ({ ...c, chatId: e.target.value }))}
            data-testid="tg-chat"
          />
        </div>
        <div className="grid min-w-0 gap-2 sm:grid-cols-2 sm:gap-4">
          <div className="grid gap-2">
            <Label htmlFor="tg-sched">发送时间（北京时间）</Label>
            <Input
              id="tg-sched"
              disabled={locked}
              placeholder="08:00"
              value={p.schedule}
              onChange={(e) => setP((c) => ({ ...c, schedule: e.target.value }))}
              data-testid="tg-sched"
            />
          </div>
          <div className="grid min-w-0 gap-2">
            <Label>简报分区</Label>
            {/* alignItemWithTrigger=false + modal=false: popup overlays via portal without resizing the
                trigger / locking page scroll (otherwise the settings page jumps upward). */}
            <Select
              value={p.section || "all"}
              onValueChange={(v) => setP((c) => ({ ...c, section: (v as string) || "all" }))}
              disabled={locked}
              modal={false}
            >
              <SelectTrigger className="w-full max-w-full" data-testid="tg-section">
                <SelectValue />
              </SelectTrigger>
              <SelectContent
                alignItemWithTrigger={false}
                align="start"
                side="bottom"
                sideOffset={6}
                className="max-h-72"
              >
                <SelectItem value="all">全部</SelectItem>
                {sections.map((s) => (
                  <SelectItem key={s.key} value={s.key}>{s.zh}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        {p.lastAt ? (
          <p className={`type-meta ${p.lastError ? "text-red-500" : "text-faint"}`} data-testid="tg-last">
            上次 {new Date(p.lastAt * 1000).toLocaleString("zh-CN", { hour: "2-digit", minute: "2-digit", month: "numeric", day: "numeric" })}
            {" · "}
            {p.lastError || (p.lastOk ? "已发送" : "")}
          </p>
        ) : null}
      </CardContent>
      <CardFooter className="flex flex-wrap justify-end gap-2">
        {p.hasToken && (
          <Button variant="ghost" className="mr-auto text-muted-foreground hover:text-destructive" disabled={locked || !!busy} onClick={() => void clearToken()}>
            {busy === "save" ? <Loader2 className="animate-spin" /> : <Trash2 />}清除 Token
          </Button>
        )}
        <Button variant="outline" disabled={locked || !!busy || (!p.hasToken && !token.trim()) || !p.chatId.trim()} onClick={() => void test()} data-testid="tg-test">
          {busy === "test" ? <Loader2 className="animate-spin" /> : <Send />}发送测试
        </Button>
        <Button disabled={locked || !!busy} onClick={() => void save()} data-testid="tg-save">
          {busy === "save" ? <Loader2 className="animate-spin" /> : null}保存
        </Button>
      </CardFooter>
    </Card>
  );
}
