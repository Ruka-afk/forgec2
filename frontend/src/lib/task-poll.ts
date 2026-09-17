// Single source of truth for task-result polling deadlines (ms).
//
// The server keeps a task `running` until StaleRunningTaskTimeout (10 min)
// before the sweeps touch it. UI deadlines must never report a task dead
// before that: every "timeout" shown earlier is a lie that invites the
// operator to retry a live task and double-execute it.
//
// Contract:
// - SOFT: tell the operator the task is still running, keep tracking.
// - HARD: stop tracking and report timeout (matches the server sweep).
// - HIDDEN: background-tab cadence (browsers throttle timers anyway).
export const TASK_POLL_INTERVAL_MS = 1500;
export const TASK_SOFT_TIMEOUT_MS = 60_000;
export const TASK_HARD_TIMEOUT_MS = 600_000;
export const TASK_HIDDEN_POLL_MS = 10_000;

export type TaskTerminalStatus = "completed" | "failed" | "cancelled";

/** Server-terminal states plus the alias spellings agents/history use. */
export function isTerminalTaskStatus(status: string): boolean {
  return (
    status === "completed" ||
    status === "success" ||
    status === "done" ||
    status === "failed" ||
    status === "error" ||
    status === "cancelled"
  );
}

/** Active (non-terminal, non-idle) states eligible for continued polling. */
export function isActiveTaskStatus(status: string): boolean {
  const st = status.toLowerCase();
  return st === "pending" || st === "running" || st === "sent" || st === "pending_approval";
}
