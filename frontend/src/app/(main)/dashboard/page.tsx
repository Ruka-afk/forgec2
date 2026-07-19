"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { DASHBOARD_RANGE_KEY } from "@/lib/shortcuts";
import { EmptyState, PageHeader } from "@/components/UI";
import StatCard from "@/components/StatCard";
import { formatTime } from "@/lib/utils";
import { ChartCard } from "@/components/ChartCard";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useI18n } from "@/lib/i18n";
import { AlertCircle, AlertTriangle, Calendar, Cpu, Globe, Inbox, Key, PieChart, Route, Shield, X } from "lucide-react";
import {
  HeatmapGrid,
  OSDistChart,
  TaskStatusChart,
  CredentialTypes,
  AgentGeo,
  AttackPath,
  MonitorAlertsSection,
  ListenerTrafficSection,
  TaskGanttSection,
} from "./charts";

/* ── Audit Strip ── */
function AuditStrip() {
  const { t } = useI18n();
  const [logs, setLogs] = useState<{ action?: string; username?: string; created_at?: string; details?: string }[]>([]);
  useEffect(() => {
    api.get<{ success?: boolean; data?: { action?: string; username?: string; created_at?: string; details?: string }[]; logs?: { action?: string; username?: string; created_at?: string; details?: string }[] }>("/audit/logs?page=1&pageSize=6")
      .then((d) => setLogs(d.data || d.logs || []))
      .catch(() => setLogs([]));
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
              <Badge variant="secondary" className="text-[10px] font-mono shrink-0">{log.action || "-"}</Badge>
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
  const [timeRange, setTimeRange] = useState("24h");
  const [stats, setStats] = useState<DashboardPageStats>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const { t } = useI18n();

  const changeRange = (r: string) => {
    setTimeRange(r);
    try { localStorage.setItem(DASHBOARD_RANGE_KEY, r); } catch { /* ignore */ }
  };

  useEffect(() => {
    try {
      const saved = localStorage.getItem(DASHBOARD_RANGE_KEY);
      if (saved && ["24h", "7d", "30d"].includes(saved)) setTimeRange(saved);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    setLoading(true);
    setError(null);
    Promise.all([
      api.get<DashboardPageStats>("/api/v1/dashboard").catch(() => { setError(t("dashboard.load_failed")); return {}; }),
    ]).then(([d]) => setStats(d)).finally(() => setLoading(false));
  }, [timeRange]);

  useVisibleInterval(() => {
    api.get<DashboardPageStats>("/api/v1/dashboard")
      .then((d) => setStats(d))
      .catch(() => {});
  }, 30000);

  const s = stats;

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("dashboard.title")} subtitle={`${t("dashboard.subtitle")} · ${s.total_tasks || 0} ${t("dashboard.total_tasks_suffix")}`}>
          <div className="flex gap-1">
            {["24h", "7d", "30d"].map((r) => (
              <Button key={r} onClick={() => changeRange(r)} variant={timeRange === r ? "default" : "ghost"} size="xs">{r}</Button>
            ))}
          </div>
            <Badge variant="success" className="gap-1.5 px-3 py-1.5 text-xs">
            <span className="w-1.5 h-1.5 bg-emerald-500 rounded-full animate-pulse"></span>
            v{s.server_version || "2.0"}
          </Badge>
      </PageHeader>

      {error && (
        <div className="mb-4 px-4 py-3 bg-destructive/10 border border-destructive/20 rounded-xl flex items-center gap-3 text-sm text-destructive">
          <AlertCircle className="w-4 h-4" />
          <span>{error}</span>
           <Button variant="outline" size="xs" className="ml-auto mr-2" onClick={() => { setError(null); setLoading(true); api.get<DashboardPageStats>("/api/v1/dashboard").then((d) => setStats(d)).catch(() => setError(t("dashboard.refresh_failed"))).finally(() => setLoading(false)); }}>{t("dashboard.retry")}</Button>
          <Button variant="ghost" size="icon-sm" onClick={() => setError(null)} className="text-muted-foreground hover:text-destructive" aria-label={t("dashboard.dismiss_error")}>
            <X className="w-4 h-4" />
          </Button>
        </div>
      )}

      <AuditStrip />

      {/* Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        {loading ? (
          <>
            {[...Array(4)].map((_, i) => (
              <Card key={i} className="p-4 sm:p-5 opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: `${i * 40}ms` }}>
                <Skeleton className="h-3 w-20 mb-3" />
                <Skeleton className="h-7 w-16 mb-2" />
                <Skeleton className="h-3 w-24" />
              </Card>
            ))}
          </>
        ) : (
          <>
            <StatCard label={t("dashboard.beacons")} value={s.total_agents || 0} color="emerald" sub={`${s.online_agents || 0} ${t("dashboard.online_suffix")}`} className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "0ms" }} />
            <StatCard label={t("dashboard.tasks_today")} value={s.today_tasks || 0} color="indigo" sub={`${s.pending_tasks || 0} ${t("dashboard.pending_suffix")}`} subColor="text-muted-foreground" className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "40ms" }} />
            <StatCard label={t("dashboard.credentials")} value={s.total_creds || 0} color="purple" sub={`${s.total_tokens || 0} ${t("dashboard.tokens")}`} subColor="text-muted-foreground" className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "80ms" }} />
            <StatCard label={t("dashboard.listeners")} value={s.total_listeners || 0} color="cyan" sub={t("dashboard.active")} subColor="text-muted-foreground" className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "120ms" }} />
          </>
        )}
      </div>

      {/* Charts Row 1 */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 mb-4">
        <ChartCard title={t("dashboard.heatmap")} icon={Calendar} iconColor="text-emerald-500 dark:text-emerald-400" exportFilename="activity-heatmap.png" className="animate-fade-slide-up"><HeatmapGrid /></ChartCard>
        <ChartCard title={t("dashboard.os_dist")} icon={Cpu} iconColor="text-blue-500 dark:text-blue-400" exportFilename="os-distribution.png"><OSDistChart /></ChartCard>
        <ChartCard title={t("dashboard.task_status")} icon={PieChart} iconColor="text-amber-500 dark:text-amber-400" exportFilename="task-status.png"><TaskStatusChart /></ChartCard>
        <ChartCard title={t("dashboard.cred_types")} icon={Key} iconColor="text-purple-500 dark:text-purple-400" exportFilename="credential-types.png"><CredentialTypes /></ChartCard>
      </div>

      {/* Charts Row 2 */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 mb-6">
        <ListenerTrafficSection range={timeRange} className="animate-fade-slide-up" />
        <ChartCard title={t("dashboard.beacon_geo")} icon={Globe} iconColor="text-rose-500 dark:text-rose-400" exportFilename="agent-geo.png"><AgentGeo /></ChartCard>
        <ChartCard title={t("dashboard.attack_path")} icon={Route} iconColor="text-orange-500 dark:text-orange-400" exportFilename="attack-path.png"><AttackPath /></ChartCard>
      </div>

      {/* Gantt + Alerts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
        <TaskGanttSection range={timeRange} />
        <ChartCard title={t("dashboard.active_alerts")} icon={AlertTriangle} iconColor="text-amber-500"><MonitorAlertsSection /></ChartCard>
      </div>

      {/* Recent Tasks */}
      <Card className="overflow-hidden animate-fade-slide-up">
        <CardHeader className="px-5 py-3 border-b border-border">
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
                <span className={`w-2 h-2 rounded-full ${task.status === "completed" ? "bg-emerald-500" : task.status === "failed" ? "bg-red-500" : task.status === "pending" ? "bg-amber-500" : "bg-blue-500"}`}></span>
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
