import { API_BASE } from "./constants";

export interface AgentSummary {
  id: string;
  hostname: string;
  ip: string;
  os: string;
  status: string;
  username?: string;
}

function normalizeAgent(raw: Record<string, unknown>): AgentSummary {
  return {
    id: String(raw.id || raw.ID || ""),
    hostname: String(raw.hostname || raw.Hostname || "unknown"),
    ip: String(raw.ip || raw.IP || "-"),
    os: String(raw.os || raw.OS || ""),
    status: String(raw.status || raw.Status || "offline"),
    username: String(raw.username || raw.Username || ""),
  };
}

export async function fetchAgentList(onlineOnly = false): Promise<AgentSummary[]> {
  const res = await fetch(`${API_BASE}?p=/agents&pageSize=500&format=json`, { credentials: "include" });
  if (!res.ok) return [];
  const data = await res.json();
  const raw = (data.Beacons || data.Agents || data.agents || data.data || []) as Record<string, unknown>[];
  const list = (Array.isArray(raw) ? raw : []).map(normalizeAgent).filter((a) => a.id);
  return onlineOnly ? list.filter((a) => a.status === "online") : list;
}