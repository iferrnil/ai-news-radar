"use client";
// 设置 › 关键词提醒: watch keywords matched server-side against every new item (title, or title + content). Matches
// are highlighted in the feed; with 通知 on they also raise a notification (native on desktop; on the web the opt-in
// web notification / toast). Editable when settings are locked, like the translation switches.
import { useState } from "react";
import { BellRing, Plus, X } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { api } from "./common";

export type WatchPrefs = { keywords: string[]; scope: "all" | "title"; caseSensitive: boolean; notify: boolean };

export default function WatchCard({ initial, desktop }: { initial: WatchPrefs | null; desktop: boolean }) {
  const [p, setP] = useState<WatchPrefs>(initial ?? { keywords: [], scope: "all", caseSensitive: false, notify: true });
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const save = async (next: Partial<WatchPrefs>) => {
    setBusy(true);
    try { const r = await api<{ watch: WatchPrefs }>("/api/settings", { method: "PATCH", body: JSON.stringify({ watch: next }) }); setP(r.watch); return true; }
    catch (e) { toast.error((e as Error).message); return false; }
    finally { setBusy(false); }
  };
  const add = async () => {
    const ks = text.split(/[,，、\n]/).map((k) => k.trim()).filter(Boolean);
    if (!ks.length) return;
    if (await save({ keywords: [...p.keywords, ...ks] })) setText("");
  };
  return (
    <Card className="mt-4" data-testid="watch-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><BellRing className="size-4 text-faint" />关键词提醒</CardTitle>
        <CardDescription>新内容命中关键词时高亮{p.notify ? (desktop ? "并发送系统通知" : "并提醒（需开启网页通知）") : ""}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <div className="flex gap-2">
          <Input value={text} onChange={(e) => setText(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter" && !e.nativeEvent.isComposing) { e.preventDefault(); void add(); } }}
            placeholder="GPT-6, DeepSeek, Claude" aria-label="关键词" data-testid="watch-input" maxLength={200} />
          <Button variant="outline" onClick={() => void add()} disabled={busy || !text.trim()} data-testid="watch-add"><Plus />添加</Button>
        </div>
        {p.keywords.length > 0 && (
          <div className="flex flex-wrap gap-1.5" data-testid="watch-list">
            {p.keywords.map((k) => (
              <Badge key={k} variant="outline" className="h-6 gap-1 pr-1 border-amber-500/40 text-amber-700 dark:text-amber-300">
                {k}
                <button type="button" aria-label={`删除 ${k}`} disabled={busy} onClick={() => void save({ keywords: p.keywords.filter((x) => x !== k) })} className="grid size-4 place-items-center rounded-sm hover:bg-foreground/10"><X className="size-3" /></button>
              </Badge>
            ))}
          </div>
        )}
        <div className="grid gap-2.5 sm:grid-cols-3">
          <Label className="gap-2 font-normal"><Switch size="sm" checked={p.scope === "title"} disabled={busy} onCheckedChange={(v) => void save({ scope: v ? "title" : "all" })} data-testid="watch-title" />只匹配标题</Label>
          <Label className="gap-2 font-normal"><Switch size="sm" checked={p.caseSensitive} disabled={busy} onCheckedChange={(v) => void save({ caseSensitive: !!v })} data-testid="watch-case" />区分大小写</Label>
          <Label className="gap-2 font-normal"><Switch size="sm" checked={p.notify} disabled={busy} onCheckedChange={(v) => void save({ notify: !!v })} data-testid="watch-notify" />通知</Label>
        </div>
      </CardContent>
    </Card>
  );
}
