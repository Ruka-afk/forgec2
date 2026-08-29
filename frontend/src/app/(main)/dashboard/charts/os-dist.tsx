"use client";

import { useMemo } from "react";
import { withChartData } from "@/components/withChartData";
import { paths } from "@/lib/api-paths";
import { osColor } from "@/lib/chart-palette";
import { useI18n } from "@/lib/i18n";

interface OSPoint { name: string; value: number; color: string }

function OSBody({ data }: { data: OSPoint[] }) {
  const { t } = useI18n();
  const { total, items } = useMemo(() => {
    const total = data.reduce((s, d) => s + d.value, 0);
    const items = data.map((d) => ({ ...d, color: osColor(d.name) }));
    return { total, items };
  }, [data]);
  const gradient = useMemo(() => {
    if (total <= 0) return undefined;
    let acc = 0;
    const segments = items.map((d) => {
      const start = (acc / total) * 360;
      acc += d.value;
      const end = (acc / total) * 360;
      return `${d.color} ${start}deg ${end}deg`;
    });
    return `conic-gradient(${segments.join(", ")})`;
  }, [items, total]);
  return (
    <div className="flex items-center gap-4">
      <div className="relative size-20 shrink-0" aria-hidden="true">
        {total > 0 ? (
          <div className="w-full h-full rounded-full" style={{ background: gradient }}></div>
        ) :           <div className="w-full h-full rounded-full bg-secondary"></div>}
        <div className="absolute inset-2 rounded-full bg-card flex items-center justify-center text-xs font-bold text-foreground">{total}</div>
      </div>
      <div className="flex-1 space-y-1.5">
        {items.length === 0 ? <p className="text-xs text-muted-foreground/100">{t("common.no_data")}</p> : items.map((d) => (
          <div key={d.name} className="flex items-center gap-2 text-xs">
            <span className="size-2.5 rounded-full shrink-0" style={{ backgroundColor: d.color }}></span>
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
  paths.dashboard.osDistribution,
  (raw) => {
    // Endpoint returns a bare [{name,count}] array; tolerate envelope-wrapped
    // arrays and plain Record<string, number> maps for robustness.
    let entries: Array<Record<string, unknown> | [string, unknown]> = [];
    if (Array.isArray(raw)) {
      entries = raw as Array<Record<string, unknown>>;
    } else if (Array.isArray((raw as { data?: unknown }).data)) {
      entries = (raw as { data: Array<Record<string, unknown>> }).data;
    } else if (raw && typeof raw === "object") {
      entries = Object.entries(raw as Record<string, unknown>);
    }
    return entries
      .map((item) => {
        if (Array.isArray(item)) return { name: String(item[0]), value: Number(item[1]) || 0, color: osColor(String(item[0])) };
        const name = String(item.name ?? item.os ?? "Unknown");
        return { name, value: Number(item.count ?? item.value) || 0, color: osColor(name) };
      })
      .sort((a, b) => b.value - a.value);
  },
);
