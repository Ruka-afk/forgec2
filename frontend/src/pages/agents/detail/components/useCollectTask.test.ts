import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useCollectTask } from "./useCollectTask";
import { collectTaskResult } from "./task-collect";
import { toast } from "sonner";

vi.mock("./task-collect", () => ({
  collectTaskResult: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn() },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const mockedCollect = vi.mocked(collectTaskResult);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useCollectTask", () => {
  it("stores the collected output and clears busy on success", async () => {
    mockedCollect.mockResolvedValue("whoami-output");
    const { result } = renderHook(() => useCollectTask("agent-1"));

    let out: string | null = null;
    await act(async () => {
      out = await result.current.collect("drives", "/agents/agent-1/drives");
    });

    expect(out).toBe("whoami-output");
    expect(result.current.result).toBe("whoami-output");
    expect(result.current.busy).toBeNull();
    expect(mockedCollect).toHaveBeenCalledWith("agent-1", "/agents/agent-1/drives", undefined, undefined);
  });

  it("falls back to emptyText and toasts on success when requested", async () => {
    mockedCollect.mockResolvedValue("");
    const { result } = renderHook(() => useCollectTask("agent-1"));

    await act(async () => {
      await result.current.collect("av", "/p", { emptyText: "(empty)", successText: "done" });
    });

    expect(result.current.result).toBe("(empty)");
    expect(toast.success).toHaveBeenCalledWith("done");
  });

  it("reports failure via toast and leaves result untouched", async () => {
    mockedCollect.mockRejectedValue(new Error("task failed"));
    const { result } = renderHook(() => useCollectTask("agent-1"));

    let out: string | null = "sentinel";
    await act(async () => {
      out = await result.current.collect("users", "/p", { errorText: "Recon failed" });
    });

    expect(out).toBeNull();
    expect(result.current.result).toBeNull();
    expect(result.current.busy).toBeNull();
    expect(toast.error).toHaveBeenCalledWith("Recon failed");
  });

  it("stays silent on AbortError", async () => {
    const abort = new Error("aborted");
    abort.name = "AbortError";
    mockedCollect.mockRejectedValue(abort);
    const { result } = renderHook(() => useCollectTask("agent-1"));

    await act(async () => {
      await result.current.collect("users", "/p");
    });

    expect(toast.error).not.toHaveBeenCalled();
    expect(result.current.busy).toBeNull();
  });

  it("ignores a second collect while busy", async () => {
    let release!: (v: string) => void;
    mockedCollect.mockReturnValue(new Promise<string>((res) => { release = res; }));
    const { result } = renderHook(() => useCollectTask("agent-1"));

    let first: Promise<string | null>;
    act(() => {
      first = result.current.collect("a", "/a");
    });
    let second: string | null = "sentinel";
    await act(async () => {
      second = await result.current.collect("b", "/b");
    });

    expect(second).toBeNull();
    expect(mockedCollect).toHaveBeenCalledTimes(1);
    await act(async () => {
      release("late");
      await first;
    });
    expect(result.current.result).toBe("late");
  });

  it("skips storing when storeResult is false but still returns output", async () => {
    mockedCollect.mockResolvedValue("ack: started");
    const { result } = renderHook(() => useCollectTask("agent-1"));

    let out: string | null = null;
    await act(async () => {
      out = await result.current.collect("start", "/p", { storeResult: false, successText: "on" });
    });

    expect(out).toBe("ack: started");
    expect(result.current.result).toBeNull();
    expect(toast.success).toHaveBeenCalledWith("on");
  });

  it("reset clears the stored result", async () => {
    mockedCollect.mockResolvedValue("x");
    const { result } = renderHook(() => useCollectTask("agent-1"));

    await act(async () => {
      await result.current.collect("a", "/a");
    });
    act(() => {
      result.current.reset();
    });
    expect(result.current.result).toBeNull();
  });
});
