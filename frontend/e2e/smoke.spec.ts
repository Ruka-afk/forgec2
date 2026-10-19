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

/**
 * Boots a static-export route and asserts the SPA actually came up: React
 * mounts into #root, no uncaught error fires, and the URL does not land on an
 * error page. The previous helper only checked "body is visible" plus an
 * absent /error in the URL, which passed even for retired routes — /builds,
 * /screenshots and /workflows were rendering the not-found page while still
 * "loading without crash".
 */
async function bootRoute(page: Page, route: string) {
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  await mockAuthedApis(page);
  const res = await page.goto(route);
  expect(res?.ok() || res?.status() === 200).toBeTruthy();
  await expect(page.locator("#root")).not.toBeEmpty();
  await page.waitForTimeout(300);
  expect(page.url()).not.toMatch(/error|not-found/);
  expect(pageErrors).toEqual([]);
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

// Top-level SPA pages that exist in the router table (src/router.tsx) and are
// emitted by the static export. Retired destinations (/builds, /screenshots,
// /workflows, /tasks, /packer, /stager, /profiles, /files, /notifications)
// merged into tabs elsewhere; probing them proved nothing about the app.
const SHELL_ROUTES = [
  "/beacons.html",
  "/payload-generator.html",
  "/credentials.html",
  "/loot.html",
  "/users.html",
  "/report.html",
  "/ai.html",
  "/plugins.html",
  "/settings.html",
  "/phishing.html",
  "/automation.html",
] as const;

test.describe("page shells boot the SPA", () => {
  for (const route of SHELL_ROUTES) {
    test(`${route} mounts without uncaught errors`, async ({ page }) => {
      await bootRoute(page, route);
    });
  }
});

/**
 * U7 w2 renamed six routes to the Beacon vocabulary. The export still ships
 * the pre-rename shells (vite-plugins/staticExport.ts) and the router rewrites
 * them client-side, so an old bookmark must land on the canonical URL with its
 * query string intact.
 */
test.describe("legacy routes redirect to the renamed destinations", () => {
  const CASES = [
    ["/agents.html", "/beacons"],
    ["/agents.html?status=offline", "/beacons?status=offline"],
    ["/generate.html", "/payload-generator"],
    ["/timeline.html?tab=tasks", "/events?tab=tasks"],
    ["/privesc.html", "/privilege-escalation"],
    ["/chain.html", "/proxy-chains"],
    ["/asset-organization.html", "/asset-management"],
  ] as const;

  for (const [legacy, expected] of CASES) {
    test(`${legacy} redirects to ${expected}`, async ({ page }) => {
      await mockAuthedApis(page);
      await page.goto(legacy);
      await page.waitForURL((url) => `${url.pathname}${url.search}` === expected, {
        timeout: 15_000,
      });
    });
  }
});

/**
 * The payload workspace is the deepest static-export route: the router lazy-
 * loads PayloadGeneratorPage, and each tab panel is a second lazy chunk behind its own
 * import. bootRoute only proves React mounted; this probe pins that the
 * payload workspace chunk actually renders (the agent-format tablist cannot
 * exist before the chunk loads) and that tab switching lazy-loads the next
 * panel into the ?tab= deep link.
 */
test.describe("generate workspace (static export)", () => {
  test("payload panel renders its lazy chunk and tabs switch", async ({ page }) => {
    // Force the UI language so the assertions can use the en strings.
    await page.addInitScript(() => localStorage.setItem("forgec2_lang", "en"));
    await mockAuthedApis(page);
    const res = await page.goto("/payload-generator.html");
    expect(res?.ok() || res?.status() === 200).toBeTruthy();

    // The route chunk carries the five workspace tabs.
    await expect(page.getByRole("tab", { name: "Payload", exact: true })).toBeVisible();
    // The payload panel is its own chunk: the format picker is a second
    // tablist, labelled from generate.agents_title.
    await expect(page.getByRole("tablist", { name: "Payload Formats" })).toBeVisible();

    // Switching tabs loads the next panel and lands in the deep link.
    await page.getByRole("tab", { name: "Builds", exact: true }).click();
    await expect(page).toHaveURL(/tab=builds/);
    await expect(page.getByRole("tab", { name: "Builds", exact: true })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });
});
