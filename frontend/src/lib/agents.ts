import { api } from "./api";
import { paths } from "./api-paths";
import { firstArray } from "./envelope";
import { fetchCached } from "./hooks/useCachedData";
import { fetchAgentsPage } from "./typed-api";
import { isAgentStatus } from "./status";
import type { Agent, AgentStatus, NormalizedAgent } from "@/types/agent";

export type { NormalizedAgent as AgentSummary };

/** Normalize agent list envelopes. Canonical keys are agents then data. */
export function normalizeAgentList(data: unknown): Agent[] {
  return firstArray(data, ["agents", "data", "Agents", "Beacons", "beacons"]) as Agent[];
}

function toNormalized(r: Record<string, unknown>): NormalizedAgent {
  const statusRaw = String(r.status ?? "");
  const status: AgentStatus = isAgentStatus(statusRaw) ? statusRaw : "offline";
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

/**
 * Shared agent-list cache (single key across the app, 60s TTL).
 *
 * The backend caps page_size at 100 and defaults to 20, so dropdown-style
 * consumers that used the default were silently seeing only 20 agents; use
 * page_size=100 here so every consumer gets the full fleet from one request.
 * Callers that need fresher data (polling dashboards, paginated tables) should
 * keep their own requests — the cache is intentionally a snapshot for lists.
 */
export const AGENTS_CACHE_KEY = "agents:list";
const AGENTS_CACHE_TTL_MS = 60_000;

export async function fetchAgentListCached(): Promise<Agent[]> {
  return fetchCached<Agent[]>(
    AGENTS_CACHE_KEY,
    async () => {
      const { agents } = await fetchAgentsPage({ page_size: 100 });
      return normalizeAgentList(agents);
    },
    AGENTS_CACHE_TTL_MS,
  );
}

interface FetchAgentListResult {
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
