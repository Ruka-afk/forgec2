"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";

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
type EventType = typeof EVENT_TYPES[number];

const EVENT_COLORS: Record<string, { dot: string; bg: string; text: string; icon: string }> = {
  agent_online: { dot: "bg-emerald-500", bg: "bg-emerald-50 dark:bg-emerald-900/30", text: "text-emerald-600 dark:text-emerald-400", icon: "fa-bug" },
  task: { dot: "bg-blue-500", bg: "bg-blue-50 dark:bg-blue-900/30", text: "text-blue-600 dark:text-blue-400", icon: "fa-list-check" },
  credential: { dot: "bg-amber-500", bg: "bg-amber-50 dark:bg-amber-900/30", text: "text-amber-600 dark:text-amber-400", icon: "fa-key" },
  user: { dot: "bg-purple-500", bg: "bg-purple-50 dark:bg-purple-900/30", text: "text-purple-600 dark:text-purple-400", icon: "fa-user" },
  system: { dot: "bg-purple-500", bg: "bg-purple-50 dark:bg-purple-900/30", text: "text-purple-600 dark:text-purple-400", icon: "fa-server" },
  alert: { dot: "bg-red-500", bg: "bg-red-50 dark:bg-red-900/30", text: "text-red-600 dark:text-red-400", icon: "fa-triangle-exclamation" },
};

const POLL_INTERVAL = 10000;

export default function TimelinePage() {
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

  const loadTimeline = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        p: "/api/timeline/data",
        format: "json",
        page: String(page),
      });
      const activeTypes = [...selectedTypes];
      if (activeTypes.length > 0) params.set("type", activeTypes.join(","));
      if (userFilter) params.set("user", userFilter);
      if (agentFilter) params.set("agent", agentFilter);
      if (dateFrom) params.set("from", dateFrom);
      if (dateTo) params.set("to", dateTo);
      const resp = await fetch(`${API_BASE}?${params}`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      setEvents(data.events || data.Events || data.timeline || []);
      setTotalEvents(data.total || data.Total || 0);
      setTotalPages(data.total_pages || data.TotalPages || Math.ceil((data.total || data.Total || 0) / 50));
    } catch {
      setEvents([]);
      setTotalEvents(0);
      setTotalPages(1);
    } finally {
      setLoading(false);
    }
  }, [page, selectedTypes, userFilter, agentFilter, dateFrom, dateTo]);

  useEffect(() => { Promise.resolve().then(() => loadTimeline()); }, [loadTimeline]);

  useEffect(() => {
    Promise.resolve().then(() => {
      const interval = setInterval(() => { loadTimeline(); }, POLL_INTERVAL);
      return () => clearInterval(interval);
    });
  }, [loadTimeline]);

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
      const params = new URLSearchParams({ p: "/api/timeline/export", format: "csv" });
      const activeTypes = [...selectedTypes];
      if (activeTypes.length > 0) params.set("type", activeTypes.join(","));
      if (userFilter) params.set("user", userFilter);
      if (agentFilter) params.set("agent", agentFilter);
      const res = await fetch(`${API_BASE}?${params}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "timeline-export.csv";
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) { console.error("Timeline: CSV export failed", e); }
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

  const getEventId = (e: TimelineEvent, i: number) => e.id || e.ID || String(i);
  const getEventTime = (e: TimelineEvent) => e.timestamp || e.Timestamp || "";
  const getEventType = (e: TimelineEvent) => (e.type || e.Type || "").toLowerCase();
  const getEventTitle = (e: TimelineEvent) => e.title || e.Title || "";
  const getEventDesc = (e: TimelineEvent) => e.description || e.Description || "";
  const getEventUser = (e: TimelineEvent) => e.username || e.Username || "";
  const getEventAgent = (e: TimelineEvent) => e.agent_id || e.AgentID || "";
  const getEventUrl = (e: TimelineEvent) => e.url || "";

  const filteredEvents = textSearch
    ? events.filter(e => {
        const t = getEventTitle(e).toLowerCase();
        const d = getEventDesc(e).toLowerCase();
        const srch = textSearch.toLowerCase();
        return t.includes(srch) || d.includes(srch);
      })
    : events;

  const colorFor = (type: string) => EVENT_COLORS[type] || { dot: "bg-slate-400", bg: "bg-slate-50 dark:bg-slate-700/30", text: "text-slate-500", icon: "fa-circle" };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">操作时间线</h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs sm:text-sm mt-1">实时监控所有操作和安全事件</p>
        </div>
        <div className="flex items-center gap-3">
          <button onClick={() => setShowSidebar(!showSidebar)} className="px-4 h-10 bg-[var(--card-bg)] border border-[var(--border)] hover:bg-slate-50 rounded-xl text-sm flex items-center gap-2 transition-colors">
            <i className="fa-solid fa-sidebar-flip"></i>
            <span className="text-[var(--text-secondary)]">{showSidebar ? "隐藏" : "显示"}筛选</span>
          </button>
          <button onClick={handleExport} className="px-4 h-10 bg-[var(--card-bg)] border border-[var(--border)] hover:bg-slate-50 rounded-xl text-sm flex items-center gap-2 transition-colors">
            <i className="fa-solid fa-download text-slate-600 dark:text-slate-400"></i>
            <span className="text-[var(--text-secondary)]">导出</span>
          </button>
          <button onClick={loadTimeline} className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm flex items-center gap-2 transition-colors">
            <i className="fa-solid fa-sync"></i>
            <span>刷新</span>
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
        {showSidebar && (
          <div className="lg:col-span-1 space-y-3">
            <div className="ui-card p-4 shadow-sm">
              <h3 className="text-xs font-semibold text-slate-600 dark:text-slate-400 mb-3 uppercase tracking-wider">事件类型</h3>
              <div className="space-y-2">
                {EVENT_TYPES.map(type => {
                  const color = colorFor(type);
                  const isSelected = selectedTypes.has(type);
                  return (
                    <label key={type} className="flex items-center gap-2 cursor-pointer group">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => toggleType(type)}
                        className="w-4 h-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                      />
                      <span className={`w-2.5 h-2.5 rounded-full ${color.dot}`}></span>
                      <span className="text-sm text-[var(--text-secondary)] group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">
                        {type.replace("_", " ").replace(/^\w/, c => c.toUpperCase())}
                      </span>
                    </label>
                  );
                })}
              </div>
            </div>

            <div className="ui-card p-4 shadow-sm space-y-3">
              <h3 className="text-xs font-semibold text-slate-600 dark:text-slate-400 uppercase tracking-wider">日期范围</h3>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">开始</label>
                <input type="date" value={dateFrom} onChange={e => { setDateFrom(e.target.value); setPage(1); }}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-9 text-sm focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">结束</label>
                <input type="date" value={dateTo} onChange={e => { setDateTo(e.target.value); setPage(1); }}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-9 text-sm focus:outline-none focus:border-indigo-500" />
              </div>
            </div>

            <div className="ui-card p-4 shadow-sm space-y-3">
              <h3 className="text-xs font-semibold text-slate-600 dark:text-slate-400 uppercase tracking-wider">用户筛选</h3>
              <div className="relative">
                <i className="fa-solid fa-user absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-sm"></i>
                <input type="text" placeholder="筛选用户..." value={userFilter} onChange={e => { setUserFilter(e.target.value); setPage(1); }}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-lg pl-9 pr-3 h-9 focus:outline-none focus:border-indigo-500" />
              </div>
            </div>

            {(selectedTypes.size > 0 || dateFrom || dateTo || userFilter || agentFilter) && (
              <button onClick={clearFilters} className="w-full px-4 h-10 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-600 dark:text-red-400 rounded-xl text-sm flex items-center justify-center gap-2 hover:bg-red-100 dark:hover:bg-red-900/30 transition-colors">
                <i className="fa-solid fa-xmark"></i>
                <span>清除全部筛选</span>
              </button>
            )}
          </div>
        )}

        <div className={showSidebar ? "lg:col-span-3" : "lg:col-span-4"}>
          <div className="flex flex-wrap items-center gap-2 mb-4">
            <div className="flex-1 min-w-[200px] relative">
              <i className="fa-solid fa-search absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-sm"></i>
              <input type="text" placeholder="搜索事件内容..." value={textSearch} onChange={e => setTextSearch(e.target.value)}
                className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl pl-9 pr-3 h-10 focus:outline-none focus:border-indigo-500 shadow-sm" />
            </div>
            <span className="text-xs text-slate-500 dark:text-slate-400">
              <i className="fa-solid fa-circle text-emerald-500 text-[8px] mr-1 animate-pulse"></i>
              实时事件流            </span>
          </div>

          <div className="ui-card p-4 mb-4 shadow-sm">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <div className="text-center">
                <div className="text-2xl font-bold text-slate-900 dark:text-slate-100">{totalEvents}</div>
                <div className="text-[10px] text-slate-500 uppercase tracking-wider mt-0.5">总事件</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-emerald-600">{filteredEvents.filter(e => getEventType(e) === "agent_online").length}</div>
                <div className="text-[10px] text-slate-500 uppercase tracking-wider mt-0.5">Agent 事件</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-amber-600">{filteredEvents.filter(e => getEventType(e) === "credential").length}</div>
                <div className="text-[10px] text-slate-500 uppercase tracking-wider mt-0.5">凭据事件</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-red-600">{filteredEvents.filter(e => getEventType(e) === "alert").length}</div>
                <div className="text-[10px] text-slate-500 uppercase tracking-wider mt-0.5">告警</div>
              </div>
            </div>
          </div>

          <div className="ui-card p-6 shadow-sm max-h-[65vh] overflow-y-auto">
            {loading ? (
              <div className="text-center py-12">
                <i className="fa-solid fa-circle-notch fa-spin text-4xl text-indigo-500 mb-3"></i>
                <p className="text-slate-500 text-sm">加载时间线中...</p>
              </div>
            ) : filteredEvents.length === 0 ? (
              <div className="text-center py-12">
                <i className="fa-solid fa-timeline text-6xl text-slate-300 dark:text-slate-600 mb-4"></i>
                <h3 className="text-lg font-medium text-[var(--text-secondary)] mb-2">暂无时间线事件</h3>
                <p className="text-slate-500 dark:text-slate-400 text-sm">开始执行操作后，操作时间线将在此处显示</p>
              </div>
            ) : (
              <div className="relative">
                <div className="absolute left-5 top-0 bottom-0 w-px bg-slate-200 dark:bg-slate-700"></div>
                <div className="space-y-6">
                  {filteredEvents.map((e, i) => {
                    const type = getEventType(e);
                    const color = colorFor(type);
                    const url = getEventUrl(e);
                    return (
                      <div
                        key={getEventId(e, i)}
                        onClick={() => setSelectedEvent(e)}
                        className="relative flex gap-4 pl-12 cursor-pointer group hover:bg-slate-50 dark:hover:bg-slate-700/30 -mx-2 px-2 py-2 rounded-lg transition-colors"
                      >
                        <div className={`absolute left-3 top-1 w-5 h-5 rounded-full ${color.bg} flex items-center justify-center ring-4 ring-white dark:ring-slate-800`}>
                          <i className={`fa-solid ${color.icon} text-[10px] ${color.text}`}></i>
                        </div>
                        <div className="flex-1 -mt-0.5">
                          <div className="flex items-center gap-2 mb-1 flex-wrap">
                            <span className="text-xs text-slate-400 dark:text-slate-500 font-mono">{getEventTime(e)}</span>
                            {getEventUser(e) && <span className="text-xs bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded text-slate-500 dark:text-slate-400">{getEventUser(e)}</span>}
                            {getEventAgent(e) && <span className="text-xs bg-indigo-50 dark:bg-indigo-900/30 px-1.5 py-0.5 rounded text-indigo-600 dark:text-indigo-400 font-mono">{getEventAgent(e)}</span>}
                            <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${color.bg} ${color.text} font-medium`}>{type}</span>
                          </div>
                          <p className="text-sm font-medium text-[var(--text-secondary)] group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">{getEventTitle(e)}</p>
                          {getEventDesc(e) && <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">{getEventDesc(e)}</p>}
                          {url && <span className="text-[10px] text-indigo-500 dark:text-indigo-400 inline-block mt-1"><i className="fa-solid fa-link mr-0.5"></i>查看详情</span>}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-4 px-2">
              <span className="text-xs text-slate-500 dark:text-slate-400">
                第 {page} / {totalPages} 页              </span>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPage(Math.max(1, page - 1))}
                  disabled={page <= 1}
                  className="px-3 py-1.5 ui-card rounded-lg text-xs text-slate-600 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors"
                >
                  <i className="fa-solid fa-chevron-left"></i>
                </button>
                {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                  const start = Math.max(1, Math.min(page - 2, totalPages - 4));
                  const p = start + i;
                  if (p > totalPages) return null;
                  return (
                    <button
                      key={p}
                      onClick={() => setPage(p)}
                      className={`w-8 h-8 rounded-lg text-xs font-medium transition-colors ${
                        p === page ? "bg-indigo-600 text-white" : "bg-[var(--card-bg)] border border-[var(--border)] text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700"
                      }`}
                    >{p}</button>
                  );
                })}
                <button
                  onClick={() => setPage(Math.min(totalPages, page + 1))}
                  disabled={page >= totalPages}
                  className="px-3 py-1.5 ui-card rounded-lg text-xs text-slate-600 dark:text-slate-300 disabled:opacity-40 disabled:cursor-not-allowed hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors"
                >
                  <i className="fa-solid fa-chevron-right"></i>
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {selectedEvent && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" onClick={() => setSelectedEvent(null)}>
          <div className="ui-card shadow-xl max-w-lg w-full max-h-[80vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)]">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">事件详情</h3>
              <button onClick={() => setSelectedEvent(null)} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300">
                <i className="fa-solid fa-xmark"></i>
              </button>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">时间</label>
                <p className="text-sm text-slate-900 dark:text-slate-100 font-mono mt-0.5">{getEventTime(selectedEvent)}</p>
              </div>
              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">类型</label>
                <p className="text-sm text-slate-900 dark:text-slate-100 mt-0.5">
                  <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${colorFor(getEventType(selectedEvent)).bg} ${colorFor(getEventType(selectedEvent)).text}`}>
                    <span className={`w-1.5 h-1.5 rounded-full ${colorFor(getEventType(selectedEvent)).dot}`}></span>
                    {getEventType(selectedEvent)}
                  </span>
                </p>
              </div>
              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">标题</label>
                <p className="text-sm text-slate-900 dark:text-slate-100 mt-0.5">{getEventTitle(selectedEvent)}</p>
              </div>
              {getEventDesc(selectedEvent) && (
                <div>
                  <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">描述</label>
                  <p className="text-sm text-[var(--text-secondary)] mt-0.5">{getEventDesc(selectedEvent)}</p>
                </div>
              )}
              {getEventUser(selectedEvent) && (
                <div>
                  <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">用户</label>
                  <p className="text-sm text-slate-900 dark:text-slate-100 mt-0.5">{getEventUser(selectedEvent)}</p>
                </div>
              )}
              {getEventAgent(selectedEvent) && (
                <div>
                  <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">Agent ID</label>
                  <p className="text-sm text-slate-900 dark:text-slate-100 font-mono mt-0.5">{getEventAgent(selectedEvent)}</p>
                </div>
              )}
              {getEventUrl(selectedEvent) && (
                <div className="pt-2">
                  <a href={getEventUrl(selectedEvent)} className="inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm hover:bg-indigo-700 transition-colors">
                    <i className="fa-solid fa-external-link-alt"></i>
                    <span>查看关联对象</span>
                  </a>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
