import type { AgentTaskRecord, TaskStatus } from "@/types/agent";

const TASK_EVENT_TYPES = new Set(["task_update", "task_created", "task_output"]);

export function isDockTaskEvent(msg: Record<string, unknown>, agentId: string): boolean {
  if (!agentId) return false;
  if (!TASK_EVENT_TYPES.has(String(msg.type || ""))) return false;
  return String(msg.agent_id ?? "") === agentId;
}

export function shouldRevealTaskResult(msg: Record<string, unknown>): boolean {
  const type = String(msg.type || "");
  if (type === "task_output") return msg.done === true;
  if (type !== "task_update" && type !== "task_created") return false;
  const status = String(msg.status || "");
  return status === "completed" || status === "failed";
}

function asTaskStatus(raw: unknown, fallback: TaskStatus = "pending"): TaskStatus {
  const s = String(raw || "");
  if (s === "pending" || s === "running" || s === "completed" || s === "failed" || s === "cancelled" || s === "pending_approval") return s;
  return fallback;
}

export function applyTaskEvent(
  tasks: AgentTaskRecord[],
  msg: Record<string, unknown>,
): AgentTaskRecord[] {
  const id = Number(msg.task_id ?? msg.id);
  if (!Number.isFinite(id) || id <= 0) return tasks;
  const idx = tasks.findIndex((t) => Number(t.id) === id);
  const prev = idx >= 0 ? tasks[idx] : undefined;

  if (String(msg.type) === "task_output") {
    const chunk = String(msg.chunk ?? "");
    const next: AgentTaskRecord = {
      id,
      type: prev?.type || "shell",
      command: prev?.command || "",
      status: msg.done === true ? "completed" : "running",
      created_at: prev?.created_at || new Date().toISOString(),
      result: `${prev?.result || ""}${chunk}`,
      error: prev?.error,
      created_by: prev?.created_by,
    };
    if (idx < 0) return [next, ...tasks];
    const copy = tasks.slice();
    copy[idx] = next;
    return copy;
  }

  const next: AgentTaskRecord = {
    id,
    type: String(msg.task_type ?? prev?.type ?? ""),
    command: String(msg.command ?? prev?.command ?? ""),
    status: asTaskStatus(msg.status, prev?.status ?? "pending"),
    created_at: prev?.created_at || new Date().toISOString(),
    result: msg.result != null ? String(msg.result) : prev?.result,
    error: msg.error != null ? String(msg.error) : prev?.error,
    created_by: msg.created_by != null ? String(msg.created_by) : prev?.created_by,
  };
  if (idx < 0) return [next, ...tasks];
  const copy = tasks.slice();
  copy[idx] = { ...prev, ...next };
  return copy;
}

export function canCancelTask(status: string | undefined): boolean {
  return status === "pending" || status === "running";
}

export function canReviewTask(status: string | undefined): boolean {
  return status === "pending_approval";
}

export function canApproveOwnTask(createdBy: string | undefined, username: string | undefined): boolean {
  const owner = (createdBy || "").trim();
  const me = (username || "").trim();
  if (!owner || owner === "ai" || owner === "automation" || owner === "system") return true;
  if (!me) return true;
  return owner !== me;
}

export function taskEventId(msg: Record<string, unknown>): number | null {
  const id = Number(msg.task_id ?? msg.id);
  return Number.isFinite(id) && id > 0 ? id : null;
}
