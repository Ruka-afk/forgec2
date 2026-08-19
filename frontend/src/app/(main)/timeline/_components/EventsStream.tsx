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
import { RefreshCw } from "lucide-react";
import type { Task } from "@/types/task";
import type { TimelineEvent, UnifiedEvent, UnifiedSource } from "./types";
import { filterUnified, mergeEvents, type AlertLike } from "./merge-events";
import { mergePolledWithLive, unifiedFromWS, upsertLiveEvent } from "./ws-to-event";
import { useInteractStore } from "@/lib/interact-store";

const SOURCES: { id: UnifiedSource | "all"; labelKey: string }[] = [
  { id: "all", labelKey: "events.filter_all" },
  { id: "timeline", labelKey: "events.source_timeline" },
  { id: "task", labelKey: "events.source_task" },
  { id: "alert", labelKey: "events.source_alert" },
];

export default function EventsStream() {
  const { t } = useI18n();
  const { connected } = useWS();
  const [source, setSource] = useState<UnifiedSource | "all">("all");
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
      // Partial failure degrades gracefully (only the failed source is empty);
      // only when every source is down do we surface the error state.
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

  const rows = useMemo(
    () => filterUnified(
      mergePolledWithLive(mergeEvents(data?.timeline ?? [], data?.tasks ?? [], data?.alerts ?? []), live),
      source,
      query,
      followSession ? dockAgentId : null,
    ),
    [data, live, source, query, followSession, dockAgentId],
  );

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        {SOURCES.map((s) => (
          <Button
            key={s.id}
            type="button"
            size="xs"
            variant={source === s.id ? "default" : "outline"}
            onClick={() => setSource(s.id)}
          >
            {t(s.labelKey)}
          </Button>
        ))}
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
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder={t("events.search_placeholder")}
          className="min-w-[200px] flex-1"
          label={t("events.search_placeholder")}
        />
        <Badge variant={connected ? "success" : "secondary"} className="font-mono text-(--fs-micro-sm)">
          {connected ? t("events.live") : t("events.polling")}
        </Badge>
        <Button variant="outline" size="sm" onClick={() => void refresh()} className="gap-1.5">
          <RefreshCw className="size-3.5" />
          {t("events.refresh")}
        </Button>
      </div>

      <Card className="overflow-hidden">
        <DataState
          loading={loading}
          error={error}
          onRetry={() => void refresh()}
          empty={!loading && !error && rows.length === 0}
          emptyTitle={t("events.empty")}
          emptyMessage={t("events.empty_hint")}
        >
          <ul className="divide-y divide-border">
            {rows.slice(0, 80).map((ev) => (
              <li key={ev.id} className="flex items-start gap-3 px-4 py-2.5 text-sm">
                <span className="w-36 shrink-0 font-mono text-xs text-muted-foreground">
                  {ev.at ? formatTime(ev.at) : "—"}
                </span>
                <Badge variant={ev.source === "alert" ? "destructive" : ev.source === "task" ? "secondary" : "outline"}>
                  {t(ev.source === "alert" ? "events.source_alert" : ev.source === "task" ? "events.source_task" : "events.source_timeline")}
                </Badge>
                {ev.status && <StatusBadge status={ev.status} />}
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium">{ev.title}</div>
                  {ev.detail && <div className="truncate font-mono text-xs text-muted-foreground">{ev.detail}</div>}
                </div>
                {ev.agentId && (
                  <Link href={`/agents/${ev.agentId}`} className="shrink-0 font-mono text-xs text-primary hover:underline">
                    {ev.agentId.slice(0, 8)}
                  </Link>
                )}
              </li>
            ))}
          </ul>
        </DataState>
      </Card>
    </div>
  );
}
