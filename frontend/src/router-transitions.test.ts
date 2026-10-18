import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// M1: every router-driven navigation opts into the View Transitions API. The
// wrapper is installed on `router.navigate` at import time; here
// createBrowserRouter is replaced with a spy so the wrapper's options handling
// is asserted without running a real navigation (React Router's data router
// needs browser APIs jsdom does not implement).
const { navigateCalls } = vi.hoisted(() => ({
  navigateCalls: [] as Array<{ to: unknown; options: Record<string, unknown> | undefined }>,
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    createBrowserRouter: () => ({
      routes: [],
      navigate: (to: unknown, options?: Record<string, unknown>) => {
        navigateCalls.push({ to, options });
        return Promise.resolve();
      },
    }),
  };
});

import { router } from "./router";

function installTransitionApi() {
  Object.defineProperty(document, "startViewTransition", {
    configurable: true,
    writable: true,
    value: () => ({
      finished: Promise.resolve(),
      ready: Promise.resolve(),
      updateCallbackDone: Promise.resolve(),
      skipTransition: () => {},
    }),
  });
}

function removeTransitionApi() {
  delete (document as unknown as Record<string, unknown>).startViewTransition;
}

describe("route transitions", () => {
  beforeEach(() => {
    navigateCalls.length = 0;
    installTransitionApi();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    removeTransitionApi();
  });

  // `Link` forwards its own `viewTransition` prop on every click, so the key is
  // present-but-undefined when unset; an unconditional spread would clobber the
  // default and transitions would silently stop firing on link navigation.
  it("defaults link-driven navigations into a view transition", () => {
    router.navigate("/agents", { viewTransition: undefined });
    expect(navigateCalls).toHaveLength(1);
    expect(navigateCalls[0].options?.viewTransition).toBe(true);
  });

  it("keeps an explicit opt-out untouched", () => {
    router.navigate("/agents", { viewTransition: false });
    expect(navigateCalls).toHaveLength(1);
    expect(navigateCalls[0].options?.viewTransition).toBe(false);
  });

  it("skips the API when the user prefers reduced motion", () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })),
    );
    router.navigate("/agents", { viewTransition: undefined });
    expect(navigateCalls).toHaveLength(1);
    expect(navigateCalls[0].options?.viewTransition).not.toBe(true);
  });

  it("leaves navigations untouched when the API is unavailable", () => {
    removeTransitionApi();
    router.navigate("/agents", { viewTransition: undefined });
    expect(navigateCalls).toHaveLength(1);
    expect(navigateCalls[0].options?.viewTransition).not.toBe(true);
  });
});
