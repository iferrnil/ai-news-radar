import type { NextConfig } from "next";

// Static export: the UI is plain HTML/JS/CSS in out/, served by the Go backend (server/), which also provides /api/*.
// `npm run dev` proxies /api to a running Go server (API_URL, default http://127.0.0.1:3001):
//   (cd server && PORT=3001 go run ./cmd/radar-server) & npm run dev
const dev = process.env.NODE_ENV === "development";
const nextConfig: NextConfig = {
  output: dev ? undefined : "export",
  images: { unoptimized: true },
  ...(dev ? { rewrites: async () => [{ source: "/api/:path*", destination: `${process.env.API_URL ?? "http://127.0.0.1:3001"}/api/:path*` }] } : {}),
  turbopack: {
    rules: { "*.css": { loaders: ["@tailwindcss/turbopack"], as: "*.css" } },
  },
};

export default nextConfig;
