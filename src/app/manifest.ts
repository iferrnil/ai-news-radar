import type { MetadataRoute } from "next";

// Web app manifest: the same purple radar icon as the desktop app (desktop/resources/icon.png).
export const dynamic = "force-static";
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "AI Radar · 国内外 AI 情报", short_name: "AI Radar", description: "国内外 AI 资讯实时聚合",
    start_url: "/", display: "standalone", background_color: "#0e0e11", theme_color: "#6d28d9",
    icons: [
      { src: "/icons/icon-192.png", sizes: "192x192", type: "image/png" },
      { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png" },
    ],
  };
}
