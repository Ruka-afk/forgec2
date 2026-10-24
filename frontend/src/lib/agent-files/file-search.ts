/** Metadata-only file search shared by the media browser and its tests. */

export const FILE_SEARCH_CATEGORIES = [
  "image",
  "video",
  "audio",
  "document",
  "archive",
  "key",
  "config",
  "other",
] as const;

export type FileSearchCategory = (typeof FILE_SEARCH_CATEGORIES)[number];

export const MEDIA_DEFAULT_CATEGORIES: FileSearchCategory[] = ["image", "video", "audio"];

export interface FileSearchHit {
  path: string;
  name: string;
  ext: string;
  category: FileSearchCategory;
  size: number;
  modified: string;
}

export interface FileSearchResult {
  root: string;
  hits: FileSearchHit[];
  scanned: number;
  errors: number;
  truncated: boolean;
  truncatedReason: string;
  elapsedMs: number;
}

interface FileSearchParams {
  root: string;
  pattern: string;
  categories: FileSearchCategory[];
  minSize?: number;
  maxSize?: number;
  modifiedAfter?: string;
  modifiedBefore?: string;
  includeHidden?: boolean;
  maxDepth?: number;
  maxResults?: number;
  timeoutSeconds?: number;
}

export function emptyFileSearchResult(root = ""): FileSearchResult {
  return {
    root,
    hits: [],
    scanned: 0,
    errors: 0,
    truncated: false,
    truncatedReason: "",
    elapsedMs: 0,
  };
}

function positive(value: number | undefined, fallback: number): number {
  return Number.isFinite(value) && (value as number) > 0 ? Math.floor(value as number) : fallback;
}

export function buildFileSearchForm(params: FileSearchParams): Record<string, string> {
  return {
    path: params.root,
    pattern: params.pattern.trim() || "*",
    categories: params.categories.join(","),
    min_size: String(positive(params.minSize, 0)),
    max_size: String(positive(params.maxSize, 0)),
    modified_after: params.modifiedAfter ?? "",
    modified_before: params.modifiedBefore ?? "",
    include_hidden: params.includeHidden ? "1" : "0",
    max_depth: String(positive(params.maxDepth, 8)),
    max_results: String(positive(params.maxResults, 2000)),
    timeout_seconds: String(positive(params.timeoutSeconds, 20)),
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function asNumber(value: unknown, fallback = 0): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : fallback;
}

function isCategory(value: string): value is FileSearchCategory {
  return (FILE_SEARCH_CATEGORIES as readonly string[]).includes(value);
}

function basename(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

function extension(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot + 1).toLowerCase() : "";
}

function parseHit(value: unknown): FileSearchHit | null {
  if (!isRecord(value)) return null;
  const path = asString(value.path);
  if (!path) return null;
  const name = asString(value.name) || basename(path);
  const categoryRaw = asString(value.category).toLowerCase();
  const category = isCategory(categoryRaw) ? categoryRaw : "other";
  return {
    path,
    name,
    ext: asString(value.ext).toLowerCase() || extension(name),
    category,
    size: Math.max(0, asNumber(value.size)),
    modified: asString(value.modified),
  };
}

/**
 * Parse one `file_search` task result. Returns null when the payload is not a
 * search document at all (an ACK, an error blob, or a truncated JSON result),
 * so the caller can show an honest error instead of an empty list.
 */
export function parseFileSearchResult(raw: string): FileSearchResult | null {
  if (!raw.trim()) return null;
  let decoded: unknown;
  try {
    decoded = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!isRecord(decoded) || !Array.isArray(decoded.hits)) return null;

  const hits: FileSearchHit[] = [];
  for (const item of decoded.hits) {
    const hit = parseHit(item);
    if (hit) hits.push(hit);
  }
  return {
    root: asString(decoded.root),
    hits,
    scanned: Math.max(0, asNumber(decoded.scanned)),
    errors: Math.max(0, asNumber(decoded.errors)),
    truncated: decoded.truncated === true,
    truncatedReason: asString(decoded.truncated_reason),
    elapsedMs: Math.max(0, asNumber(decoded.elapsed_ms)),
  };
}
