/**
 * Parsing and request shaping for the bounded `file_scan` task.
 *
 * The agent returns raw match values. They are intentionally rendered without
 * masking in the operator UI, but must never be sent to AI, callbacks or
 * exports. That redaction happens server-side; this module only parses the
 * encrypted task result the operator is allowed to see.
 */

export const FILE_SCAN_RULES = [
  "email",
  "phone",
  "bank_card",
  "ssn",
  "aws_key",
  "private_key",
  "jwt",
  "password",
] as const;

export type FileScanRule = (typeof FILE_SCAN_RULES)[number];

export const FILE_SCAN_DEFAULT_RULES: FileScanRule[] = ["email", "phone", "bank_card"];

interface FileScanFinding {
  path: string;
  name: string;
  rule: string;
  value: string;
  line: number;
  occurrences: number;
}

export interface FileScanResult {
  root: string;
  rules: string[];
  filesScanned: number;
  filesSkipped: number;
  skippedBinary: number;
  skippedLarge: number;
  bytesScanned: number;
  findings: FileScanFinding[];
  counts: Record<string, number>;
  errors: number;
  truncated: boolean;
  truncatedReason: string;
  elapsedMs: number;
}

interface FileScanParams {
  root: string;
  pattern: string;
  rules: string[];
  includeHidden?: boolean;
  maxDepth?: number;
  maxFiles?: number;
  maxFileBytes?: number;
  maxTotalBytes?: number;
  maxFindings?: number;
  timeoutSeconds?: number;
}

function positive(value: number | undefined, fallback: number): number {
  return Number.isFinite(value) && (value as number) > 0 ? Math.floor(value as number) : fallback;
}

export function buildFileScanForm(params: FileScanParams): Record<string, string> {
  return {
    path: params.root,
    pattern: params.pattern.trim() || "*",
    rules: params.rules.join(","),
    include_hidden: params.includeHidden ? "1" : "0",
    max_depth: String(positive(params.maxDepth, 8)),
    max_files: String(positive(params.maxFiles, 200)),
    max_file_bytes: String(positive(params.maxFileBytes, 1024 * 1024)),
    max_total_bytes: String(positive(params.maxTotalBytes, 32 * 1024 * 1024)),
    max_findings: String(positive(params.maxFindings, 500)),
    timeout_seconds: String(positive(params.timeoutSeconds, 30)),
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function asNumber(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) ? Math.max(0, n) : 0;
}

function basename(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

function parseFinding(value: unknown): FileScanFinding | null {
  if (!isRecord(value)) return null;
  const path = asString(value.path);
  const rule = asString(value.rule);
  const matched = asString(value.value);
  if (!path || !rule || !matched) return null;
  return {
    path,
    name: asString(value.name) || basename(path),
    rule,
    value: matched,
    line: Math.max(1, Math.floor(asNumber(value.line))),
    occurrences: Math.max(1, Math.floor(asNumber(value.occurrences))),
  };
}

/**
 * Parse one file_scan result. Returns null for an ACK/error/truncated JSON
 * payload so the panel can show an honest failure instead of "no findings".
 */
export function parseFileScanResult(raw: string): FileScanResult | null {
  if (!raw.trim()) return null;
  let decoded: unknown;
  try {
    decoded = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!isRecord(decoded) || !Array.isArray(decoded.findings)) return null;

  const findings: FileScanFinding[] = [];
  for (const item of decoded.findings) {
    const finding = parseFinding(item);
    if (finding) findings.push(finding);
  }
  const counts: Record<string, number> = {};
  if (isRecord(decoded.counts)) {
    for (const [rule, value] of Object.entries(decoded.counts)) {
      const n = asNumber(value);
      if (n > 0) counts[rule] = n;
    }
  }
  return {
    root: asString(decoded.root),
    rules: Array.isArray(decoded.rules) ? decoded.rules.filter((rule): rule is string => typeof rule === "string") : [],
    filesScanned: Math.floor(asNumber(decoded.files_scanned)),
    filesSkipped: Math.floor(asNumber(decoded.files_skipped)),
    skippedBinary: Math.floor(asNumber(decoded.skipped_binary)),
    skippedLarge: Math.floor(asNumber(decoded.skipped_large)),
    bytesScanned: Math.floor(asNumber(decoded.bytes_scanned)),
    findings,
    counts,
    errors: Math.floor(asNumber(decoded.errors)),
    truncated: decoded.truncated === true,
    truncatedReason: asString(decoded.truncated_reason),
    elapsedMs: Math.floor(asNumber(decoded.elapsed_ms)),
  };
}
