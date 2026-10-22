import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";

const mocks = vi.hoisted(() => {
  class MockApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  }
  return {
    get: vi.fn(),
    post: vi.fn(),
    runTask: vi.fn(),
    ApiError: MockApiError,
  };
});

vi.mock("@/lib/api", () => ({
  api: { get: mocks.get, post: mocks.post },
  runTask: mocks.runTask,
  ApiError: mocks.ApiError,
}));

import { useAgentProcessTree } from "./useAgentProcessTree";
import { paths } from "@/lib/api-paths";

function setup(agentId = "a1") {
  return renderHook(() => useAgentProcessTree(agentId));
}

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
});

describe("useAgentProcessTree", () => {
  it("treats a 404 snapshot as empty instead of a load failure", async () => {
    mocks.get.mockRejectedValue(new mocks.ApiError(404, "No completed process tree"));
    const { result } = setup();

    await act(async () => {
      await result.current.load();
    });

    expect(mocks.get).toHaveBeenCalledWith(paths.agents.processTree("a1"), expect.any(Object));
    expect(result.current.snapshot).toBeNull();
    expect(result.current.loadFailed).toBe(false);
    expect(result.current.loading).toBe(false);
  });

  it("loads the last completed snapshot envelope", async () => {
    mocks.get.mockResolvedValue({
      processes: "PID\tPPID\tUSER\tNAME\n1\t0\troot\tinit",
      source: "process_tree",
      kind: "process_tree",
      live: false,
      task_id: 12,
      collected_at: "2026-10-21T12:00:00Z",
    });
    const { result } = setup();

    await act(async () => {
      await result.current.load();
    });

    expect(result.current.snapshot).toMatchObject({
      source: "process_tree",
      taskId: 12,
      collectedAt: "2026-10-21T12:00:00Z",
    });
    expect(result.current.loadFailed).toBe(false);
  });

  it("queues a fresh collection and then replaces the snapshot", async () => {
    mocks.runTask.mockResolvedValue({ status: "completed", id: 44, result: "--" });
    mocks.get.mockResolvedValue({
      processes: "PID\tPPID\tUSER\tNAME\n7\t1\troot\tsshd",
      source: "process_tree",
      kind: "process_tree",
      live: false,
      task_id: 44,
      collected_at: "2026-10-21T12:02:00Z",
    });
    const { result } = setup();

    await act(async () => {
      await result.current.refresh();
    });

    expect(mocks.runTask).toHaveBeenCalledWith(
      "a1",
      paths.agents.collectProcesses("a1"),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(mocks.get).toHaveBeenCalledWith(paths.agents.processTree("a1"), expect.any(Object));
    expect(result.current.snapshot).toMatchObject({ taskId: 44, text: expect.stringContaining("sshd") });
    expect(result.current.collecting).toBe(false);
  });

  it("falls back to the completed task result when the envelope read 404s", async () => {
    mocks.runTask.mockResolvedValue({
      status: "completed",
      id: 45,
      result: "PID\tPPID\tUSER\tNAME\n9\t1\troot\tcron",
    });
    mocks.get.mockRejectedValue(new mocks.ApiError(404, "still indexing"));
    const { result } = setup();

    await act(async () => {
      await result.current.refresh();
    });

    expect(result.current.snapshot).toMatchObject({
      text: expect.stringContaining("cron"),
      taskId: 45,
      source: "process_tree",
    });
  });

  it("keeps the previous snapshot and surfaces collection errors", async () => {
    mocks.get.mockResolvedValueOnce({
      processes: "PID\tPPID\tUSER\tNAME\n1\t0\troot\tinit",
      source: "process_tree",
      kind: "process_tree",
      live: false,
      task_id: 1,
    });
    mocks.runTask.mockResolvedValue({ status: "failed", id: 46, error: "agent offline" });
    const { result } = setup();

    await act(async () => {
      await result.current.load();
    });
    const previous = result.current.snapshot;

    await act(async () => {
      await expect(result.current.refresh()).rejects.toThrow("agent offline");
    });

    expect(result.current.snapshot).toBe(previous);
    expect(result.current.loadFailed).toBe(true);
    expect(result.current.collecting).toBe(false);
  });
});
