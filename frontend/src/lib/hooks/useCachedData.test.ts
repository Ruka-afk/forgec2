import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { fetchCached, clearDataCache, useCachedData } from "./useCachedData";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

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

  it("does not let a cleared in-flight request repopulate the cache", async () => {
    const oldRequest = deferred<string>();
    const newRequest = deferred<string>();
    const fetcher = vi.fn()
      .mockImplementationOnce(() => oldRequest.promise)
      .mockImplementationOnce(() => newRequest.promise)
      .mockResolvedValue("unexpected-third");

    const old = fetchCached("race", fetcher, 1000);
    clearDataCache();
    const fresh = fetchCached("race", fetcher, 1000);
    newRequest.resolve("new");
    expect(await fresh).toBe("new");
    oldRequest.resolve("old");
    expect(await old).toBe("old");

    expect(await fetchCached("race", fetcher, 1000)).toBe("new");
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("keeps the newest refresh result when requests resolve out of order", async () => {
    vi.useRealTimers();
    const initial = deferred<string>();
    const firstRefresh = deferred<string>();
    const secondRefresh = deferred<string>();
    const fetcher = vi.fn()
      .mockImplementationOnce(() => initial.promise)
      .mockImplementationOnce(() => firstRefresh.promise)
      .mockImplementationOnce(() => secondRefresh.promise);
    const { result } = renderHook(() => useCachedData("hook-race", { fetcher }));

    initial.resolve("initial");
    await waitFor(() => expect(result.current.data).toBe("initial"));
    let first!: Promise<unknown>;
    let second!: Promise<unknown>;
    act(() => { first = result.current.refresh(); });
    act(() => { second = result.current.refresh(); });
    secondRefresh.resolve("newest");
    await act(async () => { await second; });
    firstRefresh.resolve("stale");
    await act(async () => { await first; });

    expect(result.current.data).toBe("newest");
  });

  it("reports loading immediately when no cached value exists", () => {
    vi.useRealTimers();
    const pending = deferred<string>();
    const { result } = renderHook(() => useCachedData("first-load", { fetcher: () => pending.promise }));

    expect(result.current.data).toBeNull();
    expect(result.current.loading).toBe(true);
  });
});
