export function isLootUrlFetch(command?: string): boolean {
  return /^https?:\/\//i.test(command || "");
}

/** Basename for a completed implant→server pull. Command is the remote path. */
export function lootExfilFilename(task: { command?: string; result?: string }): string | null {
  const cmd = task.command || "";
  if (isLootUrlFetch(cmd)) return null;
  const fromPath = cmd.split(/[\\/]/).filter(Boolean).pop() || "";
  if (fromPath) return fromPath;
  const m = (task.result || "").match(/:\s*([^\s]+) offset /i);
  return m?.[1] || null;
}

export function lootExfilBytes(result?: string): number | null {
  const m = (result || "").match(/\((\d+)\s*bytes?\)/i);
  if (!m) return null;
  const n = Number(m[1]);
  return Number.isFinite(n) ? n : null;
}

export function formatLootBytes(n: number | null): string {
  if (n == null) return "—";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

const DONE = new Set(["completed", "success", "done"]);

export function canDownloadLootExfil(task: { status?: string; command?: string; result?: string }): boolean {
  return DONE.has(String(task.status || "").toLowerCase()) && lootExfilFilename(task) != null;
}
