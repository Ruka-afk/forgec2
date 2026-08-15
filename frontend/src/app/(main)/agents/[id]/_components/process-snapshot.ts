/** GET /api/agents/:id/process-tree returns the last completed `ps` blob, not a live tree. */
export const PROCESS_SNAPSHOT_KIND = "last_ps_snapshot";

export function isLiveProcessTree(payload: {
  live?: unknown;
  kind?: unknown;
}): boolean {
  return payload.live === true && String(payload.kind || "") !== PROCESS_SNAPSHOT_KIND;
}

export function describeProcessSnapshot(payload: Record<string, unknown> | null | undefined): {
  text: string;
  source: string;
  live: boolean;
  kind: string;
} {
  const text = typeof payload?.processes === "string" ? payload.processes : "";
  const kind = String(payload?.kind ?? PROCESS_SNAPSHOT_KIND);
  const live = payload?.live === true && kind !== PROCESS_SNAPSHOT_KIND;
  const source = String(payload?.source ?? payload?.alias_of ?? "ps");
  return { text, source, live, kind };
}
