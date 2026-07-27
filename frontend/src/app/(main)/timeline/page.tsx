"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { downloadBlob } from "@/lib/download";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Download, Menu, PanelLeftClose, RefreshCw } from "lucide-react";
import TimelineFilters from "./_components/TimelineFilters";
import TimelineContent from "./_components/TimelineContent";
import EventDetailDialog from "./_components/EventDetailDialog";

export interface TimelineEvent {
  id?: string;
  timestamp?: string;
  type?: string;
  title?: string;
  description?: string;
  username?: string;
  agent_id?: string;
  url?: string;
}

export const EVENT_TYPES = ["agent_online", "task", "credential", "user", "system", "alert"] as const;

export const EVENT_COLORS: Record<string, { dot: string; bg: string; text: string; icon: React.ReactNode }> = {
  agent_online: { dot: "bg-emerald-500", bg: "bg-emerald-50 dark:bg-emerald-900/30", text: "text-emerald-600 dark:text-emerald-400", icon: null },
  task: { dot: "bg-blue-500", bg: "bg-blue-50 dark:bg-blue-900/30", text: "text-blue-600 dark:text-blue-400", icon: null },
  credential: { dot: "bg-amber-500", bg: "bg-amber-50 dark:bg-amber-900/30", text: "text-amber-600 dark:text-amber-400", icon: null },
  user: { dot: "bg-purple-500", bg: "bg-purple-50 dark:bg-purple-900/30", text: "text-purple-600 dark:text-purple-400", icon: null },
  system: { dot: "bg-purple-500", bg: "bg-purple-50 dark:bg-purple-900/30", text: "text-purple-600 dark:text-purple-400", icon: null },
  alert: { dot: "bg-red-500", bg: "bg-red-50 dark:bg-red-900/30", text: "text-red-600 dark:text-red-400", icon: null },
};

const POLL_INTERVAL = 10000;

export default function TimelinePage() {
  const { t } = useI18n();
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [totalEvents, setTotalEvents] = useState(0);
  const [selectedEvent, setSelectedEvent] = useState<TimelineEvent | null>(null);
  const [selectedTypes, setSelectedTypes] = useState<Set<string>>(new Set());
  const [userFilter, setUserFilter] = useState("");
  const [agentFilter, setAgentFilter] = useState("");
  const [textSearch, setTextSearch] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [showSidebar, setShowSidebar] = useState(true);
  const [mobileFilterOpen, setMobileFilterOpen] = useState(false);

  const loadTimeline = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: String(page) });
      const activeTypes = [...selectedTypes];
      if (activeTypes.length > 0) params.set("type", activeTypes.join(","));
      if (userFilter) params.set("user", userFilter);
      if (agentFilter) params.set("agent", agentFilter);
      if (dateFrom) params.set("from", dateFrom);
      if (dateTo) params.set("to", dateTo);
      const data = await api.get<{ events: TimelineEvent[]; total: number; total_pages?: number }>(`/api/timeline/data?${params}`);
      setEvents(data.events || []);
      setTotalEvents(data.total || 0);
      setTotalPages(data.total_pages || Math.ceil((data.total || 0) / 50));
    } catch {
      setEvents([]);
      setTotalEvents(0);
      setTotalPages(1);
    } finally {
      setLoading(false);
    }
  }, [page, selectedTypes, userFilter, agentFilter, dateFrom, dateTo]);

  useEffect(() => { loadTimeline(); }, [loadTimeline]);
  useVisibleInterval(() => { loadTimeline(); }, POLL_INTERVAL);

  const toggleType = (type: string) => {
    setSelectedTypes(prev => {
      const next = new Set(prev);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });
    setPage(1);
  };

  const handleExport = async () => {
    try {
      const activeTypes = [...selectedTypes];
      const dlParams: Record<string, string> = { format: "csv" };
      if (activeTypes.length > 0) dlParams.type = activeTypes.join(",");
      if (userFilter) dlParams.user = userFilter;
      if (agentFilter) dlParams.agent = agentFilter;
      const { blob } = await api.download("/api/timeline/export", dlParams);
      downloadBlob(blob, "timeline-export.csv");
    } catch { toast.error(t("timeline.toast.export_failed")); }
  };

  const clearFilters = () => {
    setSelectedTypes(new Set());
    setUserFilter("");
    setAgentFilter("");
    setTextSearch("");
    setDateFrom("");
    setDateTo("");
    setPage(1);
  };

  const filteredEvents = textSearch
    ? events.filter(e => {
        const title = (e.title || "").toLowerCase();
        const desc = (e.description || "").toLowerCase();
        const srch = textSearch.toLowerCase();
        return title.includes(srch) || desc.includes(srch);
      })
    : events;

  const hasActiveFilters = selectedTypes.size > 0 || !!dateFrom || !!dateTo || !!userFilter || !!agentFilter;

  const filterProps = {
    selectedTypes, onToggleType: toggleType,
    dateFrom, onDateFromChange: (v: string) => { setDateFrom(v); setPage(1); },
    dateTo, onDateToChange: (v: string) => { setDateTo(v); setPage(1); },
    userFilter, onUserFilterChange: (v: string) => { setUserFilter(v); setPage(1); },
    hasActiveFilters, onClearFilters: clearFilters,
  };

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("timeline.page_title")} subtitle={t("timeline.page_subtitle")}>
        <div className="lg:hidden">
          <Button variant="outline" size="icon" className="min-w-[44px] min-h-[44px] gap-2" onClick={() => setMobileFilterOpen(true)}>
            <Menu className="w-4 h-4" />
          </Button>
        </div>
        <Button variant="outline" onClick={() => setShowSidebar(!showSidebar)} className="gap-2 max-lg:hidden">
          <PanelLeftClose className="w-4 h-4" />
          <span>{showSidebar ? "Hide" : "Show"} Filters</span>
        </Button>
        <Button variant="outline" onClick={handleExport} className="gap-2">
          <Download className="w-4 h-4" />
          <span>Export</span>
        </Button>
        <Button onClick={loadTimeline} className="gap-2">
          <RefreshCw className="w-4 h-4" />
          <span>Refresh</span>
        </Button>
      </PageHeader>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
        {showSidebar && (
          <div className="lg:col-span-1 max-lg:hidden">
            <TimelineFilters {...filterProps} />
          </div>
        )}

        <div className={showSidebar ? "lg:col-span-3" : "lg:col-span-4"}>
          <TimelineContent
            filteredEvents={filteredEvents}
            loading={loading}
            page={page}
            totalEvents={totalEvents}
            textSearch={textSearch}
            onTextSearchChange={setTextSearch}
            onPageChange={setPage}
            onSelectEvent={setSelectedEvent}
          />
        </div>
      </div>

      <EventDetailDialog event={selectedEvent} onClose={() => setSelectedEvent(null)} />

      <Sheet open={mobileFilterOpen} onOpenChange={setMobileFilterOpen}>
        <SheetContent side="left" className="w-[300px] sm:w-[320px] overflow-y-auto">
          <div className="pt-8">
            <TimelineFilters {...filterProps} />
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}
