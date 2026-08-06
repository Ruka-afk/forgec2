"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { ChartCard } from "@/components/ChartCard";
import { BarChart3 } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface GanttItem { agent: string; task: string; start: number; duration: number; status: string }

function GanttBody({ data }: { data: GanttItem[] }) {
  const { t } = useI18n();
  return (
    <div className="space-y-1.5 max-h-40 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-6">{t("dashboard.no_gantt_data")}</p> : data.slice(0, 12).map((item, i) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="w-16 truncate text-muted-foreground font-mono text-(--fs-micro-sm)">{item.agent}</span>
          <div className="flex-1 h-3 bg-secondary rounded-full overflow-hidden">
            <div className={`h-full rounded-full ${item.status === "completed" ? "bg-emerald-500" : item.status === "failed" ? "bg-red-500" : "bg-amber-500"}`}
              style={{ width: `${Math.min(100, Math.max(8, item.duration * 8))}%`, marginLeft: `${Math.min(40, item.start)}%` }} />
          </div>
          <span className="text-(--fs-micro-sm) text-muted-foreground/70 w-20 truncate">{item.task}</span>
          <span className="text-(--fs-micro-sm) w-14 truncate text-muted-foreground">{item.status}</span>
        </div>
      ))}
    </div>
  );
}

export default function TaskGanttSection({ range }: { range: string }) {
  const { t } = useI18n();
  const [items, setItems] = useState<GanttItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    setLoadError(false);
    api.get<GanttItem[] | { data: GanttItem[] }>(paths.dashboard.taskGantt(range), { signal: controller.signal })
      .then((d) => {
        if (controller.signal.aborted) return;
        setItems((d as { data: GanttItem[] }).data || (d as GanttItem[]) || []);
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setItems([]);
        setLoadError(true);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [range]);
  return (
    <ChartCard title={t("dashboard.task_gantt")} icon={BarChart3} iconColor="text-violet-500 dark:text-violet-400" loading={loading} exportFilename="task-gantt.png">
      {items.length === 0 ? (
        <p className="text-xs text-muted-foreground/70 text-center py-6">
          {loadError ? t("dashboard.gantt_load_failed") : t("dashboard.no_gantt_data")}
        </p>
      ) : (
        <GanttBody data={items} />
      )}
    </ChartCard>
  );
}
