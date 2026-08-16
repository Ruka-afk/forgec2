import { describe, it, expect } from "vitest";
import { applyTaskUpdate } from "./useAgentTaskSync";
import type { AgentDetailResponse, TaskEntry } from "../_components/agent-detail-utils";

function snapshot(tasks: TaskEntry[] = [], stats: Partial<AgentDetailResponse> = {}): AgentDetailResponse {
  return { agent: {}, tasks, total_tasks: tasks.length, pending_tasks: 0, completed_tasks: 0, failed_tasks: 0, ...stats };
}

describe("applyTaskUpdate", () => {
  it("inserts a brand-new task at the head with stats bump", () => {
    const prev = snapshot([
      { id: 10, type: "shell", status: "completed", result: "old" },
    ], { total_tasks: 2, completed_tasks: 1 });
    const next = applyTaskUpdate(prev, {
      type: "task_update",
      agent_id: "a1",
      task_id: 11,
      task_type: "screenshot",
      status: "pending",
      command: "",
    });
    expect(next.tasks?.length).toBe(2);
    expect(next.tasks?.[0]).toMatchObject({ id: 11, type: "screenshot", status: "pending" });
    expect(next.tasks?.[1]?.id).toBe(10);
    expect(next.total_tasks).toBe(3);
    expect(next.pending_tasks).toBe(1);
    expect(next.completed_tasks).toBe(1);
  });

  it("flips existing pending -> completed and adjusts stats once", () => {
    const prev = snapshot([
      { id: 5, type: "shell", status: "pending" },
    ], { total_tasks: 3, pending_tasks: 1, completed_tasks: 1, failed_tasks: 1 });
    const next = applyTaskUpdate(prev, {
      type: "task_update",
      agent_id: "a1",
      task_id: 5,
      task_type: "shell",
      status: "completed",
      result: "whoami",
    });
    expect(next.tasks?.[0]?.status).toBe("completed");
    expect(next.pending_tasks).toBe(0);
    expect(next.completed_tasks).toBe(2);
    expect(next.failed_tasks).toBe(1);
    expect(next.total_tasks).toBe(3);
  });

  it("never replaces an existing full result with the truncated WS preview", () => {
    const prev = snapshot([
      { id: 7, type: "shell", status: "completed", result: "FULL-RESULT-BODY" },
    ], { completed_tasks: 1 });
    const next = applyTaskUpdate(prev, {
      type: "task_update",
      agent_id: "a1",
      task_id: 7,
      status: "completed",
      result: "preview…",
    });
    expect(next.tasks?.[0]?.result).toBe("FULL-RESULT-BODY");
  });

  it("fills an empty result with the preview", () => {
    const prev = snapshot([{ id: 7, type: "shell", status: "completed" }], { completed_tasks: 1 });
    const next = applyTaskUpdate(prev, {
      type: "task_update",
      agent_id: "a1",
      task_id: 7,
      status: "completed",
      result: "preview…",
    });
    expect(next.tasks?.[0]?.result).toBe("preview…");
  });

  it("ignores frames with invalid task ids", () => {
    const prev = snapshot([{ id: 1, type: "shell", status: "completed" }], { completed_tasks: 1 });
    const next = applyTaskUpdate(prev, { type: "task_update", agent_id: "a1", task_id: 0, status: "completed" });
    expect(next).toBe(prev);
  });

  it("no-ops on same-status frames", () => {
    const prev = snapshot([{ id: 3, type: "shell", status: "completed" }], { completed_tasks: 1 });
    const next = applyTaskUpdate(prev, { type: "task_update", agent_id: "a1", task_id: 3, status: "completed" });
    expect(next).toBe(prev);
  });

  it("adjusts failed -> pending", () => {
    const prev = snapshot([{ id: 3, type: "shell", status: "failed" }], { total_tasks: 2, failed_tasks: 1, pending_tasks: 0 });
    const next = applyTaskUpdate(prev, { type: "task_update", agent_id: "a1", task_id: 3, status: "pending" });
    expect(next.tasks?.[0]?.status).toBe("pending");
    expect(next.failed_tasks).toBe(0);
    expect(next.pending_tasks).toBe(1);
  });
});