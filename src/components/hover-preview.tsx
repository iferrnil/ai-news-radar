"use client";
// One hover preview for the whole feed (not one popover per card): a single open timer, so at most one preview exists;
// it closes when the pointer leaves, when another card's preview opens, and on any scroll / wheel (incl. momentum),
// and nothing opens while scrolling or within 250 ms after it.
import { useEffect, useSyncExternalStore } from "react";
import type { ItemOut } from "@/lib/types";
import { ItemPreview } from "./item-preview";

const OPEN_MS = 400, CLOSE_MS = 150, QUIET_MS = 250, W = 420;
type State = { it: ItemOut; rect: DOMRect } | null;
let state: State = null; let openT: ReturnType<typeof setTimeout> | undefined; let closeT: typeof openT;
let pendingId = 0; let lastScroll = 0; let onOpened: ((it: ItemOut) => void) | null = null;
const subs = new Set<() => void>();
const set = (s: State) => { if (s === state) return; state = s; subs.forEach((f) => f()); };
const scrolling = () => performance.now() - lastScroll < QUIET_MS;

export const hoverPreview = {
  /** pointer entered / moved on a card */
  enter(it: ItemOut, el: HTMLElement) {
    clearTimeout(closeT);
    if (state?.it.id === it.id || pendingId === it.id || scrolling()) return;
    clearTimeout(openT); pendingId = it.id;
    if (state) set(null); // another card's preview: close it now
    openT = setTimeout(() => {
      pendingId = 0;
      if (scrolling() || !el.isConnected || !el.matches(":hover")) return;
      set({ it, rect: el.getBoundingClientRect() }); onOpened?.(it);
    }, OPEN_MS);
  },
  leave() { clearTimeout(openT); pendingId = 0; clearTimeout(closeT); closeT = setTimeout(() => set(null), CLOSE_MS); },
  close() { clearTimeout(openT); clearTimeout(closeT); pendingId = 0; set(null); },
};

export function HoverPreviewLayer({ onOpen }: { onOpen: (it: ItemOut) => void }) {
  const s = useSyncExternalStore((f) => { subs.add(f); return () => { subs.delete(f); }; }, () => state, () => null);
  useEffect(() => { onOpened = onOpen; return () => { onOpened = null; }; }, [onOpen]);
  useEffect(() => {
    const stop = () => { lastScroll = performance.now(); if (state || pendingId) hoverPreview.close(); };
    const o = { capture: true, passive: true } as const;
    window.addEventListener("scroll", stop, o); window.addEventListener("wheel", stop, o); window.addEventListener("touchmove", stop, o);
    const blur = () => hoverPreview.close(); window.addEventListener("blur", blur);
    return () => { window.removeEventListener("scroll", stop, o); window.removeEventListener("wheel", stop, o); window.removeEventListener("touchmove", stop, o); window.removeEventListener("blur", blur); hoverPreview.close(); };
  }, []);
  if (!s) return null;
  const vw = window.innerWidth, vh = window.innerHeight, w = Math.min(W, vw - 24);
  const right = s.rect.right + 8 + w <= vw - 8;
  const left = right ? s.rect.right + 8 : Math.max(8, s.rect.left - 8 - w);
  const top = Math.max(64, Math.min(s.rect.top, vh - 360));
  return (
    <div data-testid="hover-preview" role="tooltip" style={{ left, top, width: w }}
      onPointerEnter={() => clearTimeout(closeT)} onPointerLeave={() => hoverPreview.leave()}
      className="animate-in fade-in-0 zoom-in-95 fixed z-50 max-h-[min(520px,calc(100dvh-80px))] overflow-hidden rounded-xl bg-popover p-0 text-popover-foreground shadow-pop ring-1 ring-border duration-100">
      <ItemPreview it={s.it} />
    </div>
  );
}
