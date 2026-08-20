import type { NormalizedAgent } from "@/types/agent";
import type { Task } from "@/types/task";
import type { LootData } from "@/types/loot";
import { isProblemHealth, type ListenerHealth } from "../../listeners/_components/listener-health";

export const DASHBOARD_VIEWS = ["ops", "analytics"] as const;
export type DashboardView = (typeof DASHBOARD_VIEWS)[number];

function byCreatedDesc<T extends { created_at?: string }>(a: T, b: T): number {
  return Date.parse(b.created_at || "") - Date.parse(a.created_at || "");
}

export function splitSessions(agents: NormalizedAgent[], limit = 6): {
  online: NormalizedAgent[];
  dropped: NormalizedAgent[];
} {
  const bySeen = (a: NormalizedAgent, b: NormalizedAgent) =>
    Date.parse(b.last_seen || "") - Date.parse(a.last_seen || "");
  return {
    online: agents.filter((a) => a.status === "online").sort(bySeen).slice(0, limit),
    dropped: agents.filter((a) => a.status !== "online").sort(bySeen).slice(0, limit),
  };
}

export function pickUnhealthyListeners(healthByTarget: Record<string, ListenerHealth>): ListenerHealth[] {
  return Object.values(healthByTarget)
    .filter(isProblemHealth)
    .sort((a, b) => (b.consecutive_fails ?? 0) - (a.consecutive_fails ?? 0));
}

export function mergeAttention(failed: Task[], pending: Task[], approvals: Task[] = [], limit = 10): Task[] {
  return [...approvals].sort(byCreatedDesc)
    .concat([...failed].sort(byCreatedDesc))
    .concat([...pending].sort(byCreatedDesc))
    .slice(0, limit);
}

type LootInboxKind = "screenshot" | "keylog" | "download";

interface LootInboxItem {
  id: string;
  kind: LootInboxKind;
  created_at: string;
  agent_id: string;
  label: string;
}

export function flattenLoot(data: LootData | null | undefined, limit = 8): LootInboxItem[] {
  if (!data) return [];
  const items: LootInboxItem[] = [
    ...(data.screenshots || []).map((s) => ({
      id: `s-${s.id}`,
      kind: "screenshot" as const,
      created_at: s.created_at || "",
      agent_id: s.agent_id || "",
      label: s.filename || s.id,
    })),
    ...(data.keylog_tasks || []).map((k) => ({
      id: `k-${k.id}`,
      kind: "keylog" as const,
      created_at: k.created_at || "",
      agent_id: k.agent_id || "",
      label: k.hostname || k.agent?.hostname || k.agent_id || String(k.id),
    })),
    ...(data.download_tasks || []).map((d) => ({
      id: `d-${d.id}`,
      kind: "download" as const,
      created_at: d.created_at || "",
      agent_id: d.agent_id || "",
      label: d.command || d.hostname || d.agent_id || String(d.id),
    })),
  ];
  return items.sort(byCreatedDesc).slice(0, limit);
}

export const LOOT_TAB: Record<LootInboxKind, string> = {
  screenshot: "screenshots",
  keylog: "keylogs",
  download: "downloads",
};
