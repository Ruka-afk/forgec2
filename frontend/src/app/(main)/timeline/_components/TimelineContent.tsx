"use client";

import { useI18n } from "@/lib/i18n";
import { EmptyState } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Pagination } from "@/components/ui/pagination";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Circle, Clock, Search } from "lucide-react";
import { EVENT_COLORS } from "../page";
import type { TimelineEvent } from "../page";

interface TimelineContentProps {
  filteredEvents: TimelineEvent[];
  loading: boolean;
  page: number;
  totalEvents: number;
  textSearch: string;
  onTextSearchChange: (val: string) => void;
  onPageChange: (page: number) => void;
  onSelectEvent: (event: TimelineEvent) => void;
}

function colorFor(type: string) {
  return EVENT_COLORS[type] || { dot: "bg-muted-foreground", bg: "bg-muted", text: "text-muted-foreground", icon: <Circle className="w-2.5 h-2.5" /> };
}

function variantFor(type: string): "success" | "destructive" | "warning" | "outline" {
  if (type === "agent_online") return "success";
  if (type === "alert") return "destructive";
  if (type === "credential") return "warning";
  return "outline";
}

const getEventId = (e: TimelineEvent, i: number) => e.id || String(i);
const getEventTime = (e: TimelineEvent) => e.timestamp || "";
const getEventType = (e: TimelineEvent) => (e.type || "").toLowerCase();
const getEventTitle = (e: TimelineEvent) => e.title || "";
const getEventDesc = (e: TimelineEvent) => e.description || "";
const getEventUser = (e: TimelineEvent) => e.username || "";
const getEventAgent = (e: TimelineEvent) => e.agent_id || "";
const getEventUrl = (e: TimelineEvent) => e.url || "";

export default function TimelineContent({
  filteredEvents, loading, page, totalEvents,
  textSearch, onTextSearchChange, onPageChange, onSelectEvent,
}: TimelineContentProps) {
  const { t } = useI18n();

  return (
    <>
      <div className="flex flex-wrap items-center gap-2 mb-4">
        <div className="flex-1 min-w-[200px] relative">
          <Search className="w-4 h-4" />
          <Input placeholder="Search event content..." value={textSearch} onChange={e => onTextSearchChange(e.target.value)} className="pl-9" aria-label="Search event content" />
        </div>
        <span className="text-xs text-muted-foreground">
          <Circle className="w-4 h-4" />
          Live Event Stream
        </span>
      </div>

      <Card className="p-4 mb-4">
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div className="text-center">
            <div className="text-2xl font-bold">{totalEvents}</div>
            <div className="text-(--font-size-micro-sm) text-muted-foreground uppercase tracking-wider mt-0.5">Total Events</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-emerald-600">{filteredEvents.filter(e => getEventType(e) === "agent_online").length}</div>
            <div className="text-(--font-size-micro-sm) text-muted-foreground uppercase tracking-wider mt-0.5">Agent Events</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-amber-600">{filteredEvents.filter(e => getEventType(e) === "credential").length}</div>
            <div className="text-(--font-size-micro-sm) text-muted-foreground uppercase tracking-wider mt-0.5">Credential Events</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-red-600">{filteredEvents.filter(e => getEventType(e) === "alert").length}</div>
            <div className="text-(--font-size-micro-sm) text-muted-foreground uppercase tracking-wider mt-0.5">Alerts</div>
          </div>
        </div>
      </Card>

      <Card className="p-4 sm:p-5 max-h-[65vh] overflow-y-auto">
        {loading ? (
          <div className="space-y-6 pl-12">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="relative">
                <Skeleton className="absolute left-[-2.5rem] top-0 w-5 h-5 rounded-full" />
                <Skeleton className="h-4 w-24 mb-2" />
                <Skeleton className="h-4 w-3/4 mb-1" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            ))}
          </div>
        ) : filteredEvents.length === 0 ? (
          <div className="text-center py-16 sm:py-20">
            <EmptyState icon={Clock} title={t("timeline.empty_title")} message={t("timeline.empty_message")} />
          </div>
        ) : (
          <div className="relative">
            <div className="absolute left-5 top-0 bottom-0 w-px bg-border"></div>
            <div className="space-y-6">
              {filteredEvents.map((e, i) => {
                const type = getEventType(e);
                const color = colorFor(type);
                const url = getEventUrl(e);
                return (
                  <div
                    key={getEventId(e, i)}
                    onClick={() => onSelectEvent(e)}
                    className="relative flex gap-4 pl-12 cursor-pointer group hover:bg-muted/50 -mx-2 px-2 py-2 rounded-lg transition-colors"
                  >
                    <div className={`absolute left-3 top-1 w-5 h-5 rounded-full ${color.bg} flex items-center justify-center ring-4 ring-background`}>
                      <span className={color.text}>{color.icon}</span>
                    </div>
                    <div className="flex-1 -mt-0.5">
                      <div className="flex items-center gap-2 mb-1 flex-wrap">
                        <span className="text-xs text-muted-foreground font-mono">{getEventTime(e)}</span>
                        {getEventUser(e) && <Badge variant="secondary">{getEventUser(e)}</Badge>}
                        {getEventAgent(e) && <Badge variant="outline" className="font-mono">{getEventAgent(e)}</Badge>}
                        <Badge variant={variantFor(type)}>{type}</Badge>
                      </div>
                      <p className="text-sm font-medium text-muted-foreground group-hover:text-foreground transition-colors">{getEventTitle(e)}</p>
                      {getEventDesc(e) && <p className="text-xs text-muted-foreground mt-0.5">{getEventDesc(e)}</p>}
                      {url && <span className="text-(--font-size-micro-sm) text-indigo-500 dark:text-indigo-400 inline-block mt-1">View Details</span>}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </Card>

      <Pagination page={page} pageSize={50} total={totalEvents} onPageChange={onPageChange} />
    </>
  );
}
