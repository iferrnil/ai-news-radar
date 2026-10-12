// Web fallback for P0 notifications (opt-in): `notify` batches from SSE become a system Notification when the tab
// is in the background, or a toast when it is in front. The desktop app shows native notifications instead.
import { toast } from "sonner";
import { isDesktop } from "./read-store";

export interface NotifyItem { id: number; title: string; source: string; section: string | null; url: string; published: number }
export interface NotifyBatch { at: number; count: number; items: NotifyItem[]; sections: string[] }
export interface WebNotifyPrefs { enabled: boolean; sections: string[] } // sections: [] = all

const KEY = "notify.web";
export const webNotifySupported = () => typeof window !== "undefined" && "Notification" in window;
export function webPrefs(): WebNotifyPrefs {
  try { const p = JSON.parse(localStorage.getItem(KEY) ?? "") as WebNotifyPrefs; return { enabled: !!p.enabled, sections: Array.isArray(p.sections) ? p.sections : [] }; }
  catch { return { enabled: false, sections: [] }; }
}
export function setWebPrefs(p: Partial<WebNotifyPrefs>) {
  const n = { ...webPrefs(), ...p };
  try { localStorage.setItem(KEY, JSON.stringify(n)); } catch {}
  return n;
}
export async function askPermission(): Promise<NotificationPermission | "unsupported"> {
  if (!webNotifySupported()) return "unsupported";
  if (Notification.permission !== "default") return Notification.permission;
  return Notification.requestPermission();
}

export function filterBatch(b: NotifyBatch, sections: string[]): NotifyBatch | null {
  if (!sections.length) return b;
  const items = b.items.filter((i) => sections.includes(i.section ?? ""));
  if (!items.length) return null;
  return { ...b, items, count: items.length < b.items.length ? items.length : b.count };
}
export function batchText(b: NotifyBatch) {
  const one = b.count === 1 && b.items[0];
  return {
    title: one ? `P0 · ${one.source}` : `${b.count} 条新 P0 动态`,
    body: one ? one.title : b.items.slice(0, 3).map((i) => `· ${i.source}：${i.title}`).join("\n") + (b.count > 3 ? `\n…还有 ${b.count - 3} 条` : ""),
  };
}

/** Handle one batch in the page. `open(id)` shows the item. */
export function handleBatch(b: NotifyBatch, open: (id: number) => void) {
  if (isDesktop()) return; // native notifications from the shell
  const p = webPrefs();
  if (!p.enabled) return;
  const f = filterBatch(b, p.sections);
  if (!f) return;
  const { title, body } = batchText(f);
  const first = f.items[0]?.id;
  const front = document.visibilityState === "visible" && document.hasFocus();
  if (!front && webNotifySupported() && Notification.permission === "granted") {
    // one tag: a newer batch replaces the old one instead of stacking up
    const n = new Notification(title, { body, tag: "ai-radar-p0", icon: "/favicon.ico" });
    n.onclick = () => { window.focus(); if (first) open(first); n.close(); };
    return;
  }
  toast(title, { description: body, duration: 8000, action: first ? { label: "查看", onClick: () => open(first) } : undefined });
}
