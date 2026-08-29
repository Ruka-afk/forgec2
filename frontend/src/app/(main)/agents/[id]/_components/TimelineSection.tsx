"use client";

import { memo, useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { timeAgo, formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import {
  Camera, ChevronDown, History, KeyRound, RefreshCw, Terminal,
} from "lucide-react";

interface TimelineEvent {
  time: string;
  kind: "task" | "screenshot" | "credential" | "status" | string;
  type?: string;
  title: string;
  detail?: string;
  status?: string;
  ref_id?: number;
}

const KIND_STYLES: Record<string, { icon: typeof Terminal; ring: string; badge: "default" | "success" | "warning" | "destructive" | "secondary" }> = {
  task: { icon: Terminal, ring: "bg-primary/15 text-primary", badge: "default" },
  screenshot: { icon: Camera, ring: "bg-info/15 text-info", badge: "secondary" },
  credential: { icon: KeyRound, ring: "bg-warning/15 text-warning", badge: "warning" },
  status: { icon: History, ring: "bg-muted text-muted-foreground", badge: "secondary" },
};

const STATUS_VARIANT: Record<string, "success" | "destructive" | "warning" | "secondary"> = {
  completed: "success",
  success: "success",
  failed: "destructive",
  error: "destructive",
  running: "warning",
  pending: "secondary",
};

const KINDS = ["task", "screenshot", "credential", "status"] as const;

/**
 * TimelineSection — unified per-host activity feed: tasks, screenshots,
 * harvested credentials and online/offline transitions merged into one
 * reverse-chronological axis with kind filters.
 */
export default memo(function TimelineSection({ agentId, online }: { agentId: string; online: boolean }) {
  const { t } = useI18n();
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [kinds, setKinds] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState(false);
  // Request-generation guard: refresh spam / kind toggles / expand fired
  // concurrent loads whose resolutions could land out of order, installing
  // data that contradicts the current filter/limit.
  const genRef = useRef(0);

  const load = useCallback(() => {
    const gen = ++genRef.current;
    setLoading(true);
    const params = new URLSearchParams({ limit: expanded ? "400" : "60" });
    if (kinds.size > 0) params.set("kinds", [...kinds].join(","));
    api.get<{ events?: TimelineEvent[] }>(paths.agents.timeline(agentId, params.toString()))
      .then((d) => {
        if (gen !== genRef.current) return;
        setEvents(d.events || []);
      })
      .catch(() => {
        if (gen !== genRef.current) return;
        setEvents([]);
      })
      .finally(() => {
        if (gen === genRef.current) setLoading(false);
      });
    // Reload when the agent transitions to online so fresh activity shows up.
  }, [agentId, kinds, expanded]);

  // Reload when the agent transitions to online so fresh activity shows up.
  useEffect(() => { void load(); }, [load, online]);

  const toggleKind = (kind: string) => {
    setKinds(prev => {
      const next = new Set(prev);
      if (next.has(kind)) next.delete(kind); else next.add(kind);
      return next;
    });
  };

  return (
    <Card className="mb-4 gap-0">
      <div className="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 className="text-sm font-semibold text-foreground flex items-center gap-1.5">
          <History className="size-3.5" />{t("agents.timeline_title")}
        </h3>
        <Button variant="ghost" size="icon-xs" onClick={load} aria-label={t("common.refresh")}>
          <RefreshCw className={`size-3.5 ${loading ? "animate-spin" : ""}`} />
        </Button>
      </div>

      {/* Kind filter chips */}
      <div className="px-4 pt-2.5 flex items-center gap-1.5 flex-wrap">
        {KINDS.map((kind) => {
          const active = kinds.size === 0 || kinds.has(kind);
          return (
            <button
              key={kind}
              onClick={() => toggleKind(kind)}
              className={`text-(--fs-micro-sm) px-2 py-0.5 rounded-full border transition-colors ${
                active
                  ? "border-primary/40 bg-primary/10 text-primary"
                  : "border-border text-muted-foreground hover:text-foreground"
              }`}
            >
              {t(`agents.timeline_kind_${kind}`)}
            </button>
          );
        })}
      </div>

      <div className="p-4">
        {loading && events.length === 0 ? (
          <div className="py-8 text-center"><Spinner /></div>
        ) : events.length === 0 ? (
          <p className="text-xs text-muted-foreground text-center py-6">{t("agents.timeline_empty")}</p>
        ) : (
          <>
            <div className="relative space-y-0">
              {events.map((ev, i) => {
                const style = KIND_STYLES[ev.kind] || KIND_STYLES.task;
                const Icon = style.icon;
                return (
                  <div key={`${ev.kind}-${ev.ref_id}-${i}`} className="relative flex gap-3 pb-4 last:pb-0">
                    {/* Axis line + node */}
                    {i < events.length - 1 && (
                      <span className="absolute left-[13px] top-7 bottom-0 w-px bg-border" aria-hidden="true" />
                    )}
                    <span className={`relative z-10 size-7 rounded-lg flex items-center justify-center shrink-0 ${style.ring}`}>
                      <Icon className="size-3.5" />
                    </span>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        {ev.kind === "task" && ev.status && (
                          <Badge variant={STATUS_VARIANT[ev.status] || "secondary"} className="text-(--fs-micro)">
                            {ev.status}
                          </Badge>
                        )}
                        {ev.kind === "status" && (
                          <Badge variant={ev.status === "online" ? "success" : "secondary"} className="text-(--fs-micro)">
                            {ev.title}
                          </Badge>
                        )}
                        <span className="text-xs text-foreground break-all">{ev.title}</span>
                      </div>
                      <div className="text-(--fs-micro-sm) text-muted-foreground mt-0.5 flex items-center gap-2">
                        <span title={formatTime(ev.time)}>{timeAgo(ev.time, t)}</span>
                        {ev.detail && <span>· {ev.detail}</span>}
                        {ev.type && ev.kind === "task" && <span className="font-mono">· {ev.type}</span>}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>

            {!expanded && events.length >= 60 && (
              <button
                onClick={() => setExpanded(true)}
                className="w-full mt-2 flex items-center justify-center gap-1 text-xs text-primary hover:underline"
              >
                <ChevronDown className="size-4" />{t("agents.timeline_more")}
              </button>
            )}
          </>
        )}
      </div>
    </Card>
  );
});
