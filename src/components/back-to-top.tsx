"use client";
// Light back-to-top: one passive, rAF-coalesced window scroll listener (same visibility
// threshold as the feed). Sources mounts this alone; Feed keeps its own listener because
// it also toggles .feed-list[data-scrolling] for scroll jank.
import { useEffect, useState } from "react";
import { ArrowUp } from "lucide-react";

export function BackToTop({ threshold = 900 }: { threshold?: number }) {
  const [show, setShow] = useState(false);
  useEffect(() => {
    let raf = 0, on = false;
    const h = () => {
      if (!raf) raf = requestAnimationFrame(() => {
        raf = 0;
        const t = window.scrollY > threshold;
        if (t !== on) { on = t; setShow(t); }
      });
    };
    window.addEventListener("scroll", h, { passive: true });
    return () => { window.removeEventListener("scroll", h); cancelAnimationFrame(raf); };
  }, [threshold]);
  if (!show) return null;
  return (
    <button
      type="button"
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      aria-label="回到顶部"
      data-testid="back-to-top"
      className="animate-in fade-in-0 slide-in-from-bottom-2 duration-200 ease-out fixed bottom-6 right-6 z-30 grid size-9 place-items-center rounded-full bg-popover text-muted-foreground shadow-card ring-1 ring-border transition-colors duration-150 ease-out hover:text-foreground"
    >
      <ArrowUp className="size-4" />
    </button>
  );
}
