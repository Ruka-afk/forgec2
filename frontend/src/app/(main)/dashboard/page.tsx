"use client";

import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { apiGet } from "@/lib/api";
import { exportElementPng } from "@/lib/chartExport";
import { DASHBOARD_RANGE_KEY } from "@/lib/shortcuts";
import { withChartData } from "@/components/withChartData";

function ChartCard({ title, icon, iconColor, children, onRefresh, loading, error, exportFilename }: {
  title: string; icon: string; iconColor: string;
  children: React.ReactNode; onRefresh?: () => void; loading?: boolean; error?: boolean;
  exportFilename?: string;
}) {
  const chartRef = useRef<HTMLDivElement>(null);
  const handleExport = async () => {
    if (!chartRef.current || !exportFilename) return;
    try { await exportElementPng(chartRef.current, exportFilename); } catch { /* ignore */ }
  };
  return (
    <div className="ui-card p-4 sm:p-5 shadow-sm">
      <div className="flex items-center justify-between mb-3">
        <div className="font-semibold text-[var(--text-primary)] flex items-center gap-x-2 text-sm">
          <i className={`${icon} ${iconColor}`}></i>
          <span>{title}</span>
        </div>
        <div className="flex items-center gap-1">
          {exportFilename && (
            <button onClick={handleExport} className="w-8 h-8 rounded-lg hover:bg-[var(--card-bg-secondary)] flex items-center justify-center" title="Export PNG">
              <i className="fa-solid fa-download text-xs text-[var(--text-tertiary)]"></i>
            </button>
          )}
          {onRefresh && (
            <button onClick={onRefresh} disabled={loading} className="w-8 h-8 rounded-lg hover:bg-[var(--card-bg-secondary)] flex items-center justify-center disabled:opacity-50">
              <i className={`fa-solid fa-rotate-right text-xs ${loading ? "fa-spin text-indigo-500" : "text-[var(--text-tertiary)]"}`}></i>
            </button>
          )}
        </div>
      </div>
      {error ? (
        <div className="flex flex-col items-center justify-center py-8 text-[var(--text-tertiary)]">
          <i className="fa-solid fa-triangle-exclamation text-2xl mb-2 text-amber-400"></i>
          <span className="text-xs">Failed to load</span>
          {onRefresh && <button onClick={onRefresh} className="mt-2 text-xs text-indigo-500 hover:underline">Retry</button>}
        </div>
      ) : <div ref={chartRef}>{children}</div>}
    </div>
  );
}

/* ── Chart components ── */

interface HeatmapPoint { day: number; hour: number; count: number }
function HeatmapBody({ data }: { data: HeatmapPoint[] }) {
  const dayLabels = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const lookup = useMemo(() => {
    const m: Record<string, number> = {};
    data.forEach((p) => { m[`${p.day}-${p.hour}`] = p.count; });
    return m;
  }, [data]);
  return (
    <div className="space-y-1 overflow-x-auto">
      <div className="grid grid-cols-12 gap-0.5 text-[8px] text-center text-[var(--text-tertiary)] mb-1">
        {Array.from({ length: 12 }, (_, h) => <span key={h * 2}>{(h * 2) % 4 === 0 ? h * 2 : ""}</span>)}
      </div>
      {dayLabels.map((day, di) => (
        <div key={day} className="flex items-center gap-1">
          <span className="text-[10px] text-[var(--text-tertiary)] w-6 text-right shrink-0">{day.slice(0, 2)}</span>
          <div className="flex-1 grid grid-cols-12 gap-0.5">
            {Array.from({ length: 12 }, (_, hi) => {
              const h1 = hi * 2, h2 = hi * 2 + 1;
              const count = (lookup[`${di}-${h1}`] || 0) + (lookup[`${di}-${h2}`] || 0);
              const bg = count === 0 ? "bg-[var(--card-bg-secondary)]" : count < 3 ? "bg-emerald-200 dark:bg-emerald-800" : count < 6 ? "bg-emerald-400 dark:bg-emerald-600" : "bg-emerald-600 dark:bg-emerald-400";
              return <div key={di * 12 + hi} className={`h-3 rounded-sm ${bg}`} title={`${day} ${h1}:00-${h2 + 1}:00 - ${count} events`}></div>;
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

interface OSPoint { name: string; value: number; color: string }
function OSBody({ data }: { data: OSPoint[] }) {
  const total = data.reduce((s, d) => s + d.value, 0);
  const colors: Record<string, string> = { Windows: "#3b82f6", Linux: "#f59e0b", macOS: "#8b5cf6", darwin: "#8b5cf6" };
  const items = data.map((d) => ({ ...d, color: colors[d.name] || d.color || "#6b7280" }));
  return (
    <div className="flex items-center gap-4">
      <div className="relative w-20 h-20 shrink-0">
        {total > 0 ? (
          <div className="w-full h-full rounded-full" style={{ background: `conic-gradient(${items.map((d, i) => { const prev = items.slice(0, i).reduce((s, x) => s + x.value, 0); return `${d.color} ${(prev / total) * 360}deg ${((prev + d.value) / total) * 360}deg`; }).join(", ")})` }}></div>
        ) : <div className="w-full h-full rounded-full bg-[var(--card-bg-secondary)]"></div>}
        <div className="absolute inset-2 rounded-full bg-[var(--card-bg)] flex items-center justify-center text-xs font-bold text-[var(--text-primary)]">{total}</div>
      </div>
      <div className="flex-1 space-y-1.5">
        {items.length === 0 ? <p className="text-xs text-[var(--text-tertiary)]">No data</p> : items.map((d) => (
          <div key={d.name} className="flex items-center gap-2 text-xs">
            <span className="w-2.5 h-2.5 rounded-full shrink-0" style={{ backgroundColor: d.color }}></span>
            <span className="text-[var(--text-primary)] truncate flex-1">{d.name}</span>
            <span className="font-mono text-[var(--text-secondary)]">{d.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

interface TaskCounts { completed: number; pending: number; failed: number; running: number }
function TaskBody({ data }: { data: TaskCounts }) {
  const items = [
    { label: "Completed", value: Number(data.completed) || 0, color: "bg-emerald-500" },
    { label: "Pending", value: Number(data.pending) || 0, color: "bg-amber-500" },
    { label: "Failed", value: Number(data.failed) || 0, color: "bg-red-500" },
    { label: "Running", value: Number(data.running) || 0, color: "bg-blue-500" },
  ];
  const total = items.reduce((s, i) => s + i.value, 0);
  if (total === 0) return <p className="text-xs text-[var(--text-tertiary)] text-center py-4">No tasks yet</p>;
  return (
    <div className="space-y-2">
      <div className="h-4 rounded-full overflow-hidden flex bg-[var(--card-bg-secondary)]">
        {items.filter((i) => i.value > 0).map((i) => (
          <div key={i.label} className={`${i.color} h-full transition-all`} style={{ width: `${(i.value / total) * 100}%` }} title={`${i.label}: ${i.value}`}></div>
        ))}
      </div>
      <div className="grid grid-cols-4 gap-1 text-[10px] text-center">
        {items.map((i) => (
          <div key={i.label} className="flex flex-col items-center gap-0.5">
            <span className={`w-2 h-2 rounded-full ${i.color}`}></span>
            <span className="text-[var(--text-tertiary)]">{i.label}</span>
            <span className="font-mono text-[var(--text-primary)] font-medium">{i.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

interface TrafficPoint { value: number; time?: string }
function TrafficBody({ data, range }: { data: TrafficPoint[]; range: string }) {
  const maxVal = Math.max(...data.map((d) => Number(d.value) || 0), 1);
  return (
    <div className="space-y-2">
      <div className="flex items-end gap-0.5 h-16">
        {data.length === 0 ? <span className="text-xs text-[var(--text-tertiary)]">No traffic data</span> : data.slice(0, 30).map((d, i) => (
          <div key={i} className="flex-1 bg-indigo-400 dark:bg-indigo-600 rounded-t-sm min-h-[2px] transition-all" style={{ height: `${Math.max(2, ((Number(d.value) || 0) / maxVal) * 100)}%` }} title={String(d.time ?? i)}></div>
        ))}
      </div>
    </div>
  );
}

function CredBody({ data }: { data: Record<string, number> }) {
  const entries = Object.entries(data).sort(([, a], [, b]) => Number(b) - Number(a));
  const maxValue = Math.max(...entries.map(([, v]) => Number(v) || 0), 1);
  return (
    <div className="space-y-1.5">
      {entries.length === 0 ? <p className="text-xs text-[var(--text-tertiary)] text-center py-4">No credential data</p> : entries.slice(0, 8).map(([k, v]) => (
        <div key={k} className="flex items-center gap-2 text-xs">
          <span className="w-16 text-[var(--text-secondary)] truncate text-[10px]">{k}</span>
          <div className="flex-1 h-3 bg-[var(--card-bg-secondary)] rounded-full overflow-hidden">
            <div className="h-full bg-purple-500 rounded-full transition-all" style={{ width: `${(Number(v) / maxValue) * 100}%` }}></div>
          </div>
          <span className="font-mono text-[10px] text-[var(--text-secondary)] text-right w-6">{Number(v)}</span>
        </div>
      ))}
    </div>
  );
}

interface GeoPoint { flag?: string; country?: string; count: number }
function GeoBody({ data }: { data: GeoPoint[] }) {
  return (
    <div className="space-y-1.5 max-h-32 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-[var(--text-tertiary)] text-center py-4">No geo data</p> : data.map((d, i) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="text-[10px]">{d.flag || "??"}</span>
          <span className="flex-1 text-[var(--text-primary)] truncate">{d.country || "Unknown"}</span>
          <span className="font-mono text-[10px] text-[var(--text-tertiary)]">{d.count || 0}</span>
        </div>
      ))}
    </div>
  );
}

interface AttackPathPoint { name?: string; host?: string; target?: string; type?: string }
function AttackBody({ data }: { data: AttackPathPoint[] }) {
  return (
    <div className="space-y-1.5 max-h-32 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-[var(--text-tertiary)] text-center py-4">No attack path data</p> : data.map((d, i) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="w-5 h-5 rounded-full bg-indigo-100 dark:bg-indigo-900/40 flex items-center justify-center text-[9px] text-indigo-600 dark:text-indigo-400 font-bold shrink-0">{i + 1}</span>
          <span className="flex-1 text-[var(--text-primary)] truncate">{d.name || d.host || d.target || "Unknown"}</span>
          <span className="text-[10px] text-[var(--text-tertiary)]">{d.type || ""}</span>
        </div>
      ))}
    </div>
  );
}

interface GanttItem { agent: string; task: string; start: number; duration: number; status: string }
function GanttBody({ data }: { data: GanttItem[] }) {
  return (
    <div className="space-y-1.5 max-h-40 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-[var(--text-tertiary)] text-center py-6">No gantt data</p> : data.slice(0, 12).map((item, i) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="w-16 truncate text-[var(--text-secondary)] font-mono text-[10px]">{item.agent}</span>
          <div className="flex-1 h-3 bg-[var(--card-bg-secondary)] rounded-full overflow-hidden">
            <div className={`h-full rounded-full ${item.status === "completed" ? "bg-emerald-500" : item.status === "failed" ? "bg-red-500" : "bg-amber-500"}`}
              style={{ width: `${Math.min(100, Math.max(8, item.duration * 8))}%`, marginLeft: `${Math.min(40, item.start)}%` }} />
          </div>
          <span className="text-[10px] text-[var(--text-tertiary)] w-20 truncate">{item.task}</span>
        </div>
      ))}
    </div>
  );
}

function AlertBody({ data }: { data: { message?: string; severity?: string; title?: string }[] }) {
  return (
    <div className="space-y-2 max-h-40 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-[var(--text-tertiary)] text-center py-6">No active alerts</p> : data.map((a, i) => (
        <div key={i} className="flex items-start gap-2 text-xs px-2 py-1.5 bg-amber-50 dark:bg-amber-900/20 rounded-lg">
          <i className="fa-solid fa-bell text-amber-500 mt-0.5"></i>
          <span className="text-[var(--text-primary)]">{a.message || a.title || "Alert"}</span>
        </div>
      ))}
    </div>
  );
}

/* ── Extracted chart wrappers using withChartData ── */

const HeatmapGrid = withChartData<HeatmapPoint[]>(
  ({ data }) => <HeatmapBody data={data} />,
  "/api/dashboard/activity-heatmap",
  (raw) => (Array.isArray(raw) ? raw : ((raw as { Data: HeatmapPoint[] })?.Data || [])),
);

const OSDistChart = withChartData<OSPoint[]>(
  ({ data }) => <OSBody data={data} />,
  "/api/dashboard/os-distribution",
  (raw) => {
    const r = (raw as { Data: Record<string, number> })?.Data || raw || {};
    const colors: Record<string, string> = { Windows: "#3b82f6", Linux: "#f59e0b", macOS: "#8b5cf6", darwin: "#8b5cf6" };
    return Object.entries(r as Record<string, number>).map(([k, v]) => ({ name: k, value: Number(v) || 0, color: colors[k] || "#6b7280" })).sort((a, b) => b.value - a.value);
  },
);

const TaskStatusChart = withChartData<TaskCounts>(
  ({ data }) => <TaskBody data={data} />,
  "/api/dashboard/task-status",
  (raw) => {
    const d = (raw as { Data: Record<string, number> })?.Data || raw || {};
    return {
      completed: Number((d as Record<string, number>).completed) || 0,
      pending: Number((d as Record<string, number>).pending) || 0,
      failed: Number((d as Record<string, number>).failed) || 0,
      running: Number((d as Record<string, number>).running) || 0,
    };
  },
);

const CredentialTypes = withChartData<Record<string, number>>(
  ({ data }) => <CredBody data={data} />,
  "/api/dashboard/credential-types",
  (raw) => (raw as { Data: Record<string, number> })?.Data || (raw as Record<string, number>) || {},
);

const AgentGeo = withChartData<GeoPoint[]>(
  ({ data }) => <GeoBody data={data} />,
  "/api/dashboard/agent-geo",
  (raw) => (Array.isArray(raw) ? raw : ((raw as { Data: GeoPoint[] })?.Data || [])),
);

const AttackPath = withChartData<AttackPathPoint[]>(
  ({ data }) => <AttackBody data={data} />,
  "/api/dashboard/attack-path",
  (raw) => (Array.isArray(raw) ? raw : ((raw as { Data: AttackPathPoint[] })?.Data || [])),
);

const MonitorAlertsSection = withChartData<{ message?: string; severity?: string; title?: string }[]>(
  ({ data }) => <AlertBody data={data} />,
  "/api/monitor/alerts?status=active&limit=8",
  (raw) => {
    const d = raw as { alerts?: unknown[]; data?: unknown[] };
    return (d.alerts || d.data || []) as { message?: string; severity?: string; title?: string }[];
  },
);

/* ── ListenerTraffic with time range ── */
function ListenerTrafficInner({ range, onRangeChange }: { range: string; onRangeChange: (r: string) => void }) {
  const [data, setData] = useState<TrafficPoint[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    setLoading(true);
    apiGet<TrafficPoint[] | { Data: TrafficPoint[] }>(`/api/dashboard/listener-traffic?range=${range}`)
      .then((d) => setData(Array.isArray(d) ? d : ((d as { Data: TrafficPoint[] })?.Data || [])))
      .catch(() => setData([]))
      .finally(() => setLoading(false));
  }, [range]);
  if (loading) return <div className="h-24 flex items-center justify-center text-[var(--text-tertiary)] text-xs"><i className="fa-solid fa-circle-notch fa-spin mr-2"></i></div>;
  return (
    <div className="space-y-2">
      <div className="flex gap-1">
        {["24h", "7d", "30d"].map((r) => (
          <button key={r} onClick={() => onRangeChange(r)} className={`px-2 py-0.5 rounded text-[10px] ${range === r ? "bg-indigo-600 text-white" : "bg-[var(--card-bg-secondary)] text-[var(--text-secondary)]"}`}>{r}</button>
        ))}
      </div>
      <TrafficBody data={data} range={range} />
    </div>
  );
}

function ListenerTrafficSection({ range, onRangeChange }: { range: string; onRangeChange: (r: string) => void }) {
  return (
    <ChartCard title="Listener Traffic" icon="fa-solid fa-arrows-left-right" iconColor="text-cyan-500" exportFilename="listener-traffic.png">
      <ListenerTrafficInner range={range} onRangeChange={onRangeChange} />
    </ChartCard>
  );
}

function TaskGanttSection({ range }: { range: string }) {
  const [items, setItems] = useState<GanttItem[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    apiGet<GanttItem[] | { Data: GanttItem[] }>(`/api/dashboard/task-gantt?range=${range}`)
      .then((d) => setItems(Array.isArray(d) ? d : ((d as { Data: GanttItem[] }).Data || [])))
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [range]);
  return (
    <ChartCard title="Task Gantt" icon="fa-solid fa-bars-staggered" iconColor="text-violet-500" loading={loading} exportFilename="task-gantt.png">
      {items.length === 0 ? <p className="text-xs text-[var(--text-tertiary)] text-center py-6">No gantt data</p> : <GanttBody data={items} />}
    </ChartCard>
  );
}

/* ── Audit Strip ── */
function AuditStrip() {
  const [logs, setLogs] = useState<{ action?: string; username?: string; created_at?: string; details?: string }[]>([]);
  useEffect(() => {
    apiGet<{ success?: boolean; logs?: { action?: string; username?: string; created_at?: string; details?: string }[] }>("/audit/logs?page=1&pageSize=6")
      .then((d) => setLogs(d.logs || []))
      .catch(() => setLogs([]));
  }, []);
  if (logs.length === 0) return null;
  return (
    <div className="ui-card overflow-hidden shadow-sm mb-4">
      <div className="px-5 py-3 border-b border-[var(--border)] flex items-center justify-between">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">
          <i className="fa-solid fa-shield text-indigo-500 mr-2"></i>Recent Audit
        </h3>
        <a href="/audit" className="text-xs text-indigo-600 hover:underline">View all</a>
      </div>
      <div className="divide-y divide-[var(--border)]">
        {logs.map((log, i) => (
          <div key={i} className="flex items-center justify-between px-5 py-2.5 text-xs">
            <div className="flex items-center gap-2 min-w-0">
              <span className="px-1.5 py-0.5 bg-[var(--card-bg-secondary)] rounded text-[10px] font-mono text-[var(--text-secondary)] shrink-0">{log.action || "-"}</span>
              <span className="text-[var(--text-primary)] truncate">{log.details || log.username || ""}</span>
            </div>
            <span className="text-[var(--text-tertiary)] shrink-0 ml-2">
              {log.created_at ? new Date(log.created_at).toLocaleTimeString() : ""}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ── Dashboard Stats ── */
interface DashboardStats {
  TotalAgents?: number; OnlineAgents?: number; TodayTasks?: number;
  PendingTasks?: number; TotalCreds?: number; TotalTokens?: number;
  TotalListeners?: number; TotalTasks?: number; ServerVersion?: string;
  RecentTasks?: { status: string; type: string; command: string; created_at: string }[];
}

/* ── Main Dashboard Page ── */
export default function DashboardPage() {
  const [timeRange, setTimeRange] = useState("24h");
  const [stats, setStats] = useState<DashboardStats>({});

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
    Promise.all([
      apiGet<DashboardStats>("/dashboard").catch(() => ({})),
    ]).then(([d]) => setStats(d));
  }, []);

  const s = stats;

  return (
    <div>
      <div className="flex items-end justify-between mb-4 gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-[var(--text-primary)]">Dashboard</h1>
          <p className="text-[var(--text-secondary)] mt-1 text-xs">C2 Operations Overview &middot; <span className="font-semibold">{s.TotalTasks || 0}</span> total tasks</p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex gap-1">
            {["24h", "7d", "30d"].map((r) => (
              <button key={r} onClick={() => changeRange(r)}
                className={`px-2.5 py-1 rounded-lg text-xs font-medium ${timeRange === r ? "bg-indigo-600 text-white" : "bg-[var(--card-bg-secondary)] text-[var(--text-secondary)]"}`}>{r}</button>
            ))}
          </div>
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-emerald-50 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400 rounded-xl text-xs font-medium">
            <span className="w-1.5 h-1.5 bg-emerald-500 rounded-full animate-pulse"></span>
            v{s.ServerVersion || "2.0"}
          </span>
        </div>
      </div>

      <AuditStrip />

      {/* Stat Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
        <div className="ui-card p-4 shadow-sm">
          <p className="text-xs font-semibold text-[var(--text-secondary)] uppercase tracking-wider">Beacons</p>
          <p className="text-2xl font-bold mt-1 text-[var(--text-primary)]">{s.TotalAgents || 0}</p>
          <p className="text-xs text-emerald-600 mt-1">{s.OnlineAgents || 0} online</p>
        </div>
        <div className="ui-card p-4 shadow-sm">
          <p className="text-xs font-semibold text-[var(--text-secondary)] uppercase tracking-wider">Tasks Today</p>
          <p className="text-2xl font-bold mt-1 text-indigo-600">{s.TodayTasks || 0}</p>
          <p className="text-xs text-[var(--text-secondary)] mt-1">{s.PendingTasks || 0} pending</p>
        </div>
        <div className="ui-card p-4 shadow-sm">
          <p className="text-xs font-semibold text-[var(--text-secondary)] uppercase tracking-wider">Credentials</p>
          <p className="text-2xl font-bold mt-1 text-purple-600">{s.TotalCreds || 0}</p>
          <p className="text-xs text-[var(--text-secondary)] mt-1">{s.TotalTokens || 0} tokens</p>
        </div>
        <div className="ui-card p-4 shadow-sm">
          <p className="text-xs font-semibold text-[var(--text-secondary)] uppercase tracking-wider">Listeners</p>
          <p className="text-2xl font-bold mt-1 text-cyan-600">{s.TotalListeners || 0}</p>
          <p className="text-xs text-[var(--text-secondary)] mt-1">active</p>
        </div>
      </div>

      {/* Charts Row 1 */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 mb-4">
        <ChartCard title="Activity Heatmap" icon="fa-solid fa-calendar-days" iconColor="text-emerald-500" exportFilename="activity-heatmap.png"><HeatmapGrid /></ChartCard>
        <ChartCard title="OS Distribution" icon="fa-solid fa-microchip" iconColor="text-blue-500" exportFilename="os-distribution.png"><OSDistChart /></ChartCard>
        <ChartCard title="Task Status" icon="fa-solid fa-chart-pie" iconColor="text-amber-500" exportFilename="task-status.png"><TaskStatusChart /></ChartCard>
        <ChartCard title="Credential Types" icon="fa-solid fa-key" iconColor="text-purple-500" exportFilename="credential-types.png"><CredentialTypes /></ChartCard>
      </div>

      {/* Charts Row 2 */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 mb-6">
        <ListenerTrafficSection range={timeRange} onRangeChange={changeRange} />
        <ChartCard title="Beacon Geo-Map" icon="fa-solid fa-earth-americas" iconColor="text-rose-500" exportFilename="agent-geo.png"><AgentGeo /></ChartCard>
        <ChartCard title="Attack Path" icon="fa-solid fa-route" iconColor="text-orange-500" exportFilename="attack-path.png"><AttackPath /></ChartCard>
      </div>

      {/* Gantt + Alerts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
        <TaskGanttSection range={timeRange} />
        <ChartCard title="Active Alerts" icon="fa-solid fa-triangle-exclamation" iconColor="text-amber-500"><MonitorAlertsSection /></ChartCard>
      </div>

      {/* Recent Tasks */}
      <div className="ui-card overflow-hidden shadow-sm">
        <div className="px-5 py-3 border-b border-[var(--border)]">
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">Recent Tasks</h3>
        </div>
        <div className="divide-y divide-[var(--border)]">
          {(!s.RecentTasks || s.RecentTasks.length === 0) ? (
            <div className="p-6 text-center text-[var(--text-tertiary)] text-sm">No recent tasks</div>
          ) : s.RecentTasks.slice(0, 10).map((t, i) => (
            <div key={i} className="flex items-center justify-between px-5 py-3 hover:bg-[var(--card-bg-secondary)] transition-colors">
              <div className="flex items-center gap-3">
                <span className={`w-2 h-2 rounded-full ${t.status === "completed" ? "bg-emerald-500" : t.status === "failed" ? "bg-red-500" : t.status === "pending" ? "bg-amber-500" : "bg-blue-500"}`}></span>
                <span className="text-xs font-mono text-[var(--text-primary)]">{t.type}</span>
                <span className="text-xs text-[var(--text-tertiary)] truncate max-w-xs">{t.command}</span>
              </div>
              <span className="text-xs text-[var(--text-tertiary)]">{t.created_at ? new Date(t.created_at).toLocaleTimeString() : ""}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
