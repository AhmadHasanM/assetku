import { defineConfig, devices } from "@playwright/test"

/**
 * E2E golden path. The stack must already be running and seeded
 * (`server seed-demo --tenant "PT Demo" --email demo@assetku.test --password demo12345`):
 * - local: web on :3000 proxying /api to the API (default base URL);
 * - compose: `docker compose -f docker-compose.e2e.yml up -d --build`, then
 *   `E2E_BASE_URL=http://localhost:8088 npx playwright test`.
 */
const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3000"

export default defineConfig({
  testDir: "./e2e",
  // One seeded tenant shared by every step: run serially.
  fullyParallel: false,
  workers: 1,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    // The compose stack may sit behind a self-signed certificate.
    ignoreHTTPSErrors: true,
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 800 } } }],
})
