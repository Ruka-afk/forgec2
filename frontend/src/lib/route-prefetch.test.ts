import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Prefetch is judged by *what* it imports, so the import map's entries are
// replaced with spies. resetModules gives every test its own copy of the
// module-level "already started" bookkeeping and of the map it points at.
async function fresh() {
  vi.resetModules();
  const { ROUTE_PAGE_IMPORTS } = await import("./route-imports");
  const mod = await import("./route-prefetch");
  const spy = (id: keyof typeof ROUTE_PAGE_IMPORTS) =>
    vi.spyOn(ROUTE_PAGE_IMPORTS, id).mockResolvedValue({ default: () => null });
  return { ...mod, spy };
}

describe("routeIdFromPath", () => {
  it("maps only top-level routes that have a lazy page", async () => {
    const { routeIdFromPath } = await fresh();
    expect(routeIdFromPath("/beacons")).toBe("beacons");
    expect(routeIdFromPath("/beacons/")).toBe("beacons");
    expect(routeIdFromPath("/beacons?tab=1")).toBe("beacons");
    expect(routeIdFromPath("/dashboard#top")).toBe("dashboard");
    // Detail routes must not warm the list chunk they are not going to use.
    expect(routeIdFromPath("/beacons/42/shell")).toBeNull();
    expect(routeIdFromPath("/not-a-route")).toBeNull();
    expect(routeIdFromPath("/")).toBeNull();
    expect(routeIdFromPath("")).toBeNull();
  });
});

describe("prefetchRoute", () => {
  it("imports the chunk once, however often hover fires", async () => {
    const { prefetchRoute, spy } = await fresh();
    const load = spy("beacons");
    prefetchRoute("/beacons");
    prefetchRoute("/beacons");
    prefetchRoute("/beacons/");
    await vi.waitFor(() => expect(load).toHaveBeenCalledTimes(1));
  });

  it("ignores detail routes and unknown paths", async () => {
    const { prefetchRoute, spy } = await fresh();
    const load = spy("beacons");
    prefetchRoute("/beacons/42/shell");
    prefetchRoute("/not-a-route");
    await Promise.resolve();
    expect(load).not.toHaveBeenCalled();
  });

  it("forgets a failed load so the next hover retries", async () => {
    const { prefetchRoute, spy } = await fresh();
    const load = spy("listeners").mockRejectedValueOnce(new Error("offline"));
    prefetchRoute("/listeners");
    await vi.waitFor(() => expect(load).toHaveBeenCalledTimes(1));
    await vi.waitFor(() => expect(load).toHaveBeenCalledTimes(1));
    prefetchRoute("/listeners");
    await vi.waitFor(() => expect(load).toHaveBeenCalledTimes(2));
  });
});

describe("startIdleRoutePrefetch", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // jsdom does not ship requestIdleCallback; pin the setTimeout fallback so
    // the test exercises the path every browser without the API takes.
    Object.defineProperty(window, "requestIdleCallback", { value: undefined, configurable: true });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    Object.defineProperty(navigator, "connection", { value: undefined, configurable: true });
  });

  it("warms the priority pages once the browser is idle", async () => {
    const { startIdleRoutePrefetch, spy } = await fresh();
    const load = spy("dashboard");
    const cancel = startIdleRoutePrefetch();
    vi.advanceTimersByTime(1500);
    await vi.waitFor(() => expect(load).toHaveBeenCalledTimes(1));
    cancel();
  });

  it("cancels a pending warm on unmount", async () => {
    const { startIdleRoutePrefetch, spy } = await fresh();
    const load = spy("dashboard");
    const cancel = startIdleRoutePrefetch();
    cancel();
    vi.advanceTimersByTime(5000);
    expect(load).not.toHaveBeenCalled();
  });

  it("skips speculative prefetch on save-data connections", async () => {
    Object.defineProperty(navigator, "connection", { value: { saveData: true }, configurable: true });
    const { startIdleRoutePrefetch, spy } = await fresh();
    const load = spy("dashboard");
    startIdleRoutePrefetch();
    vi.advanceTimersByTime(5000);
    expect(load).not.toHaveBeenCalled();
  });
});
