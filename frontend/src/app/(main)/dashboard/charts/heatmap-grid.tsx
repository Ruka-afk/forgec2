"use client";

import { useMemo } from "react";
import { withChartData } from "@/components/withChartData";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

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
      <div className="grid grid-cols-12 gap-0.5 text-[8px] text-center text-muted-foreground/70 mb-1">
        {Array.from({ length: 12 }, (_, h) => <span key={h * 2}>{(h * 2) % 4 === 0 ? h * 2 : ""}</span>)}
      </div>
      {dayLabels.map((day, di) => (
        <div key={day} className="flex items-center gap-1">
          <span className="text-(--font-size-micro-sm) text-muted-foreground/70 w-6 text-right shrink-0">{day.slice(0, 2)}</span>
          <div className="flex-1 grid grid-cols-12 gap-0.5">
            {Array.from({ length: 12 }, (_, hi) => {
              const h1 = hi * 2, h2 = hi * 2 + 1;
              const count = (lookup[`${di}-${h1}`] || 0) + (lookup[`${di}-${h2}`] || 0);
              const bg = count === 0 ? "bg-secondary" : count < 3 ? "bg-emerald-200 dark:bg-emerald-800" : count < 6 ? "bg-emerald-400 dark:bg-emerald-600" : "bg-emerald-600 dark:bg-emerald-400";
              return <Tooltip key={di * 12 + hi}><TooltipTrigger><div className={`h-3 rounded-sm ${bg}`}></div></TooltipTrigger><TooltipContent>{`${day} ${h1}:00-${h2 + 1}:00 - ${count} events`}</TooltipContent></Tooltip>;
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

export default withChartData<HeatmapPoint[]>(
  ({ data }) => <HeatmapBody data={data} />,
  "/api/dashboard/activity-heatmap",
  (raw) => (raw as { data: HeatmapPoint[] }).data || [],
);
