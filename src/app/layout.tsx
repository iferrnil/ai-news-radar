import type { Metadata, Viewport } from "next";
import { Inter, Noto_Sans_SC, JetBrains_Mono } from "next/font/google";
import AppShell from "@/components/app-shell";
import "./globals.css";

// Latin: Inter (OFL) — Chinese: Noto Sans SC (OFL, redistributable), self-hosted by next/font and
// sliced by unicode-range so browsers only download the CJK chunks a page actually uses.
const inter = Inter({ subsets: ["latin"], variable: "--font-latin", display: "swap" });
const notoSC = Noto_Sans_SC({ weight: ["400", "500", "600"], variable: "--font-cjk", display: "swap", preload: false });
const mono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono-face", display: "swap" });
const latin = inter;

export const metadata: Metadata = { title: "AI Radar · 国内外 AI 情报", description: "国内外 AI 资讯实时聚合" };
export const viewport: Viewport = { themeColor: [{ media: "(prefers-color-scheme: dark)", color: "#0e0e11" }, { color: "#fcfcfd" }] };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" className={`${latin.variable} ${notoSC.variable} ${mono.variable}`} suppressHydrationWarning>
      <body><AppShell>{children}</AppShell></body>
    </html>
  );
}
