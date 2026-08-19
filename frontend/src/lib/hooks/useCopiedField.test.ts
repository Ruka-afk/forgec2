import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useCopiedField } from "./useCopiedField";

describe("useCopiedField", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("marks the copied key and reverts after the interval", async () => {
    const { result } = renderHook(() => useCopiedField());

    act(() => { void result.current.copy("text", "k1"); });
    await act(async () => { await vi.advanceTimersByTimeAsync(0); });
    expect(result.current.copiedField).toBe("k1");

    act(() => { void result.current.copy("other", "k1"); });
    await act(async () => { await vi.advanceTimersByTimeAsync(1499); });
    expect(result.current.copiedField).toBe("k1");
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(result.current.copiedField).toBe("");
  });

  it("replaces a pending revert when a new key is copied", async () => {
    const { result } = renderHook(() => useCopiedField());

    act(() => { void result.current.copy("a", "k1"); });
    await act(async () => { await vi.advanceTimersByTimeAsync(0); });
    act(() => { void result.current.copy("b", "k2"); });
    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    expect(result.current.copiedField).toBe("k2");
    await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(result.current.copiedField).toBe("");
  });

  it("stays idle when the clipboard write is rejected", async () => {
    (navigator.clipboard.writeText as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(new Error("denied"));
    const { result } = renderHook(() => useCopiedField());

    act(() => { void result.current.copy("a", "k1"); });
    await act(async () => { await vi.advanceTimersByTimeAsync(0); });
    expect(result.current.copiedField).toBe("");
  });
});