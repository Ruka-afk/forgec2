"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchCached } from "@/lib/hooks/useCachedData";
import { ChartCard } from "@/components/ChartCard";
import { Spinner } from "@/components/UI";
import { ArrowLeftRight } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useI18n } from "@/lib/i18n";

interface TrafficPoint { value: number; time?: string }

function TrafficBody({ data }: { data: TrafficPoint[] }) {
  const { t } = useI18n();
  const maxVal = Math.max(...data.map((d) => Number(d.value) || 0), 1);
  return (
    <div className="space-y-2">
      <div className="flex items-end gap-0.5 h-16">
        {data.length === 0 ? <span className="text-xs text-muted-foreground/70">{t("dashboard.no_traffic_data")}</span> : data.slice(0, 30).map((d, i) => (
          <Tooltip key={d.time ?? i}><TooltipTrigger><div className="flex-1 bg-primary rounded-t-sm min-h-[2px] transition-all" style={{ height: `${Math.max(2, ((Number(d.value) || 0) / maxVal) * 100)}%` }}></div></TooltipTrigger><TooltipContent>{String(d.time ?? i)}</TooltipContent></Tooltip>
        ))}
      </div>
    </div>
  );
}

function ListenerTrafficInner({ range }: { range: string }) {
  const { t } = useI18n();
  const [data, setData] = useState<TrafficPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setLoadError(false);
    const endpoint = paths.dashboard.listenerTraffic(range);
    fetchCached<TrafficPoint[]>(`traffic:${endpoint}`, async () => {
      const d = await api.get<TrafficPoint[] | { data: TrafficPoint[] } | { labels?: string[]; bytes_in?: number[]; bytes_out?: number[] }>(endpoint, { signal: controller.signal });
      const o = (d as { data?: { labels?: string[]; bytes_in?: number[]; bytes_out?: number[] } })?.data ?? d;
      const obj = (o as { labels?: string[]; bytes_in?: number[]; bytes_out?: number[] }) || {};
      const labels = obj.labels || [];
      const bins = obj.bytes_in || [];
      const bouts = obj.bytes_out || [];
      return labels.map((t, i) => ({ time: t, value: (Number(bins[i]) || 0) + (Number(bouts[i]) || 0) }));
    }, 30_000)
      .then((parsed) => {
        if (controller.signal.aborted) return;
        setData(parsed);
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setData([]);
        setLoadError(true);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [range]);
  if (loading) return <div className="h-24 flex items-center justify-center text-muted-foreground/70 text-xs"><Spinner size="sm" /></div>;
  if (loadError && data.length === 0) {
    return <div className="h-24 flex items-center justify-center text-muted-foreground/70 text-xs">{t("dashboard.traffic_load_failed")}</div>;
  }
  return (
    <div className="space-y-2">
      <TrafficBody data={data} />
    </div>
  );
}

export default function ListenerTrafficSection({ range, className }: { range: string; className?: string }) {
  const { t } = useI18n();
  return (
    <ChartCard title={t("dashboard.listener_traffic")} icon={ArrowLeftRight} iconColor="text-cyan-500 dark:text-cyan-400" exportFilename="listener-traffic.png" className={className}>
      <ListenerTrafficInner range={range} />
    </ChartCard>
  );
}
