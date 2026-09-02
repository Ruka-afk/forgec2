export type AIToolOutputStatus = "running" | "success" | "error" | "waiting_approval";

export interface AIToolOutputView {
  raw: string;
  formatted: string;
  isJson: boolean;
  status: AIToolOutputStatus;
  itemCount: number | null;
  lineCount: number;
  byteCount: number;
  partial: boolean;
  originalBytes: number | null;
}

function stripResultLabel(content: string): string {
  return content.replace(/^\s*(?:result|结果)\s*:\s*/i, "").trim();
}

function countItems(value: unknown): number | null {
  if (Array.isArray(value)) return value.length;
  if (!value || typeof value !== "object") return null;
  const record = value as Record<string, unknown>;
  for (const key of ["items", "results", "agents", "tasks", "credentials", "alerts", "data"]) {
    if (Array.isArray(record[key])) return record[key].length;
  }
  if (typeof record.count === "number") return record.count;
  if (typeof record.total === "number") return record.total;
  if (record.data && typeof record.data === "object") return countItems(record.data);
  return null;
}

function statusFromValue(value: unknown, fallback: AIToolOutputStatus): AIToolOutputStatus {
  if (!value || typeof value !== "object") return fallback;
  const record = value as Record<string, unknown>;
  if (record.error || record.ok === false || record.success === false || record.status === "failed" || record.status === "error") {
    return "error";
  }
  if (record.status === "pending" || record.status === "waiting_approval") return "waiting_approval";
  return fallback;
}

export function describeToolOutput(
  content: string,
  fallbackStatus: AIToolOutputStatus = "success",
): AIToolOutputView {
  const raw = stripResultLabel(content);
  let formatted = raw;
  let isJson = false;
  let itemCount: number | null = null;
  let status = fallbackStatus;
  let partial = false;
  let originalBytes: number | null = null;
  try {
    const parsed = JSON.parse(raw) as unknown;
    formatted = JSON.stringify(parsed, null, 2);
    isJson = true;
    itemCount = countItems(parsed);
    status = statusFromValue(parsed, fallbackStatus);
    if (parsed && typeof parsed === "object") {
      const meta = (parsed as Record<string, unknown>)._meta;
      if (meta && typeof meta === "object") {
        const record = meta as Record<string, unknown>;
        partial = record.partial === true;
        originalBytes = typeof record.original_bytes === "number" ? record.original_bytes : null;
      }
    }
  } catch {
    if (/\b(error|failed|failure|denied|forbidden)\b|失败|错误|拒绝/i.test(raw)) status = "error";
  }
  return {
    raw,
    formatted,
    isJson,
    status,
    itemCount,
    lineCount: formatted ? formatted.split(/\r?\n/).length : 0,
    byteCount: new TextEncoder().encode(raw).length,
    partial,
    originalBytes,
  };
}

export function formatAIRunDuration(durationMs?: number): string {
  if (durationMs == null || durationMs < 0) return "";
  if (durationMs < 1000) return `${Math.round(durationMs)}ms`;
  if (durationMs < 60_000) return `${(durationMs / 1000).toFixed(durationMs < 10_000 ? 1 : 0)}s`;
  const minutes = Math.floor(durationMs / 60_000);
  const seconds = Math.round((durationMs % 60_000) / 1000);
  return `${minutes}m ${seconds}s`;
}

export function extractAICitations(content: string): string[] {
  const citations: string[] = [];
  const seen = new Set<string>();
  const pattern = /\[(?:source|来源)\s*:\s*([^\]\r\n]{1,180})\]/gi;
  for (const match of content.matchAll(pattern)) {
    const label = match[1].trim();
    if (!label || seen.has(label)) continue;
    seen.add(label);
    citations.push(label);
    if (citations.length >= 8) break;
  }
  return citations;
}
