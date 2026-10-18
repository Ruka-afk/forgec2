import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useCollectTask } from "./useCollectTask";
import { collectTaskResult } from "./task-collect";
import { api } from "@/lib/api";
import { toast } from "sonner";

vi.mock("./task-collect", () => ({
  collectTaskResult: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => 5),
}));

vi.mock("@/lib/i18n", () => {
  // `t` must be referentially stable: useErrorToast memoizes on it.
  const t = (key: string) => `T:${key}`;
  return { useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const mockedCollect = vi.mocked(collectTaskResult);
const mockedPost = vi.mocked(api.post);

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

  it("surfaces the failure text and leaves result untouched", async () => {
    mockedCollect.mockRejectedValue(new Error("agent is offline"));
    const { result } = renderHook(() => useCollectTask("agent-1"));

    let out: string | null = "sentinel";
    await act(async () => {
      out = await result.current.collect("users", "/p", { errorKey: "agents.recon_failed" });
    });

    expect(out).toBeNull();
    expect(result.current.result).toBeNull();
    expect(result.current.busy).toBeNull();
    expect(toast.error).toHaveBeenCalledWith("agent is offline");
  });

  it("localizes a mapped status instead of showing the raw text", async () => {
    mockedCollect.mockRejectedValue({ status: 429, message: "slow down" });
    const { result } = renderHook(() => useCollectTask("agent-1"));

    await act(async () => {
      await result.current.collect("av", "/p", { errorKey: "agents.recon_failed" });
    });

    expect(toast.error).toHaveBeenCalledWith("T:common.rate_limited");
  });

  it("surfaces a 4xx backend message over the fallback key", async () => {
    mockedCollect.mockRejectedValue({ status: 400, message: "target agent is offline" });
    const { result } = renderHook(() => useCollectTask("agent-1"));

    await act(async () => {
      await result.current.collect("av", "/p", { errorKey: "agents.recon_failed" });
    });

    expect(toast.error).toHaveBeenCalledWith("target agent is offline");
  });

  it("falls back to the context key when the failure carries no text", async () => {
    mockedCollect.mockRejectedValue(new Error(""));
    const { result } = renderHook(() => useCollectTask("agent-1"));

    await act(async () => {
      await result.current.collect("av", "/p", { errorKey: "agents.recon_failed" });
    });

    expect(toast.error).toHaveBeenCalledWith("T:agents.recon_failed");
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

  it("drops a fire while a collect is in flight", async () => {
    let release!: (v: string) => void;
    mockedCollect.mockReturnValue(new Promise<string>((res) => { release = res; }));
    const { result } = renderHook(() => useCollectTask("agent-1"));

    let first: Promise<string | null>;
    act(() => {
      first = result.current.collect("a", "/a");
    });
    let ok = true;
    await act(async () => {
      ok = await result.current.fire("b", "/b");
    });

    expect(ok).toBe(false);
    expect(mockedPost).not.toHaveBeenCalled();
    await act(async () => {
      release("ok");
      await first;
    });
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

  it("drops an in-flight completion when the agent switches", async () => {
    let release!: (v: string) => void;
    mockedCollect.mockReturnValue(new Promise<string>((res) => { release = res; }));
    const { result, rerender } = renderHook(({ id }) => useCollectTask(id), {
      initialProps: { id: "agent-1" },
    });

    let first: Promise<string | null>;
    act(() => {
      first = result.current.collect("drives", "/p", { successText: "done", errorKey: "agents.recon_failed" });
    });
    expect(result.current.busy).toBe("drives");

    rerender({ id: "agent-2" });
    expect(result.current.busy).toBeNull();

    let out: string | null = "sentinel";
    await act(async () => {
      release("late");
      out = await first;
    });

    // The completion belongs to the previous agent: no result, no toast, and
    // the caller gets `null` instead of a value it might render for the new one.
    expect(out).toBeNull();
    expect(result.current.result).toBeNull();
    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("fires a request, toasts success and reports true", async () => {
    mockedPost.mockResolvedValue(undefined);
    const { result } = renderHook(() => useCollectTask("agent-1"));

    let ok = false;
    await act(async () => {
      ok = await result.current.fire("set", "/agents/agent-1/clipboard/set", {
        body: { data: "clip" },
        successText: "done",
      });
    });

    expect(ok).toBe(true);
    expect(mockedPost).toHaveBeenCalledWith("/agents/agent-1/clipboard/set", { data: "clip" });
    expect(toast.success).toHaveBeenCalledWith("done");
    expect(result.current.busy).toBeNull();
  });

  it("routes a fire failure through the normalizer and reports false", async () => {
    mockedPost.mockRejectedValue({ status: 429, message: "slow down" });
    const { result } = renderHook(() => useCollectTask("agent-1"));

    let ok = true;
    await act(async () => {
      ok = await result.current.fire("window", "/p", { errorKey: "agents.trigger_failed" });
    });

    expect(ok).toBe(false);
    expect(toast.error).toHaveBeenCalledWith("T:common.rate_limited");
    expect(result.current.busy).toBeNull();
  });
});
