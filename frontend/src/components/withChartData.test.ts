import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { fetchChartData, CHART_CACHE_TTL_MS } from "./withChartData";
import { clearDataCache } from "@/lib/hooks/useCachedData";

describe("fetchChartData", () => {
  beforeEach(() => {
    clearDataCache();
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("applies the transform and caches the transformed value", async () => {
    const fetchFn = vi.fn().mockResolvedValue({ a: 1 });
    const transform = (raw: unknown) => ({ total: (raw as { a: number }).a });

    const first = await fetchChartData("/api/dashboard/os-distribution", fetchFn, transform);
    const second = await fetchChartData("/api/dashboard/os-distribution", fetchFn, transform);

    expect(first).toEqual({ total: 1 });
    expect(second).toEqual({ total: 1 });
    expect(fetchFn).toHaveBeenCalledTimes(1);
    expect(fetchFn).toHaveBeenCalledWith("/api/dashboard/os-distribution");
  });

  it("refetches after the TTL lapses", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce({ v: "old" })
      .mockResolvedValueOnce({ v: "new" });
    await fetchChartData("/api/dashboard/attack-path", fetchFn);
    vi.advanceTimersByTime(CHART_CACHE_TTL_MS + 1);
    const v = await fetchChartData("/api/dashboard/attack-path", fetchFn);
    expect(v).toEqual({ v: "new" });
    expect(fetchFn).toHaveBeenCalledTimes(2);
  });

  it("keeps distinct chart endpoints in separate cache entries", async () => {
    const fetchFn = vi.fn().mockResolvedValue({ x: 1 });
    await fetchChartData("/api/dashboard/os-distribution", fetchFn);
    await fetchChartData("/api/dashboard/task-status", fetchFn);
    expect(fetchFn).toHaveBeenCalledTimes(2);
  });

  it("returns raw data when no transform is given", async () => {
    const fetchFn = vi.fn().mockResolvedValue(["a", "b"]);
    const v = await fetchChartData("/api/dashboard/plain", fetchFn);
    expect(v).toEqual(["a", "b"]);
  });
});