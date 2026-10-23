/** URL search without Next `useSearchParams` (no Suspense bailout).
 *  Soft nav uses History API; we patch push/replace once and subscribe. */
"use client";

import { useMemo, useSyncExternalStore } from "react";

type Listener = () => void;
const listeners = new Set<Listener>();
let patched = false;

function emit() {
  for (const l of listeners) l();
}

function ensureHistoryPatch() {
  if (patched || typeof window === "undefined") return;
  patched = true;
  for (const method of ["pushState", "replaceState"] as const) {
    const orig = history[method];
    history[method] = function (this: History, ...args: Parameters<History["pushState"]>) {
      const ret = orig.apply(this, args);
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

/** Current `location.search` (including leading `?`), updates on soft nav. */
export function useClientSearch(): string {
  return useSyncExternalStore(subscribe, getSearch, () => "");
}

/** Parsed query — drop-in for the string form of `useSearchParams()`. */
export function useClientSearchParams(): URLSearchParams {
  const search = useClientSearch();
  return useMemo(() => new URLSearchParams(search.startsWith("?") ? search.slice(1) : search), [search]);
}
