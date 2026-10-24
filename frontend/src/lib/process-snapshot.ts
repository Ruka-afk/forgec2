/** GET /api/agents/:id/process-tree returns the last completed process listing. */
const PROCESS_SNAPSHOT_KIND = "last_ps_snapshot";

export interface ProcessRow {
  /** Stable row identity; PIDs can theoretically repeat in malformed output. */
  key: string;
  pid: string;
  ppid?: string;
  user?: string;
  name: string;
  threads?: number;
  /** Normalized to KiB so every snapshot format displays consistently. */
  memoryKb?: number;
  /** Full command line when the source snapshot exposes one. */
  command?: string;
  /** Raw row, used by the detail view and tooltips. */
  raw: string;
  /** Indentation depth from the source. The tree view recomputes this from PPIDs. */
  depth: number;
}

type ProcessListingFormat = "tree" | "c-ps" | "ps" | "powershell" | "table" | "generic";

interface ProcessTreeNode extends ProcessRow {
  children: ProcessTreeNode[];
  depth: number;
}

interface ParsedProcessSnapshot {
  rows: ProcessRow[];
  roots: ProcessTreeNode[];
  format: ProcessListingFormat;
}

export interface ProcessSnapshotMeta {
  text: string;
  source: string;
  live: boolean;
  kind: string;
  taskId?: number;
  collectedAt?: string;
}

interface PsRow {
  pid: string;
  name: string;
  raw: string;
}

interface HeaderColumn {
  label: string;
  normalized: string;
  start: number;
}

const PS_HEADER_RE = /(processid|^pid\b|PID)/;
const PS_TABLE_WORDS = ["name", "command", "user", "cmd"];
const SEPARATOR_RE = /^[\s\-=]+$/;

const PID_HEADERS = new Set(["pid", "processid", "id"]);
const PPID_HEADERS = new Set(["ppid", "parentprocessid", "parentpid"]);
const USER_HEADERS = new Set(["user", "username", "owner", "uid"]);
const NAME_HEADERS = new Set(["name", "processname", "image", "imagename", "comm", "exe"]);
const COMMAND_HEADERS = new Set(["command", "cmd", "cmdline", "commandline"]);
const THREAD_HEADERS = new Set(["threads", "threadcount", "threads."]);
const MEMORY_HEADERS = new Set([
  "mem_kb",
  "memkb",
  "memory",
  "workingsetmb",
  "workingset64",
  "rss",
  "rsskb",
]);

/** Best-effort free-form `ps` rows retained for callers that only need PID/name. */
export function parsePsRows(text: string, cap = 200): PsRow[] {
  return parseProcessRows(text, cap).map(({ pid, name, raw }) => ({ pid, name, raw }));
}

/** Parse every process listing shape the Go and C implants can return. */
function parseProcessRows(text: string, cap = 0): ProcessRow[] {
  return parseProcessSnapshot(text, cap).rows;
}

export function parseProcessSnapshot(text: string, cap = 0): ParsedProcessSnapshot {
  const lines = String(text ?? "").split(/\r?\n/);
  const tabular = parseTabRows(lines, cap);
  if (tabular) return { ...tabular, roots: buildProcessTree(tabular.rows) };

  const columnar = parseColumnRows(lines, cap);
  if (columnar) return { ...columnar, roots: buildProcessTree(columnar.rows) };

  const rows = parseGenericRows(lines, cap);
  return { rows, roots: buildProcessTree(rows), format: "generic" };
}

function parseTabRows(lines: string[], cap: number): { rows: ProcessRow[]; format: ProcessListingFormat } | null {
  let headerIndex = -1;
  let mapping = new Map<string, number>();
  const tabLines = lines.filter((line) => line.includes("\t"));
  if (tabLines.length === 0) return null;

  for (let i = 0; i < lines.length; i++) {
    if (!lines[i].includes("\t")) continue;
    const parts = lines[i].split("\t");
    const normalized = parts.map(normalizeHeader);
    if (!normalized.some((part) => PID_HEADERS.has(part))) continue;
    if (!normalized.some((part) => NAME_HEADERS.has(part) || COMMAND_HEADERS.has(part))) continue;
    headerIndex = i;
    mapping = new Map(normalized.map((part, idx) => [part, idx]));
    break;
  }

  const rows: ProcessRow[] = [];
  for (let i = headerIndex + 1; i < lines.length; i++) {
    if (cap > 0 && rows.length >= cap) break;
    const rawLine = lines[i];
    if (!rawLine.trim() || !rawLine.includes("\t")) continue;
    const parts = rawLine.split("\t");
    if (parts.length < 2) continue;

    let pid = fieldAt(parts, mapping, PID_HEADERS);
    let inferred: ReturnType<typeof inferTabFields> = null;
    if (!isNumeric(pid)) {
      // Older agents omitted the TSV header. Fall back to the first column so
      // the same tree/C layouts still parse instead of silently going empty.
      const first = parts[0]?.trim() ?? "";
      if (!isNumeric(first)) continue;
      pid = first;
      inferred = inferTabFields(parts);
    }
    let ppid = fieldAt(parts, mapping, PPID_HEADERS) || undefined;
    let user = fieldAt(parts, mapping, USER_HEADERS) || undefined;
    let threads = parseNumeric(fieldAt(parts, mapping, THREAD_HEADERS));
    const memoryLabel = headerAt(mapping, MEMORY_HEADERS);
    let memoryKb = parseMemoryKb(fieldAt(parts, mapping, MEMORY_HEADERS), memoryLabel);
    const command = fieldAt(parts, mapping, COMMAND_HEADERS) || undefined;
    let name = fieldAt(parts, mapping, NAME_HEADERS);
    if (!name && command) name = nameFromCommand(command);
    if (!name) {
      // Headerless tree/C output: PID, PPID, USER, NAME or
      // PID, PPID, THREADS, MEM_KB, NAME.
      inferred ??= inferTabFields(parts);
      if (inferred) {
        name = inferred.name;
        if (!ppid && inferred.ppid) ppid = inferred.ppid;
        if (!user && inferred.user) user = inferred.user;
        if (threads === undefined) threads = inferred.threads;
        if (memoryKb === undefined) memoryKb = inferred.memoryKb;
      }
    }
    if (!name) continue;

    rows.push({
      key: `${pid}:${i}`,
      pid,
      ppid: ppid && isNumeric(ppid) ? ppid : undefined,
      user,
      name: name.slice(0, 200),
      threads,
      memoryKb,
      command,
      raw: rawLine.trim(),
      depth: leadingDepth(rawLine),
    });
  }

  if (rows.length === 0) return null;
  const hasThreadsOrMemory =
    mapping.has("threads") ||
    memoryHeaderPresent(mapping) ||
    rows.some((row) => row.threads !== undefined || row.memoryKb !== undefined);
  const format: ProcessListingFormat = hasThreadsOrMemory ? "c-ps" : "tree";
  return { rows, format };
}

function parseColumnRows(lines: string[], cap: number): { rows: ProcessRow[]; format: ProcessListingFormat } | null {
  let headerIndex = -1;
  let columns: HeaderColumn[] = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!line.trim() || line.includes("\t")) continue;
    const lower = line.toLowerCase();
    const hasPid = /\b(processid|pid|id)\b/.test(lower);
    const hasNameOrCommand = /\b(name|processname|command|cmdline|cmd)\b/.test(lower);
    if (!hasPid || !hasNameOrCommand) continue;
    columns = headerColumns(line);
    // tasklist prints "Image Name" as one logical column; without merging the
    // two header tokens the name field starts mid-value and loses the image
    // prefix (e.g. "System Idle Process" became "Idle Process").
    const imageIdx = columns.findIndex((c) => c.normalized === "image");
    if (imageIdx >= 0 && columns[imageIdx + 1]?.normalized === "name") {
      columns[imageIdx] = { ...columns[imageIdx], label: "Image Name", normalized: "imagename" };
      columns.splice(imageIdx + 1, 1);
    }
    if (!columns.some((c) => PID_HEADERS.has(c.normalized))) continue;
    headerIndex = i;
    break;
  }
  if (headerIndex < 0) return null;

  const pidColumn = columns.find((c) => PID_HEADERS.has(c.normalized));
  if (!pidColumn) return null;
  const commandColumn = columns.find((c) => COMMAND_HEADERS.has(c.normalized));

  const rows: ProcessRow[] = [];
  for (let i = headerIndex + 1; i < lines.length; i++) {
    if (cap > 0 && rows.length >= cap) break;
    const line = lines[i];
    if (!line.trim() || SEPARATOR_RE.test(line.trim())) continue;
    const columnEnd = (idx: number) => {
      if (idx + 1 >= columns.length) return undefined;
      const next = columns[idx + 1];
      const nextStart = next.start;
      if (isNumericColumn(next.normalized)) {
        const token = overlappingToken(line, nextStart);
        // A right-aligned number can begin before its header. End the
        // preceding string field at the number, not inside it.
        if (token && token.start < nextStart) return token.start;
      }
      return nextStart;
    };
    const fields = columns.map((column, idx) => line.slice(column.start, columnEnd(idx)).trim());
    const field = (...normalized: string[]) => {
      const idx = columns.findIndex((c) => normalized.includes(c.normalized));
      return idx >= 0 ? (fields[idx] ?? "") : "";
    };
    // Format-Table right-aligns numeric fields beneath their headers. A slice
    // starting at the header offset can drop the leading digit ("1234" ->
    // "234"), so take the whole token that overlaps that offset instead.
    const numericField = (...normalized: string[]) => {
      const idx = columns.findIndex((c) => normalized.includes(c.normalized));
      if (idx < 0) return "";
      return tokenOverlapping(line, columns[idx].start) || (fields[idx] ?? "");
    };
    const pid = numericField(...PID_HEADERS);
    if (!isNumeric(pid)) continue;

    const command = field(...COMMAND_HEADERS) || undefined;
    let name = field(...NAME_HEADERS);
    if (!name && command) name = nameFromCommand(command);
    if (!name) continue;

    const memoryColumn = columns.find((c) => MEMORY_HEADERS.has(c.normalized));
    const memoryKb = parseMemoryKb(
      memoryColumn ? numericField(memoryColumn.normalized) : "",
      memoryColumn?.normalized,
    );
    const ppid = numericField(...PPID_HEADERS) || undefined;
    const user = field(...USER_HEADERS) || undefined;
    const threads = parseNumeric(numericField(...THREAD_HEADERS));

    rows.push({
      key: `${pid}:${i}`,
      pid,
      ppid: ppid && isNumeric(ppid) ? ppid : undefined,
      user,
      name: name.slice(0, 200),
      threads,
      memoryKb,
      command,
      raw: line.trim(),
      depth: leadingDepth(line),
    });
  }

  if (rows.length === 0) return null;
  const format: ProcessListingFormat = commandColumn ? "ps" : "powershell";
  return { rows, format };
}

function parseGenericRows(lines: string[], cap: number): ProcessRow[] {
  const rows: ProcessRow[] = [];
  for (let i = 0; i < lines.length; i++) {
    if (cap > 0 && rows.length >= cap) break;
    const rawLine = lines[i];
    const line = rawLine.trim();
    if (!line || SEPARATOR_RE.test(line)) continue;
    const lower = line.toLowerCase();
    if (PS_HEADER_RE.test(line) && PS_TABLE_WORDS.some((word) => lower.includes(word))) continue;

    const tokens = line.split(/\s+/);
    const pidIdx = tokens.findIndex((token) => isNumeric(token));
    if (pidIdx < 0) continue;
    const rest = tokens.slice(pidIdx + 1);
    let ppid: string | undefined;
    if (rest.length >= 2 && isNumeric(rest[0])) ppid = rest.shift();

    const meaningful = rest.filter((token) => !/^[\d.:?%]+$/.test(token));
    const name = (
      meaningful.find((token) => /[/\\]/.test(token)) ||
      meaningful[0] ||
      rest[0] ||
      line
    ).slice(0, 200);
    rows.push({
      key: `${tokens[pidIdx]}:${i}`,
      pid: tokens[pidIdx],
      ppid,
      name,
      command: rest.length > 1 ? rest.join(" ") : undefined,
      raw: line,
      depth: leadingDepth(rawLine),
    });
  }
  return rows;
}

function inferTabFields(parts: string[]): {
  name: string;
  ppid?: string;
  user?: string;
  threads?: number;
  memoryKb?: number;
} | null {
  if (parts.length >= 5) {
    const second = parts[1]?.trim() ?? "";
    const third = parts[2]?.trim() ?? "";
    const fourth = parts[3]?.trim() ?? "";
    if (isNumeric(second) && isNumeric(third) && (isNumeric(fourth) || fourth === "-")) {
      return {
        name: parts.slice(4).join(" ").trim(),
        ppid: second,
        threads: parseNumeric(third),
        memoryKb: parseNumeric(fourth),
      };
    }
  }
  const first = parts[0]?.trim() ?? "";
  if (first.includes("PID")) return null;
  const pidPart = first.replace(/^\s+/, "");
  if (!isNumeric(pidPart) || !isNumeric(parts[1]?.trim() ?? "")) return null;
  return {
    name: parts.slice(parts.length >= 4 ? 3 : 2).join(" ").trim(),
    ppid: parts[1]?.trim(),
    user: parts.length >= 4 ? parts[2]?.trim() || undefined : undefined,
  };
}

function overlappingToken(line: string, start: number): { value: string; start: number } | null {
  const re = /\S+/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(line)) !== null) {
    if (match.index <= start && start < match.index + match[0].length) {
      return { value: match[0], start: match.index };
    }
  }
  return null;
}

function tokenOverlapping(line: string, start: number): string {
  return overlappingToken(line, start)?.value ?? "";
}

function isNumericColumn(normalized: string): boolean {
  return (
    PID_HEADERS.has(normalized) ||
    PPID_HEADERS.has(normalized) ||
    THREAD_HEADERS.has(normalized) ||
    MEMORY_HEADERS.has(normalized)
  );
}

function headerColumns(line: string): HeaderColumn[] {
  const columns: HeaderColumn[] = [];
  const re = /\S+/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(line)) !== null) {
    columns.push({
      label: match[0],
      normalized: normalizeHeader(match[0]),
      start: match.index,
    });
  }
  return columns;
}

function fieldAt(parts: string[], mapping: Map<string, number>, aliases: Set<string>): string {
  const idx = indexFor(mapping, aliases);
  return idx >= 0 ? (parts[idx] ?? "").trim() : "";
}

function headerAt(mapping: Map<string, number>, aliases: Set<string>): string {
  for (const [name, idx] of mapping) {
    if (aliases.has(name)) return `${name}:${idx}`;
  }
  return "";
}

function indexFor(mapping: Map<string, number>, aliases: Set<string>): number {
  for (const [name, idx] of mapping) {
    if (aliases.has(name)) return idx;
  }
  return -1;
}

function memoryHeaderPresent(mapping: Map<string, number>): boolean {
  return [...mapping.keys()].some((name) => MEMORY_HEADERS.has(name));
}

function normalizeHeader(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9_]/g, "");
}

function parseMemoryKb(value: string, label?: string): number | undefined {
  const parsed = parseNumeric(value);
  if (parsed === undefined || parsed < 0) return undefined;
  const normalized = (label ?? "").split(":")[0];
  if (normalized === "workingsetmb" || normalized === "workingset64mb") return Math.round(parsed * 1024);
  if (normalized === "workingset64") return Math.round(parsed / 1024);
  return parsed;
}

function parseNumeric(value: string | undefined): number | undefined {
  if (!value) return undefined;
  const cleaned = value.replace(/,/g, "").replace(/[^\d.-]/g, "");
  if (!cleaned) return undefined;
  const parsed = Number(cleaned);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function isNumeric(value: string | undefined): boolean {
  return !!value && /^\d+$/.test(value.trim());
}

function leadingDepth(rawLine: string): number {
  const leading = rawLine.match(/^[ \t]*/)?.[0] ?? "";
  if (!leading) return 0;
  const spaces = leading.replace(/\t/g, "  ");
  return Math.max(0, Math.floor(spaces.length / 2));
}

function nameFromCommand(command: string): string {
  const first = command.trim().split(/\s+/)[0] || command.trim();
  if (!first) return "";
  if (first.startsWith("[")) return first;
  return first.replace(/\\/g, "/").split("/").pop() || first;
}

/** Build a real parent/child tree, promoting orphans and breaking cycles safely. */
export function buildProcessTree(rows: ProcessRow[]): ProcessTreeNode[] {
  const nodes: ProcessTreeNode[] = rows.map((row) => ({ ...row, children: [] }));
  const byPid = new Map<string, ProcessTreeNode>();
  for (const node of nodes) {
    if (!byPid.has(node.pid)) byPid.set(node.pid, node);
  }

  const roots: ProcessTreeNode[] = [];
  for (const node of nodes) {
    // A duplicate PID cannot safely own children; keep it visible as an orphan.
    if (byPid.get(node.pid) !== node) {
      roots.push(node);
      continue;
    }
    if (!node.ppid || node.ppid === node.pid) {
      roots.push(node);
      continue;
    }
    const parent = byPid.get(node.ppid);
    if (!parent) roots.push(node);
    else parent.children.push(node);
  }

  const visited = new Set<ProcessTreeNode>();
  const visit = (node: ProcessTreeNode, depth: number) => {
    if (visited.has(node)) return;
    visited.add(node);
    node.depth = depth;
    node.children = node.children.filter((child) => !visited.has(child));
    for (const child of node.children) visit(child, depth + 1);
  };
  for (const node of roots) visit(node, 0);
  // A parent cycle has no natural root. Promote one node so the whole component
  // stays visible (the recursive visit already prevents an infinite loop).
  for (const node of nodes) {
    if (!visited.has(node)) {
      roots.push(node);
      visit(node, 0);
    }
  }
  return roots;
}

export function flattenProcessTree(
  nodes: ProcessTreeNode[],
  collapsed: ReadonlySet<string> = new Set(),
): ProcessTreeNode[] {
  const out: ProcessTreeNode[] = [];
  const visit = (node: ProcessTreeNode) => {
    out.push(node);
    if (!collapsed.has(node.key)) for (const child of node.children) visit(child);
  };
  for (const node of nodes) visit(node);
  return out;
}

export function filterProcessRows(rows: ProcessRow[], query: string): ProcessRow[] {
  const q = query.trim().toLowerCase();
  if (!q) return rows;
  return rows.filter((row) => matchesProcessRow(row, q));
}

/** Keep matching nodes and every ancestor needed to reach them in the tree. */
export function filterProcessTree(nodes: ProcessTreeNode[], query: string): ProcessTreeNode[] {
  const q = query.trim().toLowerCase();
  if (!q) return nodes;
  const visit = (node: ProcessTreeNode): ProcessTreeNode | null => {
    const children = node.children
      .map(visit)
      .filter((child): child is ProcessTreeNode => child !== null);
    if (!matchesProcessRow(node, q) && children.length === 0) return null;
    return { ...node, children };
  };
  return nodes
    .map(visit)
    .filter((node): node is ProcessTreeNode => node !== null);
}

function matchesProcessRow(row: ProcessRow, q: string): boolean {
  return [row.pid, row.ppid, row.name, row.user, row.command, row.raw]
    .some((value) => !!value && value.toLowerCase().includes(q));
}

export type ProcessSortKey = "pid" | "ppid" | "name" | "user" | "threads" | "memory";
export type ProcessSortDir = "asc" | "desc";

export function sortProcessRows(
  rows: ProcessRow[],
  key: ProcessSortKey,
  dir: ProcessSortDir,
): ProcessRow[] {
  return [...rows].sort((a, b) => compareRows(a, b, key, dir));
}

export function sortProcessTree(
  nodes: ProcessTreeNode[],
  key: ProcessSortKey,
  dir: ProcessSortDir,
): ProcessTreeNode[] {
  return [...nodes]
    .sort((a, b) => compareRows(a, b, key, dir))
    .map((node) => ({
      ...node,
      children: sortProcessTree(node.children, key, dir),
    }));
}

function compareRows(a: ProcessRow, b: ProcessRow, key: ProcessSortKey, dir: ProcessSortDir): number {
  const sign = dir === "asc" ? 1 : -1;
  if (key === "pid" || key === "ppid" || key === "threads" || key === "memory") {
    const av = numericValue(a, key);
    const bv = numericValue(b, key);
    if (av === bv) return 0;
    return (av - bv) * sign;
  }
  const av = String(a[key] ?? "").toLowerCase();
  const bv = String(b[key] ?? "").toLowerCase();
  return av.localeCompare(bv) * sign;
}

function numericValue(row: ProcessRow, key: "pid" | "ppid" | "threads" | "memory"): number {
  if (key === "memory") return row.memoryKb ?? Number.POSITIVE_INFINITY;
  if (key === "threads") return row.threads ?? Number.POSITIVE_INFINITY;
  const value = row[key];
  return value !== undefined && value !== "" ? Number(value) : Number.POSITIVE_INFINITY;
}

const PROTECTED_NAMES = new Set([
  "system idle process",
  "system",
  "smss.exe",
  "csrss.exe",
  "wininit.exe",
  "winlogon.exe",
  "services.exe",
  "lsass.exe",
  "init",
  "systemd",
  "launchd",
  "kthreadd",
  "[kthreadd]",
]);

/** Returns a protection reason when killing this row could destabilize the host. */
export function processProtection(row: Pick<ProcessRow, "pid" | "name">): "core" | "critical" | null {
  const pid = Number(row.pid);
  if (Number.isFinite(pid) && pid >= 0 && pid <= 4) return "core";
  if (PROTECTED_NAMES.has(row.name.trim().toLowerCase())) return "critical";
  return null;
}

export function isProtectedPid(pid: string, rows: ProcessRow[] = []): boolean {
  const row = rows.find((candidate) => candidate.pid === pid);
  if (row) return processProtection(row) !== null;
  return isNumeric(pid) && Number(pid) <= 4;
}

export function describeProcessSnapshot(payload: Record<string, unknown> | null | undefined): ProcessSnapshotMeta {
  const text = typeof payload?.processes === "string" ? payload.processes : "";
  const kind = String(payload?.kind ?? PROCESS_SNAPSHOT_KIND);
  const live = payload?.live === true && kind !== PROCESS_SNAPSHOT_KIND;
  const source = String(payload?.source ?? payload?.alias_of ?? "ps");
  const taskIdRaw = payload?.task_id;
  const taskId =
    typeof taskIdRaw === "number" && Number.isFinite(taskIdRaw)
      ? taskIdRaw
      : typeof taskIdRaw === "string" && /^\d+$/.test(taskIdRaw)
        ? Number(taskIdRaw)
        : undefined;
  const collectedAt = typeof payload?.collected_at === "string" ? payload.collected_at : undefined;
  return { text, source, live, kind, taskId, collectedAt };
}
