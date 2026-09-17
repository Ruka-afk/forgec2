import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "@/lib/api";
import { useTaskResult } from "./useTaskResult";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn() },
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

describe("useTaskResult", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it("stops polling after a terminal result", async () => {
    vi.useFakeTimers();
    vi.mocked(api.get).mockResolvedValue({ status: "completed", result: "done" });
    const { result } = renderHook(() => useTaskResult("agent-1", 1000, 10));

    act(() => result.current.start(11));
    await act(async () => { await vi.advanceTimersByTimeAsync(0); });
    expect(result.current.status).toBe("completed");
    expect(result.current.result).toBe("done");

    await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
    expect(api.get).toHaveBeenCalledTimes(1);
  });

  it("starts a replacement task while the previous request is still pending", async () => {
    const first = deferred<{ status: string; result: string }>();
    vi.mocked(api.get)
      .mockImplementationOnce(() => first.promise)
      .mockResolvedValueOnce({ status: "completed", result: "new-result" });
    const { result } = renderHook(() => useTaskResult("agent-1", 1000, 10));

    act(() => result.current.start(11));
    await waitFor(() => expect(api.get).toHaveBeenCalledTimes(1));
    act(() => result.current.start(12));

    await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.current.status).toBe("completed"));
    expect(result.current.result).toBe("new-result");

    await act(async () => { first.resolve({ status: "completed", result: "stale" }); });
    expect(result.current.result).toBe("new-result");
  });

  it("stalls instead of timing out at maxAttempts, then times out at the hard deadline", async () => {
    vi.useFakeTimers();
    vi.mocked(api.get).mockResolvedValue({ status: "running", result: "" });
    const { result } = renderHook(() => useTaskResult("agent-1", 1000, 3));

    act(() => result.current.start(11));
    await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
    // Past 3 attempts: still tracking, flagged stalled, no false timeout.
    expect(result.current.stalled).toBe(true);
    expect(result.current.status).toBe("running");
    expect(result.current.polling).toBe(true);

    // A late completion still wins while stalled.
    vi.mocked(api.get).mockResolvedValueOnce({ status: "completed", result: "late" });
    await act(async () => { await vi.advanceTimersByTimeAsync(20000); });
    expect(result.current.status).toBe("completed");
    expect(result.current.result).toBe("late");
  });

  it("reports timeout only after the hard deadline", async () => {
    vi.useFakeTimers();
    vi.mocked(api.get).mockResolvedValue({ status: "running", result: "" });
    const { result } = renderHook(() => useTaskResult("agent-1", 1000, 2));

    act(() => result.current.start(11));
    await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
    expect(result.current.status).not.toBe("timeout");
    await act(async () => { await vi.advanceTimersByTimeAsync(600_000); });
    expect(result.current.status).toBe("timeout");
    expect(result.current.polling).toBe(false);
  });
});
