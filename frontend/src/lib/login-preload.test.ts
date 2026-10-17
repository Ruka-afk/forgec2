import { describe, expect, it, vi } from "vitest";

// The router lazy-loads LoginPage; this helper only pre-warms that same chunk
// before the 401 redirect. Both behaviors are pinned here: a healthy import
// resolves with the page module, and a failed chunk fetch is swallowed (the
// router retries on render, and an unhandled rejection here would surface as
// a console error during an already-degraded session).
describe("preloadLoginPage", () => {
  it("resolves with the lazily imported login page module", async () => {
    vi.doMock("@/pages/login/LoginPage", () => ({ default: () => null }));
    const { preloadLoginPage } = await import("./login-preload");
    await expect(preloadLoginPage()).resolves.toBeTruthy();
  });

  it("swallows a failed chunk load", async () => {
    vi.resetModules();
    vi.doMock("@/pages/login/LoginPage", () => {
      throw new Error("chunk load failed");
    });
    const { preloadLoginPage } = await import("./login-preload");
    await expect(preloadLoginPage()).resolves.toBeUndefined();
  });
});
