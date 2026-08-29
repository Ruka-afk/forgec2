import { test, expect, type Page, type Route } from "@playwright/test";

/**
 * Key interaction flows against the static export with fully mocked APIs.
 * Covers: agents list render + debounced search, bulk selection bar,
 * tasks list render + status filter.
 */

interface Beacon {
  id?: string;
  hostname?: string;
  username?: string;
  ip?: string;
  os?: string;
  status?: string;
  last_seen?: string;
  version?: string;
}

const AGENTS: Beacon[] = [
  {
    id: "beacon-0001",
    hostname: "acme-domain-wks01",
    username: "jdupont",
    ip: "10.10.0.42",
    os: "windows",
    status: "active",
    last_seen: "2026-08-06T00:05:00Z",
    version: "1.0.0",
  },
  {
    id: "beacon-0002",
    hostname: "edge-linux-node",
    username: "root",
    ip: "10.10.0.77",
    os: "linux",
    status: "active",
    last_seen: "2026-08-06T00:05:01Z",
    version: "1.0.0",
  },
];

const TASKS = [
  {
    id: 1001,
    agent_id: "beacon-0001",
    type: "shell",
    command: `whoami`,
    status: "completed",
    created_at: "2026-08-06T00:04:00Z",
    result: "acme\\jdupont",
  },
  {
    id: 1002,
    agent_id: "beacon-0002",
    type: "screenshot",
    command: "capture",
    status: "running",
    created_at: "2026-08-06T00:05:00Z",
    result: "",
  },
];

const okEnvelope = {
  status: "ok",
  version: "test",
  success: true,
  agents: [],
  tasks: [],
  listeners: [],
  total: 0,
  tags: [],
  CurrentUsername: "admin",
  CurrentUserRole: "admin",
};

function mockAuthedApis(page: Page) {
  return page.route("**/*", async (route: Route) => {    const req = route.request();
    const type = req.resourceType();
    if (type === "websocket") {
      await route.abort();
      return;
    }
    if (type === "image") {
      // 1x1 transparent PNG so <img> elements load instead of erroring out.
      await route.fulfill({
        status: 200,
        contentType: "image/png",
        body: Buffer.from(
          "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
          "base64",
        ),
      });
      return;
    }
    if (type !== "xhr" && type !== "fetch") {
      await route.continue();
      return;
    }
    const url = new URL(req.url());
    const path = url.pathname;
    let body: unknown = okEnvelope;
    const status = 200;

    if (path === "/api/agents") {
      const q = (url.searchParams.get("search") || "").toLowerCase();
      const list = q
        ? AGENTS.filter((a) =>
            (a.hostname || "").toLowerCase().includes(q) ||
            (a.username || "").toLowerCase().includes(q),
          )
        : AGENTS;
      body = { agents: list, total: list.length };
    } else if (path === "/agents/batch/tags") {
      body = { tags: {} };
    } else if (path === "/api/tags" || path === "/api/v1/tasks" || path === "/api/v1/task-types" || path === "/tasks") {
      if (path === "/api/v1/tasks" || path === "/tasks") {
        const statusFilter = url.searchParams.get("status") || "";
        const list = statusFilter
          ? TASKS.filter((t) => t.status === statusFilter)
          : TASKS;
        body = { tasks: list, total: list.length };
      } else {
        body = { tags: [], taskTypes: [] };
      }
    } else if (path === "/collab/agents") {
      body = { agents: [] };
    } else if (path === "/loot") {
      body = {
        screenshots: Array.from({ length: 60 }, (_, i) => ({
          id: `shot-${i}`,
          agent_id: "beacon-0001",
          filename: `screenshot-${i}.png`,
          path: `screenshots/beacon-0001/screenshot-${i}.png`,
          created_at: "2026-08-06T00:00:00Z",
        })),
        keylog_tasks: [],
        download_tasks: [],
      };
    } else if (path === "/health") {
      body = { status: "ok", version: "test" };
    }

    await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
  });
}

async function withSession(page: Page) {
  // wsContext bounces to /login when the session cookie is missing after the
  // WebSocket closes (mocked: aborted), so seed a fake session first.
  await page.context().addCookies([
    { name: "forgec2_session", value: "e2e", domain: "127.0.0.1", path: "/" },
  ]);
}

test.describe("agents list interplay", () => {
  test("renders beacon items, debounced search narrows the list, bulk bar appears on select", async ({ page }) => {
    await mockAuthedApis(page);
    await withSession(page);

    await page.goto("/agents.html");
    await expect(page.getByText("acme-domain-wks01")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText("edge-linux-node")).toBeVisible();

    const searchBox = page.getByPlaceholder(/搜索 Agent|search agent/i);
    await searchBox.fill("edge");
    await expect(page.getByText("acme-domain-wks01")).toBeHidden({ timeout: 10_000 });
    await expect(page.getByText("edge-linux-node")).toBeVisible();

    // Bulk mode on, select the single visible row → bar appears, then clear.
    await page.getByRole("button", { name: /批量操作|bulk ops/i }).click();
    await page.getByRole("checkbox", { name: /选择项目|选择 Agent|select (item|agent)/i }).first().click();
    await expect(page.getByRole("button", { name: /执行命令|execute command/i })).toBeVisible();
    await page.getByRole("button", { name: /清除选择|clear selection/i }).click();
    await expect(page.getByRole("button", { name: /执行命令|execute command/i })).toHaveCount(0);
  });
});

test.describe("tasks list interplay", () => {
  test("renders task rows and the status filter re-queries", async ({ page }) => {
    await mockAuthedApis(page);
    await withSession(page);

    await page.goto("/tasks.html");
    await expect(page.getByText("whoami")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText("capture")).toBeVisible();

    const statusFilter = page.getByRole("combobox", { name: /状态|status/i });
    if (await statusFilter.count()) {
      await statusFilter.click();
      await page.getByRole("option", { name: /已完成|completed/i }).click();
      await expect(page.getByText("capture")).toBeHidden({ timeout: 10_000 });
      await expect(page.getByText("whoami")).toBeVisible();
    }
  });
});

test.describe("loot screenshots pagination", () => {
  test("renders first page of 48 and flips to the remaining 12", async ({ page }) => {
    await mockAuthedApis(page);
    await withSession(page);

    await page.goto("/loot.html");
    const imgs = page.getByRole("img", { name: /screenshot-\d+\.png/ });
    await expect(imgs).toHaveCount(48, { timeout: 10_000 });
    await expect(page.getByText(/\/ 60/)).toBeVisible();

    await page.getByRole("button", { name: /下一页|next/i }).click();
    await expect(imgs).toHaveCount(12, { timeout: 10_000 });
  });
});
