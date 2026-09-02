"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { timeAgo } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { StatTile } from "@/components/ui/stat-tile";
import { Target, RefreshCw } from "lucide-react";

interface EffectivenessAgent {
  id: string;
  hostname: string;
  status: string;
  version: string;
}

interface EffectiveBuild {
  id: number;
  platform: string;
  format: string;
  filename: string;
  c2_url: string;
  user?: string;
  created_at: string;
  deployed: number;
  online_now: number;
  agents: EffectivenessAgent[];
}

interface EffectivenessData {
  builds: EffectiveBuild[];
  window_hours: number;
  total_success: number;
  total_failed: number;
  deployed_builds: number;
  online_now: number;
}

/**
 * EffectivenessCard — per-build "did this payload actually run?" signal:
 * correlates successful builds with implants that checked in afterwards on
 * the same listener (time-window heuristic, see handlers_build_effectiveness).
 */
export default function EffectivenessCard() {
  const { t } = useI18n();
  const [data, setData] = useState<EffectivenessData | null>(null);
  const [loading, setLoading] = useState(true);
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const load = () => {
    setLoading(true);
    api.get<EffectivenessData>(paths.builds.effectiveness(30))
      .then(setData)
      .catch(() => setData(null))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const reachRate = data && data.total_success > 0
    ? Math.round((data.deployed_builds / data.total_success) * 100)
    : 0;

  return (
    <Card className="p-(--card-spacing) mb-4">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-semibold text-foreground flex items-center gap-2">
          <Target className="size-4" /> {t("builds.effectiveness_title")}
        </span>
        <Button variant="ghost" size="icon-sm" onClick={load} aria-label={t("common.refresh")}>
          {loading ? <Spinner size="xs" /> : <RefreshCw className="size-4" />}
        </Button>
      </div>

      {!data ? (
        loading ? <div className="py-6 text-center"><Spinner /></div>
          : <p className="text-xs text-muted-foreground text-center py-4">{t("builds.effectiveness_empty")}</p>
      ) : (
        <>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
            <StatTile label={t("builds.eff_total")} value={String(data.total_success)} tone="info" />
            <StatTile label={t("builds.eff_deployed")} value={`${data.deployed_builds}`} tone="success" />
            <StatTile label={t("builds.eff_reach_rate")} value={`${reachRate}%`} tone="warning" />
            <StatTile label={t("builds.eff_online")} value={`${data.online_now}`} tone="muted" />
          </div>

          {data.builds.length === 0 ? (
            <p className="text-xs text-muted-foreground text-center py-3">{t("builds.effectiveness_empty")}</p>
          ) : (
            <div className="divide-y divide-border max-h-72 overflow-y-auto">
              {data.builds.slice(0, 20).map((b) => (
                <div key={b.id} className="py-2">
                  <button
                    className="w-full flex items-center gap-3 text-left hover:bg-muted/40 rounded-lg px-2 py-1.5 transition-colors"
                    onClick={() => setExpandedId(expandedId === b.id ? null : b.id)}
                  >
                    <Badge variant={b.deployed > 0 ? "success" : "secondary"} className="shrink-0 font-mono text-(--fs-micro)">
                      {b.deployed > 0 ? `${b.deployed} ${t("builds.eff_agents_short")}` : t("builds.eff_no_beacon")}
                    </Badge>
                    <span className="text-xs font-medium text-foreground truncate flex-1">{b.filename}</span>
                    <span className="text-xs text-muted-foreground shrink-0">{timeAgo(b.created_at, t)}</span>
                    {b.online_now > 0 && (
                      <Badge variant="warning" className="shrink-0 text-(--fs-micro)">{b.online_now} online</Badge>
                    )}
                  </button>
                  {expandedId === b.id && b.agents.length > 0 && (
                    <div className="pl-10 pr-3 pb-2 space-y-1">
                      {b.agents.map((a) => (
                        <div key={a.id} className="flex items-center gap-2 text-xs">
                          <span className={`size-1.5 rounded-full ${a.status === "online" ? "bg-success" : "bg-muted-foreground"}`} />
                          <span className="font-medium text-foreground">{a.hostname || a.id.substring(0, 8)}</span>
                          <span className="font-mono text-muted-foreground">{a.version || ""}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
          <p className="text-(--fs-micro-sm) text-muted-foreground mt-2">
            {t("builds.eff_window_hint", { hours: String(data.window_hours) })}
          </p>
        </>
      )}
    </Card>
  );
}
