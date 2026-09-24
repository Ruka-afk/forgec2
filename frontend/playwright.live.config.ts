import { defineConfig, devices } from "@playwright/test";

/**
 * Live UI e2e against a REAL ForgeC2 server (Go binary + embedded frontend).
 *
 * The default `playwright.config.ts` serves the static export; this config
 * talks to a running server instead, so login, session cookies, CSRF and API
 * wiring are exercised for real.
 *
 * Usage (repo frontend/):
 *   FORGEC2_E2E_BASE=http://127.0.0.1:8000 \
 *   FORGEC2_E2E_USER=admin FORGEC2_E2E_PASS='...' \
 *   npx playwright test --config=playwright.live.config.ts
 */
const LIVE_BASE = process.env.FORGEC2_E2E_BASE?.replace(/\/$/, "") || "http://127.0.0.1:8000";

export default defineConfig({
  testDir: "./e2e",
  testMatch: /live\.spec\.ts/,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  timeout: 60_000,
  use: {
    baseURL: LIVE_BASE,
    trace: "on-first-retry",
    ignoreHTTPSErrors: true,
  },
  projects: [{ name: "live", use: { ...devices["Desktop Chrome"] } }],
});
