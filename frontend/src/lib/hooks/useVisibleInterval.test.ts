import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useVisibleInterval } from "./useVisibleInterval";

describe("useVisibleInterval", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("does not start in a hidden tab and catches up when visible", async () => {
    vi.useFakeTimers();
    let hidden = true;
    vi.spyOn(document, "hidden", "get").mockImplementation(() => hidden);
    const callback = vi.fn();
    renderHook(() => useVisibleInterval(callback, 1000));

    await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
    expect(callback).not.toHaveBeenCalled();

    hidden = false;
    act(() => document.dispatchEvent(new Event("visibilitychange")));
    expect(callback).toHaveBeenCalledTimes(1);

    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    expect(callback).toHaveBeenCalledTimes(2);
  });

  it("skips overlapping async ticks", async () => {
    vi.useFakeTimers();
    let release: (() => void) | undefined;
    const callback = vi.fn(() => new Promise<void>((resolve) => { release = resolve }));
    renderHook(() => useVisibleInterval(callback, 1000));

    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    expect(callback).toHaveBeenCalledTimes(1);

    await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
    expect(callback).toHaveBeenCalledTimes(1);

    act(() => release?.());
    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    expect(callback).toHaveBeenCalledTimes(2);
  });
});
