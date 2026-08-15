export type DiffKind = "same" | "add" | "del";

export interface DiffLine {
  kind: DiffKind;
  text: string;
}

export interface ResultDiff {
  lines: DiffLine[];
  added: number;
  removed: number;
  mode: "sequence" | "unique";
  truncated: boolean;
}

const SEQ_LINE_CAP = 400;
const SEQ_CELL_CAP = 160_000;
const OUT_LINE_CAP = 800;

export function splitResultLines(text: string): string[] {
  return (text || "").replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n");
}

/** Skip empty, huge, and image/base64 blobs — those are not line-diffable. */
export function resultLooksComparable(text?: string): boolean {
  const s = text || "";
  if (!s.trim()) return false;
  if (s.length > 200_000) return false;
  if (/^data:image\//i.test(s.trim())) return false;
  const compact = s.replace(/\s+/g, "");
  if (
    compact.length > 80 &&
    /^[A-Za-z0-9+/=]+$/.test(compact) &&
    /[+/=]/.test(compact)
  ) {
    return false;
  }
  return true;
}

export function previousComparableTask<T extends {
  id: number;
  type: string;
  command: string;
  status: string;
  result?: string;
}>(tasks: T[], current: Pick<T, "id" | "type" | "command">): T | null {
  const curId = Number(current.id);
  const older = tasks
    .filter((t) =>
      Number(t.id) < curId &&
      t.status === "completed" &&
      t.type === current.type &&
      resultLooksComparable(t.result),
    )
    .sort((a, b) => Number(b.id) - Number(a.id));
  const sameCmd = older.find((t) => (t.command || "") === (current.command || ""));
  return sameCmd || older[0] || null;
}

function lcsDiff(a: string[], b: string[]): DiffLine[] {
  const n = a.length;
  const m = b.length;
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ kind: "same", text: a[i] });
      i += 1;
      j += 1;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      out.push({ kind: "del", text: a[i] });
      i += 1;
    } else {
      out.push({ kind: "add", text: b[j] });
      j += 1;
    }
  }
  while (i < n) {
    out.push({ kind: "del", text: a[i] });
    i += 1;
  }
  while (j < m) {
    out.push({ kind: "add", text: b[j] });
    j += 1;
  }
  return out;
}

function uniqueDiff(a: string[], b: string[]): DiffLine[] {
  const aSet = new Set(a);
  const bSet = new Set(b);
  const out: DiffLine[] = [];
  for (const line of a) {
    if (!bSet.has(line)) out.push({ kind: "del", text: line });
  }
  for (const line of b) {
    if (!aSet.has(line)) out.push({ kind: "add", text: line });
  }
  return out;
}

export function diffResults(before: string, after: string): ResultDiff {
  const a = splitResultLines(before);
  const b = splitResultLines(after);
  const tooLong = a.length > SEQ_LINE_CAP || b.length > SEQ_LINE_CAP || a.length * b.length > SEQ_CELL_CAP;
  const mode: ResultDiff["mode"] = tooLong ? "unique" : "sequence";
  let lines = mode === "sequence" ? lcsDiff(a, b) : uniqueDiff(a, b);
  let truncated = tooLong;
  if (lines.length > OUT_LINE_CAP) {
    lines = lines.filter((line) => line.kind !== "same").slice(0, OUT_LINE_CAP);
    truncated = true;
  }
  let added = 0;
  let removed = 0;
  for (const line of lines) {
    if (line.kind === "add") added += 1;
    else if (line.kind === "del") removed += 1;
  }
  return { lines, added, removed, mode, truncated };
}

export function diffChangeLines(diff: ResultDiff): DiffLine[] {
  return diff.lines.filter((line) => line.kind !== "same");
}
