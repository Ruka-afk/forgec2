"use client";

import { useEffect } from "react";
import { useWS } from "@/lib/wsContext";
import type { AgentDetailResponse, TaskEntry } from "../_components/agent-detail-utils";

interface TaskUpdateFrame {
  type: string;
  agent_id?: string | number;
  task_id?: number;
  task_type?: string;
  status?: string;
  command?: string;
  created_by?: string;
  result?: string;
  error?: string;
  data?: Record<string, unknown>;
}

const CORRECTION_INTERVAL_MS = 30_000;

/**
 * Reconciles the periodic full-detail refresh with live WS events so busy
 * agents do not trigger a full-detail refetch per task_update:
 *
 * - task_update -> merge status/preview into the local task list (no refetch)
 * - agent_online / agent_offline -> throttled full reload
 * - agent_data_update -> merge payload into the local agent record
 * - periodic 30s correction -> throttled full reload (drift guard)
 */
export function useAgentTaskSync(
  agentId: string,
  online: boolean,
  setData: React.Dispatch<React.SetStateAction<AgentDetailResponse | null>>,
  reloadThrottled: (background?: boolean) => void,
) {
  const { subscribe } = useWS();

  useEffect(() => {
    if (!agentId) return;
    return subscribe((msg) => {
      if (String((msg as { agent_id?: unknown }).agent_id ?? "") !== agentId) return;
      if (msg.type === "agent_online" || msg.type === "agent_offline") {
        reloadThrottled();
      } else if (msg.type === "agent_data_update") {
        const patch = (msg as { data?: Record<string, unknown> }).data;
        if (!patch) return;
        setData((prev) =>
          prev ? { ...prev, agent: { ...(prev.agent || {}), ...patch } } : prev,
        );
      } else if (msg.type === "task_update") {
        setData((prev) => (prev ? applyTaskUpdate(prev, msg as TaskUpdateFrame) : prev));
      }
    });
  }, [agentId, subscribe, setData, reloadThrottled]);

  // Periodic correction: incremental merges can drift from server truth
  // (cancel/rerun/queueing outside this session), so re-sync on a timer.
  useEffect(() => {
    if (!agentId || !online) return;
    const iv = setInterval(() => reloadThrottled(), CORRECTION_INTERVAL_MS);
    return () => clearInterval(iv);
  }, [agentId, online, reloadThrottled]);
}

/** Pure merge of one task_update frame into a detail snapshot. */
export function applyTaskUpdate(
  prev: AgentDetailResponse,
  frame: TaskUpdateFrame,
): AgentDetailResponse {
  const taskId = Number(frame.task_id);
  if (!Number.isFinite(taskId) || taskId <= 0) return prev;

  const tasks = prev.tasks || [];
  const idx = tasks.findIndex((t) => Number(t.id ?? t.ID) === taskId);

  if (idx >= 0) {
    const existing = tasks[idx];
    const statusChanged = Boolean(frame.status) && frame.status !== existing.status;
    const resultFilled = Boolean(frame.result) && !existing.result;
    const errorFilled = Boolean(frame.error) && !existing.error;
    if (!statusChanged && !resultFilled && !errorFilled) return prev;

    const updated: TaskEntry = { ...existing };
    if (frame.status) updated.status = frame.status;
    // Only fill a missing result — full server results must not be replaced
    // by the truncated WS preview.
    if (resultFilled) updated.result = frame.result;
    if (errorFilled) updated.error = frame.error;

    const next = [...tasks];
    next[idx] = updated;

    const stats = adjustStats(prev, existing.status, frame.status);
    return stats ? { ...prev, ...stats, tasks: next } : { ...prev, tasks: next };
  }

  // Brand-new task: prepend a minimal record so it shows up instantly.
  const entry: TaskEntry = {
    id: taskId,
    type: frame.task_type,
    status: frame.status ?? "pending",
    command: frame.command,
    created_by: frame.created_by,
    result: frame.result,
    error: frame.error,
    created_at: new Date().toISOString(),
  };
  const stats = adjustStats(prev, undefined, frame.status);
  return { ...prev, ...stats, tasks: [entry, ...tasks] };
}

function adjustStats(
  prev: AgentDetailResponse,
  before: string | undefined,
  after: string | undefined,
): Partial<AgentDetailResponse> | null {
  if (!after || before === after) return null;
  const stats: Partial<AgentDetailResponse> = {};
  if (!before) {
    stats.total_tasks = (prev.total_tasks ?? prev.tasks?.length ?? 0) + 1;
    if (after === "pending") stats.pending_tasks = (prev.pending_tasks ?? 0) + 1;
    if (after === "failed") stats.failed_tasks = (prev.failed_tasks ?? 0) + 1;
    if (after === "completed") stats.completed_tasks = (prev.completed_tasks ?? 0) + 1;
    return stats;
  }
  const dec = (k: "pending_tasks" | "completed_tasks" | "failed_tasks") =>
    Math.max(0, (prev[k] ?? 0) - 1);
  const inc = (k: "pending_tasks" | "completed_tasks" | "failed_tasks") =>
    (prev[k] ?? 0) + 1;
  switch (after) {
    case "pending":
      stats.pending_tasks = inc("pending_tasks");
      if (before === "completed") stats.completed_tasks = dec("completed_tasks");
      if (before === "failed") stats.failed_tasks = dec("failed_tasks");
      break;
    case "completed":
      stats.completed_tasks = inc("completed_tasks");
      if (before === "pending") stats.pending_tasks = dec("pending_tasks");
      if (before === "failed") stats.failed_tasks = dec("failed_tasks");
      break;
    case "failed":
      stats.failed_tasks = inc("failed_tasks");
      if (before === "pending") stats.pending_tasks = dec("pending_tasks");
      if (before === "completed") stats.completed_tasks = dec("completed_tasks");
      break;
    default:
      return null;
  }
  return stats;
}