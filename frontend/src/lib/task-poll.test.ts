import { describe, expect, it } from "vitest";
import {
  TASK_HARD_TIMEOUT_MS,
  TASK_HIDDEN_POLL_MS,
  TASK_POLL_INTERVAL_MS,
  TASK_SOFT_TIMEOUT_MS,
  isActiveTaskStatus,
  isTerminalTaskStatus,
} from "./task-poll";

describe("task-poll deadlines", () => {
  it("orders soft < hard with sane cadences", () => {
    expect(TASK_POLL_INTERVAL_MS).toBeGreaterThan(0);
    expect(TASK_SOFT_TIMEOUT_MS).toBeGreaterThan(TASK_POLL_INTERVAL_MS);
    expect(TASK_HARD_TIMEOUT_MS).toBeGreaterThan(TASK_SOFT_TIMEOUT_MS);
    expect(TASK_HIDDEN_POLL_MS).toBeGreaterThan(TASK_POLL_INTERVAL_MS);
  });

  it("pins the hard deadline to the server stale sweep (10m)", () => {
    // StaleRunningTaskTimeout in internal/server/constants.go. If the server
    // value moves, this test forces the UI contract to move with it.
    expect(TASK_HARD_TIMEOUT_MS).toBe(600_000);
  });

  it("classifies terminal states including aliases", () => {
    for (const st of ["completed", "success", "done", "failed", "error", "cancelled"]) {
      expect(isTerminalTaskStatus(st)).toBe(true);
    }
    for (const st of ["pending", "running", "sent", "pending_approval", ""]) {
      expect(isTerminalTaskStatus(st)).toBe(false);
    }
  });

  it("classifies active states for continued polling", () => {
    for (const st of ["pending", "PENDING", "running", "sent", "pending_approval"]) {
      expect(isActiveTaskStatus(st)).toBe(true);
    }
    for (const st of ["completed", "failed", "cancelled", "timeout", "idle"]) {
      expect(isActiveTaskStatus(st)).toBe(false);
    }
  });
});
