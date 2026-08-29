import { expect, test, type Page, type Route } from "@playwright/test";

const ROUTES = ["dashboard", "agents", "listeners", "timeline", "settings", "ai", "chat"] as const;
const VISUAL_ROUTES = ["login", "dashboard", "agents", "listeners", "timeline", "settings", "ai", "chat"] as const;

const envelope = {
  status: "ok",
  success: true,
  version: "test",
  data: { profiles: [], agents: [] },
  agents: [],
  tasks: [],
  listeners: [],
  notifications: [],
  sessions: [],
  channels: [],
  total: 0,
  CurrentUsername: "admin",
  CurrentUserRole: "admin",
};

async function prepare(page: Page, theme: "light" | "dark" = "light") {
  await page.addInitScript((selectedTheme) => {
    localStorage.setItem("forgec2_theme", selectedTheme);
    localStorage.setItem("forgec2_lang", "en");
  }, theme);
  await page.context().addCookies([
    { name: "forgec2_session", value: "layout-e2e", domain: "127.0.0.1", path: "/" },
  ]);
  await page.route("**/*", async (route: Route) => {
    const type = route.request().resourceType();
    if (type === "websocket") return route.abort();
    if (type === "xhr" || type === "fetch") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(envelope) });
    }
    return route.continue();
  });
}

async function expectNoPageOverflow(page: Page) {
  const overflow = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    document: document.documentElement.scrollWidth,
    body: document.body.scrollWidth,
  }));
  expect(overflow.document, JSON.stringify(overflow)).toBeLessThanOrEqual(overflow.viewport + 1);
  expect(overflow.body, JSON.stringify(overflow)).toBeLessThanOrEqual(overflow.viewport + 1);
}

async function waitForPageSurface(page: Page, route: typeof VISUAL_ROUTES[number]) {
  if (route === "login") {
    await expect(page.getByRole("form")).toBeVisible();
    return;
  }
  // Waiting for <main> only proves that the app shell mounted. Several pages
  // resolve permissions and initial data one tick later, which previously
  // produced misleading blank visual baselines on mobile.
  await expect(page.locator("main h1").first()).toBeAttached({ timeout: 15_000 });
}

test.describe("responsive shell", () => {
  test("login remains centered and does not overflow", async ({ page }) => {
    await prepare(page);
    await page.goto("/login.html");
    await expect(page.getByRole("form")).toBeVisible();
    await expectNoPageOverflow(page);
  });

  for (const route of ROUTES) {
    test(`${route} uses a bounded page shell`, async ({ page }) => {
      await prepare(page);
      await page.goto(`/${route}.html`);
      await waitForPageSurface(page, route);
      await expectNoPageOverflow(page);
    });
  }

  test("AI configuration opens without replacing the conversation", async ({ page }) => {
    await prepare(page);
    await page.goto("/ai.html");
    await waitForPageSurface(page, "ai");
    await page.getByRole("button", { name: /AI Configuration|AI 配置/i }).click();
    await expect(page.getByRole("heading", { name: /AI Configuration|AI 配置/i })).toBeVisible();
    await expect(page.locator("main h1")).toBeAttached();
  });
});

test.describe("light and dark visual baselines", () => {
  for (const route of VISUAL_ROUTES) {
    for (const theme of ["light", "dark"] as const) {
      test(`${route} ${theme}`, async ({ page }, testInfo) => {
        test.skip(!["mobile", "laptop"].includes(testInfo.project.name), "Visual baselines use representative mobile and laptop widths");
        await prepare(page, theme);
        await page.goto(`/${route}.html`);
        await waitForPageSurface(page, route);
        await expect(page).toHaveScreenshot(`${route}-${theme}.png`, {
          animations: "disabled",
          fullPage: true,
          maxDiffPixelRatio: 0.015,
        });
      });
    }
  }
});
