import type { NextConfig } from "next"
import createNextIntlPlugin from "next-intl/plugin"

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts")

const API_DEV_ORIGIN = process.env.API_DEV_ORIGIN ?? "http://localhost:8080"

const nextConfig: NextConfig = {
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  // In production a reverse proxy serves /api from the Go server on the same origin.
  async rewrites() {
    if (process.env.NODE_ENV !== "development") return []
    return [{ source: "/api/:path*", destination: `${API_DEV_ORIGIN}/api/:path*` }]
  },
}

export default withNextIntl(nextConfig)
