"use client";
/** Render the page for the History pathname without Next soft/hard navigation.
 *  Initial SSR `children` stay mounted until the user soft-navs away (and back uses
 *  children again when the path matches the boot URL). */
import { useRef, type ReactNode } from "react";
import dynamic from "next/dynamic";
import { useClientPathname } from "@/lib/client-url";

const fallback = <div className="min-h-[40vh] bg-background" aria-hidden />;

const FeedPage = dynamic(() => import("@/components/Feed"), { ssr: false, loading: () => fallback });
const SettingsPage = dynamic(() => import("@/components/Settings"), { ssr: false, loading: () => fallback });
const SourcesPage = dynamic(() => import("@/components/Sources"), { ssr: false, loading: () => fallback });
const DocsIndex = dynamic(() => import("@/components/DocsIndex"), { ssr: false, loading: () => fallback });
const RateLimitsPage = dynamic(() => import("@/components/RateLimits"), { ssr: false, loading: () => fallback });

function pageFor(path: string): ReactNode {
  if (path === "/settings") return <SettingsPage />;
  if (path === "/sources") return <SourcesPage />;
  if (path === "/docs") return <DocsIndex />;
  if (path === "/docs/rate-limits") return <RateLimitsPage />;
  if (path.startsWith("/docs/")) return <DocsIndex />;
  return <FeedPage />;
}

export default function ClientOutlet({ children }: { children: ReactNode }) {
  const path = useClientPathname();
  const boot = useRef(path);
  // Hydrated route: keep SSR children (no remount / no loading flash).
  if (path === boot.current) return <>{children}</>;
  return <>{pageFor(path)}</>;
}
