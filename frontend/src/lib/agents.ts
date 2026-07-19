import { api } from "./api";
import type { NormalizedAgent } from '@/types/agent';

export type { NormalizedAgent as AgentSummary };

export async function fetchAgentList(onlineOnly = false): Promise<NormalizedAgent[]> {
  try {
    const data = await api.get("/api/v1/agents");
    const raw = (data.data || data.agents || data.Beacons || data || []) as Record<string, unknown>[];
    const list = (Array.isArray(raw) ? raw : []).map((r) => ({
      id: String(r.id ?? ""),
      hostname: String(r.hostname ?? ""),
      username: String(r.username ?? ""),
      ip: String(r.ip ?? r.internal_ip ?? ""),
      os: String(r.os ?? ""),
      status: String(r.status ?? "offline"),
      last_seen: String(r.last_seen ?? ""),
      listener_id: String(r.listener_id ?? ""),
      tags: String(r.tags ?? ""),
    })).filter((a) => a.id);
    return onlineOnly ? list.filter((a) => a.status === "online") : list;
  } catch {
    return [];
  }
}
