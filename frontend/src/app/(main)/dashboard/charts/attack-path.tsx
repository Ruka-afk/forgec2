"use client";

import { withChartData } from "@/components/withChartData";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";

interface AttackPathPoint { name?: string; host?: string; target?: string; type?: string }

function AttackBody({ data }: { data: AttackPathPoint[] }) {
  const { t } = useI18n();
  return (
    <div className="space-y-1.5 max-h-32 overflow-y-auto">
      {data.length === 0 ? <p className="text-xs text-muted-foreground/70 text-center py-4">{t("dashboard.no_attack_path_data")}</p> : data.map((d, i) => (
        <div key={d.name || d.host || d.target || `node-${i}`} className="flex items-center gap-2 text-xs">
          <span className="w-5 h-5 rounded-full bg-primary/10 dark:bg-primary/25 flex items-center justify-center text-(--fs-micro) text-primary font-bold shrink-0">{i + 1}</span>
          <span className="flex-1 text-foreground truncate">{d.name || d.host || d.target || t("dashboard.unknown")}</span>
          <span className="text-(--fs-micro-sm) text-muted-foreground/70">{d.type || ""}</span>
        </div>
      ))}
    </div>
  );
}

export default withChartData<AttackPathPoint[]>(
  ({ data }) => <AttackBody data={data} />,
  paths.dashboard.attackPath,
  (raw) => {
    const o = (raw as { data?: { nodes?: { id?: string; label?: string; type?: string }[] } }).data;
    const nodes = (o?.nodes || []) as { id?: string; label?: string; type?: string }[];
    return nodes.map((n) => ({ name: n.label || n.id, host: n.id, target: n.label, type: n.type }));
  },
);
