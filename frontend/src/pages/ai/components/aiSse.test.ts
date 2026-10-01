import { afterEach, describe, expect, it, vi } from "vitest";
import { AI_SSE_CONNECT_TIMEOUT_MS, fetchAISSE } from "./aiSse";

describe("fetchAISSE", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("propagates the parent abort to the active request", async () => {
    const parent = new AbortController();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation((_input, init) => {
      return new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener("abort", () => reject(init.signal?.reason));
      });
    });

    const request = fetchAISSE("/api/ai/runs/run-1/events", { method: "GET" }, parent.signal);
    parent.abort("cancelled");

    await expect(request).rejects.toBe("cancelled");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
  });

  it("aborts a request that never reaches the SSE headers", async () => {
    vi.useFakeTimers();
    const parent = new AbortController();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(() => new Promise<Response>(() => {}));

    void fetchAISSE("/api/ai/runs/run-2/events", { method: "GET" }, parent.signal);
    await vi.advanceTimersByTimeAsync(AI_SSE_CONNECT_TIMEOUT_MS);

    const signal = fetchMock.mock.calls[0]?.[1]?.signal;
    expect(signal?.aborted).toBe(true);
    expect(signal?.reason).toBeInstanceOf(DOMException);
    expect((signal?.reason as DOMException).name).toBe("TimeoutError");
    parent.abort();
    // The mocked fetch intentionally never settles; the assertion above verifies
    // the signal observed by the request without leaving an open timer.
    await Promise.resolve();
  });
});
