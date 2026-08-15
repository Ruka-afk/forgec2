import type { AgentStatus } from "@/types/agent";
import type { Beacon } from "./types";

export interface HostGroup {
  key: string;
  hostname: string;
  os: string;
  ip: string;
  status: AgentStatus;
  last_seen: string;
  sessions: Beacon[];
}

function statusRank(status?: string): number {
  if (status === "online") return 2;
  if (status === "stale") return 1;
  return 0;
}

/** Group key: hostname, then IP, then session id. */
export function hostKey(beacon: Beacon): string {
  const host = (beacon.hostname || "").trim().toLowerCase();
  if (host) return `h:${host}`;
  const ip = (beacon.ip || beacon.public_ip || "").trim();
  if (ip) return `ip:${ip}`;
  return `id:${beacon.id || ""}`;
}

export function groupBeaconsByHost(beacons: Beacon[]): HostGroup[] {
  const buckets = new Map<string, Beacon[]>();
  for (const beacon of beacons) {
    const key = hostKey(beacon);
    const list = buckets.get(key);
    if (list) list.push(beacon);
    else buckets.set(key, [beacon]);
  }

  return [...buckets.entries()].map(([key, sessions]) => {
    const ordered = [...sessions].sort((a, b) => {
      const rank = statusRank(b.status) - statusRank(a.status);
      if (rank !== 0) return rank;
      return new Date(b.last_seen || 0).getTime() - new Date(a.last_seen || 0).getTime();
    });
    const primary = ordered[0];
    return {
      key,
      hostname: primary.hostname || primary.ip || key,
      os: primary.os || "",
      ip: primary.ip || primary.public_ip || "",
      status: (primary.status || "offline") as AgentStatus,
      last_seen: primary.last_seen || "",
      sessions: ordered,
    };
  });
}

export type FlatBeaconRow =
  | { kind: "host"; group: HostGroup }
  | { kind: "session"; beacon: Beacon; child: boolean };

export function flattenHostRows(groups: HostGroup[], expanded: ReadonlySet<string>): FlatBeaconRow[] {
  const rows: FlatBeaconRow[] = [];
  for (const group of groups) {
    if (group.sessions.length === 1) {
      rows.push({ kind: "session", beacon: group.sessions[0], child: false });
      continue;
    }
    rows.push({ kind: "host", group });
    if (expanded.has(group.key)) {
      for (const beacon of group.sessions) {
        rows.push({ kind: "session", beacon, child: true });
      }
    }
  }
  return rows;
}
