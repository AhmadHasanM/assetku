import type { NextConfig } from "next"
import createNextIntlPlugin from "next-intl/plugin"

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts")

// /api must be same-origin (session and CSRF cookies). In production nginx
// routes /api/* to the Go server and the rest here, so no rewrite is emitted.
// `next dev` always proxies to the local API; a production build proxies only
// when API_PROXY_ORIGIN is set at build time (e2e against a local stack without nginx).
const API_ORIGIN =
  process.env.NODE_ENV === "development"
    ? (process.env.API_DEV_ORIGIN ?? "http://localhost:8080")
    : process.env.API_PROXY_ORIGIN

const nextConfig: NextConfig = {
  // Self-contained server for the Docker image (web/Dockerfile).
  output: "standalone",
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  async rewrites() {
    if (!API_ORIGIN) return []
    return [{ source: "/api/:path*", destination: `${API_ORIGIN}/api/:path*` }]
  },
}

export default withNextIntl(nextConfig)
