import { api } from "./api";
import type { Agent, AgentStatus, NormalizedAgent } from "@/types/agent";

export type { NormalizedAgent as AgentSummary };

/** Normalize agent list envelopes from /agents, /api/agents, or bare arrays. */
export function normalizeAgentList(data: unknown): Agent[] {
  if (Array.isArray(data)) return data as Agent[];
  if (!data || typeof data !== "object") return [];
  const o = data as Record<string, unknown>;
  if (Array.isArray(o.agents)) return o.agents as Agent[];
  if (Array.isArray(o.Agents)) return o.Agents as Agent[];
  if (Array.isArray(o.data)) return o.data as Agent[];
  if (Array.isArray(o.Beacons)) return o.Beacons as Agent[];
  return [];
}

function toNormalized(r: Record<string, unknown>): NormalizedAgent {
  return {
    id: String(r.id ?? ""),
    hostname: String(r.hostname ?? ""),
    username: String(r.username ?? ""),
    ip: String(r.ip ?? r.internal_ip ?? ""),
    os: String(r.os ?? ""),
    status: (["online", "stale", "offline"].includes(String(r.status ?? "")) ? r.status : "offline") as AgentStatus,
    last_seen: String(r.last_seen ?? ""),
    listener_id: String(r.listener_id ?? ""),
    tags: String(r.tags ?? ""),
  };
}

export async function fetchAgentList(onlineOnly = false): Promise<NormalizedAgent[]> {
  try {
    const data = await api.get("/api/v1/agents");
    const list = normalizeAgentList(data)
      .map((a) => toNormalized(a as Record<string, unknown>))
      .filter((a) => a.id);
    return onlineOnly ? list.filter((a) => a.status === "online") : list;
  } catch {
    return [];
  }
}
