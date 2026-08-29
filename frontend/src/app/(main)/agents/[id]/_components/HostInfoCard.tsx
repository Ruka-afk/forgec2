"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { RefreshCw, ShieldAlert, Monitor, Package, Network, Clock } from "lucide-react";
import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { SectionCard } from "@/components/ui/section-card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

/**
 * HostInfoCard — dispatches the on-demand `hostinfo` sweep and renders the
 * structured JSON report as grouped tables. Category selection keeps the
 * collection targeted; the refresh button re-runs the sweep.
 */

type HostInfoSection = Record<string, unknown>;
interface HostInfoReport {
  category: string;
  collected_at: string;
  platform: string;
  filter?: string;
  sections: Record<string, HostInfoSection>;
}

const CATEGORIES = ["all", "security", "system", "software", "network", "runtime"] as const;

function asArray(v: unknown): Record<string, unknown>[] {
  if (!Array.isArray(v)) return [];
  return v.filter((x): x is Record<string, unknown> => typeof x === "object" && x !== null);
}

function asStr(v: unknown): string {
  if (v === null || v === undefined) return "";
  return String(v);
}

/** Key/value strip for flat objects (skips nested arrays/objects). */
function KeyValueGrid({ data }: { data: Record<string, unknown> }) {
  const entries = Object.entries(data).filter(
    ([, v]) => typeof v !== "object" || v === null,
  );
  if (entries.length === 0) return null;
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-1 text-xs">
      {entries.map(([k, v]) => (
        <div key={k} className="flex items-baseline gap-2 min-w-0">
          <span className="shrink-0 text-muted-foreground">{k}</span>
          <span className="font-mono truncate" title={asStr(v)}>{asStr(v)}</span>
        </div>
      ))}
    </div>
  );
}

/** Generic object-table for array-of-object sections. */
function ObjectTable({ rows }: { rows: Record<string, unknown>[] }) {
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

function SecuritySection({ data }: { data: HostInfoSection }) {
  const { t } = useI18n();
  const av = asArray(data.av_products);
  const edr = asArray(data.edr_processes);
  return (
    <div className="space-y-3">
      {av.length > 0 && (
        <div>
          <div className="text-(--fs-micro-sm) uppercase tracking-wider text-muted-foreground mb-1">AV</div>
          <ObjectTable rows={av.map((a) => ({
            name: asStr(a.name),
            protection: asStr(a.protection),
            signatures: asStr(a.signatures),
            state_hex: asStr(a.state_hex),
          }))} />
        </div>
      )}
      {/* Visual fallback when the table renderer is absent */}
      {av.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {av.map((a, i) => (
            <ProtectionBadge key={i} state={asStr(a.protection) || "unknown"} />
          ))}
        </div>
      )}
      {edr.length > 0 && (
        <div>
          <div className="text-(--fs-micro-sm) uppercase tracking-wider text-muted-foreground mb-1">EDR</div>
          <ObjectTable rows={edr.map((e) => ({
            product: asStr(e.product),
            process: asStr(e.process),
            pid: asStr(e.pid),
          }))} />
        </div>
      )}
      {av.length === 0 && edr.length === 0 && !data.error && !data.av_raw && (
        <p className="text-xs text-muted-foreground">{t("hostinfo.no_security_products")}</p>
      )}
      {typeof data.av_raw === "string" && (
        <pre className="text-(--fs-micro-sm) bg-muted/50 rounded-lg p-2 overflow-x-auto whitespace-pre-wrap">{asStr(data.av_raw)}</pre>
      )}
    </div>
  );
}

export function HostInfoCard({ agentId, online }: { agentId: string; online: boolean }) {
  const { t } = useI18n();
  // Literal-key mappers: the i18n usage scanner only counts t("…") literals,
  // so dynamic template keys would be flagged as dead.
  const catLabel = useCallback((c: string) => {
    switch (c) {
      case "security": return t("hostinfo.cat_security");
      case "system": return t("hostinfo.cat_system");
      case "software": return t("hostinfo.cat_software");
      case "network": return t("hostinfo.cat_network");
      case "runtime": return t("hostinfo.cat_runtime");
      default: return t("hostinfo.cat_all");
    }
  }, [t]);
  const secLabel = useCallback((key: string) => {
    switch (key) {
      case "security": return t("hostinfo.sec_security");
      case "system": return t("hostinfo.sec_system");
      case "software": return t("hostinfo.sec_software");
      case "network": return t("hostinfo.sec_network");
      case "runtime": return t("hostinfo.sec_runtime");
      default: return key;
    }
  }, [t]);
  const [category, setCategory] = useState<string>("security");
  const [filter, setFilter] = useState("");
  const [report, setReport] = useState<HostInfoReport | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const acRef = useRef<AbortController | null>(null);

  // Clean up any in-flight request on unmount
  useEffect(() => () => { acRef.current?.abort(); }, []);

  const collect = useCallback(async () => {
    if (!agentId || busy) return;
    acRef.current?.abort();
    const ac = new AbortController();
    acRef.current = ac;
    setBusy(true);
    setError("");
    try {
      const fd = new FormData();
      fd.set("category", category);
      if (category === "software" && filter.trim()) fd.set("filter", filter.trim());
      const dispatched = await api.postFormData<{ success?: boolean; task_id?: number }>(
        paths.agents.cmd(agentId, "hostinfo"), fd,
      );
      const taskId = dispatched.task_id;
      if (!taskId) throw new Error("no task id");
      await pollTask(agentId, taskId, { timeoutMs: 120_000, signal: ac.signal });
      // B5 fix: api.get already strips {success,data} envelope, so the
      // response IS the task object (with .result, .error, .status).
      // Previous code did status.data.result → double unwrap → undefined.
      const task = await api.get(paths.agents.task(agentId, taskId), { signal: ac.signal }) as { result?: string; error?: string; status?: string; data?: { result?: string; error?: string } };
      // Handle both unwrapped (direct task) and still-wrapped (edge case) forms.
      const taskResult = task.result ?? task.data?.result ?? "";
      const taskError = task.error ?? task.data?.error ?? "";
      if (!taskResult) {
        throw new Error(taskError || t("hostinfo.error_empty"));
      }
      let parsed: HostInfoReport;
      try {
        parsed = JSON.parse(taskResult) as HostInfoReport;
      } catch {
        throw new Error(t("hostinfo.error_generic"));
      }
      if (ac.signal.aborted) return;
      setReport(parsed);
    } catch (e) {
      if ((e as Error).name === "AbortError") return;
      setError(e instanceof Error ? e.message : t("hostinfo.error_generic"));
    } finally {
      setBusy(false);
    }
  }, [agentId, busy, category, filter, t]);

  const sectionIcon = (key: string) => {
    switch (key) {
      case "security": return <ShieldAlert className="size-3.5" />;
      case "system": return <Monitor className="size-3.5" />;
      case "software": return <Package className="size-3.5" />;
      case "network": return <Network className="size-3.5" />;
      default: return <Clock className="size-3.5" />;
    }
  };

  const renderSection = (key: string, data: HostInfoSection) => {
    if (data.error) {
      return <p className="text-xs text-destructive">{asStr(data.error)}</p>;
    }
    switch (key) {
      case "security":
        return <SecuritySection data={data} />;
      default: {
        // Generic rendering: arrays -> tables, flat fields -> key/value grid.
        const arrays = Object.entries(data).filter(([, v]) => Array.isArray(v));
        const scalars = Object.fromEntries(
          Object.entries(data).filter(([, v]) => typeof v !== "object" || v === null),
        );
        return (
          <div className="space-y-3">
            <KeyValueGrid data={scalars} />
            {arrays.map(([k, v]) => {
              const rows = asArray(v);
              if (rows.length === 0) return null;
              return (
                <div key={k}>
                  <div className="text-(--fs-micro-sm) uppercase tracking-wider text-muted-foreground mb-1">{k}</div>
                  <ObjectTable rows={rows} />
                </div>
              );
            })}
          </div>
        );
      }
    }
  };

  return (
    <SectionCard
      title={t("hostinfo.title")}
      icon={<ShieldAlert className="size-4 text-primary" />}
      description={report ? `${report.platform} · ${report.collected_at}` : undefined}
      headerAccent
      collapsible
      defaultOpen={false}
      action={
        <div className="flex items-center gap-1.5">
          <Select value={category} onValueChange={(v) => setCategory(v ?? "security")}>
            <SelectTrigger className="h-7 w-[130px] text-xs" aria-label={t("hostinfo.category")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {CATEGORIES.map((c) => (
                <SelectItem key={c} value={c} className="text-xs">{catLabel(c)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          {category === "software" && (
            <Input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder={t("hostinfo.filter_placeholder")}
              className="h-7 w-[140px] text-xs"
            />
          )}
          <Button size="sm" variant="outline" onClick={collect} disabled={busy || !online}
            aria-label={t("hostinfo.refresh")}>
            {busy ? <RefreshCw className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
            <span className="hidden sm:inline">{busy ? t("hostinfo.collecting") : t("hostinfo.refresh")}</span>
          </Button>
        </div>
      }
    >
      {!online && <p className="text-xs text-muted-foreground">{t("hostinfo.offline_hint")}</p>}
      {online && !report && !busy && !error && (
        <p className="text-xs text-muted-foreground">{t("hostinfo.hint")}</p>
      )}
      {error && <p className="text-xs text-destructive">{error}</p>}
      {report && (
        <div className="space-y-4">
          {Object.entries(report.sections ?? {}).map(([key, data]) => (
            <div key={key}>
              <div className="flex items-center gap-1.5 mb-1.5">
                {sectionIcon(key)}
                <span className="text-xs font-semibold">{secLabel(key)}</span>
              </div>
              {renderSection(key, data)}
            </div>
          ))}
        </div>
      )}
    </SectionCard>
  );
}
