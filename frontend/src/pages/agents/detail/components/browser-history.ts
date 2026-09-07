export interface HistoryRow {
  browser: string;
  time: string;
  visits: string;
  url: string;
  title: string;
}

/**
 * Parses the agent's browser_history text output. Sections look like
 * `=== Chrome (C:\...\History) ===`, rows are
 * `time\tvisits\turl\ttitle` (chromium/firefox) or `time\turl` (safari).
 * `# ...` trailers, `query: ...` errors and `(not found)` lines are skipped.
 */
export function parseBrowserHistory(result: string): HistoryRow[] {
  const rows: HistoryRow[] = [];
  if (!result) return rows;
  let browser = "";
  for (const raw of result.split(/\r?\n/)) {
    const line = raw.trimEnd();
    if (!line) continue;
    if (line.startsWith("===")) {
      const m = /^===\s*(.+?)(?:\s*\(.*\))?\s*===$/.exec(line.trim());
      browser = m ? m[1].trim() : line.replace(/=/g, "").trim();
      continue;
    }
    const trimmed = line.trim();
    if (
      trimmed.startsWith("#") ||
      trimmed.startsWith("query:") ||
      trimmed === "(not found)" ||
      trimmed.startsWith("browser_history:") ||
      trimmed.startsWith("=== browser history")
    ) {
      continue;
    }
    const cols = line.split("\t");
    if (cols.length < 2) continue;
    const time = cols[0].trim();
    if (!time) continue;
    if (cols.length >= 4) {
      rows.push({
        browser,
        time,
        visits: cols[1].trim(),
        url: cols[2].trim(),
        title: cols.slice(3).join(" ").trim(),
      });
    } else if (cols.length === 3 && /^\d+$/.test(cols[1].trim())) {
      // Chromium/firefox row with an empty title: the trailing tab is eaten
      // by trimEnd, leaving time\tvisits\turl.
      rows.push({ browser, time, visits: cols[1].trim(), url: cols[2].trim(), title: "" });
    } else {
      rows.push({ browser, time, visits: "", url: cols[1].trim(), title: cols.slice(2).join(" ").trim() });
    }
  }
  return rows;
}
