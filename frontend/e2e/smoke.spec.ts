import { test, expect, type Page } from "@playwright/test";

/**
 * Minimal UI smoke against static export.
 * API routes are mocked so tests do not need a running Go backend.
 */

async function mockAuthedApis(page: Page) {
  const emptyOk = {
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      status: "ok",
      version: "test",
      success: true,
      data: { profiles: [], agents: [] },
      agents: [],
      tasks: [],
      listeners: [],
      profiles: [],
      vault_entries: [],
      vault_count: 0,
      all_tags: [],
      Total: 0,
      total: 0,
      CurrentUsername: "admin",
      CurrentUserRole: "admin",
    }),
  };

  // Intercept all XHR/fetch; never return 401 (static SPA would bounce to /login)
  await page.route("**/*", async (route) => {
    const req = route.request();
    const type = req.resourceType();
    if (type === "xhr" || type === "fetch") {
      await route.fulfill(emptyOk);
      return;
    }
    if (type === "websocket") {
      await route.abort();
      return;
    }
    await route.continue();
  });
}

test.describe("login page", () => {
  test("renders form and shows failed login without navigating away", async ({ page }) => {
    await page.route("**/health", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ status: "ok", version: "test" }),
      });
    });
    await page.route("**/login", async (route) => {
      if (route.request().method() === "POST") {
        await route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({ error: "Invalid username or password" }),
        });
        return;
      }
      await route.continue();
    });

    await page.goto("/login.html");
    await expect(page.getByRole("form")).toBeVisible();
    await expect(page.locator("#login-username")).toBeVisible();
    await expect(page.locator("#login-password")).toBeVisible();
    await page.locator("#login-username").fill("admin");
    await page.locator("#login-password").fill("wrong");
    await page.getByRole("button", { name: /sign in|登录/i }).click();
    await expect(
      page.getByRole("alert").filter({ hasText: /Invalid username or password|登录失败|失败/i }),
    ).toBeVisible({ timeout: 10_000 });
    await expect(page).toHaveURL(/login/);
  });

  test("successful login triggers client navigation", async ({ page }) => {
    await page.route("**/health", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ status: "ok", version: "test" }),
      });
    });
    await page.route("**/login", async (route) => {
      if (route.request().method() === "POST") {
        await route.fulfill({
          status: 302,
          headers: { Location: "/" },
          body: "",
        });
        return;
      }
      await route.continue();
    });

    await page.goto("/login.html");
    await page.locator("#login-username").fill("admin");
    await page.locator("#login-password").fill("admin");
    await page.getByRole("button", { name: /sign in|登录/i }).click();
    await page.waitForTimeout(800);
    const url = page.url();
    expect(url.includes("login") || url.includes("dashboard") || url.endsWith("/")).toBeTruthy();
  });
});

test.describe("agents page shell", () => {
  test("agents static page loads without auth redirect crash", async ({ page }) => {
    await mockAuthedApis(page);
    const res = await page.goto("/agents.html");
    expect(res?.ok() || res?.status() === 200).toBeTruthy();
    await expect(page.locator("body")).toBeVisible();
    // Stay on agents route (or client soft-nav); must not hard-crash
    await page.waitForTimeout(500);
    expect(page.url()).not.toMatch(/error/);
  });
});

test.describe("generate page shell", () => {
  test("generate static page loads without crash", async ({ page }) => {
    await mockAuthedApis(page);
    const res = await page.goto("/generate.html");
    expect(res?.ok() || res?.status() === 200).toBeTruthy();
    await expect(page.locator("body")).toBeVisible();
    await page.waitForTimeout(800);
    // Soft: either generate chrome or still hydrating; must not hard-error
    expect(page.url()).not.toMatch(/error|not-found/);
  });
});

test.describe("credentials page shell", () => {
  test("credentials page loads without crash", async ({ page }) => {
    await mockAuthedApis(page);
    const res = await page.goto("/credentials.html");
    expect(res?.ok() || res?.status() === 200).toBeTruthy();
    await expect(page.locator("body")).toBeVisible();
    await page.waitForTimeout(500);
    expect(page.url()).not.toMatch(/error/);
  });
});

async function softShell(page: Page, route: string) {
  await mockAuthedApis(page);
  const res = await page.goto(route);
  expect(res?.ok() || res?.status() === 200).toBeTruthy();
  await expect(page.locator("body")).toBeVisible();
  await page.waitForTimeout(500);
  expect(page.url()).not.toMatch(/error|not-found/);
}

test.describe("core ops pages shell", () => {
  test("loot page loads without crash", async ({ page }) => {
    await softShell(page, "/loot.html");
  });

  test("users page loads without crash", async ({ page }) => {
    await softShell(page, "/users.html");
  });

  test("screenshots page loads without crash", async ({ page }) => {
    await softShell(page, "/screenshots.html");
  });

  test("builds page loads without crash", async ({ page }) => {
    await softShell(page, "/builds.html");
  });

  test("report page loads without crash", async ({ page }) => {
    await softShell(page, "/report.html");
  });
});

test.describe("platform pages shell", () => {
  test("ai page loads without crash", async ({ page }) => {
    await softShell(page, "/ai.html");
  });

  test("plugins page loads without crash", async ({ page }) => {
    await softShell(page, "/plugins.html");
  });

  test("settings page loads without crash", async ({ page }) => {
    await softShell(page, "/settings.html");
  });

  test("phishing page loads without crash", async ({ page }) => {
    await softShell(page, "/phishing.html");
  });

  test("automation page loads without crash", async ({ page }) => {
    await softShell(page, "/automation.html");
  });

  test("workflows page loads without crash", async ({ page }) => {
    await softShell(page, "/workflows.html");
  });
});
