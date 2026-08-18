/**
 * Local-only client telemetry: Web Vitals + uncaught client errors.
 *
 * Red line: this data NEVER leaves the browser. Entries live in an in-memory
 * ring buffer and are shown in Settings → Telemetry. Nothing is persisted,
 * logged to the network, or sent to any endpoint.
 */

export type VitalName = "TTFB" | "FCP" | "LCP" | "CLS" | "FID" | "INP";

export type TelemetryEntry =
  | { ts: number; kind: "vital"; name: VitalName; value: number }
  | { ts: number; kind: "error"; source: string; message: string };

const MAX_ENTRIES = 100;

const entries: TelemetryEntry[] = [];
const listeners = new Set<() => void>();

function push(entry: TelemetryEntry) {
  entries.push(entry);
  if (entries.length > MAX_ENTRIES) entries.splice(0, entries.length - MAX_ENTRIES);
  for (const fn of listeners) fn();
}

export function getTelemetryEntries(): readonly TelemetryEntry[] {
  return entries;
}

export function clearTelemetry(): void {
  entries.splice(0, entries.length);
  for (const fn of listeners) fn();
}

export function recordVital(name: VitalName, value: number): void {
  if (!Number.isFinite(value)) return;
  push({ ts: Date.now(), kind: "vital", name, value });
}

export function recordClientError(source: string, message: string): void {
  const msg = String(message ?? "").slice(0, 300);
  if (!msg) return;
  push({ ts: Date.now(), kind: "error", source, message: msg });
}

export function subscribeTelemetry(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}