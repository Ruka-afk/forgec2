import { describe, it, expect, vi, afterEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useApiResource } from "./useApiResource";

describe("useApiResource", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("loads data on mount and exposes loading/error state", async () => {
    const fetcher = vi.fn().mockResolvedValue([1, 2, 3]);
    const { result } = renderHook(() => useApiResource<number[]>({ fetcher }));

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual([1, 2, 3]);
    expect(result.current.error).toBeNull();
  });

  it("surfaces an error when the first fetch fails", async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() =>
      useApiResource<number[]>({ fetcher, errorMessage: "nope" })
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe("nope");
    expect(result.current.data).toBeNull();
  });

  it("retains stale data on silent refresh errors", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce([1])
      .mockRejectedValueOnce(new Error("boom"));
    const { result } = renderHook(() =>
      useApiResource<number[]>({ fetcher, errorMessage: "nope", toastThrottleMs: 0 })
    );

    await waitFor(() => expect(result.current.data).toEqual([1]));
    await act(async () => { await result.current.refresh(); });
    // old data kept, not flagged as a hard error
    expect(result.current.data).toEqual([1]);
    expect(result.current.error).toBeNull();
  });

  it("supersedes an in-flight refresh with a newer one", async () => {
    let resolveFirst: (v: number[]) => void = () => {};
    let resolveSecond: (v: number[]) => void = () => {};
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce([0]) // mount fetch
      .mockImplementationOnce(
        () => new Promise<number[]>((resolve) => { resolveFirst = resolve; })
      )
      .mockImplementationOnce(
        () => new Promise<number[]>((resolve) => { resolveSecond = resolve; })
      );
    const { result } = renderHook(() => useApiResource<number[]>({ fetcher }));
    await waitFor(() => expect(result.current.data).toEqual([0]));

    let p1: Promise<void>;
    let p2: Promise<void>;
    await act(async () => {
      p1 = result.current.refresh();
      p2 = result.current.refresh();
      resolveFirst([1]);
      resolveSecond([2]);
      await Promise.all([p1, p2]);
    });
    // first result was superseded (aborted); second wins
    expect(fetcher).toHaveBeenCalledTimes(3);
    expect(result.current.data).toEqual([2]);
  });

  it("polls on the requested interval", async () => {
    vi.useFakeTimers();
    const fetcher = vi.fn().mockResolvedValue([1]);
    renderHook(() => useApiResource<number[]>({ fetcher, pollMs: 5000 }));

    await act(async () => {
      await vi.advanceTimersByTimeAsync(6000);
    });
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
