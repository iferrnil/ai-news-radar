"use client";
import { RotateCcw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { type SourceRow, ago } from "./common";

// Sources-page confirm/restore dialogs, code-split: loaded on first use.
export function DeleteConfirm({ toDel, onCancel, onConfirm }: { toDel: SourceRow[] | null; onCancel: () => void; onConfirm: () => void }) {
  return (
      <AlertDialog open={!!toDel} onOpenChange={(o) => !o && onCancel()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{toDel?.length === 1 ? `删除「${toDel[0].name}」？` : `删除 ${toDel?.length ?? 0} 个来源？`}</AlertDialogTitle>
            <AlertDialogDescription>
              将移除来源本身以及它已保存的全部内容（共 {toDel?.reduce((a, x) => a + x.item_count, 0) ?? 0} 条），此操作不能撤销。
              {toDel?.some((x) => x.origin !== "user") ? " 内置源删除后不会因更新代码或重启而回来，可随时在「已删除的内置源」中恢复。" : ""}
              {" "}只想暂停抓取？用左侧开关停用即可。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={onConfirm}><Trash2 />删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
  );
}

export function RestoreDialog({ open, onOpenChange, deleted, restore }: { open: boolean; onOpenChange: (b: boolean) => void; deleted: { name: string; ts: number }[]; restore: (names: string[] | "all") => void }) {
  return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="grid-cols-[minmax(0,1fr)] sm:max-w-md">
          <DialogHeader><DialogTitle>已删除的内置源</DialogTitle><DialogDescription>恢复后按 sources.json 的默认设置重新加入并开始抓取。</DialogDescription></DialogHeader>
          <ul className="max-h-72 min-w-0 divide-y divide-border/60 overflow-y-auto rounded-lg border">
            {deleted.map((d) => (
              <li key={d.name} className="flex min-w-0 items-center gap-2 px-3 py-2 type-body">
                <span className="min-w-0 flex-1 truncate">{d.name}</span>
                <span className="shrink-0 font-mono type-meta text-faint">{ago(d.ts, true)}</span>
                <Button size="sm" variant="ghost" onClick={() => restore([d.name])}><RotateCcw />恢复</Button>
              </li>
            ))}
            {deleted.length === 0 && <li className="px-3 py-6 text-center type-body text-faint">没有已删除的内置源</li>}
          </ul>
          <DialogFooter><Button variant="outline" disabled={!deleted.length} onClick={() => restore("all")}>全部恢复</Button></DialogFooter>
        </DialogContent>
      </Dialog>
  );
}
