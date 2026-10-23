// Opt-in black-flash probe for desktop MyGo / WKWebView digs.
// Enable: localStorage.radarFlashProbe = "1"  or  ?flashProbe=1 (persists the flag).
// Samples ~500ms of rAF after each soft/hard client location change into window.__radarFlashLog.
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

export function setFlashProbeEnabled(on: boolean) {
  try {
    if (on) localStorage.setItem(FLASH_PROBE_KEY, "1");
    else localStorage.removeItem(FLASH_PROBE_KEY);
  } catch { /* */ }
}

function ensureLog() {
  if (typeof window === "undefined") return;
  if (!window.__radarFlashLog) window.__radarFlashLog = [];
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
let lastPath = "";

/** Call once on Shell mount: seed lastPath, honor ?flashProbe=1. */
export function initFlashProbe() {
  if (typeof window === "undefined") return;
  flashProbeEnabled(); // may persist from query
  lastPath = pathNow();
  ensureLog();
}

/** Start a nav event + ~500ms rAF sampling burst (no-op when probe off). */
export function noteFlashNav(to?: string) {
  if (typeof window === "undefined" || !flashProbeEnabled()) return;
  ensureLog();
  const from = lastPath || pathNow();
  const dest = to ?? pathNow();
  if (dest === from) return; // same location (effect + navigate double-fire, or no-op)
  const ev: FlashNavEvent = { event: "nav", from, to: dest, at: Math.round(performance.now()) };
  window.__radarFlashLog!.push(ev);
  console.log("[radar-flash]", JSON.stringify(ev));
  lastPath = dest;
  startBurst();
}

function startBurst() {
  until = performance.now() + BURST_MS;
  if (raf) return;
  const tick = () => {
    ensureLog();
    window.__radarFlashLog!.push(sample());
    if (performance.now() < until) raf = requestAnimationFrame(tick);
    else {
      raf = 0;
      console.log("[radar-flash]", "burst-end", "entries=", window.__radarFlashLog!.length);
    }
  };
  raf = requestAnimationFrame(tick);
}

export function flashLogLength() {
  return typeof window !== "undefined" ? (window.__radarFlashLog?.length ?? 0) : 0;
}

export async function copyFlashLog(): Promise<number> {
  ensureLog();
  const n = window.__radarFlashLog?.length ?? 0;
  const text = JSON.stringify(window.__radarFlashLog ?? [], null, 2);
  await navigator.clipboard.writeText(text);
  console.log("[radar-flash]", "copied", n, "entries");
  return n;
}

export function clearFlashLog() {
  if (typeof window === "undefined") return;
  window.__radarFlashLog = [];
  console.log("[radar-flash]", "cleared");
}
