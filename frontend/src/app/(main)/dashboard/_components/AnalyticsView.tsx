"use client";

import React, { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";
import { ChartCard } from "@/components/ChartCard";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { AlertTriangle, Calendar, Cpu, Globe, Inbox, Key, PieChart, Route, Shield } from "lucide-react";
import type { DashboardStats } from "@/types/agent";

const LazyHeatmapGrid = React.lazy(() => import("../charts/heatmap-grid"));
const LazyOSDistChart = React.lazy(() => import("../charts/os-dist"));
const LazyTaskStatusChart = React.lazy(() => import("../charts/task-status"));
const LazyCredentialTypes = React.lazy(() => import("../charts/credential-types"));
const LazyListenerTrafficSection = React.lazy(() => import("../charts/listener-traffic"));
const LazyAgentGeo = React.lazy(() => import("../charts/agent-geo"));
const LazyAttackPath = React.lazy(() => import("../charts/attack-path"));
const LazyTaskGanttSection = React.lazy(() => import("../charts/task-gantt"));
const LazyMonitorAlertsSection = React.lazy(() => import("../charts/monitor-alerts"));

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

export default function AnalyticsView({
  range,
  stats,
}: {
  range: string;
  stats: DashboardStats | null;
}) {
  const { t } = useI18n();
  const recent = stats?.recent_tasks ?? [];

  return (
    <div>
      <AuditStrip />

      <Suspense fallback={<Skeleton className="h-64 w-full rounded-md" />}>
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-5 mb-5">
          <ChartCard title={t("dashboard.heatmap")} icon={Calendar} iconColor="emerald" exportFilename="activity-heatmap.png"><LazyHeatmapGrid range={range} /></ChartCard>
          <ChartCard title={t("dashboard.os_dist")} icon={Cpu} iconColor="info" exportFilename="os-distribution.png"><LazyOSDistChart /></ChartCard>
          <ChartCard title={t("dashboard.task_status")} icon={PieChart} iconColor="warning" exportFilename="task-status.png"><LazyTaskStatusChart /></ChartCard>
          <ChartCard title={t("dashboard.cred_types")} icon={Key} iconColor="violet" exportFilename="credential-types.png"><LazyCredentialTypes /></ChartCard>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5 mb-7">
          <LazyListenerTrafficSection range={range} className="animate-fade-slide-up" />
          <ChartCard title={t("dashboard.beacon_geo")} icon={Globe} iconColor="rose" exportFilename="agent-geo.png"><LazyAgentGeo /></ChartCard>
          <ChartCard title={t("dashboard.attack_path")} icon={Route} iconColor="warning" exportFilename="attack-path.png"><LazyAttackPath /></ChartCard>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 mb-7">
          <LazyTaskGanttSection range={range} />
          <ChartCard title={t("dashboard.active_alerts")} icon={AlertTriangle} iconColor="warning"><LazyMonitorAlertsSection /></ChartCard>
        </div>
      </Suspense>

      <Card className="overflow-hidden">
        <CardHeader className="px-5 py-3.5 border-b border-border">
          <CardTitle className="text-sm font-semibold text-foreground">{t("dashboard.recent_tasks")}</CardTitle>
        </CardHeader>
        <div className="divide-y divide-border">
          {recent.length === 0 ? (
            <div className="p-(--card-spacing) text-center text-muted-foreground text-sm">
              <EmptyState icon={Inbox} title={t("dashboard.no_tasks")} />
            </div>
          ) : recent.slice(0, 10).map((task, i) => (
            <div key={i} className="flex items-center justify-between px-5 py-3 hover:bg-secondary transition-colors">
              <div className="flex items-center gap-3">
                <StatusIndicator
                  status={task.status === "completed" ? "completed" : task.status === "failed" ? "failed" : task.status === "pending" ? "pending" : task.status === "cancelled" ? "cancelled" : "running"}
                  variant="dotOnly"
                  size="sm"
                  ariaLabel={task.status}
                />
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
