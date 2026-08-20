import type { FileEntry } from "./types";

/** POST /files/ls queues an ls task and returns this ACK — not a directory listing. */
export function isFilesLsAck(data: unknown): boolean {
  if (!data || typeof data !== "object" || Array.isArray(data)) return false;
  const o = data as Record<string, unknown>;
  const taskId = Number(o.task_id);
  if (!Number.isFinite(taskId) || taskId <= 0) return false;
  if (o.kind === "ls_task" || o.queued === true) return true;
  return !hasFileArray(o);
}

export function filesLsTaskId(data: unknown): number | null {
  if (!data || typeof data !== "object") return null;
  const id = Number((data as Record<string, unknown>).task_id);
  return Number.isFinite(id) && id > 0 ? id : null;
}

function hasFileArray(o: Record<string, unknown>): boolean {
  for (const k of ["files", "entries", "Files", "Entries"]) {
    if (Array.isArray(o[k])) return true;
  }
  return false;
}

/** Immediate listing only when the payload actually carries entries — never from a queue ACK. */
export function extractImmediateListing(data: unknown): FileEntry[] | null {
  if (isFilesLsAck(data)) return null;
  if (!data || typeof data !== "object") return null;
  const o = data as Record<string, unknown>;
  for (const k of ["files", "entries", "Files", "Entries"]) {
    if (Array.isArray(o[k])) return o[k] as FileEntry[];
  }
  return null;
}

function decodeLsResult(result: string): string {
  const raw = result || "";
  const trimmed = raw.trim();
  if (!trimmed) return "";
  if (looksLikeLsTable(raw)) return raw;
  try {
    const dec = atob(trimmed);
    if (looksLikeLsTable(dec)) return dec;
  } catch {
    /* not base64 */
  }
  return raw;
}

function looksLikeLsTable(text: string): boolean {
  return /Type\tName/.test(text) || /^\s*(DIR|FILE)\t/m.test(text);
}

export function parseLsListing(result: string): FileEntry[] {
  const text = decodeLsResult(result);
  const entries: FileEntry[] = [];
  for (const line of text.split(/\r?\n/)) {
    const parts = line.split("\t");
    if (parts.length < 2) continue;
    const typ = parts[0].trim().toUpperCase();
    if (typ !== "DIR" && typ !== "FILE") continue;
    const sizeRaw = (parts[2] || "").trim();
    entries.push({
      name: parts[1],
      is_dir: typ === "DIR",
      size: typ === "DIR" || sizeRaw === "-" ? 0 : Number(sizeRaw) || 0,
      mod_time: (parts[3] || "").trim(),
    });
  }
  return entries;
}
