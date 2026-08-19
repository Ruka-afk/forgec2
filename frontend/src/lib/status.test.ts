import { describe, it, expect } from "vitest";
import { AGENT_STATUSES, TASK_STATUSES, isAgentStatus, isTaskStatus } from "./status";

describe("status vocabulary", () => {
  it("exposes the canonical agent statuses", () => {
    expect(AGENT_STATUSES).toEqual(["online", "stale", "offline"]);
  });

  it("exposes the canonical task statuses", () => {
    expect(TASK_STATUSES).toEqual(["pending", "running", "completed", "failed", "cancelled", "pending_approval"]);
  });

  it("accepts every registered status", () => {
    for (const s of AGENT_STATUSES) expect(isAgentStatus(s)).toBe(true);
    for (const s of TASK_STATUSES) expect(isTaskStatus(s)).toBe(true);
  });

  it("rejects unknown, empty, and non-string values", () => {
    expect(isAgentStatus("burned")).toBe(false);
    expect(isTaskStatus("queued")).toBe(false);
    expect(isAgentStatus("")).toBe(false);
    expect(isTaskStatus(null)).toBe(false);
    expect(isAgentStatus(undefined)).toBe(false);
    expect(isTaskStatus(3)).toBe(false);
    expect(isAgentStatus({})).toBe(false);
  });
});