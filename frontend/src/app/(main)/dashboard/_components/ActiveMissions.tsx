"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { onWSMessage } from "@/lib/wsContext";
import { useI18n } from "@/lib/i18n";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { POLL } from "@/lib/polling";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Target } from "lucide-react";

export interface Mission {
  id: number;
  agent_id: string;
  hostname?: string;
  ip?: string;
  os?: string;
  type: string;
  command: string;
  status: "pending" | "running";
  priority: number;
  created_by: string;
  created_at: string;
  claimed_by?: string;
  progress?: number;
  total_bytes?: number;
  transferred?: number;
}

const MAX_MISSIONS = 20;

function missionLabel(mission: Mission): string {
  if (mission.command) return mission.command;
  return mission.type;
}

function formatDuration(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

export default function ActiveMissions({ className = "" }: { className?: string }) {
  const { t } = useI18n();
  const [now, setNow] = useState(() => Date.now());

  useVisibleInterval(() => setNow(Date.now()), POLL.clockTick);

  const { data, error, refresh: load } = useApiResource<{ missions: Mission[] }>({
    fetcher: async () => {
      const data = await api.get<{ missions: Mission[] }>(paths.dashboard.activeMissions);
      return { missions: data?.missions || [] };
    },
    pollMs: POLL.missions,
    errorMessage: t("dashboard.missions_load_failed"),
  });

  // WS task_update events (status transitions to/from pending/running) change
  // the board — refetch on each. Debounced so beacon bursts coalesce.
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    const unsub = onWSMessage((msg) => {
      if (msg.type !== "task_update") return;
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => { load(); }, 500);
    });
    return () => {
      if (timer) clearTimeout(timer);
      unsub();
    };
  }, [load]);

  const missions = data?.missions ?? [];
  const active = missions.slice(0, MAX_MISSIONS);

  return (
    <Card className={className}>
      <CardHeader className="px-5 py-3.5 border-b border-border flex-row items-center justify-between">
        <CardTitle className="text-sm font-semibold text-foreground flex items-center gap-2">
          <Target className="w-4 h-4 text-primary" aria-hidden="true" />
          {t("dashboard.missions")}
        </CardTitle>
        <Badge variant={active.length > 0 ? "success" : "secondary"}>{active.length}</Badge>
      </CardHeader>
      {error ? (
        <div className="p-(--card-spacing)">
          <ErrorState title={t("common.error")} message={error} />
        </div>
      ) : active.length === 0 ? (
        <div className="p-(--card-spacing)">
          <EmptyState icon={Target} title={t("dashboard.missions_empty")} />
        </div>
      ) : (
        <div className="divide-y divide-border">
          {active.map((m) => (
            <div key={m.id} className="flex items-center justify-between gap-3 px-5 py-2.5 hover:bg-secondary transition-colors">
              <div className="flex items-center gap-3 min-w-0">
                <StatusIndicator
                  status={m.status === "running" ? "running" : "pending"}
                  variant="dotOnly"
                  size="sm"
                  pulse={m.status === "running"}
                  ariaLabel={m.status}
                />
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5">
                    <span className="text-xs font-mono font-medium text-foreground truncate">{m.hostname || m.agent_id.slice(0, 8)}</span>
                    {m.ip && <span className="text-(--fs-micro-sm) text-muted-foreground/70 truncate hidden sm:inline">{m.ip}</span>}
                  </div>
                  <div className="text-(--fs-micro-sm) text-muted-foreground/80 truncate max-w-[28rem]">{missionLabel(m)}</div>
                </div>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                {m.type === "download" && m.total_bytes ? (
                  <Badge variant="info">{Math.round(((m.transferred || 0) / m.total_bytes) * 100)}%</Badge>
                ) : null}
                <Badge variant="secondary" className="font-mono">{m.type}</Badge>
                <span className="text-(--fs-micro-sm) text-muted-foreground/70 whitespace-nowrap" title={m.created_by ? `${t("dashboard.missions_by")} ${m.created_by}` : undefined}>
                  {formatDuration(now - new Date(m.created_at).getTime())}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}
