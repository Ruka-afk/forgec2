"use client";

import { withChartData } from "@/components/withChartData";

function CredBody({ data }: { data: Record<string, number> }) {
  const entries = Object.entries(data || {})
    .map(([k, v]) => [k, Number(v) || 0] as [string, number])
    .sort(([, a], [, b]) => b - a);
  const maxValue = Math.max(...entries.map(([, v]) => v), 1);
  return (
    <div className="space-y-1.5">
      {entries.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-4">No credential data</p> : entries.slice(0, 8).map(([k, v]) => (
        <div key={k} className="flex items-center gap-2 text-xs">
          <span className="w-16 text-muted-foreground truncate text-(--font-size-micro-sm)">{k}</span>
          <div className="flex-1 h-3 bg-secondary rounded-full overflow-hidden">
            <div className="h-full bg-purple-500 rounded-full transition-all" style={{ width: `${maxValue > 0 ? (v / maxValue) * 100 : 0}%` }}></div>
          </div>
          <span className="font-mono text-(--font-size-micro-sm) text-muted-foreground text-right w-6">{v}</span>
        </div>
      ))}
    </div>
  );
}

export default withChartData<Record<string, number>>(
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
