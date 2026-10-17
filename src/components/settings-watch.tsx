"use client";
// 设置 › 关键词: watch keywords matched server-side against every item (title, or title + content). Matching cards get a
// keyword tag; the feed's 关键词 chip / a tag click filters to them. No notifications (removed in 0.13.0).
// Editable when settings are locked, like the translation switches.
import { useState } from "react";
import { Plus, Tag, X } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { api } from "./common";

export type WatchPrefs = { keywords: string[]; scope: "all" | "title"; caseSensitive: boolean };

export default function WatchCard({ initial }: { initial: WatchPrefs | null }) {
  const [p, setP] = useState<WatchPrefs>(initial ?? { keywords: [], scope: "all", caseSensitive: false });
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
        <CardTitle className="flex items-center gap-2"><Tag className="size-4 text-faint" />关键词</CardTitle>
        <CardDescription>命中的内容会带上关键词标签，可在信息流中筛选</CardDescription>
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
        <div className="grid gap-2.5 sm:grid-cols-2">
          <Label className="gap-2 font-normal"><Switch size="sm" checked={p.scope === "title"} disabled={busy} onCheckedChange={(v) => void save({ scope: v ? "title" : "all" })} data-testid="watch-title" />只匹配标题</Label>
          <Label className="gap-2 font-normal"><Switch size="sm" checked={p.caseSensitive} disabled={busy} onCheckedChange={(v) => void save({ caseSensitive: !!v })} data-testid="watch-case" />区分大小写</Label>
        </div>
      </CardContent>
    </Card>
  );
}
