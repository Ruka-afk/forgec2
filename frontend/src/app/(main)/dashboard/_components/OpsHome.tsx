"use client";

import Link from "next/link";
import { memo, useMemo } from "react";
import { useAppStore } from "@/lib/store";
import { useShallow } from "zustand/shallow";
import { useI18n } from "@/lib/i18n";
import { timeAgo } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { SectionCard } from "@/components/ui/section-card";
import { StatusBadge, StatusIndicator } from "@/components/ui/status-indicator";
import { StatTile } from "@/components/ui/stat-tile";
import { MetricGrid } from "@/components/ui/metric-grid";
import { DataError } from "@/components/ui/data-state";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Archive, Bug, Radio, Clock, ListChecks, Search, Terminal, Wand2 } from "lucide-react";
import type { DashboardStats } from "@/types/agent";
import { useOpsHomeData } from "./useOpsHomeData";
import {
  flattenLoot,
  LOOT_TAB,
  mergeAttention,
  pickUnhealthyListeners,
  splitSessions,
} from "./ops-home";
import { healthIndicatorStatus, translateHealthStatus } from "../../listeners/_components/listener-health";
import ActiveMissions from "./ActiveMissions";

function Panel({
  title,
  href,
  linkLabel,
  count,
  children,
}: {
  title: string;
  href: string;
  linkLabel: string;
  count?: number;
  children: React.ReactNode;
}) {
  return (
    <SectionCard title={title} href={href} linkLabel={linkLabel} count={count}>
      {children}
    </SectionCard>
  );
}

function QuickLaunch() {
  const { t } = useI18n();
  const actions = [
    { href: "/generate", label: t("dashboard.quick_generate"), icon: <Wand2 className="size-5" /> },
    { href: "/listeners", label: t("dashboard.quick_listeners"), icon: <Radio className="size-5" /> },
    { href: "/agents", label: t("dashboard.quick_agents"), icon: <Terminal className="size-5" /> },
    { href: "/search", label: t("dashboard.quick_search"), icon: <Search className="size-5" /> },
  ];
  return (
    <SectionCard title={t("dashboard.quick_actions")}>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 p-4">
        {actions.map((a) => (
          <Button key={a.href} variant="outline" render={<Link href={a.href} />}
            className="h-auto flex-col gap-2 py-4 hover:border-primary hover:bg-primary/5">
            <span className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary transition-transform group-hover:scale-105">{a.icon}</span>
            <span className="text-sm font-medium text-foreground">{a.label}</span>
          </Button>
        ))}
      </div>
    </SectionCard>
  );
}

export default memo(function OpsHome() {
  const { t } = useI18n();
  const { agents, healthByTarget, failedTasks, pendingTasks, approvalTasks, loot, loading, error, refresh } = useOpsHomeData();

  const sessions = useMemo(() => splitSessions(agents), [agents]);
  const unhealthy = useMemo(() => pickUnhealthyListeners(healthByTarget), [healthByTarget]);
  const attention = useMemo(
    () => mergeAttention(failedTasks, pendingTasks, approvalTasks),
    [failedTasks, pendingTasks, approvalTasks],
  );
  const lootItems = useMemo(() => flattenLoot(loot), [loot]);

  return (
    <div className="space-y-5">
      {error && <DataError message={error} onRetry={refresh} className="mb-2" />}

      <DashboardStatTiles loading={loading} unhealthyCount={unhealthy.length} lootCount={lootItems.length} online={sessions.online.length} total={agents.length} />

      <QuickLaunch />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <Panel title={t("dashboard.sessions")} href="/agents" linkLabel={t("dashboard.view_all")} count={agents.length}>
          {loading ? (
            <div className="p-4 space-y-2"><Skeleton className="h-8" /><Skeleton className="h-8" /><Skeleton className="h-8" /></div>
          ) : agents.length === 0 ? (
            <div className="p-5"><EmptyState icon={Bug} title={t("dashboard.no_sessions")} /></div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 divide-y sm:divide-y-0 sm:divide-x divide-border">
              <div>
                <div className="px-4 py-1.5 text-(--fs-micro-sm) uppercase tracking-wider text-muted-foreground">{t("dashboard.sessions_online")}</div>
                {sessions.online.length === 0 ? (
                  <p className="px-4 py-3 text-xs text-muted-foreground">{t("dashboard.no_sessions")}</p>
                ) : sessions.online.map((a) => (
                  <Link key={a.id} href={`/agents/${a.id}`} className="flex items-center justify-between gap-2 px-4 py-2 hover:bg-secondary/50">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <StatusBadge status="online" variant="dotOnly" size="sm" />
                        <span className="truncate font-mono text-sm">{a.hostname || a.id}</span>
                      </div>
                      <div className="truncate text-(--fs-micro-sm) text-muted-foreground">{a.username}{a.ip ? ` · ${a.ip}` : ""}</div>
                    </div>
                    <span className="shrink-0 text-(--fs-micro-sm) text-muted-foreground">{timeAgo(a.last_seen, t)}</span>
                  </Link>
                ))}
              </div>
              <div>
                <div className="px-4 py-1.5 text-(--fs-micro-sm) uppercase tracking-wider text-muted-foreground">{t("dashboard.sessions_dropped")}</div>
                {sessions.dropped.length === 0 ? (
                  <p className="px-4 py-3 text-xs text-muted-foreground">{t("dashboard.no_sessions")}</p>
                ) : sessions.dropped.map((a) => (
                  <Link key={a.id} href={`/agents/${a.id}`} className="flex items-center justify-between gap-2 px-4 py-2 hover:bg-secondary/50">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <StatusBadge status={a.status} variant="dotOnly" size="sm" />
                        <span className="truncate font-mono text-sm">{a.hostname || a.id}</span>
                      </div>
                      <div className="truncate text-(--fs-micro-sm) text-muted-foreground">{a.username}{a.ip ? ` · ${a.ip}` : ""}</div>
                    </div>
                    <span className="shrink-0 text-(--fs-micro-sm) text-muted-foreground">{timeAgo(a.last_seen, t)}</span>
                  </Link>
                ))}
              </div>
            </div>
          )}
        </Panel>

        <Panel title={t("dashboard.listener_health")} href="/listeners" linkLabel={t("dashboard.view_all")} count={unhealthy.length}>
          {loading ? (
            <div className="p-4 space-y-2"><Skeleton className="h-8" /><Skeleton className="h-8" /></div>
          ) : unhealthy.length === 0 ? (
            <div className="p-5"><EmptyState icon={Radio} title={t("dashboard.no_unhealthy_listeners")} /></div>
          ) : (
            <div className="divide-y divide-border">
              {unhealthy.map((h) => (
                <Link key={h.target} href={`/listeners/${h.target}`} className="flex items-center justify-between gap-2 px-4 py-2.5 hover:bg-secondary/50">
                  <div className="min-w-0">
                    <StatusIndicator
                      status={healthIndicatorStatus(h.status)}
                      variant="dot"
                      label={translateHealthStatus(t, h.status)}
                      pulse={h.status === "burned"}
                    />
                    <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                      {h.scheme || "http"}://{h.host}:{h.port}
                    </div>
                  </div>
                  <span className="font-mono text-xs text-destructive">{h.consecutive_fails ?? 0}</span>
                </Link>
              ))}
            </div>
          )}
        </Panel>

        <Panel title={t("dashboard.attention")} href="/timeline?tab=tasks" linkLabel={t("dashboard.view_all")} count={attention.length}>
          {loading ? (
            <div className="p-4 space-y-2"><Skeleton className="h-8" /><Skeleton className="h-8" /></div>
          ) : attention.length === 0 ? (
            <div className="p-5"><EmptyState icon={ListChecks} title={t("dashboard.no_attention")} /></div>
          ) : (
            <div className="divide-y divide-border">
              {attention.map((task) => (
                <Link
                  key={task.id}
                  href={`/timeline?tab=tasks&agent_id=${encodeURIComponent(task.agent_id || "")}`}
                  className="flex items-center justify-between gap-2 px-4 py-2.5 hover:bg-secondary/50"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <StatusIndicator
                        status={task.status}
                        variant="dotOnly"
                        size="sm"
                        ariaLabel={task.status}
                      />
                      <span className="font-mono text-xs">{task.type}</span>
                      <span className="truncate text-xs text-muted-foreground">{task.command}</span>
                    </div>
                    <div className="truncate text-(--fs-micro-sm) text-muted-foreground">{task.agent_id}</div>
                  </div>
                  <span className="shrink-0 text-(--fs-micro-sm) text-muted-foreground">{timeAgo(task.created_at, t)}</span>
                </Link>
              ))}
            </div>
          )}
        </Panel>

        <Panel title={t("dashboard.loot_inbox")} href="/loot" linkLabel={t("dashboard.view_all")} count={lootItems.length}>
          {loading ? (
            <div className="p-4 space-y-2"><Skeleton className="h-8" /><Skeleton className="h-8" /></div>
          ) : lootItems.length === 0 ? (
            <div className="p-5"><EmptyState icon={Archive} title={t("dashboard.no_loot")} /></div>
          ) : (
            <div className="divide-y divide-border">
              {lootItems.map((item) => (
                <Link
                  key={item.id}
                  href={`/loot?tab=${LOOT_TAB[item.kind]}`}
                  className="flex items-center justify-between gap-2 px-4 py-2.5 hover:bg-secondary/50"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <Badge variant="secondary" className="text-(--fs-micro-sm)">
                        {item.kind === "screenshot" ? t("dashboard.loot_screenshot") : item.kind === "keylog" ? t("dashboard.loot_keylog") : t("dashboard.loot_download")}
                      </Badge>
                      <span className="truncate text-xs">{item.label}</span>
                    </div>
                  </div>
                  <span className="shrink-0 text-(--fs-micro-sm) text-muted-foreground">{timeAgo(item.created_at, t)}</span>
                </Link>
              ))}
            </div>
          )}
        </Panel>
      </div>

      <ActiveMissions />
    </div>
  );
});

const DashboardStatTiles = memo(function DashboardStatTiles({ loading, unhealthyCount, lootCount, online, total }: {
  loading: boolean;
  unhealthyCount: number;
  lootCount: number;
  online: number;
  total: number;
}) {
  const { t } = useI18n();
  const stats = useAppStore(useShallow((s) => s.stats));
  const s: Partial<DashboardStats> = stats ?? {};
  const pending = s.pending_tasks ?? 0;
  const failed = s.failed_tasks ?? 0;
  const totalTasks = s.total_tasks ?? 0;

  return (
    <MetricGrid count={5}>
      <Link href="/agents">
        <Card interactive className="p-4 hover:ring-1 hover:ring-primary/20">
          <StatTile label={t("dashboard.beacons")} value={loading && !stats ? "…" : `${online}/${total}`} sub={online === total && total > 0 ? t("dashboard.all_online") : t("dashboard.online_suffix")} tone="success" icon={<Radio className="size-5" />} trend={total > 0 ? <span className="inline-flex items-center gap-1"><span className="size-1.5 rounded-full bg-success animate-pulse" />{t("dashboard.live_agents")}</span> : undefined} />
        </Card>
      </Link>
      <Link href="/timeline?tab=tasks">
        <Card interactive className="p-4 hover:ring-1 hover:ring-primary/20">
          <StatTile label={t("dashboard.pending_tasks_label")} value={loading && !stats ? "…" : pending} tone={pending > 0 ? "warning" : "muted"} icon={<Clock className="size-5" />} trend={totalTasks > 0 ? `${Math.round((pending/totalTasks)*100)}% ${t("dashboard.queue_share")}` : undefined} />
        </Card>
      </Link>
      <Link href="/timeline?tab=tasks">
        <Card interactive className="p-4 hover:ring-1 hover:ring-primary/20">
          <StatTile label={t("dashboard.failed_tasks")} value={loading && !stats ? "…" : failed} tone={failed > 0 ? "destructive" : "muted"} icon={<Bug className="size-5" />} trend={failed > 0 ? t("dashboard.needs_attention") : t("dashboard.all_clear")} />
        </Card>
      </Link>
      <Link href="/listeners">
        <Card interactive className="p-4 hover:ring-1 hover:ring-primary/20">
          <StatTile label={t("dashboard.unhealthy_count")} value={loading ? "…" : unhealthyCount} tone={unhealthyCount > 0 ? "destructive" : "success"} icon={<Radio className="size-5" />} trend={unhealthyCount === 0 ? t("dashboard.all_healthy") : undefined} />
        </Card>
      </Link>
      <Link href="/loot">
        <Card interactive className="p-4 hover:ring-1 hover:ring-primary/20">
          <StatTile label={t("dashboard.loot_inbox")} value={loading ? "…" : lootCount} sub={t("dashboard.loot_recent")} tone={lootCount > 0 ? "info" : "muted"} icon={<Archive className="size-5" />} />
        </Card>
      </Link>
    </MetricGrid>
  );
});
