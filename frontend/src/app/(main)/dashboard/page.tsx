"use client";

import React, { Suspense, useState, useEffect } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { DASHBOARD_RANGES } from "@/lib/shortcuts";
import { useUrlState } from "@/lib/hooks/useUrlState";
import { useAppStore } from "@/lib/store";
import { useShallow } from "zustand/shallow";
import { EmptyState, PageHeader, StatCard } from "@/components/UI";
import { formatTime } from "@/lib/utils";
import { ChartCard } from "@/components/ChartCard";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { DataError } from "@/components/ui/data-state";
import { useI18n } from "@/lib/i18n";
import { AlertTriangle, Calendar, Cpu, Globe, Inbox, Key, PieChart, Route, Shield } from "lucide-react";

const LazyHeatmapGrid = React.lazy(() => import("./charts/heatmap-grid"));
const LazyOSDistChart = React.lazy(() => import("./charts/os-dist"));
const LazyTaskStatusChart = React.lazy(() => import("./charts/task-status"));
const LazyCredentialTypes = React.lazy(() => import("./charts/credential-types"));
const LazyListenerTrafficSection = React.lazy(() => import("./charts/listener-traffic"));
const LazyAgentGeo = React.lazy(() => import("./charts/agent-geo"));
const LazyAttackPath = React.lazy(() => import("./charts/attack-path"));
const LazyTaskGanttSection = React.lazy(() => import("./charts/task-gantt"));
const LazyMonitorAlertsSection = React.lazy(() => import("./charts/monitor-alerts"));

/* ── Audit Strip ── */
function AuditStrip() {
  const { t } = useI18n();
  const [logs, setLogs] = useState<{ action?: string; username?: string; created_at?: string; details?: string }[]>([]);
  useEffect(() => {
    const controller = new AbortController();
    api.get<{ logs?: { action?: string; username?: string; created_at?: string; details?: string }[] }>(paths.audit.logs("page=1&pageSize=6"), { signal: controller.signal })
      .then((d) => setLogs(d.logs || []))
      .catch(() => setLogs([]));
    return () => controller.abort();
  }, []);
  if (logs.length === 0) return null;
  return (
    <Card className="overflow-hidden mb-4">
      <CardHeader className="px-5 py-3 border-b border-border">
        <CardTitle className="text-sm font-semibold text-foreground">
          <Shield className="w-4 h-4" />{t("dashboard.recent_audit")}
        </CardTitle>
        <Link href="/audit" className="text-xs text-primary hover:underline">{t("dashboard.view_all")}</Link>
      </CardHeader>
      <div className="divide-y divide-border">
        {logs.map((log, i) => (
          <div key={i} className="flex items-center justify-between px-5 py-2.5 text-xs">
            <div className="flex items-center gap-2 min-w-0">
              <Badge variant="secondary" className="text-(--fs-micro-sm) font-mono shrink-0">{log.action || "-"}</Badge>
              <span className="text-foreground truncate">{log.details || log.username || ""}</span>
            </div>
            <span className="text-muted-foreground/70 shrink-0 ml-2">
              {log.created_at ? formatTime(log.created_at) : ""}
            </span>
          </div>
        ))}
      </div>
    </Card>
  );
}

/* ── Dashboard Stats ── */
interface DashboardPageStats {
  total_agents?: number;
  online_agents?: number;
  today_tasks?: number;
  pending_tasks?: number;
  total_creds?: number;
  total_tokens?: number;
  total_listeners?: number;
  total_tasks?: number;
  server_version?: string;
  recent_tasks?: { status: string; type: string; command: string; created_at: string }[];
}

/* ── Main Dashboard Page ── */
export default function DashboardPage() {
  const [timeRange, setTimeRange] = useUrlState("range", "24h", DASHBOARD_RANGES);
  const { t } = useI18n();
  const stats = useAppStore(useShallow((s) => s.stats));
  const statsError = useAppStore((s) => s.statsError);
  const fetchStats = useAppStore((s) => s.fetchStats);

  const loading = stats === null && !statsError;
  const error = statsError || null;

  const changeRange = (r: (typeof DASHBOARD_RANGES)[number]) => setTimeRange(r);

  // Stats live in the app store and are refreshed there (Sidebar uses the same
  // poll loop). The dashboard is a pure consumer — no duplicate fetcher.
  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  // Charts are range-aware and keep their own TTL/polling; stats themselves are
  // independent of the range, so the store's interval refresh is sufficient.

  const s: DashboardPageStats = stats ?? {};

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("dashboard.title")} subtitle={`${t("dashboard.subtitle")} · ${s.total_tasks || 0} ${t("dashboard.total_tasks_suffix")}`}>
          <div className="inline-flex items-center gap-0.5 rounded-lg bg-secondary/70 p-0.5 ring-1 ring-border/50">
            {DASHBOARD_RANGES.map((r) => (
              <button
                key={r}
                onClick={() => changeRange(r)}
                className={cn(
                  "px-2.5 py-1 rounded-md text-xs font-mono transition-colors",
                  timeRange === r
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {r}
              </button>
            ))}
          </div>
            {s.server_version && (
              <Badge variant="success" className="gap-1.5 px-3 py-1.5 text-xs">
                <span className="w-1.5 h-1.5 bg-emerald-500 rounded-full animate-pulse"></span>
                v{s.server_version}
              </Badge>
            )}
      </PageHeader>

      {error && (
        <DataError
          message={error}
          onRetry={() => { fetchStats(); }}
          onDismiss={() => useAppStore.setState({ statsError: undefined })}
          className="mb-4"
        />
      )}

      <AuditStrip />

      {/* Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 mb-7">
        {loading ? (
          <>
            {[...Array(4)].map((_, i) => (
              <Card key={i} className="p-5 opacity-0 animate-fade-slide-up" style={{ animationDelay: `${i * 40}ms` }}>
                <Skeleton className="h-3 w-20 mb-3" />
                <Skeleton className="h-7 w-16 mb-2" />
                <Skeleton className="h-3 w-24" />
              </Card>
            ))}
          </>
        ) : (
          <>
            <StatCard label={t("dashboard.beacons")} value={s.total_agents || 0} color="emerald" sub={`${s.online_agents || 0} ${t("dashboard.online_suffix")}`} className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "0ms" }} />
            <StatCard label={t("dashboard.tasks_today")} value={s.today_tasks || 0} color="indigo" sub={`${s.pending_tasks || 0} ${t("dashboard.pending_suffix")}`} subColor="text-muted-foreground" className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "40ms" }} />
            <StatCard label={t("dashboard.credentials")} value={s.total_creds || 0} color="purple" sub={`${s.total_tokens || 0} ${t("dashboard.tokens")}`} subColor="text-muted-foreground" className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "80ms" }} />
            <StatCard label={t("dashboard.listeners")} value={s.total_listeners || 0} color="cyan" sub={t("dashboard.active")} subColor="text-muted-foreground" className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "120ms" }} />
          </>
        )}
      </div>

      <Suspense fallback={<Skeleton className="h-64 w-full rounded-md" />}>
        {/* Charts Row 1 */}
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-5 mb-5">
          <ChartCard title={t("dashboard.heatmap")} icon={Calendar} iconColor="text-emerald-500 dark:text-emerald-400" exportFilename="activity-heatmap.png" className="animate-fade-slide-up"><LazyHeatmapGrid range={timeRange} /></ChartCard>
          <ChartCard title={t("dashboard.os_dist")} icon={Cpu} iconColor="text-blue-500 dark:text-blue-400" exportFilename="os-distribution.png"><LazyOSDistChart /></ChartCard>
          <ChartCard title={t("dashboard.task_status")} icon={PieChart} iconColor="text-amber-500 dark:text-amber-400" exportFilename="task-status.png"><LazyTaskStatusChart /></ChartCard>
          <ChartCard title={t("dashboard.cred_types")} icon={Key} iconColor="text-purple-500 dark:text-purple-400" exportFilename="credential-types.png"><LazyCredentialTypes /></ChartCard>
        </div>

        {/* Charts Row 2 */}
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5 mb-7">
          <LazyListenerTrafficSection range={timeRange} className="animate-fade-slide-up" />
          <ChartCard title={t("dashboard.beacon_geo")} icon={Globe} iconColor="text-rose-500 dark:text-rose-400" exportFilename="agent-geo.png"><LazyAgentGeo /></ChartCard>
          <ChartCard title={t("dashboard.attack_path")} icon={Route} iconColor="text-orange-500 dark:text-orange-400" exportFilename="attack-path.png"><LazyAttackPath /></ChartCard>
        </div>

        {/* Gantt + Alerts */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 mb-7">
          <LazyTaskGanttSection range={timeRange} />
          <ChartCard title={t("dashboard.active_alerts")} icon={AlertTriangle} iconColor="text-amber-500"><LazyMonitorAlertsSection /></ChartCard>
        </div>
      </Suspense>

      {/* Recent Tasks */}
      <Card className="overflow-hidden animate-fade-slide-up">
        <CardHeader className="px-5 py-3.5 border-b border-border">
          <CardTitle className="text-sm font-semibold text-foreground">{t("dashboard.recent_tasks")}</CardTitle>
        </CardHeader>
        <div className="divide-y divide-border">
          {(!s.recent_tasks || s.recent_tasks.length === 0) ? (
            <div className="p-4 sm:p-5 text-center text-muted-foreground text-sm">
              <EmptyState icon={Inbox} title={t("dashboard.no_tasks")} />
            </div>
          ) : s.recent_tasks.slice(0, 10).map((task, i) => (
            <div key={i} className="flex items-center justify-between px-5 py-3 hover:bg-secondary transition-colors">
              <div className="flex items-center gap-3">
                <span aria-hidden="true" className={`w-2 h-2 rounded-full shrink-0 ${task.status === "completed" ? "bg-emerald-500" : task.status === "failed" ? "bg-red-500" : task.status === "pending" ? "bg-amber-500" : "bg-blue-500"}`}></span>
                <span className="text-(--fs-micro-sm) text-muted-foreground w-16 shrink-0">
                  {task.status === "completed" ? t("tasks.completed") : task.status === "failed" ? t("tasks.failed") : task.status === "pending" ? t("tasks.pending") : task.status === "cancelled" ? t("tasks.cancelled") : t("tasks.running")}
                </span>
                <span className="text-xs font-mono text-foreground">{task.type}</span>
                <span className="text-xs text-muted-foreground/70 truncate max-w-xs">{task.command}</span>
              </div>
              <span className="text-xs text-muted-foreground/70">{task.created_at ? formatTime(task.created_at) : ""}</span>
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
