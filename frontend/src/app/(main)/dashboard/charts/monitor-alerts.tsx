"use client";

import { withChartData } from "@/components/withChartData";
import { paths } from "@/lib/api-paths";
import { Bell } from "lucide-react";
import { useI18n } from "@/lib/i18n";

function AlertBody({ data }: { data: { message?: string; severity?: string; title?: string }[] }) {
  const { t } = useI18n();
  return (
    <div className="space-y-2 max-h-40 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/100 text-center py-6">{t("dashboard.no_active_alerts")}</p> : data.map((a, i) => (
        <div key={i} className="flex items-start gap-2 text-xs px-2 py-1.5 bg-warning/15 rounded-lg">
          <Bell className="size-4" />
          <span className="text-foreground">{a.message || a.title || t("dashboard.alert")}</span>
        </div>
      ))}
    </div>
  );
}

export default withChartData<{ message?: string; severity?: string; title?: string }[]>(
  ({ data }) => <AlertBody data={data} />,
  paths.monitor.alerts("status=active&limit=8"),
  (raw) => {
    const d = raw as { alerts?: unknown[]; data?: unknown[] };
    return (d.alerts || d.data || []) as { message?: string; severity?: string; title?: string }[];
  },
);
