/** GET /api/agents/:id/process-tree returns the last completed `ps` blob, not a live tree. */
export const PROCESS_SNAPSHOT_KIND = "last_ps_snapshot";

export interface PsRow {
  pid: string;
  name: string;
  raw: string;
}

const PS_HEADER_RE = /(processid|^pid\b|PID)/;
const PS_TABLE_WORDS = ["name", "command", "user", "cmd"];

/** Best-effort row parse of free-form `ps` text (PowerShell Format-Table or `ps aux`). */
export function parsePsRows(text: string, cap = 200): PsRow[] {
  const rows: PsRow[] = [];
  for (const rawLine of (text || "").split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || rows.length >= cap) continue;
    if (/^[-=\s]+$/.test(line)) continue;
    const lower = line.toLowerCase();
    if (PS_HEADER_RE.test(line) && PS_TABLE_WORDS.some((w) => lower.includes(w))) continue;
    const tokens = line.split(/\s+/);
    const pidIdx = tokens.findIndex((tok) => /^\d+$/.test(tok));
    if (pidIdx < 0) continue;
    // Drop metric noise (VSZ/RSS integers, CPU% "0.1", "?", "10:00") before
    // picking a display name; prefer a path-like token (ps aux COMMAND or
    // an image path), else the first remaining token.
    const rest = tokens.slice(pidIdx + 1).filter((tok) => !/^[\d.:?]+$/.test(tok));
    const name = (rest.find((tok) => /[/\\]/.test(tok)) || rest[0] || tokens[tokens.length - 1] || "").slice(0, 80);
    rows.push({ pid: tokens[pidIdx], name, raw: rawLine });
  }
  return rows;
}

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
