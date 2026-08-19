import type { WSMessage } from "@/lib/wsContext";
import { isWSEvent } from "@/lib/typed-ws";
import type { UnifiedEvent } from "./types";

export function unifiedFromWS(msg: WSMessage, now = new Date().toISOString()): UnifiedEvent | null {
  const agentId = msg.agent_id != null ? String(msg.agent_id) : undefined;

  if (isWSEvent(msg, "agent_online") || isWSEvent(msg, "agent_offline")) {
    const host = String(msg.hostname || agentId || msg.type);
    return {
      id: `ws-${msg.type}-${agentId || host}`,
      at: now,
      source: "timeline",
      kind: msg.type,
      title: host,
      detail: [msg.type === "agent_online" ? "online" : "offline", msg.username, msg.ip].filter(Boolean).join(" · "),
      agentId,
    };
  }

  if (isWSEvent(msg, "task_update") || isWSEvent(msg, "task_created")) {
    const taskId = msg.task_id ?? msg.id;
    if (taskId == null) return null;
    return {
      id: `task-${taskId}`,
      at: now,
      source: "task",
      kind: String(msg.task_type || msg.type || "task"),
      title: String(msg.task_type || "task"),
      detail: String(msg.command || msg.result || msg.error || ""),
      agentId,
      status: String(msg.status || ""),
    };
  }

  if (isWSEvent(msg, "system_alert") || isWSEvent(msg, "credential_found")) {
    return {
      id: `ws-${msg.type}-${agentId || now}`,
      at: now,
      source: "alert",
      kind: msg.type,
      title: String(msg.title || msg.type),
      detail: String(msg.message || msg.alert_type || ""),
      agentId,
    };
  }

  return null;
}

export function upsertLiveEvent(existing: UnifiedEvent[], incoming: UnifiedEvent, cap = 80): UnifiedEvent[] {
  const next = existing.filter((e) => e.id !== incoming.id);
  next.unshift(incoming);
  return next.slice(0, cap);
}

export function mergePolledWithLive(polled: UnifiedEvent[], live: UnifiedEvent[]): UnifiedEvent[] {
  const byId = new Map<string, UnifiedEvent>();
  for (const ev of polled) byId.set(ev.id, ev);
  for (const ev of live) byId.set(ev.id, ev);
  return [...byId.values()].sort((a, b) => {
    const at = a.at ? new Date(a.at).getTime() : 0;
    const bt = b.at ? new Date(b.at).getTime() : 0;
    return bt - at;
  });
}
