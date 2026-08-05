"use client";

import { withChartData } from "@/components/withChartData";
import { useI18n } from "@/lib/i18n";

interface GeoPoint { flag?: string; country?: string; count: number }

function GeoBody({ data }: { data: GeoPoint[] }) {
  const { t } = useI18n();
  return (
    <div className="space-y-1.5 max-h-32 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-4">{t("dashboard.no_geo_data")}</p> : data.map((d) => (
        <div key={d.country || d.flag || "unknown"} className="flex items-center gap-2 text-xs">
          <span className="text-(--fs-micro-sm)">{d.flag || "??"}</span>
          <span className="flex-1 text-foreground truncate">{d.country || t("dashboard.unknown")}</span>
          <span className="font-mono text-(--fs-micro-sm) text-muted-foreground/70">{d.count || 0}</span>
        </div>
      ))}
    </div>
  );
}

export default withChartData<GeoPoint[]>(
  ({ data }) => <GeoBody data={data} />,
  "/api/dashboard/agent-geo",
  (raw) => (raw as { data: GeoPoint[] }).data || [],
);
