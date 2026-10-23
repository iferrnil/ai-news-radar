// Opt-in black-flash probe for desktop MyGo / WKWebView digs.
// Enable: Settings › 常规 toggle, localStorage.radarFlashProbe="1", or ?flashProbe=1.
// After each location / settings-category change, samples ~500ms of rAF into window.__radarFlashLog.
"use client";

export type FlashNavEvent = { event: "nav"; from: string; to: string; at: number };
export type FlashSample = {
  t: number;
  path: string;
  asideConnected: boolean;
  headerConnected: boolean;
  fallbackSole: boolean;
  textLen: number;
  brandHint?: boolean;
};
export type FlashLogEntry = FlashNavEvent | FlashSample;

export const FLASH_PROBE_KEY = "radarFlashProbe";
export const FLASH_PROBE_EVENT = "radar-flash-log";
const BURST_MS = 500;

declare global {
  interface Window {
    __radarFlashLog?: FlashLogEntry[];
  }
}

function pathNow() {
  if (typeof window === "undefined") return "";
  return `${location.pathname}${location.search}`;
}

export function flashProbeEnabled(): boolean {
  if (typeof window === "undefined") return false;
  try {
    if (localStorage.getItem(FLASH_PROBE_KEY) === "1") return true;
    if (new URLSearchParams(location.search).get("flashProbe") === "1") {
      localStorage.setItem(FLASH_PROBE_KEY, "1");
      return true;
    }
  } catch { /* */ }
  return false;
}

function ensureLog() {
  if (typeof window === "undefined") return;
  if (!window.__radarFlashLog) window.__radarFlashLog = [];
}

function emit() {
  if (typeof window === "undefined") return;
  try { window.dispatchEvent(new CustomEvent(FLASH_PROBE_EVENT, { detail: { n: flashLogLength() } })); } catch { /* */ }
}

export function setFlashProbeEnabled(on: boolean) {
  try {
    if (on) localStorage.setItem(FLASH_PROBE_KEY, "1");
    else localStorage.removeItem(FLASH_PROBE_KEY);
  } catch { /* */ }
  if (on) armFlashProbe();
  else {
    // leave log in place so they can still copy; just stop new bursts
    lastLabel = pathNow();
  }
}

/** Seed lastLabel + log. Safe to call repeatedly (boot + toggle ON). */
export function armFlashProbe() {
  if (typeof window === "undefined") return;
  ensureLog();
  lastLabel = pathNow();
  // baseline sample so UI is not stuck at 0 before the first switch
  if (flashProbeEnabled()) {
    window.__radarFlashLog!.push({
      event: "nav",
      from: "(probe-on)",
      to: lastLabel || pathNow(),
      at: Math.round(performance.now()),
    });
    startBurst();
    emit();
  }
}

function sample(): FlashSample {
  const aside = document.querySelector(".app-sidebar");
  const header = document.querySelector(".app-header");
  const fallback = [...document.querySelectorAll('div.min-h-screen.bg-background[aria-hidden="true"]')]
    .find((el) => el.parentElement === document.body);
  const brand = document.querySelector(".app-brand");
  const brandHint = !!(brand && brand.getClientRects().length > 0 && brand.isConnected);
  return {
    t: Math.round(performance.now()),
    path: pathNow(),
    asideConnected: !!aside?.isConnected,
    headerConnected: !!header?.isConnected,
    fallbackSole: !!fallback,
    textLen: (document.body?.innerText || "").length,
    brandHint,
  };
}

let raf = 0;
let until = 0;
/** Last recorded label (URL or synthetic `/settings#cat`). */
let lastLabel = "";

/** Call once on Shell mount: honor ?flashProbe=1 and seed lastLabel. */
export function initFlashProbe() {
  if (typeof window === "undefined") return;
  flashProbeEnabled(); // may persist from query
  ensureLog();
  lastLabel = pathNow();
}

/**
 * Record a navigation (URL or synthetic label like `/settings#notify`).
 * No-op when probe off. Same-label calls are ignored.
 */
export function noteFlashNav(to?: string) {
  if (typeof window === "undefined" || !flashProbeEnabled()) return;
  ensureLog();
  const from = lastLabel || pathNow();
  const dest = to ?? pathNow();
  if (!dest || dest === from) return;
  const ev: FlashNavEvent = { event: "nav", from, to: dest, at: Math.round(performance.now()) };
  window.__radarFlashLog!.push(ev);
  console.log("[radar-flash]", JSON.stringify(ev));
  lastLabel = dest;
  startBurst();
  emit();
}

function startBurst() {
  until = performance.now() + BURST_MS;
  if (raf) return;
  const tick = () => {
    ensureLog();
    window.__radarFlashLog!.push(sample());
    emit();
    if (performance.now() < until) raf = requestAnimationFrame(tick);
    else {
      raf = 0;
      console.log("[radar-flash]", "burst-end", "entries=", window.__radarFlashLog!.length);
      emit();
    }
  };
  raf = requestAnimationFrame(tick);
}

export function flashLogLength() {
  return typeof window !== "undefined" ? (window.__radarFlashLog?.length ?? 0) : 0;
}

export async function copyFlashLog(): Promise<{ n: number; via: "clipboard" | "download" }> {
  ensureLog();
  const n = window.__radarFlashLog?.length ?? 0;
  const text = JSON.stringify(window.__radarFlashLog ?? [], null, 2);
  try {
    await navigator.clipboard.writeText(text);
    console.log("[radar-flash]", "copied", n, "entries");
    return { n, via: "clipboard" };
  } catch (e) {
    // MyGo / permission edge: fall back to a JSON download the user can attach
    const a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([text], { type: "application/json" }));
    a.download = `radar-flash-log-${Date.now()}.json`;
    a.rel = "noopener";
    document.body.appendChild(a);
    a.click();
    a.remove();
    console.log("[radar-flash]", "download", n, "entries", String(e));
    return { n, via: "download" };
  }
}

export function clearFlashLog() {
  if (typeof window === "undefined") return;
  window.__radarFlashLog = [];
  emit();
  console.log("[radar-flash]", "cleared");
}
