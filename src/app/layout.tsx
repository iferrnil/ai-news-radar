import type { Metadata, Viewport } from "next";
// Fonts are self-hosted from npm (@fontsource-variable + lxgw-wenkai-webfont, OFL-1.1) — `next build` makes no request to Google,
// so the build needs no access to Google Fonts. Each CSS ships unicode-range slices: browsers fetch only the chunks a page uses.
import "lxgw-wenkai-webfont/lxgwwenkai-regular.css";
import "lxgw-wenkai-webfont/lxgwwenkai-bold.css"; // upstream Medium, served as weight 700 (600 titles resolve to it)
import "@fontsource-variable/jetbrains-mono";
import AppShell from "@/components/app-shell";
import "./globals.css";

export const metadata: Metadata = { title: "AI Radar · 国内外 AI 情报", description: "国内外 AI 资讯实时聚合" };
export const viewport: Viewport = { themeColor: [{ media: "(prefers-color-scheme: dark)", color: "#0e0e11" }, { color: "#fcfcfd" }] };

// Runs in <head> before first paint: the collapsed sidebar, and the desktop app flags set by the MyGo preload script
// (window.__radarDesktop / __radarTitlebar; at document start there is no <html> element to mark yet).
const SIDEBAR_INIT = `try{var d=document.documentElement,w=window;if(localStorage.getItem("sidebar.collapsed")==="1")d.dataset.sidebar="collapsed";if(w.__radarDesktop){d.dataset.desktop="1";if(w.__radarTitlebar)d.dataset.titlebar="1"}}catch(e){}`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        {/* sidebar state + desktop flags before first paint (no open→closed flash); see app-shell toggleSidebar */}
        <script dangerouslySetInnerHTML={{ __html: SIDEBAR_INIT }} />
      </head>
      <body><AppShell>{children}</AppShell></body>
    </html>
  );
}
