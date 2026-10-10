import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // STANDALONE=1 (used by the Dockerfile) builds a self-contained server; plain `next start` otherwise
  output: process.env.STANDALONE === "1" ? "standalone" : undefined,
  serverExternalPackages: ["better-sqlite3"],
  turbopack: {
    rules: { "*.css": { loaders: ["@tailwindcss/turbopack"], as: "*.css" } },
  },
};

export default nextConfig;
