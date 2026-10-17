"use client";
// 设置 › 热点词表: extra names for 热点追踪 grouping, one per line with aliases joined by 「=」 (「元宝=Yuanbao=腾讯元宝」).
// Built-in: common Chinese AI companies / products and their English names (server/radar/hotlex.go). Hidden when
// settings are locked — the lexicon changes every visitor's topics.
import { useState } from "react";
import { Flame } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { api } from "./common";

export default function HotLexCard({ initial }: { initial: string }) {
  const [saved, setSaved] = useState(initial);
  const [text, setText] = useState(initial);
  const [busy, setBusy] = useState(false);
  const save = async () => {
    setBusy(true);
    try {
      const r = await api<{ hotLexicon: string }>("/api/settings", { method: "PATCH", body: JSON.stringify({ hotLexicon: text }) });
      setSaved(r.hotLexicon); setText(r.hotLexicon); toast.success("已保存热点词表");
    } catch (e) { toast.error((e as Error).message); }
    finally { setBusy(false); }
  };
  return (
    <Card className="mt-4" data-testid="hotlex-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Flame className="size-4 text-faint" />热点词表</CardTitle>
        <CardDescription>补充产品 / 公司名，每行一个，别名用 = 连接；已内置常见中文 AI 名称</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <Textarea value={text} onChange={(e) => setText(e.target.value)} rows={3} spellCheck={false} placeholder={"元宝=Yuanbao=腾讯元宝\n即梦=Jimeng"} aria-label="热点词表" data-testid="hotlex-input" className="font-mono type-caption" />
        <div className="flex justify-end">
          <Button variant="outline" size="sm" onClick={() => void save()} disabled={busy || text === saved} data-testid="hotlex-save">保存</Button>
        </div>
      </CardContent>
    </Card>
  );
}
