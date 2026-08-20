"use client";

/**
 * Typed WebSocket event registry.
 *
 * Single source of truth for wire event names (the `type` field of WS frames)
 * and their payload shapes, mirroring what the Go WS hub emits. The payload
 * interfaces only model the fields consumers actually read; unknown fields
 * remain accessible on the raw message.
 *
 * Extend this map when adding a new consumer — never reach into
 * `{ type: string; [key: string]: unknown }` in component code again.
 */

interface TaskUpdateEvent {
  task_id: number | string;
  status: string;
  task_type?: string;
  command?: string;
  result?: string;
  error?: string;
  agent_id?: number | string;
}

interface TaskOutputEvent {
  task_id: number | string;
  chunk?: string;
  done?: boolean;
  agent_id?: number | string;
}

interface TaskCreatedEvent {
  task_id?: number | string;
  id?: number | string;
  task_type?: string;
  status?: string;
  command?: string;
  result?: string;
  error?: string;
  agent_id?: number | string;
}

interface AgentOnlineEvent {
  agent_id?: number | string;
  hostname?: string;
  username?: string;
  ip?: string;
}

interface AgentOfflineEvent {
  agent_id?: number | string;
  hostname?: string;
  username?: string;
  ip?: string;
}

interface AgentLockedEvent {
  agent_id?: number | string;
}

interface AgentUnlockedEvent {
  agent_id?: number | string;
}

interface AgentDataUpdateEvent {
  agent_id?: number | string;
  [key: string]: unknown;
}

interface NotificationEvent {
  id?: number | string;
  message?: string;
  title?: string;
  severity?: string;
  type?: string;
  created_at?: string;
  read?: boolean;
}

interface CredentialFoundEvent {
  message?: string;
  title?: string;
}

interface SystemAlertEvent {
  message?: string;
  title?: string;
  alert_type?: string;
}

interface UpdateAvailableEvent {
  latest?: string;
}

/** Payloads not yet modeled: widen as consumers migrate to typed access. */
interface UnmodeledEvent {
  [key: string]: unknown;
}

export interface WSEventMap {
  task_update: TaskUpdateEvent;
  task_output: TaskOutputEvent;
  task_created: TaskCreatedEvent;
  agent_online: AgentOnlineEvent;
  agent_offline: AgentOfflineEvent;
  agent_locked: AgentLockedEvent;
  agent_unlocked: AgentUnlockedEvent;
  agent_data_update: AgentDataUpdateEvent;
  notification: NotificationEvent;
  credential_found: CredentialFoundEvent;
  system_alert: SystemAlertEvent;
  update_available: UpdateAvailableEvent;
  build_update: UnmodeledEvent;
  listener_update: UnmodeledEvent;
  credential_update: UnmodeledEvent;
  chat: UnmodeledEvent;
  notes_update: UnmodeledEvent;
  sleep_update: UnmodeledEvent;
}

export type WSEventName = keyof WSEventMap;

/** Events the timeline stream maps into UnifiedEvents. */
export const TIMELINE_EVENTS = [
  "agent_online",
  "agent_offline",
  "task_update",
  "task_created",
  "system_alert",
  "credential_found",
] as const satisfies readonly WSEventName[];