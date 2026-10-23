/** Soft in-app URL without Next `useSearchParams` / `router.push`.
 *
 *  Next App Router + `output: "export"` falls back to `location.assign` (MPA) when soft
 *  Flight nav fails — common in WKWebView. That fires MyGo `did-navigate` and blanks to
 *  underPage `#0e0e11`. We drive filters + routes via the native History API instead.
 *
 *  Important: call the *native* push/replace (captured before Next patches history). Next's
 *  wrapper treats external pushState as `restore()` which can again MPA on path changes.
 */
"use client";

import { useMemo, useSyncExternalStore } from "react";

type Listener = () => void;
const listeners = new Set<Listener>();
let patched = false;

/** Native History methods — set once before any framework wraps them. */
let nativePush: History["pushState"] | null = null;
let nativeReplace: History["replaceState"] | null = null;

function emit() {
  for (const l of listeners) l();
}

function ensureHistoryPatch() {
  if (patched || typeof window === "undefined") return;
  patched = true;
  nativePush = History.prototype.pushState.bind(history);
  nativeReplace = History.prototype.replaceState.bind(history);
  for (const method of ["pushState", "replaceState"] as const) {
    const orig = history[method].bind(history);
    history[method] = function (this: History, ...args: Parameters<History["pushState"]>) {
      const ret = orig(...args);
      queueMicrotask(emit);
      return ret;
    };
  }
  window.addEventListener("popstate", emit);
}

function subscribe(onStoreChange: Listener) {
  ensureHistoryPatch();
  listeners.add(onStoreChange);
  return () => { listeners.delete(onStoreChange); };
}

function getSearch() {
  return typeof window !== "undefined" ? window.location.search : "";
}

function getPathname() {
  return typeof window !== "undefined" ? window.location.pathname : "/";
}

/** Current `location.search` (including leading `?`), updates on soft nav. */
export function useClientSearch(): string {
  return useSyncExternalStore(subscribe, getSearch, () => "");
}

/** Current pathname, updates on soft nav / popstate. */
export function useClientPathname(): string {
  return useSyncExternalStore(subscribe, getPathname, () => "/");
}

/** Parsed query — drop-in for the string form of `useSearchParams()`. */
export function useClientSearchParams(): URLSearchParams {
  const search = useClientSearch();
  return useMemo(() => new URLSearchParams(search.startsWith("?") ? search.slice(1) : search), [search]);
}

export type SoftNavOpts = { replace?: boolean; scroll?: boolean };

/**
 * Same-origin soft navigation: native History only (no document load, no Next MPA).
 * `scroll: false` skips scrolling to top (section switches / strip query).
 */
export function softNav(href: string, opts: SoftNavOpts = {}) {
  if (typeof window === "undefined") return;
  ensureHistoryPatch();
  const url = new URL(href, window.location.href);
  if (url.origin !== window.location.origin) {
    window.location.assign(url.href);
    return;
  }
  const next = url.pathname + url.search + url.hash;
  const cur = window.location.pathname + window.location.search + window.location.hash;
  if (next === cur) return;
  // null history state: Next's popstate handler no-ops (avoids reload / traverse→MPA)
  const write = opts.replace ? nativeReplace! : nativePush!;
  write(null, "", next);
  emit();
  if (opts.scroll !== false) {
    try { window.scrollTo(0, 0); } catch { /* */ }
  }
}
