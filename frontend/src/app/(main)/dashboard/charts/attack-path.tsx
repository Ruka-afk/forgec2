"use client";

import { withChartData } from "@/components/withChartData";

interface AttackPathPoint { name?: string; host?: string; target?: string; type?: string }

function AttackBody({ data }: { data: AttackPathPoint[] }) {
  return (
    <div className="space-y-1.5 max-h-32 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-4">No attack path data</p> : data.map((d, i) => (
        <div key={d.name || d.host || d.target || `node-${i}`} className="flex items-center gap-2 text-xs">
          <span className="w-5 h-5 rounded-full bg-indigo-100 dark:bg-indigo-900/40 flex items-center justify-center text-(--font-size-micro) text-indigo-600 dark:text-indigo-400 font-bold shrink-0">{i + 1}</span>
          <span className="flex-1 text-foreground truncate">{d.name || d.host || d.target || "Unknown"}</span>
          <span className="text-(--font-size-micro-sm) text-muted-foreground/70">{d.type || ""}</span>
        </div>
      ))}
    </div>
  );
}

export default withChartData<AttackPathPoint[]>(
  ({ data }) => <AttackBody data={data} />,
  "/api/dashboard/attack-path",
  (raw) => {
    const o = (raw as { data?: { nodes?: { id?: string; label?: string; type?: string }[] } }).data;
    const nodes = (o?.nodes || []) as { id?: string; label?: string; type?: string }[];
    return nodes.map((n) => ({ name: n.label || n.id, host: n.id, target: n.label, type: n.type }));
  },
);
