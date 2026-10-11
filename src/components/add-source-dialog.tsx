"use client";
import { useState } from "react";
import { AlertTriangle, CheckCircle2, Loader2, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { type SectionKey, TYPE, api, bj } from "./common";
import { EMPTY_NEW, EMPTY_RULE, NEW_SECTION, NewSectionFields, RuleFields, SectionSelect, type NewSection, type RuleForm } from "./source-form-parts";
import { ChevronRight, Filter } from "lucide-react";

// Add-source dialog (sources page), code-split: loaded the first time it opens.
type Detect = { type: string; name: string; url: string; count: number; overlap: number; sample: { title: string; published: number | null }[] };

export default function AddDialog({ open, onOpenChange, onAdded }: { open: boolean; onOpenChange: (b: boolean) => void; onAdded: () => void }) {
  const [form, setForm] = useState({ input: "", name: "", section: "media" as SectionKey, priority: "P1", force: false });
  const [secTouched, setSecTouched] = useState(false); // stop auto-suggesting once the user picks a section
  const suggest = (input: string, type?: string) => (/^@|x\.com\/|twitter\.com\//i.test(input.trim()) || type?.startsWith("x") ? "x" : /\.cn(\/|$)|[\u4e00-\u9fff]/.test(input) ? "cn" : "media") as SectionKey;
  const [res, setRes] = useState<Detect | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState<"" | "test" | "add">("");
  const [newSec, setNewSec] = useState<NewSection>(EMPTY_NEW);
  const [rule, setRule] = useState<RuleForm>(EMPTY_RULE);
  const [ruleOpen, setRuleOpen] = useState(false);
  const [ruleErr, setRuleErr] = useState("");
  const test = async () => {
    setBusy("test"); setErr(""); setRes(null);
    try { const r = await api<Detect>("/api/sources/test", { method: "POST", body: JSON.stringify(form) }); setRes(r); if (!secTouched) setForm((f) => ({ ...f, section: suggest(f.input, r.type) })); } catch (e) { setErr((e as Error).message); }
    setBusy("");
  };
  const add = async () => {
    setBusy("add"); setErr("");
    try {
      const isNew = form.section === NEW_SECTION;
      if (isNew && !newSec.name.trim()) throw new Error("请填写新分区名称");
      const d = await api("/api/sources", { method: "POST", body: JSON.stringify({ ...form, section: isNew ? undefined : form.section, newSection: isNew ? newSec : undefined, rule }) });
      toast.success(`已添加「${d.source.name}」`, { description: d.fetch.error ? `首次抓取失败：${d.fetch.error}` : `首次抓取 ${d.fetch.fetched} 条，新增 ${d.fetch.new} 条` });
      onAdded(); onOpenChange(false); setRes(null); setSecTouched(false); setForm({ ...form, input: "", name: "", force: false, section: isNew ? d.source.grp.replace(/^sec:/, "") : form.section });
      setNewSec(EMPTY_NEW); setRule(EMPTY_RULE); setRuleOpen(false);
    } catch (e) { setErr((e as Error).message); }
    setBusy("");
  };
  const sel = (k: "section" | "priority", opts: [string, string][]) => (
    <Select value={form[k]} onValueChange={(v) => { if (k === "section") setSecTouched(true); setForm({ ...form, [k]: v as never }); }}>
      <SelectTrigger className="w-full"><SelectValue>{(v: string) => opts.find((o) => o[0] === v)?.[1]}</SelectValue></SelectTrigger>
      <SelectContent>{opts.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
    </Select>
  );
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92dvh] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>添加数据源</DialogTitle>
          <DialogDescription>网页或 Feed URL（自动发现 RSS/Atom）、X 用户名 @handle，或 X 列表链接。</DialogDescription>
        </DialogHeader>
        <div className="grid min-w-0 gap-4">
          <div className="grid min-w-0 gap-1.5">
            <Label htmlFor="src-input">地址 / 账号</Label>
            <div className="flex min-w-0 gap-2">
              <Input id="src-input" className="min-w-0 flex-1" autoFocus placeholder="https://simonwillison.net 或 @karpathy" value={form.input}
                onChange={(e) => { const v = e.target.value; setForm({ ...form, input: v, section: secTouched ? form.section : suggest(v) }); setRes(null); }} onKeyDown={(e) => e.key === "Enter" && form.input && test()} />
              <Button variant="outline" className="shrink-0" onClick={test} disabled={!form.input || !!busy}>{busy === "test" ? <Loader2 className="animate-spin" /> : <RefreshCw />}测试</Button>
            </div>
          </div>
          <div className="grid min-w-0 grid-cols-2 gap-3 sm:grid-cols-[minmax(0,2fr)_minmax(0,1.3fr)_minmax(0,1fr)]">
            <div className="col-span-2 grid min-w-0 gap-1.5 sm:col-span-1"><Label>名称</Label><Input className="w-full" placeholder="可选" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></div>
            <div className="grid min-w-0 gap-1.5"><Label>分区</Label><SectionSelect value={form.section} onChange={(v) => { setSecTouched(true); setForm({ ...form, section: v }); }} /></div>
            <div className="grid min-w-0 gap-1.5"><Label>优先级</Label>{sel("priority", [["P0", "P0"], ["P1", "P1"], ["P2", "P2"]])}</div>
          </div>
          {form.section === NEW_SECTION && <NewSectionFields v={newSec} set={setNewSec} />}
          <div className="grid min-w-0 gap-3">
            <button type="button" onClick={() => setRuleOpen((o) => !o)} aria-expanded={ruleOpen}
              className="flex w-fit items-center gap-1.5 rounded-md type-body font-medium text-muted-foreground outline-none transition-colors duration-150 hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50">
              <ChevronRight className={cn("size-4 transition-transform duration-200", ruleOpen && "rotate-90")} /><Filter className="size-3.5" />过滤规则<span className="font-normal text-faint">（可选，按标题正则保留 / 隐藏）</span>
            </button>
            {ruleOpen && <RuleFields rule={rule} set={setRule} samples={res?.sample.map((x) => x.title)} onError={setRuleErr} />}
          </div>
          {err && <Alert variant="destructive" className="p-3"><AlertTriangle /><AlertDescription className="type-body">{err}</AlertDescription></Alert>}
          {res && (
            <div className="min-w-0 overflow-hidden rounded-lg border bg-muted/30">
              <div className="flex items-start gap-2 border-b p-3">
                <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-500" />
                <div className="min-w-0 flex-1 type-body">
                  <div className="flex min-w-0 items-center gap-1 font-medium"><span className="truncate">{res.name}</span><Badge variant="secondary" className="ml-1 shrink-0">{TYPE[res.type]}</Badge></div>
                  <div className="break-all text-muted-foreground">{res.url} · {res.count} 条</div>
                </div>
              </div>
              {res.overlap >= 0.5 && (
                <Alert variant="warning" className="rounded-none border-0 border-b p-3">
                  <AlertTriangle />
                  <AlertDescription className="type-body">疑似重复：{Math.round(res.overlap * 100)}% 的示例标题已被现有来源收录。
                    <Label className="mt-1.5 font-normal"><Checkbox checked={form.force} onCheckedChange={(v) => setForm({ ...form, force: !!v })} />仍然添加</Label>
                  </AlertDescription>
                </Alert>
              )}
              <ul className="max-h-48 min-w-0 space-y-1.5 overflow-y-auto overflow-x-hidden p-3 type-caption">
                {res.sample.map((s, i) => <li key={i} className="flex min-w-0 gap-2"><span className="shrink-0 font-mono text-muted-foreground">{bj(s.published)}</span><span className="min-w-0 flex-1 truncate" title={s.title}>{s.title}</span></li>)}
              </ul>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>取消</Button>
          <Button onClick={add} disabled={!form.input || !!busy || !!ruleErr}>{busy === "add" && <Loader2 className="animate-spin" />}添加并订阅</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
