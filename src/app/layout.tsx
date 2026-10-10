import type { Metadata, Viewport } from "next";
// Fonts are self-hosted from npm (@fontsource-variable + lxgw-wenkai-webfont, OFL-1.1) — `next build` makes no request to Google,
// so it works in mainland China. Each CSS ships unicode-range slices: browsers fetch only the chunks a page uses.
import "lxgw-wenkai-webfont/lxgwwenkai-regular.css";
import "lxgw-wenkai-webfont/lxgwwenkai-bold.css"; // upstream Medium, served as weight 700 (600 titles resolve to it)
import "@fontsource-variable/jetbrains-mono";
import AppShell from "@/components/app-shell";
import "./globals.css";

export const metadata: Metadata = { title: "AI Radar · 国内外 AI 情报", description: "国内外 AI 资讯实时聚合" };
export const viewport: Viewport = { themeColor: [{ media: "(prefers-color-scheme: dark)", color: "#0e0e11" }, { color: "#fcfcfd" }] };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body><AppShell>{children}</AppShell></body>
    </html>
  );
}
