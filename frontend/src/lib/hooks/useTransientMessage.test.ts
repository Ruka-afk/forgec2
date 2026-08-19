import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useTransientMessage } from "./useTransientMessage";

describe("useTransientMessage", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("shows an info message until the duration elapses", () => {
    const { result } = renderHook(() => useTransientMessage());

    act(() => result.current.showMessage("hello"));
    expect(result.current.message).toEqual({ text: "hello", tone: "info" });

    act(() => vi.advanceTimersByTime(5000));
    expect(result.current.message).toBeNull();
  });

  it("supports explicit tone and custom duration", () => {
    const { result } = renderHook(() => useTransientMessage());

    act(() => result.current.showMessage("bad", { tone: "destructive", durationMs: 1000 }));
    expect(result.current.message?.tone).toBe("destructive");
    act(() => vi.advanceTimersByTime(999));
    expect(result.current.message).not.toBeNull();
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.message).toBeNull();
  });

  it("overrides a pending message and reschedules its timer", () => {
    const { result } = renderHook(() => useTransientMessage(2000));

    act(() => result.current.showMessage("first"));
    act(() => result.current.showMessage("second"));
    expect(result.current.message?.text).toBe("second");

    act(() => vi.advanceTimersByTime(1500));
    expect(result.current.message).not.toBeNull();
    act(() => vi.advanceTimersByTime(500));
    expect(result.current.message).toBeNull();
  });

  it("durationMs 0 keeps the message until cleared; clearMessage cancels the timer", () => {
    const { result } = renderHook(() => useTransientMessage());

    act(() => result.current.showMessage("sticky", { durationMs: 0 }));
    act(() => vi.advanceTimersByTime(100_000));
    expect(result.current.message?.text).toBe("sticky");

    act(() => result.current.showMessage("ticking"));
    act(() => result.current.clearMessage());
    expect(result.current.message).toBeNull();
  });
});