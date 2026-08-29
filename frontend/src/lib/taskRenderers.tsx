"use client";

// taskRenderers — pluggable per-task-type result rendering.
//
// Modules can ship their own result view instead of forcing operators to read
// raw text: register a component against the task type and every result
// surface (task rows, detail modals) picks it up automatically. Types without
// a registration fall back to the classic <pre> output, so the registry is
// purely additive and never blocks a new command's results.

import type { ComponentType } from "react";
import { RefreshCw, ShieldAlert, Monitor, Package, Network, Clock } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

interface TaskRendererProps {
  /** Raw result payload exactly as stored on the task row. */
  result: string;
  /** Canonical task type the renderer was selected for. */
  taskType: string;
}

type TaskRenderer = ComponentType<TaskRendererProps>;

const registry = new Map<string, TaskRenderer>();

/**
 * Register a renderer for a canonical task type. Later registrations
 * overwrite earlier ones (useful for operator overrides).
 */
export function registerTaskRenderer(taskType: string, renderer: TaskRenderer): void {
  registry.set(taskType, renderer);
}

/** Look up the renderer for a task type, if any. */
export function getTaskRenderer(taskType: string): TaskRenderer | undefined {
  return registry.get(taskType);
}

/** Test/debug helper: currently registered task types. */
export function registeredRendererTypes(): string[] {
  return [...registry.keys()].sort();
}

// ── hostinfo renderer ───────────────────────────────────────────────────────

interface HostInfoReport {
  category?: string;
  collected_at?: string;
  platform?: string;
  sections?: Record<string, Record<string, unknown>>;
}

function asStr(v: unknown): string {
  if (v === null || v === undefined) return "";
  return String(v);
}

function asRows(v: unknown): Record<string, unknown>[] {
  if (!Array.isArray(v)) return [];
  return v.filter((x): x is Record<string, unknown> => typeof x === "object" && x !== null);
}

/** Literal-key mapper: the i18n scanner counts only t("…") literals. */


function sectionIcon(key: string) {
  switch (key) {
    case "security": return <ShieldAlert className="size-3.5" />;
    case "system": return <Monitor className="size-3.5" />;
    case "software": return <Package className="size-3.5" />;
    case "network": return <Network className="size-3.5" />;
    default: return <Clock className="size-3.5" />;
  }
}

function MiniTable({ rows }: { rows: Record<string, unknown>[] }) {
  if (rows.length === 0) return null;
  const cols = Object.keys(rows[0]);
  return (
    <div className="overflow-x-auto scrollbar-thin">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border text-left text-muted-foreground">
            {cols.map((c) => (
              <th key={c} className="py-1 pr-4 font-medium whitespace-nowrap">{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i} className="border-b border-border/40 last:border-0">
              {cols.map((c) => (
                <td key={c} className="py-1 pr-4 font-mono align-top max-w-[280px] truncate" title={asStr(row[c])}>
                  {asStr(row[c])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ProtectionBadge({ state }: { state: string }) {
  const tone =
    state === "enabled"
      ? "bg-destructive/10 text-destructive"
      : state === "disabled"
        ? "bg-success/10 text-success"
        : "bg-muted text-muted-foreground";
  return <Badge variant="outline" className={cn("text-(--fs-micro-sm)", tone)}>{state}</Badge>;
}

/**
 * Structured view for `hostinfo` sweeps: grouped sections with mini tables
 * plus protection badges for AV entries. Falls back to the raw payload when
 * the result is not the expected JSON shape.
 */
export function HostInfoResultView({ result }: TaskRendererProps) {
  let report: HostInfoReport;
  try {
    report = JSON.parse(result) as HostInfoReport;
    if (!report || typeof report !== "object" || !report.sections) throw new Error("shape");
  } catch {
    return <pre className="text-xs text-success font-mono overflow-x-auto max-h-96 p-2 whitespace-pre-wrap break-all">{result}</pre>;
  }

  return (
    <div className="space-y-4" data-testid="hostinfo-renderer">
      <div className="flex items-center gap-2 text-(--fs-micro-sm) text-muted-foreground">
        <RefreshCw className="size-3" />
        <span className="font-mono">{asStr(report.platform)}</span>
        <span>·</span>
        <span className="font-mono">{asStr(report.collected_at)}</span>
      </div>
      {Object.entries(report.sections).map(([key, data]) => {
        // A section can legitimately be null (e.g. a failed sweep serializes
        // "network": null); guard before property access or this crashes.
        if (!data || typeof data !== "object" || Array.isArray(data)) return null;
        const av = asRows(data.av_products);
        return (
          <div key={key}>
            <div className="flex items-center gap-1.5 mb-1.5">
              {sectionIcon(key)}
              <span className="text-xs font-semibold">{key}</span>
            </div>
            {typeof data.error === "string" && (
              <p className="text-xs text-destructive">{data.error}</p>
            )}
            {av.length > 0 && (
              <div className="mb-1.5 flex flex-wrap gap-1.5">
                {av.map((a, i) => (
                  <span key={i} className="inline-flex items-center gap-1.5 rounded-lg border border-border px-2 py-0.5 text-xs">
                    <span className="font-medium">{asStr(a.name)}</span>
                    <ProtectionBadge state={asStr(a.protection) || "unknown"} />
                  </span>
                ))}
              </div>
            )}
            {Object.entries(data)
              .filter(([, v]) => Array.isArray(v))
              .map(([k, v]) => {
                const rows = asRows(v);
                if (rows.length === 0) return null;
                return (
                  <div key={k} className="mt-1.5">
                    <MiniTable rows={rows} />
                  </div>
                );
              })}
          </div>
        );
      })}
    </div>
  );
}

registerTaskRenderer("hostinfo", HostInfoResultView);
