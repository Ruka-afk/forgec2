"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { firstArray } from "@/lib/envelope";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { POLL } from "@/lib/polling";
import { useWS } from "@/lib/wsContext";
import { subscribeTyped } from "@/lib/typed-ws";
import { TIMELINE_EVENTS } from "@/lib/ws-events";
import { formatTime } from "@/lib/utils";
import { DataState } from "@/components/ui/data-state";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { SearchInput } from "@/components/framework/SearchInput";
import { StatusBadge } from "@/components/ui/status-indicator";
import { RefreshCw, Activity, AlertTriangle, ListTodo, Zap, Radio } from "lucide-react";
import type { Task } from "@/types/task";
import type { TimelineEvent, UnifiedEvent } from "./types";
import { filterUnified, mergeEvents, type AlertLike } from "./merge-events";
import { mergePolledWithLive, unifiedFromWS, upsertLiveEvent } from "./ws-to-event";
import { useInteractStore } from "@/lib/interact-store";

const SOURCE_ICONS: Record<string, React.ReactNode> = {
  timeline: <Zap className="size-3.5" />,
  task: <ListTodo className="size-3.5" />,
  alert: <AlertTriangle className="size-3.5" />,
};

const SOURCE_STYLES: Record<string, { border: string; iconBg: string; iconText: string; bg: string }> = {
  timeline: { border: "border-l-info", iconBg: "bg-info/15", iconText: "text-info", bg: "" },
  task: { border: "border-l-chart-6", iconBg: "bg-chart-6/15", iconText: "text-chart-6", bg: "bg-chart-6/[0.03]" },
  alert: { border: "border-l-destructive", iconBg: "bg-destructive/15", iconText: "text-destructive", bg: "bg-destructive/[0.03]" },
};

export default function EventsStream() {
  const { t } = useI18n();
  const { connected } = useWS();
  const [query, setQuery] = useState("");
  const [live, setLive] = useState<UnifiedEvent[]>([]);
  const dockAgentId = useInteractStore((s) => s.agentId);
  const [followSession, setFollowSession] = useState(true);

  const liveBuffer = useRef<UnifiedEvent[]>([]);
  const rafId = useRef<number | null>(null);
  const flushLive = useCallback(() => {
    rafId.current = null;
    const incoming = liveBuffer.current;
    liveBuffer.current = [];
    if (incoming.length === 0) return;
    setLive((prev) => incoming.reduce((next, ev) => upsertLiveEvent(next, ev), prev));
  }, []);

  const { data, loading, error, refresh } = useApiResource<{ timeline: TimelineEvent[]; tasks: Task[]; alerts: AlertLike[] }>({
    fetcher: async () => {
      const [tl, tk, nt] = await Promise.allSettled([
        api.get(paths.timeline.data("page=1")),
        api.get(paths.tasks.list("page=1&pageSize=50")),
        api.get(paths.notifications.list("page=1&pageSize=50")),
      ]);
      const failures = [tl, tk, nt].filter((r) => r.status === "rejected").length;
      if (failures === 3) throw new Error(t("events.load_failed"));
      return {
        timeline: tl.status === "fulfilled" ? firstArray(tl.value, ["events", "data", "Events"]) as TimelineEvent[] : [],
        tasks: tk.status === "fulfilled" ? firstArray(tk.value, ["tasks", "data", "Tasks"]) as Task[] : [],
        alerts: nt.status === "fulfilled" ? firstArray(nt.value, ["notifications", "data"]) as AlertLike[] : [],
      };
    },
    pollMs: connected ? POLL.events : POLL.eventsFallback,
    errorMessage: t("events.load_failed"),
  });

  useEffect(() => {
    const unsub = subscribeTyped(TIMELINE_EVENTS, (msg) => {
      const ev = unifiedFromWS(msg);
      if (!ev) return;
      liveBuffer.current.push(ev);
      if (rafId.current === null) {
        rafId.current = requestAnimationFrame(flushLive);
      }
    });
    return () => {
      unsub();
      if (rafId.current !== null) {
        cancelAnimationFrame(rafId.current);
        rafId.current = null;
      }
    };
  }, [flushLive]);

  const allRows = useMemo(
    () => mergeEvents(data?.timeline ?? [], data?.tasks ?? [], data?.alerts ?? []),
    [data],
  );

  const rows = useMemo(
    () => filterUnified(
      mergePolledWithLive(allRows, live),
      "all",
      query,
      followSession ? dockAgentId : null,
    ),
    [allRows, live, query, followSession, dockAgentId],
  );

  const counts = useMemo(() => {
    const c = { timeline: 0, task: 0, alert: 0 };
    for (const r of allRows) { if (r.source in c) c[r.source as keyof typeof c]++; }
    return c;
  }, [allRows]);

  return (
    <div className="flex flex-col gap-4">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-2">
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder={t("events.search_placeholder")}
          className="min-w-[200px] flex-1"
          label={t("events.search_placeholder")}
        />
        {dockAgentId && (
          <Button
            type="button"
            size="xs"
            variant={followSession ? "default" : "outline"}
            onClick={() => setFollowSession((v) => !v)}
          >
            {followSession ? t("events.follow_session") : t("events.follow_all")}
          </Button>
        )}
        <Badge variant={connected ? "success" : "secondary"} className="font-mono text-(--fs-micro-sm)">
          <Radio className="size-3 mr-1" />
          {connected ? t("events.live") : t("events.polling")}
        </Badge>
        <Button variant="outline" size="sm" onClick={() => void refresh()} className="gap-1.5">
          <RefreshCw className="size-3.5" />
          {t("events.refresh")}
        </Button>
      </div>

      {/* Source summary chips */}
      <div className="flex items-center gap-3">
        {(["timeline", "task", "alert"] as const).map((src) => {
          const s = SOURCE_STYLES[src];
          const labelKey = src === "timeline" ? "events.source_timeline" : src === "task" ? "events.source_task" : "events.source_alert";
          return (
            <div key={src} className={`flex items-center gap-1.5 rounded-md border border-border/50 bg-background px-2.5 py-1 text-xs`}>
              <span className={`flex size-5 items-center justify-center rounded ${s.iconBg} ${s.iconText}`}>
                {SOURCE_ICONS[src]}
              </span>
              <span className="text-muted-foreground">{t(labelKey)}</span>
              <span className="font-mono font-semibold text-foreground">{counts[src]}</span>
            </div>
          );
        })}
        <div className="ml-auto text-xs text-muted-foreground font-mono">
          {rows.length} {rows.length === 1 ? "event" : "events"}
        </div>
      </div>

      {/* Event list */}
      <Card className="overflow-hidden divide-y divide-border/50">
        <DataState
          loading={loading}
          error={error}
          onRetry={() => void refresh()}
          empty={!loading && !error && rows.length === 0}
          emptyTitle={t("events.empty")}
          emptyMessage={t("events.empty_hint")}
        >
          <ul>
            {rows.slice(0, 80).map((ev) => {
              const s = SOURCE_STYLES[ev.source] ?? SOURCE_STYLES.timeline;
              return (
                <li
                  key={ev.id}
                  className={`group flex items-start gap-3 border-l-2 ${s.border} ${s.bg} px-4 py-3 text-sm transition-colors hover:bg-muted/30`}
                >
                  {/* Source icon */}
                  <span className={`mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-md ${s.iconBg} ${s.iconText}`}>
                    {SOURCE_ICONS[ev.source] ?? <Activity className="size-3.5" />}
                  </span>

                  {/* Time */}
                  <span className="w-28 shrink-0 pt-0.5 font-mono text-xs text-muted-foreground">
                    {ev.at ? formatTime(ev.at) : "—"}
                  </span>

                  {/* Content */}
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium leading-snug">{ev.title}</span>
                      {ev.source === "task" && ev.kind && ev.kind !== ev.title && (
                        <Badge variant="outline" className="border-dashed text-[10px] px-1.5 py-0 font-mono">{ev.kind}</Badge>
                      )}
                      {ev.source === "alert" && ev.kind && (
                        <Badge variant={ev.kind === "critical" || ev.kind === "error" ? "destructive" : ev.kind === "warning" ? "warning" : "secondary"} className="text-[10px] px-1.5 py-0">{ev.kind}</Badge>
                      )}
                      {ev.status && <StatusBadge status={ev.status} />}
                    </div>
                    {ev.detail && (
                      <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground/80">
                        {ev.detail}
                      </div>
                    )}
                  </div>

                  {/* Agent link */}
                  {ev.agentId && (
                    <Link
                      href={`/agents/${ev.agentId}`}
                      className="shrink-0 rounded-md bg-primary/10 px-1.5 py-0.5 font-mono text-xs text-primary transition-colors hover:bg-primary/20"
                    >
                      {ev.agentId.slice(0, 8)}
                    </Link>
                  )}
                </li>
              );
            })}
          </ul>
        </DataState>
      </Card>
    </div>
  );
}
