import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { fetchCached, clearDataCache } from "./useCachedData";

describe("fetchCached", () => {
  beforeEach(() => {
    clearDataCache();
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("fetches once and serves the shared cache", async () => {
    const fetcher = vi.fn().mockResolvedValue("v");
    const a = await fetchCached("k1", fetcher, 1000);
    const b = await fetchCached("k1", fetcher, 1000);
    expect(a).toBe("v");
    expect(b).toBe("v");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("refetches after the TTL lapses", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce("old")
      .mockResolvedValueOnce("new");
    await fetchCached("k2", fetcher, 1000);
    vi.advanceTimersByTime(1001);
    const v = await fetchCached("k2", fetcher, 1000);
    expect(v).toBe("new");
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("collapses concurrent callers into a single request", async () => {
    let resolve!: (v: string) => void;
    const fetcher = vi.fn(
      () => new Promise<string>((r) => { resolve = r; }),
    );
    const p1 = fetchCached("k3", fetcher, 1000);
    const p2 = fetchCached("k3", fetcher, 1000);
    resolve("done");
    const [a, b] = await Promise.all([p1, p2]);
    expect(a).toBe("done");
    expect(b).toBe("done");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("clearDataCache drops entries so the next call refetches", async () => {
    const fetcher = vi.fn().mockResolvedValue("v");
    await fetchCached("k4", fetcher, 1000);
    clearDataCache();
    await fetchCached("k4", fetcher, 1000);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});