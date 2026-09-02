import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

type WSListener = (msg: Record<string, unknown>) => void;
const listeners: WSListener[] = [];

vi.mock("./wsContext", () => ({
  onWSMessage: (l: WSListener) => {
    listeners.push(l);
    return () => {
      const i = listeners.indexOf(l);
      if (i >= 0) listeners.splice(i, 1);
    };
  },
}));

import { pollTask, api } from "./api";

function emit(msg: Record<string, unknown>) {
  for (const l of [...listeners]) l(msg);
}

let completed = false;

describe("pollTask streaming", () => {
  beforeEach(() => {
    listeners.length = 0;
    completed = false;
    vi.spyOn(api, "get").mockImplementation(async () => {
      if (completed) return { id: 1, status: "completed", result: "FULL-RESULT" };
      return { id: 1, status: "pending", result: "" };
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    listeners.length = 0;
    completed = false;
  });

  it("forwards accumulated output via onStatus as task_output frames arrive", async () => {
    const seen: Array<{ status: string; result?: string }> = [];
    const p = pollTask("agent-1", 1, {
      intervalMs: 60000,
      timeoutMs: 60000,
      onStatus: (st) => seen.push({ status: st.status, result: st.result }),
    });

    await Promise.resolve();
    emit({ type: "task_output", task_id: 1, chunk: "part-1-", done: false });
    await Promise.resolve();
    emit({ type: "task_output", task_id: 1, chunk: "part-2", done: false });
    await Promise.resolve();
    completed = true;
    emit({ type: "task_output", task_id: 1, chunk: "-end", done: true });

    const final = await p;
    expect(seen.filter((s) => s.status !== "pending").map((s) => s.result)).toEqual([
      "part-1-",
      "part-1-part-2",
      "part-1-part-2-end",
    ]);
    // final result comes from the authoritative task detail fetch
    expect(final.result).toBe("FULL-RESULT");
    expect(final.status).toBe("completed");
  });

  it("completes on task_update even without streaming frames", async () => {
    const seen: Array<{ status: string; result?: string }> = [];
    const p = pollTask("agent-1", 1, {
      intervalMs: 60000,
      timeoutMs: 60000,
      onStatus: (st) => seen.push({ status: st.status, result: st.result }),
    });

    await Promise.resolve();
    emit({ type: "task_update", task_id: 1, status: "running" });
    await Promise.resolve();
    completed = true;
    emit({ type: "task_update", task_id: 1, status: "completed", result: "preview" });

    const final = await p;
    expect(final.status).toBe("completed");
    expect(final.result).toBe("FULL-RESULT");
    expect(seen.some((status) => status.result === "preview")).toBe(false);
  });

  it("uses an explicitly complete shell result when the detail fetch fails", async () => {
    const seen: Array<{ status: string; result?: string }> = [];
    const p = pollTask("agent-1", 1, {
      intervalMs: 60000,
      timeoutMs: 60000,
      onStatus: (st) => seen.push({ status: st.status, result: st.result }),
    });

    await Promise.resolve();
    vi.mocked(api.get).mockRejectedValue(new Error("detail unavailable"));
    emit({
      type: "task_update",
      task_id: 1,
      status: "completed",
      result: "COMPLETE-SHELL-OUTPUT",
      result_complete: true,
    });

    const final = await p;
    expect(seen.some((status) => status.result === "COMPLETE-SHELL-OUTPUT")).toBe(true);
    expect(final.result).toBe("COMPLETE-SHELL-OUTPUT");
  });

  it("does not let an empty final detail erase complete WebSocket output", async () => {
    const p = pollTask("agent-1", 1, {
      intervalMs: 60000,
      timeoutMs: 60000,
    });

    await Promise.resolve();
    vi.mocked(api.get).mockResolvedValue({ id: 1, status: "completed", result: "" });
    emit({
      type: "task_update",
      task_id: 1,
      status: "completed",
      result: "COMPLETE-SHELL-OUTPUT",
      result_complete: true,
    });

    const final = await p;
    expect(final.result).toBe("COMPLETE-SHELL-OUTPUT");
  });

  it("ignores an older REST snapshot that resolves after a terminal event", async () => {
    let releaseInitial: ((value: { id: number; status: "pending"; result: string }) => void) | undefined;
    vi.mocked(api.get).mockImplementationOnce(() => new Promise((resolve) => {
      releaseInitial = resolve;
    }));

    const seen: Array<{ status: string; result?: string }> = [];
    const p = pollTask("agent-1", 1, {
      intervalMs: 60000,
      timeoutMs: 60000,
      onStatus: (st) => seen.push({ status: st.status, result: st.result }),
    });

    await Promise.resolve();
    completed = true;
    emit({
      type: "task_update",
      task_id: 1,
      status: "completed",
      result: "COMPLETE-SHELL-OUTPUT",
      result_complete: true,
    });
    releaseInitial?.({ id: 1, status: "pending", result: "" });

    const final = await p;
    expect(final.status).toBe("completed");
    expect(seen.at(-1)?.status).toBe("completed");
  });

  it("preserves a terminal WebSocket state when the final REST read still lags", async () => {
    vi.mocked(api.get).mockResolvedValue({ id: 1, status: "pending", result: "" });
    const p = pollTask("agent-1", 1, { intervalMs: 60000, timeoutMs: 60000 });

    await Promise.resolve();
    emit({
      type: "task_update",
      task_id: 1,
      status: "completed",
      result: "COMPLETE-SHELL-OUTPUT",
      result_complete: true,
    });

    const final = await p;
    expect(final.status).toBe("completed");
    expect(final.result).toBe("COMPLETE-SHELL-OUTPUT");
  });

  it("treats cancellation as terminal instead of polling until timeout", async () => {
    vi.mocked(api.get).mockResolvedValue({ id: 1, status: "pending", result: "" });
    const p = pollTask("agent-1", 1, { intervalMs: 60000, timeoutMs: 60000 });

    await Promise.resolve();
    emit({ type: "task_update", task_id: 1, status: "cancelled", error: "cancelled by operator" });

    const final = await p;
    expect(final.status).toBe("cancelled");
    expect(final.error).toBe("cancelled by operator");
  });

  it("never exposes at-rest ciphertext while recovering the final plaintext", async () => {
    const seen: Array<{ status: string; result?: string }> = [];
    const p = pollTask("agent-1", 1, {
      intervalMs: 60000,
      timeoutMs: 60000,
      onStatus: (st) => seen.push({ status: st.status, result: st.result }),
    });

    await Promise.resolve();
    completed = true;
    emit({ type: "task_update", task_id: 1, status: "completed", result: "FC2ENC:ciphertext" });

    const final = await p;
    expect(seen.some((status) => status.result?.startsWith("FC2ENC:"))).toBe(false);
    expect(final.result).toBe("FULL-RESULT");
  });

  it("ignores task_output frames for other tasks", async () => {
    const seen: Array<{ status: string; result?: string }> = [];
    const p = pollTask("agent-1", 2, {
      intervalMs: 60000,
      timeoutMs: 60000,
      onStatus: (st) => seen.push({ status: st.status, result: st.result }),
    });

    await Promise.resolve();
    emit({ type: "task_output", task_id: 1, chunk: "foreign", done: true });
    await Promise.resolve();
    expect(seen.filter((s) => s.status !== "pending").length).toBe(0);

    completed = true;
    emit({ type: "task_update", task_id: 2, status: "completed", result: "ok" });
    const final = await p;
    expect(final.status).toBe("completed");
  });
});
