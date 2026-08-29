"use client";

import { useMemo } from "react";
import { withChartData } from "@/components/withChartData";
import { paths } from "@/lib/api-paths";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useI18n } from "@/lib/i18n";

interface TaskCounts { completed: number; pending: number; failed: number; running: number }

function TaskBody({ data }: { data: TaskCounts }) {
  const { t } = useI18n();
  const { items, total } = useMemo(() => {
    const items = [
      { label: t("tasks.completed"), value: Number(data.completed) || 0, color: "bg-success" },
      { label: t("tasks.pending"), value: Number(data.pending) || 0, color: "bg-warning" },
      { label: t("tasks.failed"), value: Number(data.failed) || 0, color: "bg-destructive" },
      { label: t("tasks.running"), value: Number(data.running) || 0, color: "bg-info" },
    ];
    return { items, total: items.reduce((s, i) => s + i.value, 0) };
  }, [data, t]);
  if (total === 0) return <p className="text-xs text-muted-foreground/100 text-center py-4">{t("dashboard.no_tasks_yet")}</p>;
  return (
    <div className="space-y-2">
      <div className="h-4 rounded-full overflow-hidden flex bg-secondary">
        {items.filter((i) => i.value > 0).map((i) => (
          <Tooltip key={i.label}><TooltipTrigger><div className={`${i.color} h-full transition-all`} style={{ width: `${(i.value / total) * 100}%` }}></div></TooltipTrigger><TooltipContent>{`${i.label}: ${i.value}`}</TooltipContent></Tooltip>
        ))}
      </div>
      <div className="grid grid-cols-4 gap-1 text-(--fs-micro-sm) text-center">
        {items.map((i) => (
          <div key={i.label} className="flex flex-col items-center gap-0.5">
            <span className={`size-2 rounded-full ${i.color}`}></span>
            <span className="text-muted-foreground/100">{i.label}</span>
            <span className="font-mono text-foreground font-medium">{i.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export default withChartData<TaskCounts>(
  ({ data }) => <TaskBody data={data} />,
  paths.dashboard.taskStatus,
  (raw) => {
    const list = Array.isArray(raw)
      ? (raw as Array<{ name?: string; status?: string; count?: number; value?: number }>)
      : [];
    const countFor = (...labels: string[]) => {
      const hit = list.find((d) => {
        const n = String(d.name ?? d.status ?? "").toLowerCase();
        return labels.includes(n);
      });
      return Number(hit?.count ?? hit?.value) || 0;
    };
    return {
      completed: countFor("completed", "success"),
      pending: countFor("pending", "queued"),
      failed: countFor("failed", "error"),
      running: countFor("running", "active"),
    };
  },
);
