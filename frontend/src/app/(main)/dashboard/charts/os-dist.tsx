"use client";

import { withChartData } from "@/components/withChartData";
import { OS_CHART_COLORS } from "@/lib/colors";
import { useI18n } from "@/lib/i18n";

interface OSPoint { name: string; value: number; color: string }

function OSBody({ data }: { data: OSPoint[] }) {
  const { t } = useI18n();
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
        {items.length === 0 ? <p className="text-xs text-muted-foreground/70">{t("common.no_data")}</p> : items.map((d) => (
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

export default withChartData<OSPoint[]>(
  ({ data }) => <OSBody data={data} />,
  "/api/dashboard/os-distribution",
  (raw) => {
    const r = (raw as { data: Record<string, number> }).data || raw || {};
  const colors = OS_CHART_COLORS;
    return Object.entries(r as Record<string, number>).map(([k, v]) => ({ name: k, value: Number(v) || 0, color: colors[k] || "#6b7280" })).sort((a, b) => b.value - a.value);
  },
);
