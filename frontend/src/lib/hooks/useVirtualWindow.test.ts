import { act, renderHook } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import { computeVirtualRange, VIRTUAL_THRESHOLD } from "@/lib/virtual";
import { useVirtualWindow } from "./useVirtualWindow";

/** jsdom gives every element clientHeight 0; fake the two metrics the hook reads. */
function fakeScroller(clientHeight: number, scrollTop: number) {
  const el = document.createElement("div");
  Object.defineProperty(el, "clientHeight", { value: clientHeight, configurable: true });
  let top = scrollTop;
  Object.defineProperty(el, "scrollTop", {
    get: () => top,
    set: (v: number) => {
      top = v;
    },
    configurable: true,
  });
  return el;
}

/** The hook exposes scrollRef readonly; tests attach the fake element directly. */
function attach(result: { scrollRef: { current: HTMLDivElement | null } }, el: HTMLDivElement) {
  (result.scrollRef as { current: HTMLDivElement | null }).current = el;
}

describe("useVirtualWindow", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("renders the full list below the threshold", () => {
    const { result } = renderHook(() => useVirtualWindow({ count: 10, rowHeight: 20 }));

    expect(result.current.virtualized).toBe(false);
    expect(result.current.start).toBe(0);
    expect(result.current.end).toBe(10);
    expect(result.current.offsetTop).toBe(0);
    expect(result.current.totalHeight).toBe(200);
    expect(result.current.visibleCount).toBe(10);
  });

  it("activates at the threshold and mirrors computeVirtualRange", () => {
    const { result } = renderHook(() =>
      useVirtualWindow({ count: VIRTUAL_THRESHOLD, rowHeight: 30 }),
    );

    expect(result.current.virtualized).toBe(true);
    // No element is attached, so the hook keeps its 480px viewport default.
    const expected = computeVirtualRange(VIRTUAL_THRESHOLD, 30, 0, 480, 6);
    expect({
      start: result.current.start,
      end: result.current.end,
      offsetTop: result.current.offsetTop,
      totalHeight: result.current.totalHeight,
      visibleCount: result.current.visibleCount,
    }).toEqual(expected);
  });

  it("measures the attached scroller when windowing starts", () => {
    const { result, rerender } = renderHook(
      ({ count }: { count: number }) => useVirtualWindow({ count, rowHeight: 30 }),
      { initialProps: { count: 60 } },
    );
    attach(result.current, fakeScroller(300, 0));

    // Changing count re-runs the measure effect with the element attached.
    rerender({ count: 61 });

    const expected = computeVirtualRange(61, 30, 0, 300, 6);
    expect(result.current.end).toBe(expected.end);
    expect(result.current.visibleCount).toBe(expected.visibleCount);
  });

  it("updates the range from the live scrollTop, throttled to one rAF per frame", () => {
    vi.useFakeTimers({ toFake: ["requestAnimationFrame", "cancelAnimationFrame"] });
    const { result } = renderHook(() => useVirtualWindow({ count: 100, rowHeight: 30 }));
    attach(result.current, fakeScroller(480, 300));
    const raf = vi.spyOn(window, "requestAnimationFrame");

    act(() => {
      result.current.onScroll();
      result.current.onScroll();
    });
    expect(raf).toHaveBeenCalledTimes(1);

    act(() => {
      vi.advanceTimersByTime(16);
    });
    const expected = computeVirtualRange(100, 30, 300, 480, 6);
    expect(result.current.start).toBe(expected.start);
    expect(result.current.offsetTop).toBe(expected.offsetTop);
  });

  it("ignores scroll events while windowing is inactive", () => {
    const raf = vi.spyOn(window, "requestAnimationFrame");
    const { result } = renderHook(() => useVirtualWindow({ count: 5, rowHeight: 20 }));

    act(() => {
      result.current.onScroll();
    });
    expect(raf).not.toHaveBeenCalled();
  });

  it("honours enabled:false on a large list", () => {
    const { result } = renderHook(() =>
      useVirtualWindow({ count: 100, rowHeight: 30, enabled: false }),
    );

    expect(result.current.virtualized).toBe(false);
    expect(result.current.end).toBe(100);
  });

  it("cancels a pending rAF on unmount", () => {
    vi.useFakeTimers({ toFake: ["requestAnimationFrame", "cancelAnimationFrame"] });
    // jsdom has no ResizeObserver; without it the effect bails out before
    // registering its cleanup, so the cancel path needs the stub.
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        disconnect() {}
      },
    );
    const cancel = vi.spyOn(window, "cancelAnimationFrame");
    const { result, rerender, unmount } = renderHook(
      ({ count }: { count: number }) => useVirtualWindow({ count, rowHeight: 30 }),
      { initialProps: { count: 60 } },
    );
    attach(result.current, fakeScroller(480, 0));
    rerender({ count: 61 });
    act(() => {
      result.current.onScroll();
    });

    unmount();
    expect(cancel).toHaveBeenCalled();
  });
});
