"use client";
// Sources page: edit one source (section + filter rule) and manage user-created sections. Code-split, loaded on first use.
import { useEffect, useState } from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { type Section, type SourceRow, api, customSections, sourceSection } from "./common";
import { EMPTY_NEW, NEW_SECTION, NewSectionFields, RuleFields, SectionSelect, ruleOf, type NewSection, type RuleForm } from "./source-form-parts";

export function EditSourceDialog({ src, onClose, onSaved }: { src: SourceRow | null; onClose: () => void; onSaved: () => void }) {
  const [section, setSection] = useState("media");
  const [newSec, setNewSec] = useState<NewSection>(EMPTY_NEW);
  const [rule, setRule] = useState<RuleForm>(ruleOf({}));
  const [ruleErr, setRuleErr] = useState(""); const [err, setErr] = useState(""); const [busy, setBusy] = useState(false);
  useEffect(() => { if (src) { setSection(sourceSection(src)); setRule(ruleOf(src)); setNewSec(EMPTY_NEW); setErr(""); } }, [src]);
  const save = async () => {
    if (!src) return;
    const isNew = section === NEW_SECTION;
    if (isNew && !newSec.name.trim()) { setErr("请填写新分区名称"); return; }
    setBusy(true); setErr("");
    try {
      const r = await api<{ hidden?: number }>(`/api/sources/${src.id}`, { method: "PATCH", body: JSON.stringify({ section: isNew ? undefined : section, newSection: isNew ? newSec : undefined, rule }) });
      toast.success(`已保存「${src.name}」`, { description: r.hidden ? `规则隐藏了 ${r.hidden} 条已抓取内容` : undefined });
      onSaved(); onClose();
    } catch (e) { setErr((e as Error).message); }
    setBusy(false);
  };
  return (
    <Dialog open={!!src} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[92dvh] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-2xl">
        <DialogHeader><DialogTitle className="truncate pr-6">编辑「{src?.name}」</DialogTitle><DialogDescription>分区与过滤规则。规则修改后立即对已抓取的内容生效（隐藏 / 恢复），之后的抓取不再保存被过滤的条目。</DialogDescription></DialogHeader>
        <div className="grid min-w-0 gap-4">
          <div className="grid min-w-0 gap-1.5 sm:max-w-xs"><Label>分区</Label><SectionSelect value={section} onChange={setSection} /></div>
          {section === NEW_SECTION && <NewSectionFields v={newSec} set={setNewSec} />}
          {src && <RuleFields rule={rule} set={setRule} sourceId={src.id} onError={setRuleErr} />}
          {err && <Alert variant="destructive" className="p-3"><AlertDescription className="type-body">{err}</AlertDescription></Alert>}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button onClick={save} disabled={busy || !!ruleErr}>{busy && <Loader2 className="animate-spin" />}保存</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function SectionRow({ s, count, onChanged, onDelete }: { s: Section; count: number; onChanged: () => void; onDelete: () => void }) {
  const [name, setName] = useState(s.zh); const [icon, setIcon] = useState(s.icon ?? "");
  useEffect(() => { setName(s.zh); setIcon(s.icon ?? ""); }, [s.zh, s.icon]);
  const patch = async (b: Record<string, unknown>) => {
    try { await api(`/api/sections/${s.key}`, { method: "PATCH", body: JSON.stringify(b) }); onChanged(); }
    catch (e) { toast.error((e as Error).message); setName(s.zh); setIcon(s.icon ?? ""); }
  };
  return (
    <li className="flex min-w-0 items-center gap-2 px-3 py-2" data-testid={`section-${s.key}`}>
      <Input aria-label="图标" maxLength={4} placeholder="📁" className="w-11 shrink-0 px-1 text-center" value={icon} onChange={(e) => setIcon(e.target.value)} onBlur={() => icon !== (s.icon ?? "") && patch({ icon })} />
      <Input aria-label="分区名称" maxLength={24} className="min-w-0 flex-1" value={name} onChange={(e) => setName(e.target.value)}
        onBlur={() => name.trim() && name.trim() !== s.zh && patch({ name })} onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()} />
      <span className="w-12 shrink-0 text-right font-mono type-meta tabular-nums text-faint" title="来源数">{count} 源</span>
      <Label className="shrink-0 font-normal" title="显示在「全部」"><Switch size="sm" aria-label="显示在全部" checked={s.includeInAll !== false} onCheckedChange={(b) => patch({ inAll: b })} /><span className="hidden sm:inline">全部</span></Label>
      <Button variant="ghost" size="icon-sm" aria-label={`删除分区「${s.zh}」`} onClick={onDelete} className="text-muted-foreground hover:text-destructive"><Trash2 /></Button>
    </li>
  );
}

/** Manage user-created sections: create, rename, icon, 全部 toggle, delete (empty, or move its sources to another section first). */
export function SectionsDialog({ open, onOpenChange, sources, onChanged }: { open: boolean; onOpenChange: (b: boolean) => void; sources: SourceRow[]; onChanged: () => void }) {
  const list = customSections();
  const countOf = (k: string) => sources.filter((s) => sourceSection(s) === k).length;
  const [adding, setAdding] = useState<NewSection>(EMPTY_NEW);
  const [del, setDel] = useState<Section | null>(null); const [moveTo, setMoveTo] = useState("media");
  const create = async () => {
    try { await api("/api/sections", { method: "POST", body: JSON.stringify(adding) }); setAdding(EMPTY_NEW); onChanged(); toast.success(`已创建分区「${adding.name.trim()}」`); }
    catch (e) { toast.error((e as Error).message); }
  };
  const doDelete = async () => {
    if (!del) return; const n = countOf(del.key);
    try { await api(`/api/sections/${del.key}${n ? `?moveTo=${encodeURIComponent(moveTo)}` : ""}`, { method: "DELETE" }); toast.success(`已删除分区「${del.zh}」`, { description: n ? `${n} 个来源已移入其他分区` : undefined }); onChanged(); }
    catch (e) { toast.error((e as Error).message); }
    setDel(null);
  };
  const delCount = del ? countOf(del.key) : 0;
  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-h-[92dvh] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-lg">
          <DialogHeader><DialogTitle>管理分区</DialogTitle><DialogDescription>自建分区会出现在侧栏；内置分区（实验室、国内媒体…）不可修改。</DialogDescription></DialogHeader>
          <ul className="min-w-0 divide-y divide-border/60 rounded-lg border">
            {list.map((s) => <SectionRow key={s.key} s={s} count={countOf(s.key)} onChanged={onChanged} onDelete={() => { setMoveTo("media"); setDel(s); }} />)}
            {!list.length && <li className="px-3 py-6 text-center type-body text-faint">还没有自建分区</li>}
          </ul>
          <div className="grid min-w-0 gap-2">
            <NewSectionFields v={adding} set={setAdding} />
            <Button variant="outline" className="justify-self-end" disabled={!adding.name.trim()} onClick={create}><Plus />新建分区</Button>
          </div>
        </DialogContent>
      </Dialog>
      <AlertDialog open={!!del} onOpenChange={(o) => !o && setDel(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除分区「{del?.zh}」？</AlertDialogTitle>
            <AlertDialogDescription>{delCount ? `该分区还有 ${delCount} 个来源，删除前会移入下面选择的分区；来源和内容不会被删除。` : "该分区没有来源，删除后侧栏不再显示。"}</AlertDialogDescription>
          </AlertDialogHeader>
          {delCount > 0 && <div className="grid gap-1.5"><Label>移入分区</Label><SectionSelect value={moveTo} onChange={setMoveTo} allowNew={false} exclude={del?.key} /></div>}
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={doDelete}><Trash2 />删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
