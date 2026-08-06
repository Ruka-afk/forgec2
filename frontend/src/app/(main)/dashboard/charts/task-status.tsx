"use client";

import { withChartData } from "@/components/withChartData";
import { paths } from "@/lib/api-paths";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useI18n } from "@/lib/i18n";

interface TaskCounts { completed: number; pending: number; failed: number; running: number }

function TaskBody({ data }: { data: TaskCounts }) {
  const { t } = useI18n();
  const items = [
    { label: t("tasks.completed"), value: Number(data.completed) || 0, color: "bg-emerald-500" },
    { label: t("tasks.pending"), value: Number(data.pending) || 0, color: "bg-amber-500" },
    { label: t("tasks.failed"), value: Number(data.failed) || 0, color: "bg-red-500" },
    { label: t("tasks.running"), value: Number(data.running) || 0, color: "bg-blue-500" },
  ];
  const total = items.reduce((s, i) => s + i.value, 0);
  if (total === 0) return <p className="text-xs text-muted-foreground/70 text-center py-4">{t("dashboard.no_tasks_yet")}</p>;
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
            <span className={`w-2 h-2 rounded-full ${i.color}`}></span>
            <span className="text-muted-foreground/70">{i.label}</span>
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
    const d = (raw as { data: Record<string, number> }).data || raw || {};
    return {
      completed: Number((d as Record<string, number>).completed) || 0,
      pending: Number((d as Record<string, number>).pending) || 0,
      failed: Number((d as Record<string, number>).failed) || 0,
      running: Number((d as Record<string, number>).running) || 0,
    };
  },
);
