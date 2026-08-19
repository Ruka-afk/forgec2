"use client";

/**
 * Canonical runtime registries for status vocabularies.
 *
 * `src/types/agent.ts` re-exports `AgentStatus`/`TaskStatus` from here so the
 * type unions and the runtime arrays used for validation never drift apart.
 * Always validate untrusted API/websocket payloads with the `is*` guards
 * instead of re-declaring literal arrays.
 */

export const AGENT_STATUSES = ["online", "stale", "offline"] as const;
export type AgentStatus = (typeof AGENT_STATUSES)[number];

export const TASK_STATUSES = ["pending", "running", "completed", "failed", "cancelled", "pending_approval"] as const;
export type TaskStatus = (typeof TASK_STATUSES)[number];

export function isAgentStatus(v: unknown): v is AgentStatus {
  return typeof v === "string" && (AGENT_STATUSES as readonly string[]).includes(v);
}

export function isTaskStatus(v: unknown): v is TaskStatus {
  return typeof v === "string" && (TASK_STATUSES as readonly string[]).includes(v);
}