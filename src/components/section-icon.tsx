// The ONE place that turns a section's stored icon into UI. Stored values: `lucide:<name>` tokens (the seeded
// defaults lab/cn/media/x/ent, or any icon of the library in icon-lib.tsx), an emoji, or null. Display never shows
// the raw token; exports keep it verbatim. The library is a separate chunk: loaded only when a section uses one of
// its icons (or the picker opens), so the sidebar's first load stays the same size.
import { createElement, useSyncExternalStore } from "react";
import { Clapperboard, FlaskConical, Folder, Globe2, Newspaper } from "lucide-react";
import { XLogo as Twitter } from "./x-logo";
import { cn } from "@/lib/utils";
import { type Section, allSections } from "./common";

const LUCIDE: Record<string, React.ElementType> = { lab: FlaskConical, cn: Globe2, media: Newspaper, x: Twitter, ent: Clapperboard };
const EMOJI = new Map<string, React.ElementType>();

// lazily loaded icon library
let lib: Record<string, React.ElementType> | null = null;
let libP: Promise<void> | null = null;
const subs = new Set<() => void>();
export const loadIconLib = () => (libP ??= import("./icon-lib").then((m) => { lib = m.LIB; subs.forEach((f) => f()); }));
const subscribe = (f: () => void) => { subs.add(f); void loadIconLib(); return () => { subs.delete(f); }; };
const getLib = () => lib;
const LAZY = new Map<string, React.ElementType>();
function libIcon(name: string): React.ElementType {
  let C = LAZY.get(name);
  if (!C) {
    const LibIcon = ({ className, ...p }: { className?: string }) => {
      const l = useSyncExternalStore(subscribe, getLib, () => null);
      if (!l) return <span aria-hidden className={cn("inline-block size-4 shrink-0", className)} />;
      return createElement(l[name] ?? Folder, { ...p, className });
    };
    C = LibIcon; LAZY.set(name, C);
  }
  return C;
}

/** icon value -> stable component (lucide token, emoji, or Folder). Emoji get the same box as lucide (size from className). */
export function secIcon(s: Pick<Section, "icon">): React.ElementType {
  const v = s.icon;
  if (!v) return Folder;
  if (v.startsWith("lucide:")) { const k = v.slice(7); return LUCIDE[k] ?? libIcon(k); }
  let C = EMOJI.get(v);
  if (!C) {
    const Emoji = ({ className }: { className?: string }) => <span aria-hidden className={cn("inline-grid size-4 shrink-0 place-items-center overflow-hidden text-[14px] leading-none", className)}>{v}</span>;
    C = Emoji; EMOJI.set(v, C);
  }
  return C;
}
export const isIconToken = (v?: string | null) => !!v?.startsWith("lucide:");

/** Icon only, by stored value. */
export function SectionIcon({ icon, className }: { icon?: string | null; className?: string }) {
  return createElement(secIcon({ icon }), { "aria-hidden": true, className: cn("size-4 shrink-0", className) });
}

/** Icon + name for a section key (or explicit section). Used in selects, table, badges, palette. */
export function SectionName({ k, s, className, iconClassName }: { k?: string; s?: Pick<Section, "icon" | "zh">; className?: string; iconClassName?: string }) {
  const sec = s ?? allSections().find((x) => x.key === k);
  return (
    <span className={cn("inline-flex min-w-0 items-center gap-1.5", className)}>
      <SectionIcon icon={sec?.icon} className={cn("size-3.5 opacity-80", iconClassName)} />
      <span className="truncate">{sec?.zh ?? "未分类"}</span>
    </span>
  );
}
