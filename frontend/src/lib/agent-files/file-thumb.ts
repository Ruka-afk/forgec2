/** Parsing and batching for the bounded `file_thumb` task. */

import type { FileSearchCategory, FileSearchHit } from "./file-search";

interface FileThumbItem {
  path: string;
  ok: boolean;
  thumbId: string;
  width: number;
  height: number;
  sourceSize: number;
  modified: string;
  error: string;
}

interface FileThumbResult {
  items: FileThumbItem[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function asNumber(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

function parseItem(value: unknown): FileThumbItem | null {
  if (!isRecord(value)) return null;
  const path = asString(value.path);
  if (!path) return null;
  const thumbId = asString(value.thumb_id);
  return {
    path,
    ok: value.ok === true && thumbId.length > 0,
    thumbId,
    width: Math.max(0, asNumber(value.width)),
    height: Math.max(0, asNumber(value.height)),
    sourceSize: Math.max(0, asNumber(value.source_size)),
    modified: asString(value.modified),
    error: asString(value.error),
  };
}

/** Returns null when the task result is not a thumbnail document at all. */
export function parseFileThumbResult(raw: string): FileThumbResult | null {
  if (!raw.trim()) return null;
  let decoded: unknown;
  try {
    decoded = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!isRecord(decoded) || !Array.isArray(decoded.items)) return null;
  const items: FileThumbItem[] = [];
  for (const value of decoded.items) {
    const item = parseItem(value);
    if (item) items.push(item);
  }
  return { items };
}

export function isThumbCandidate(category: FileSearchCategory): boolean {
  return category === "image" || category === "video";
}

export function thumbCandidateBatches(
  hits: FileSearchHit[],
  limit = 24,
  batchSize = 12,
): FileSearchHit[][] {
  const candidates = hits.filter((hit) => isThumbCandidate(hit.category)).slice(0, limit);
  const batches: FileSearchHit[][] = [];
  for (let i = 0; i < candidates.length; i += batchSize) {
    batches.push(candidates.slice(i, i + batchSize));
  }
  return batches;
}
