"use client";

import { useState, useEffect, useMemo } from "react";
import { api } from "@/lib/api";
import { withChartData } from "@/components/withChartData";
import { ChartCard } from "@/components/ChartCard";
import { Spinner } from "@/components/UI";
import { ArrowLeftRight, BarChart3, Bell } from "lucide-react";
import { OS_CHART_COLORS } from "@/lib/colors";

/* ── Types ── */

interface HeatmapPoint { day: number; hour: number; count: number }
interface OSPoint { name: string; value: number; color: string }
interface TaskCounts { completed: number; pending: number; failed: number; running: number }
interface TrafficPoint { value: number; time?: string }
interface GeoPoint { flag?: string; country?: string; count: number }
interface AttackPathPoint { name?: string; host?: string; target?: string; type?: string }
interface GanttItem { agent: string; task: string; start: number; duration: number; status: string }

/* ── Bodies ── */

function HeatmapBody({ data }: { data: HeatmapPoint[] }) {
  const dayLabels = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const lookup = useMemo(() => {
    const m: Record<string, number> = {};
    data.forEach((p) => { m[`${p.day}-${p.hour}`] = p.count; });
    return m;
  }, [data]);
  return (
    <div className="space-y-1 overflow-x-auto">
      <div className="grid grid-cols-12 gap-0.5 text-[8px] text-center text-muted-foreground/70 mb-1">
        {Array.from({ length: 12 }, (_, h) => <span key={h * 2}>{(h * 2) % 4 === 0 ? h * 2 : ""}</span>)}
      </div>
      {dayLabels.map((day, di) => (
        <div key={day} className="flex items-center gap-1">
          <span className="text-[10px] text-muted-foreground/70 w-6 text-right shrink-0">{day.slice(0, 2)}</span>
          <div className="flex-1 grid grid-cols-12 gap-0.5">
            {Array.from({ length: 12 }, (_, hi) => {
              const h1 = hi * 2, h2 = hi * 2 + 1;
              const count = (lookup[`${di}-${h1}`] || 0) + (lookup[`${di}-${h2}`] || 0);
              const bg = count === 0 ? "bg-secondary" : count < 3 ? "bg-emerald-200 dark:bg-emerald-800" : count < 6 ? "bg-emerald-400 dark:bg-emerald-600" : "bg-emerald-600 dark:bg-emerald-400";
              return <div key={di * 12 + hi} className={`h-3 rounded-sm ${bg}`} title={`${day} ${h1}:00-${h2 + 1}:00 - ${count} events`}></div>;
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

function OSBody({ data }: { data: OSPoint[] }) {
  const total = data.reduce((s, d) => s + d.value, 0);
  const colors: Record<string, string> = { Windows: "#3b82f6", Linux: "#f59e0b", macOS: "#8b5cf6", darwin: "#8b5cf6" };
  const items = data.map((d) => ({ ...d, color: colors[d.name] || d.color || "#6b7280" }));
  return (
    <div className="flex items-center gap-4">
      <div className="relative w-20 h-20 shrink-0">
        {total > 0 ? (
          <div className="w-full h-full rounded-full" style={{ background: `conic-gradient(${items.map((d, i) => { const prev = items.slice(0, i).reduce((s, x) => s + x.value, 0); return `${d.color} ${(prev / total) * 360}deg ${((prev + d.value) / total) * 360}deg`; }).join(", ")})` }}></div>
        ) :           <div className="w-full h-full rounded-full bg-secondary"></div>}
        <div className="absolute inset-2 rounded-full bg-card flex items-center justify-center text-xs font-bold text-foreground">{total}</div>
      </div>
      <div className="flex-1 space-y-1.5">
        {items.length === 0 ? <p className="text-xs text-muted-foreground/70">No data</p> : items.map((d) => (
          <div key={d.name} className="flex items-center gap-2 text-xs">
            <span className="w-2.5 h-2.5 rounded-full shrink-0" style={{ backgroundColor: d.color }}></span>
            <span className="text-foreground truncate flex-1">{d.name}</span>
            <span className="font-mono text-muted-foreground">{d.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function TaskBody({ data }: { data: TaskCounts }) {
  const items = [
    { label: "Completed", value: Number(data.completed) || 0, color: "bg-emerald-500" },
    { label: "Pending", value: Number(data.pending) || 0, color: "bg-amber-500" },
    { label: "Failed", value: Number(data.failed) || 0, color: "bg-red-500" },
    { label: "Running", value: Number(data.running) || 0, color: "bg-blue-500" },
  ];
  const total = items.reduce((s, i) => s + i.value, 0);
  if (total === 0) return <p className="text-xs text-muted-foreground/70 text-center py-4">No tasks yet</p>;
  return (
    <div className="space-y-2">
      <div className="h-4 rounded-full overflow-hidden flex bg-secondary">
        {items.filter((i) => i.value > 0).map((i) => (
          <div key={i.label} className={`${i.color} h-full transition-all`} style={{ width: `${(i.value / total) * 100}%` }} title={`${i.label}: ${i.value}`}></div>
        ))}
      </div>
      <div className="grid grid-cols-4 gap-1 text-[10px] text-center">
        {items.map((i) => (
          <div key={i.label} className="flex flex-col items-center gap-0.5">
            <span className={`w-2 h-2 rounded-full ${i.color}`}></span>
            <span className="text-muted-foreground/70">{i.label}</span>
            <span className="font-mono text-foreground font-medium">{i.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function TrafficBody({ data }: { data: TrafficPoint[] }) {
  const maxVal = Math.max(...data.map((d) => Number(d.value) || 0), 1);
  return (
    <div className="space-y-2">
      <div className="flex items-end gap-0.5 h-16">
        {data.length === 0 ? <span className="text-xs text-muted-foreground/70">No traffic data</span> : data.slice(0, 30).map((d, i) => (
          <div key={i} className="flex-1 bg-indigo-400 dark:bg-indigo-600 rounded-t-sm min-h-[2px] transition-all" style={{ height: `${Math.max(2, ((Number(d.value) || 0) / maxVal) * 100)}%` }} title={String(d.time ?? i)}></div>
        ))}
      </div>
    </div>
  );
}

function CredBody({ data }: { data: Record<string, number> }) {
  const entries = Object.entries(data || {})
    .map(([k, v]) => [k, Number(v) || 0] as [string, number])
    .sort(([, a], [, b]) => b - a);
  const maxValue = Math.max(...entries.map(([, v]) => v), 1);
  return (
    <div className="space-y-1.5">
      {entries.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-4">No credential data</p> : entries.slice(0, 8).map(([k, v]) => (
        <div key={k} className="flex items-center gap-2 text-xs">
          <span className="w-16 text-muted-foreground truncate text-[10px]">{k}</span>
          <div className="flex-1 h-3 bg-secondary rounded-full overflow-hidden">
            <div className="h-full bg-purple-500 rounded-full transition-all" style={{ width: `${maxValue > 0 ? (v / maxValue) * 100 : 0}%` }}></div>
          </div>
          <span className="font-mono text-[10px] text-muted-foreground text-right w-6">{v}</span>
        </div>
      ))}
    </div>
  );
}

function GeoBody({ data }: { data: GeoPoint[] }) {
  return (
    <div className="space-y-1.5 max-h-32 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-4">No geo data</p> : data.map((d, i) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="text-[10px]">{d.flag || "??"}</span>
          <span className="flex-1 text-foreground truncate">{d.country || "Unknown"}</span>
          <span className="font-mono text-[10px] text-muted-foreground/70">{d.count || 0}</span>
        </div>
      ))}
    </div>
  );
}

function AttackBody({ data }: { data: AttackPathPoint[] }) {
  return (
    <div className="space-y-1.5 max-h-32 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-4">No attack path data</p> : data.map((d, i) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="w-5 h-5 rounded-full bg-indigo-100 dark:bg-indigo-900/40 flex items-center justify-center text-[9px] text-indigo-600 dark:text-indigo-400 font-bold shrink-0">{i + 1}</span>
          <span className="flex-1 text-foreground truncate">{d.name || d.host || d.target || "Unknown"}</span>
          <span className="text-[10px] text-muted-foreground/70">{d.type || ""}</span>
        </div>
      ))}
    </div>
  );
}

function GanttBody({ data }: { data: GanttItem[] }) {
  return (
    <div className="space-y-1.5 max-h-40 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-6">No gantt data</p> : data.slice(0, 12).map((item, i) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="w-16 truncate text-muted-foreground font-mono text-[10px]">{item.agent}</span>
          <div className="flex-1 h-3 bg-secondary rounded-full overflow-hidden">
            <div className={`h-full rounded-full ${item.status === "completed" ? "bg-emerald-500" : item.status === "failed" ? "bg-red-500" : "bg-amber-500"}`}
              style={{ width: `${Math.min(100, Math.max(8, item.duration * 8))}%`, marginLeft: `${Math.min(40, item.start)}%` }} />
          </div>
          <span className="text-[10px] text-muted-foreground/70 w-20 truncate">{item.task}</span>
        </div>
      ))}
    </div>
  );
}

function AlertBody({ data }: { data: { message?: string; severity?: string; title?: string }[] }) {
  return (
    <div className="space-y-2 max-h-40 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-6">No active alerts</p> : data.map((a, i) => (
        <div key={i} className="flex items-start gap-2 text-xs px-2 py-1.5 bg-amber-50 dark:bg-amber-900/20 rounded-lg">
          <Bell className="w-4 h-4" />
          <span className="text-foreground">{a.message || a.title || "Alert"}</span>
        </div>
      ))}
    </div>
  );
}

/* ── Chart wrappers (withChartData) ── */

export const HeatmapGrid = withChartData<HeatmapPoint[]>(
  ({ data }) => <HeatmapBody data={data} />,
  "/api/dashboard/activity-heatmap",
  (raw) => (raw as { data: HeatmapPoint[] }).data || [],
);

export const OSDistChart = withChartData<OSPoint[]>(
  ({ data }) => <OSBody data={data} />,
  "/api/dashboard/os-distribution",
  (raw) => {
    const r = (raw as { data: Record<string, number> }).data || raw || {};
  const colors = OS_CHART_COLORS;
    return Object.entries(r as Record<string, number>).map(([k, v]) => ({ name: k, value: Number(v) || 0, color: colors[k] || "#6b7280" })).sort((a, b) => b.value - a.value);
  },
);

export const TaskStatusChart = withChartData<TaskCounts>(
  ({ data }) => <TaskBody data={data} />,
  "/api/dashboard/task-status",
  (raw) => {
    const d = (raw as { data: Record<string, number> }).data || raw || {};
    return {
      completed: Number((d as Record<string, number>).completed) || 0,
      pending: Number((d as Record<string, number>).pending) || 0,
      failed: Number((d as Record<string, number>).failed) || 0,
      running: Number((d as Record<string, number>).running) || 0,
    };
  },
);

export const CredentialTypes = withChartData<Record<string, number>>(
  ({ data }) => <CredBody data={data} />,
  "/api/dashboard/credential-types",
  (raw) => {
    const arr = Array.isArray(raw)
      ? (raw as { Name?: string; Count?: number }[])
      : (((raw as { data?: { Name?: string; Count?: number }[] })?.data) || []);
    const rec: Record<string, number> = {};
    (arr as { name?: string; count?: number }[]).forEach((x) => {
      rec[x.name || "Unknown"] = Number(x.count) || 0;
    });
    return rec;
  },
);

export const AgentGeo = withChartData<GeoPoint[]>(
  ({ data }) => <GeoBody data={data} />,
  "/api/dashboard/agent-geo",
  (raw) => (raw as { data: GeoPoint[] }).data || [],
);

export const AttackPath = withChartData<AttackPathPoint[]>(
  ({ data }) => <AttackBody data={data} />,
  "/api/dashboard/attack-path",
  (raw) => {
    const o = (raw as { data?: { nodes?: { id?: string; label?: string; type?: string }[] } }).data;
    const nodes = (o?.nodes || []) as { id?: string; label?: string; type?: string }[];
    return nodes.map((n) => ({ name: n.label || n.id, host: n.id, target: n.label, type: n.type }));
  },
);

export const MonitorAlertsSection = withChartData<{ message?: string; severity?: string; title?: string }[]>(
  ({ data }) => <AlertBody data={data} />,
  "/api/monitor/alerts?status=active&limit=8",
  (raw) => {
    const d = raw as { alerts?: unknown[]; data?: unknown[] };
    return (d.alerts || d.data || []) as { message?: string; severity?: string; title?: string }[];
  },
);

/* ── ListenerTraffic with time range ── */

function ListenerTrafficInner({ range }: { range: string }) {
  const [data, setData] = useState<TrafficPoint[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    setLoading(true);
    api.get<TrafficPoint[] | { data: TrafficPoint[] } | { labels?: string[]; bytes_in?: number[]; bytes_out?: number[] }>(`/api/dashboard/listener-traffic?range=${range}`)
      .then((d) => {
        const o = (d as { data?: { labels?: string[]; bytes_in?: number[]; bytes_out?: number[] } })?.data ?? d;
        const obj = (o as { labels?: string[]; bytes_in?: number[]; bytes_out?: number[] }) || {};
        const labels = obj.labels || [];
        const bins = obj.bytes_in || [];
        const bouts = obj.bytes_out || [];
        setData(labels.map((t, i) => ({ time: t, value: (Number(bins[i]) || 0) + (Number(bouts[i]) || 0) })));
      })
      .catch(() => setData([]))
      .finally(() => setLoading(false));
  }, [range]);
  if (loading) return <div className="h-24 flex items-center justify-center text-muted-foreground/70 text-xs"><Spinner size="sm" /></div>;
  return (
    <div className="space-y-2">
      <TrafficBody data={data} />
    </div>
  );
}

export function ListenerTrafficSection({ range, className }: { range: string; className?: string }) {
  return (
    <ChartCard title="Listener Traffic" icon={ArrowLeftRight} iconColor="text-cyan-500 dark:text-cyan-400" exportFilename="listener-traffic.png" className={className}>
      <ListenerTrafficInner range={range} />
    </ChartCard>
  );
}

export function TaskGanttSection({ range }: { range: string }) {
  const [items, setItems] = useState<GanttItem[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    api.get<GanttItem[] | { data: GanttItem[] }>(`/api/dashboard/task-gantt?range=${range}`)
      .then((d) => setItems((d as { data: GanttItem[] }).data || (d as GanttItem[]) || []))
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [range]);
  return (
    <ChartCard title="Task Gantt" icon={BarChart3} iconColor="text-violet-500 dark:text-violet-400" loading={loading} exportFilename="task-gantt.png">
      {items.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-6">No gantt data</p> : <GanttBody data={items} />}
    </ChartCard>
  );
}
