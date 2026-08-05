import { api } from "./api";
import { paths } from "./api-paths";
import { firstArray } from "./envelope";
import type { Agent, AgentStatus, NormalizedAgent } from "@/types/agent";

export type { NormalizedAgent as AgentSummary };

/** Canonical list path — prefer /api/agents over dual-use /agents (SPA vs JSON). */
export { paths as apiPaths } from "./api-paths";
export const AGENTS_LIST_PATH = paths.agents.list();

/** Normalize agent list envelopes from /agents, /api/agents, or bare arrays. */
export function normalizeAgentList(data: unknown): Agent[] {
  return firstArray(data, ["agents", "Agents", "data", "Beacons", "beacons"]) as Agent[];
}

function toNormalized(r: Record<string, unknown>): NormalizedAgent {
  const statusRaw = String(r.status ?? "");
  const status = (["online", "stale", "offline"].includes(statusRaw) ? statusRaw : "offline") as AgentStatus;
  return {
    id: String(r.id ?? ""),
    hostname: String(r.hostname ?? ""),
    username: String(r.username ?? ""),
    ip: String(r.ip ?? r.internal_ip ?? ""),
    os: String(r.os ?? ""),
    status,
    last_seen: String(r.last_seen ?? ""),
    listener_id: String(r.listener_id ?? ""),
    tags: String(r.tags ?? ""),
  };
}

export interface FetchAgentListResult {
  agents: NormalizedAgent[];
  error: string | null;
}

/**
 * Fetch and normalize agents. Never collapses network/auth errors into an empty list
 * without reporting them — callers must handle `error`.
 */
export async function fetchAgentList(onlineOnly = false): Promise<FetchAgentListResult> {
  try {
    const data = await api.get(paths.agents.list());
    let list = normalizeAgentList(data)
      .map((a) => toNormalized(a as Record<string, unknown>))
      .filter((a) => a.id);
    if (onlineOnly) list = list.filter((a) => a.status === "online");
    return { agents: list, error: null };
  } catch (e) {
    const msg = e instanceof Error ? e.message : "Failed to load agents";
    return { agents: [], error: msg };
  }
}
