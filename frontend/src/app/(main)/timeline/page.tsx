"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { downloadBlob } from "@/lib/download";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { EmptyState, PageHeader } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { Bug, ChevronLeft, ChevronRight, Circle, Clock, Download, ExternalLink, Key, Link as LinkIcon, ListChecks, Menu, PanelLeftClose, RefreshCw, Search, Server, TriangleAlert, User, X } from "lucide-react";

interface TimelineEvent {
  id?: string;
  ID?: string;
  timestamp?: string;
  Timestamp?: string;
  type?: string;
  Type?: string;
  title?: string;
  Title?: string;
  description?: string;
  Description?: string;
  username?: string;
  Username?: string;
  agent_id?: string;
  AgentID?: string;
  url?: string;
}

const EVENT_TYPES = ["agent_online", "task", "credential", "user", "system", "alert"] as const;

const EVENT_COLORS: Record<string, { dot: string; bg: string; text: string; icon: React.ReactNode }> = {
  agent_online: { dot: "bg-emerald-500", bg: "bg-emerald-50 dark:bg-emerald-900/30", text: "text-emerald-600 dark:text-emerald-400", icon: <Bug className="w-2.5 h-2.5" /> },
  task: { dot: "bg-blue-500", bg: "bg-blue-50 dark:bg-blue-900/30", text: "text-blue-600 dark:text-blue-400", icon: <ListChecks className="w-2.5 h-2.5" /> },
  credential: { dot: "bg-amber-500", bg: "bg-amber-50 dark:bg-amber-900/30", text: "text-amber-600 dark:text-amber-400", icon: <Key className="w-2.5 h-2.5" /> },
  user: { dot: "bg-purple-500", bg: "bg-purple-50 dark:bg-purple-900/30", text: "text-purple-600 dark:text-purple-400", icon: <User className="w-2.5 h-2.5" /> },
  system: { dot: "bg-purple-500", bg: "bg-purple-50 dark:bg-purple-900/30", text: "text-purple-600 dark:text-purple-400", icon: <Server className="w-2.5 h-2.5" /> },
  alert: { dot: "bg-red-500", bg: "bg-red-50 dark:bg-red-900/30", text: "text-red-600 dark:text-red-400", icon: <TriangleAlert className="w-2.5 h-2.5" /> },
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
      const data = await api.json<{ events: TimelineEvent[]; total: number; total_pages?: number }>(`/api/timeline/data?${params}`);
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
    } catch { toast.error("Failed to export timeline"); }
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

  const getEventId = (e: TimelineEvent, i: number) => e.id || String(i);
  const getEventTime = (e: TimelineEvent) => e.timestamp || "";
  const getEventType = (e: TimelineEvent) => (e.type || "").toLowerCase();
  const getEventTitle = (e: TimelineEvent) => e.title || "";
  const getEventDesc = (e: TimelineEvent) => e.description || "";
  const getEventUser = (e: TimelineEvent) => e.username || "";
  const getEventAgent = (e: TimelineEvent) => e.agent_id || "";
  const getEventUrl = (e: TimelineEvent) => e.url || "";

  const filteredEvents = textSearch
    ? events.filter(e => {
        const t = getEventTitle(e).toLowerCase();
        const d = getEventDesc(e).toLowerCase();
        const srch = textSearch.toLowerCase();
        return t.includes(srch) || d.includes(srch);
      })
    : events;

  const colorFor = (type: string) => EVENT_COLORS[type] || { dot: "bg-muted-foreground", bg: "bg-muted", text: "text-muted-foreground", icon: <Circle className="w-2.5 h-2.5" /> };

  const variantFor = (type: string): "success" | "destructive" | "warning" | "outline" => {
    if (type === "agent_online") return "success";
    if (type === "alert") return "destructive";
    if (type === "credential") return "warning";
    return "outline";
  };

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title="Operation Timeline" subtitle="Monitor all operations and security events in real-time">
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
          <div className="lg:col-span-1 space-y-3 max-lg:hidden">
            <Card className="p-4">
              <h3 className="text-xs font-semibold text-muted-foreground mb-3 uppercase tracking-wider">Event Types</h3>
              <div className="space-y-2">
                {EVENT_TYPES.map(type => {
                  const color = colorFor(type);
                  const isSelected = selectedTypes.has(type);
                  return (
                    <Label key={type} className="flex items-center gap-2 cursor-pointer group">
                      <Checkbox
                        checked={isSelected}
                        onCheckedChange={() => toggleType(type)}
                        aria-label={`Filter by ${type}`}
                      />
                      <span className={`w-2.5 h-2.5 rounded-full ${color.dot}`}></span>
                      <span className="text-sm text-muted-foreground group-hover:text-foreground transition-colors">
                        {type.replace("_", " ").replace(/^\w/, c => c.toUpperCase())}
                      </span>
                    </Label>
                  );
                })}
              </div>
            </Card>

            <Card className="p-4 space-y-3">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Date Range</h3>
              <div>
                <Label className="text-xs mb-1">Start</Label>
                <Input type="date" value={dateFrom} onChange={e => { setDateFrom(e.target.value); setPage(1); }} aria-label="Start date" />
              </div>
              <div>
                <Label className="text-xs mb-1">End</Label>
                <Input type="date" value={dateTo} onChange={e => { setDateTo(e.target.value); setPage(1); }} aria-label="End date" />
              </div>
            </Card>

            <Card className="p-4 space-y-3">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">User Filter</h3>
              <div className="relative">
                <User className="w-4 h-4" />
                <Input placeholder="Filter users..." value={userFilter} onChange={e => { setUserFilter(e.target.value); setPage(1); }} className="pl-9" aria-label="Filter by user" />
              </div>
            </Card>

            {(selectedTypes.size > 0 || dateFrom || dateTo || userFilter || agentFilter) && (
              <Button variant="outline" onClick={clearFilters} className="w-full border-destructive/20 text-destructive hover:bg-destructive/10 gap-2">
                <X className="w-4 h-4" />
                <span>Clear All Filters</span>
              </Button>
            )}
          </div>
        )}

        <div className={showSidebar ? "lg:col-span-3" : "lg:col-span-4"}>
          <div className="flex flex-wrap items-center gap-2 mb-4">
            <div className="flex-1 min-w-[200px] relative">
              <Search className="w-4 h-4" />
              <Input placeholder="Search event content..." value={textSearch} onChange={e => setTextSearch(e.target.value)} className="pl-9" aria-label="Search event content" />
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
                <div className="text-[10px] text-muted-foreground uppercase tracking-wider mt-0.5">Total Events</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-emerald-600">{filteredEvents.filter(e => getEventType(e) === "agent_online").length}</div>
                <div className="text-[10px] text-muted-foreground uppercase tracking-wider mt-0.5">Agent Events</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-amber-600">{filteredEvents.filter(e => getEventType(e) === "credential").length}</div>
                <div className="text-[10px] text-muted-foreground uppercase tracking-wider mt-0.5">Credential Events</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-red-600">{filteredEvents.filter(e => getEventType(e) === "alert").length}</div>
                <div className="text-[10px] text-muted-foreground uppercase tracking-wider mt-0.5">Alerts</div>
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
                <EmptyState icon={Clock} title="No timeline events" message="Timeline events will appear here after you start performing operations." />
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
                        onClick={() => setSelectedEvent(e)}
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
                          {url && <span className="text-[10px] text-indigo-500 dark:text-indigo-400 inline-block mt-1"><LinkIcon className="w-4 h-4" />View Details</span>}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </Card>

          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-4 px-2">
              <span className="text-xs text-muted-foreground">
                Page {page} of {totalPages}
              </span>
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" onClick={() => setPage(Math.max(1, page - 1))} disabled={page <= 1} aria-label="Previous page">
                  <ChevronLeft className="w-4 h-4" />
                </Button>
                {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                  const start = Math.max(1, Math.min(page - 2, totalPages - 4));
                  const p = start + i;
                  if (p > totalPages) return null;
                  return (
                    <Button
                      key={p}
                      variant={p === page ? "default" : "outline"}
                      size="sm"
                      onClick={() => setPage(p)}
                      className="w-8 h-8 p-0"
                    >{p}</Button>
                  );
                })}
                <Button variant="outline" size="sm" onClick={() => setPage(Math.min(totalPages, page + 1))} disabled={page >= totalPages} aria-label="Next page">
                  <ChevronRight className="w-4 h-4" />
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>

      <Dialog open={!!selectedEvent} onOpenChange={() => setSelectedEvent(null)}>
        <DialogContent className="sm:max-w-lg max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Event Details</DialogTitle>
          </DialogHeader>
          {selectedEvent && (<div className="space-y-4">
            <div>
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.time")}</span>
              <p className="text-sm font-mono mt-0.5">{getEventTime(selectedEvent)}</p>
            </div>
            <div>
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.type")}</span>
              <p className="text-sm mt-0.5">
                <Badge variant={variantFor(getEventType(selectedEvent))}>
                  <span className={`w-1.5 h-1.5 rounded-full ${colorFor(getEventType(selectedEvent)).dot}`}></span>
                  {getEventType(selectedEvent)}
                </Badge>
              </p>
            </div>
            <div>
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.title")}</span>
              <p className="text-sm mt-0.5">{getEventTitle(selectedEvent)}</p>
            </div>
            {getEventDesc(selectedEvent) && (
              <div>
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.description")}</span>
                <p className="text-sm text-muted-foreground mt-0.5">{getEventDesc(selectedEvent)}</p>
              </div>
            )}
            {getEventUser(selectedEvent) && (
              <div>
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.user")}</span>
                <p className="text-sm mt-0.5">{getEventUser(selectedEvent)}</p>
              </div>
            )}
            {getEventAgent(selectedEvent) && (
              <div>
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Agent ID</span>
                <p className="text-sm font-mono mt-0.5">{getEventAgent(selectedEvent)}</p>
              </div>
            )}
            {getEventUrl(selectedEvent) && (
              <div className="pt-2">
                <Link href={getEventUrl(selectedEvent)}>
                  <Button className="gap-2">
                    <ExternalLink className="w-4 h-4" />
                    <span>View Related Object</span>
                  </Button>
                </Link>
              </div>
            )}
          </div>)}
        </DialogContent>
      </Dialog>

      <Sheet open={mobileFilterOpen} onOpenChange={setMobileFilterOpen}>
        <SheetContent side="left" className="w-[300px] sm:w-[320px] overflow-y-auto">
          <div className="space-y-3 pt-8">
            <Card className="p-4">
              <h3 className="text-xs font-semibold text-muted-foreground mb-3 uppercase tracking-wider">Event Types</h3>
              <div className="space-y-2">
                {EVENT_TYPES.map(type => {
                  const color = colorFor(type);
                  const isSelected = selectedTypes.has(type);
                  return (
                    <Label key={type} className="flex items-center gap-2 cursor-pointer group">
                      <Checkbox
                        checked={isSelected}
                        onCheckedChange={() => toggleType(type)}
                        aria-label={`Filter by ${type}`}
                      />
                      <span className={`w-2.5 h-2.5 rounded-full ${color.dot}`}></span>
                      <span className="text-sm text-muted-foreground group-hover:text-foreground transition-colors">
                        {type.replace("_", " ").replace(/^\w/, c => c.toUpperCase())}
                      </span>
                    </Label>
                  );
                })}
              </div>
            </Card>

            <Card className="p-4 space-y-3">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Date Range</h3>
              <div>
                <Label className="text-xs mb-1">Start</Label>
                <Input type="date" value={dateFrom} onChange={e => { setDateFrom(e.target.value); setPage(1); }} aria-label="Start date" />
              </div>
              <div>
                <Label className="text-xs mb-1">End</Label>
                <Input type="date" value={dateTo} onChange={e => { setDateTo(e.target.value); setPage(1); }} aria-label="End date" />
              </div>
            </Card>

            <Card className="p-4 space-y-3">
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">User Filter</h3>
              <div className="relative">
                <User className="w-4 h-4" />
                <Input placeholder="Filter users..." value={userFilter} onChange={e => { setUserFilter(e.target.value); setPage(1); }} className="pl-9" aria-label="Filter by user" />
              </div>
            </Card>

            {(selectedTypes.size > 0 || dateFrom || dateTo || userFilter || agentFilter) && (
              <Button variant="outline" onClick={clearFilters} className="w-full border-destructive/20 text-destructive hover:bg-destructive/10 gap-2">
                <X className="w-4 h-4" />
                <span>Clear All Filters</span>
              </Button>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}
