import { test, expect } from "@playwright/test";

/**
 * Optional live e2e against a running ForgeC2 server (not static export).
 *
 * Enable with:
 *   FORGEC2_E2E_BASE=http://127.0.0.1:8000
 *   FORGEC2_E2E_USER=admin          (optional, default admin)
 *   FORGEC2_E2E_PASS=Admin123!      (optional; CI seed password)
 *
 * When FORGEC2_E2E_BASE is unset, the suite is skipped so CI static smoke stays green.
 */

const LIVE_BASE = process.env.FORGEC2_E2E_BASE?.replace(/\/$/, "") || "";
const USER = process.env.FORGEC2_E2E_USER || "admin";
const PASS = process.env.FORGEC2_E2E_PASS || "Admin123!";

test.describe("live server login + agents", () => {
  test.skip(!LIVE_BASE, "Set FORGEC2_E2E_BASE to run live e2e (e.g. http://127.0.0.1:8000)");

  test.use({ baseURL: LIVE_BASE || "http://127.0.0.1:8000" });

  test("health is ok", async ({ request }) => {
    const res = await request.get(`${LIVE_BASE}/health`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.status || body.Status).toBeTruthy();
  });

  test("login then agents list loads", async ({ page }) => {
    await page.goto("/login");
    await expect(page.locator("#login-username")).toBeVisible({ timeout: 15_000 });
    await page.locator("#login-username").fill(USER);
    await page.locator("#login-password").fill(PASS);
    await page.getByRole("button", { name: /sign in|登录/i }).click();

    // Successful login leaves /login
    await expect(page).not.toHaveURL(/login/, { timeout: 20_000 });

    await page.goto("/agents");
    await expect(page.locator("body")).toBeVisible();
    // Page chrome or empty-state must render; no hard error page
    await page.waitForTimeout(1_200);
    expect(page.url()).not.toMatch(/error|not-found/);
    // Prefer agents heading / table presence when i18n has loaded
    const hasAgentsChrome =
      (await page.getByRole("heading").count()) > 0 ||
      (await page.locator("table, [data-testid='agents-table'], main").count()) > 0;
    expect(hasAgentsChrome).toBeTruthy();
  });
});
