export interface WindowRow {
  hwnd: string;
  pid: string;
  title: string;
}

/**
 * Parses the agent's window_list text output. Rows look like
 * `HWND\tPID\tTITLE` (decimal HWND); the `HWND\tPID\tTITLE` header,
 * `# windows=N` trailer and other noise lines are skipped.
 */
export function parseWindowList(result: string): WindowRow[] {
  const rows: WindowRow[] = [];
  if (!result) return rows;
  for (const raw of result.split(/\r?\n/)) {
    const line = raw.trimEnd();
    if (!line) continue;
    const trimmed = line.trim();
    if (
      trimmed.startsWith("#") ||
      trimmed.startsWith("HWND") ||
      trimmed.startsWith("window_list:") ||
      trimmed.startsWith("=== ")
    ) {
      continue;
    }
    const cols = line.split("\t");
    if (cols.length < 3) continue;
    const hwnd = cols[0].trim();
    const pid = cols[1].trim();
    const title = cols.slice(2).join(" ").trim();
    if (!hwnd || !/^\d+$/.test(hwnd)) continue;
    rows.push({ hwnd, pid, title });
  }
  return rows;
}
