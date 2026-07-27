"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import { ChartCard } from "@/components/ChartCard";
import { BarChart3 } from "lucide-react";

interface GanttItem { agent: string; task: string; start: number; duration: number; status: string }

function GanttBody({ data }: { data: GanttItem[] }) {
  return (
    <div className="space-y-1.5 max-h-40 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-6">No gantt data</p> : data.slice(0, 12).map((item, i) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="w-16 truncate text-muted-foreground font-mono text-(--font-size-micro-sm)">{item.agent}</span>
          <div className="flex-1 h-3 bg-secondary rounded-full overflow-hidden">
            <div className={`h-full rounded-full ${item.status === "completed" ? "bg-emerald-500" : item.status === "failed" ? "bg-red-500" : "bg-amber-500"}`}
              style={{ width: `${Math.min(100, Math.max(8, item.duration * 8))}%`, marginLeft: `${Math.min(40, item.start)}%` }} />
          </div>
          <span className="text-(--font-size-micro-sm) text-muted-foreground/70 w-20 truncate">{item.task}</span>
        </div>
      ))}
    </div>
  );
}

export default function TaskGanttSection({ range }: { range: string }) {
  const [items, setItems] = useState<GanttItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  useEffect(() => {
    setLoadError(false);
    api.get<GanttItem[] | { data: GanttItem[] }>(`/api/dashboard/task-gantt?range=${range}`)
      .then((d) => setItems((d as { data: GanttItem[] }).data || (d as GanttItem[]) || []))
      .catch(() => {
        setItems([]);
        setLoadError(true);
      })
      .finally(() => setLoading(false));
  }, [range]);
  return (
    <ChartCard title="Task Gantt" icon={BarChart3} iconColor="text-violet-500 dark:text-violet-400" loading={loading} exportFilename="task-gantt.png">
      {items.length === 0 ? (
        <p className="text-xs text-muted-foreground/70 text-center py-6">
          {loadError ? "Failed to load gantt data" : "No gantt data"}
        </p>
      ) : (
        <GanttBody data={items} />
      )}
    </ChartCard>
  );
}
