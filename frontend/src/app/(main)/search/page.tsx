"use client";

import { useState, useEffect, useCallback } from "react";
import { API_BASE } from "@/lib/constants";

type SearchType = "all" | "agents" | "credentials" | "tasks" | "listeners";

interface SearchResult {
  type: string;
  id: string;
  name: string;
  detail: string;
  status?: string;
  url: string;
}

interface GroupedResults {
  type: string;
  label: string;
  icon: string;
  items: SearchResult[];
}

const RECENT_SEARCHES_KEY = "forgec2_recent_searches";
const MAX_RECENT = 10;

export default function SearchPage() {
  const [query, setQuery] = useState("");
  const [activeType, setActiveType] = useState<SearchType>("all");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const [recentSearches, setRecentSearches] = useState<string[]>([]);
  const [showRecent, setShowRecent] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [statusFilter, setStatusFilter] = useState("");

  useEffect(() => {
    try {
      const stored = localStorage.getItem(RECENT_SEARCHES_KEY);
      if (stored) Promise.resolve().then(() => setRecentSearches(JSON.parse(stored)));
    } catch (e) { console.error("Search: load recent searches failed", e); }
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "k") {
        e.preventDefault();
        const input = document.getElementById("search-input") as HTMLInputElement;
        input?.focus();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => {
    Promise.resolve().then(() => {
      if (query.length < 2) {
        setResults([]);
        setSearched(false);
        return;
      }
    });
    const timer = setTimeout(async () => {
      setLoading(true);
      setSearched(true);
      try {
        const params = new URLSearchParams({ p: "/api/search", format: "json" });
        params.set("q", query);
        if (activeType !== "all") params.set("type", activeType);
        if (dateFrom) params.set("from", dateFrom);
        if (dateTo) params.set("to", dateTo);
        if (statusFilter) params.set("status", statusFilter);
        const res = await fetch(`${API_BASE}?${params}`);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data = await res.json();
        const r = data.results || data.Results || data || [];
        setResults(Array.isArray(r) ? r : []);
      } catch {
        setResults([]);
      }
      setLoading(false);
    }, 300);
    return () => clearTimeout(timer);
  }, [query, activeType, dateFrom, dateTo, statusFilter]);

  const saveRecent = useCallback((q: string) => {
    if (!q || q.length < 2) return;
    const updated = [q, ...recentSearches.filter(s => s !== q)].slice(0, MAX_RECENT);
    setRecentSearches(updated);
    try { localStorage.setItem(RECENT_SEARCHES_KEY, JSON.stringify(updated)); } catch {}
  }, [recentSearches]);

  const clearRecent = () => {
    setRecentSearches([]);
    try { localStorage.removeItem(RECENT_SEARCHES_KEY); } catch {}
  };

  const highlightMatch = (text: string) => {
    if (!text) return "";
    if (!query || query.length < 2) return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const regex = new RegExp(`(${escaped})`, "gi");
    const safe = text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
    return safe.replace(regex, "<mark class=\"bg-amber-200 dark:bg-amber-800 text-slate-900 dark:text-amber-100 rounded px-0.5\">$1</mark>");
  };

  const filteredResults = results.filter((r) => {
    if (activeType === "all") return true;
    return r.type === activeType || r.type === activeType.slice(0, -1);
  });

  const groupedResults: GroupedResults[] = [];
  const typeGroups: Record<string, { label: string; icon: string }> = {
    agents: { label: "Agents", icon: "fa-bug" },
    agent: { label: "Agents", icon: "fa-bug" },
    credentials: { label: "凭据", icon: "fa-key" },
    credential: { label: "凭据", icon: "fa-key" },
    tasks: { label: "任务", icon: "fa-history" },
    task: { label: "任务", icon: "fa-history" },
    listeners: { label: "Listeners", icon: "fa-tower-broadcast" },
    listener: { label: "Listeners", icon: "fa-tower-broadcast" },
  };

  if (activeType === "all") {
    const types = [...new Set(filteredResults.map(r => r.type))];
    types.forEach(t => {
      const group = typeGroups[t] || { label: t, icon: "fa-circle" };
      const items = filteredResults.filter(r => r.type === t);
      groupedResults.push({ type: t, label: group.label, icon: group.icon, items });
    });
  } else {
    const t = activeType;
    const group = typeGroups[t] || { label: t, icon: "fa-circle" };
    groupedResults.push({ type: t, label: group.label, icon: group.icon, items: filteredResults });
  }

  const getStatusDot = (status: string) => {
    switch (status?.toLowerCase()) {
      case "online": return "w-1.5 h-1.5 rounded-full bg-emerald-500";
      case "offline": return "w-1.5 h-1.5 rounded-full bg-red-500";
      case "completed": return "w-1.5 h-1.5 rounded-full bg-emerald-500";
      case "pending": return "w-1.5 h-1.5 rounded-full bg-amber-500";
      case "running": return "w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse";
      default: return "w-1.5 h-1.5 rounded-full bg-slate-400";
    }
  };

  const getTypeIcon = (type: string) => {
    switch (type?.toLowerCase()) {
      case "agent": case "agents": return "fa-bug";
      case "credential": case "credentials": return "fa-key";
      case "task": case "tasks": return "fa-history";
      case "listener": case "listeners": return "fa-tower-broadcast";
      default: return "fa-circle";
    }
  };

  const getFilteredCount = (key: string) => {
    if (key === "all") return filteredResults.length;
    return filteredResults.filter(r => r.type === key || r.type === key.slice(0, -1)).length;
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    saveRecent(query);
  };

  const handleRecentClick = (term: string) => {
    setQuery(term);
    setShowRecent(false);
  };

  const emptyStateMessages: Record<string, { icon: string; title: string; subtitle: string }> = {
    all: { icon: "fa-search", title: "开始搜索", subtitle: "输入关键词搜索Agent、凭据、任务、Listener" },
    agents: { icon: "fa-bug", title: "未找到 Agent", subtitle: "没有匹配的 Agent 记录" },
    credentials: { icon: "fa-key", title: "未找到凭证", subtitle: "没有匹配的凭据记录" },
    tasks: { icon: "fa-history", title: "未找到任务", subtitle: "没有匹配的任务记录" },
    listeners: { icon: "fa-tower-broadcast", title: "未找到Listener", subtitle: "没有匹配的Listener记录" },
  };

  const totalResults = filteredResults.length;
  const perTypeStats = Object.entries(typeGroups)
    .filter(([k]) => !typeGroups[k.slice(0, -1)])
    .map(([k, g]) => ({ key: k, label: g.label, count: getFilteredCount(k) }))
    .filter(s => s.count > 0 || activeType === "all");

  return (
    <div className="max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-1">全局搜索</h1>
        <p className="text-slate-500 dark:text-slate-400 text-sm">跨类型搜索 Agents、凭据、任务、Listeners</p>
      </div>

      <div className="relative mb-4">
        <i className="fa-solid fa-search absolute left-4 top-1/2 -translate-y-1/2 text-slate-400"></i>
        <form onSubmit={handleSearchSubmit}>
          <input
            id="search-input"
            type="text"
            value={query}
            onChange={(e) => { setQuery(e.target.value); setShowRecent(e.target.value.length === 0); }}
            onFocus={() => setShowRecent(query.length === 0)}
            onBlur={() => setTimeout(() => setShowRecent(false), 200)}
            placeholder="输入至少 2 个字符开始搜索..."
            className="w-full ui-card pl-11 pr-16 h-12 text-sm focus:outline-none focus:border-indigo-500 shadow-sm"
            autoComplete="off"
          />
        </form>
        <div className="absolute right-4 top-1/2 -translate-y-1/2 flex items-center gap-2">
          {query.length > 0 && (
            <button onClick={() => { setQuery(""); setSearched(false); setResults([]); }} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300">
              <i className="fa-solid fa-xmark"></i>
            </button>
          )}
          <kbd className="hidden sm:inline-flex items-center gap-0.5 px-1.5 py-0.5 bg-slate-100 dark:bg-slate-700 border border-[var(--border)] rounded text-[10px] text-slate-400 font-mono">
            Ctrl K
          </kbd>
        </div>
      </div>

      {showRecent && recentSearches.length > 0 && (
        <div className="relative mb-4 z-10">
          <div className="absolute top-0 left-0 right-0 ui-card shadow-lg overflow-hidden">
            <div className="flex items-center justify-between px-4 py-2 border-b border-slate-100 dark:border-slate-700">
              <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
                <i className="fa-solid fa-clock-rotate-left mr-1"></i>最近搜索
              </span>
              <button onClick={clearRecent} className="text-[10px] text-slate-400 hover:text-red-500">清除</button>
            </div>
            {recentSearches.map((term, i) => (
              <button key={i} onClick={() => handleRecentClick(term)}
                className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors text-left">
                <i className="fa-solid fa-clock-rotate-left text-xs text-slate-400"></i>
                <span className="text-sm text-[var(--text-secondary)]">{term}</span>
                <i className="fa-solid fa-arrow-turn-up text-xs text-slate-300 ml-auto -rotate-90"></i>
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="flex gap-2 mb-4 flex-wrap items-center">
        {([
          ["all", "全部", "fa-layer-group"],
          ["agents", "Agents", "fa-bug"],
          ["credentials", "凭据", "fa-key"],
          ["tasks", "任务", "fa-history"],
          ["listeners", "Listeners", "fa-tower-broadcast"],
        ] as const).map(([key, label, icon]) => {
          const count = getFilteredCount(key);
          return (
            <button
              key={key}
              onClick={() => setActiveType(key)}
              className={`px-4 py-2 rounded-lg text-xs font-medium transition-colors flex items-center gap-1.5 ${
                activeType === key
                  ? "bg-indigo-600 text-white shadow-sm"
                  : "bg-[var(--card-bg)] border border-[var(--border)] text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700"
              }`}
            >
              <i className={`fa-solid ${icon}`}></i>
              <span>{label}</span>
              {searched && !loading && count > 0 && (
                <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${activeType === key ? "bg-white/20 text-white" : "bg-slate-100 dark:bg-slate-700 text-slate-500"}`}>{count}</span>
              )}
            </button>
          );
        })}
        <button
          onClick={() => setShowAdvanced(!showAdvanced)}
          className={`ml-auto px-3 py-2 rounded-lg text-xs font-medium transition-colors flex items-center gap-1.5 border ${
            showAdvanced
              ? "border-indigo-500 text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/20"
              : "bg-[var(--card-bg)] border-[var(--border)] text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700"
          }`}
        >
          <i className="fa-solid fa-sliders"></i>
          <span>高级筛选</span>
          <i className={`fa-solid fa-chevron-down text-[10px] transition-transform ${showAdvanced ? "rotate-180" : ""}`}></i>
        </button>
      </div>

      {showAdvanced && (
        <div className="ui-card p-4 mb-4 shadow-sm space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">开始日期</label>
              <input
                type="date"
                value={dateFrom}
                onChange={(e) => setDateFrom(e.target.value)}
                className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-9 text-sm focus:outline-none focus:border-indigo-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">结束日期</label>
              <input
                type="date"
                value={dateTo}
                onChange={(e) => setDateTo(e.target.value)}
                className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-9 text-sm focus:outline-none focus:border-indigo-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-600 dark:text-slate-400 mb-1">状态筛选</label>
              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-9 text-sm focus:outline-none focus:border-indigo-500"
              >
                <option value="">全部状态</option>
                <option value="online">在线</option>
                <option value="offline">离线</option>
                <option value="pending">等待</option>
                <option value="running">运行</option>
                <option value="completed">已完成</option>
              </select>
            </div>
          </div>
          {(dateFrom || dateTo || statusFilter) && (
            <button
              onClick={() => { setDateFrom(""); setDateTo(""); setStatusFilter(""); }}
              className="text-xs text-slate-500 hover:text-red-500 transition-colors"
            >
              <i className="fa-solid fa-xmark mr-1"></i>清除筛选条件            </button>
          )}
        </div>
      )}

      {searched && !loading && totalResults > 0 && activeType === "all" && (
        <div className="flex items-center gap-4 mb-4 px-1">
          <span className="text-xs text-slate-500 dark:text-slate-400">
            找到 <span className="font-semibold text-slate-700 dark:text-slate-200">{totalResults}</span> 条结果          </span>
          <div className="flex items-center gap-2 flex-wrap">
            {perTypeStats.map(s => (
              <span key={s.key} className="text-[10px] px-2 py-0.5 bg-slate-100 dark:bg-slate-700 rounded-full text-slate-600 dark:text-slate-300">
                {s.label}: {s.count}
              </span>
            ))}
          </div>
        </div>
      )}

      <div>
        {loading && (
          <div className="text-center py-12 text-slate-400">
            <div className="animate-spin w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full mx-auto mb-3"></div>
            <p className="text-sm">搜索中...</p>
          </div>
        )}

        {!loading && searched && filteredResults.length === 0 && (
          <div className="text-center py-16 text-slate-400">
            <i className={`fa-solid ${emptyStateMessages[activeType]?.icon || "fa-search"} text-4xl mb-4 text-slate-300 dark:text-slate-600`}></i>
            <p className="text-sm font-medium">{emptyStateMessages[activeType]?.title || "未找到结果"}</p>
            <p className="text-xs mt-1">{emptyStateMessages[activeType]?.subtitle || `没有匹配 "${query}" 的结果`}</p>
          </div>
        )}

        {!loading && !searched && (
          <div className="text-center py-16 text-slate-400">
            <i className="fa-solid fa-search text-4xl mb-4 text-slate-300 dark:text-slate-600"></i>
            <p className="text-sm font-medium">开始搜索</p>
            <p className="text-xs mt-1">输入关键词搜索全部资源</p>
            <p className="text-xs mt-2"><kbd className="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-700 border border-[var(--border)] rounded text-[10px] font-mono">Ctrl+K</kbd> 快速搜索</p>
          </div>
        )}

        {!loading && groupedResults.length > 0 && (
          <div className="space-y-4">
            {groupedResults.map((group, gi) => (
              <div key={gi} className="ui-card overflow-hidden shadow-sm">
                <div className="flex items-center gap-2 px-4 py-2.5 bg-slate-50 dark:bg-slate-700/30 border-b border-slate-100 dark:border-slate-700">
                  <i className={`fa-solid ${group.icon} text-xs text-slate-500`}></i>
                  <span className="text-xs font-semibold text-slate-600 dark:text-slate-400">{group.label}</span>
                  <span className="text-[10px] px-1.5 py-0.5 bg-slate-200 dark:bg-slate-600 text-slate-600 dark:text-slate-300 rounded-full font-medium">{group.items.length}</span>
                </div>
                {group.items.map((item, i) => (
                  <a key={i} href={item.url || "#"} className="flex items-center justify-between px-4 py-3 border-b border-slate-100 dark:border-slate-700 last:border-0 hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                    <div className="flex items-center gap-3 min-w-0">
                      <div className="w-8 h-8 rounded-lg bg-slate-100 dark:bg-slate-700 flex items-center justify-center shrink-0">
                        <i className={`fa-solid ${getTypeIcon(item.type)} text-xs text-slate-500`}></i>
                      </div>
                      <div className="min-w-0">
                        <div className="font-medium text-slate-900 dark:text-slate-100 text-sm truncate" dangerouslySetInnerHTML={{ __html: highlightMatch(item.name) }}></div>
                        <div className="text-xs text-slate-500 dark:text-slate-400 truncate" dangerouslySetInnerHTML={{ __html: highlightMatch(item.detail) }}></div>
                      </div>
                    </div>
                    {item.status && (
                      <div className="flex items-center gap-1.5 shrink-0 ml-3">
                        <span className={getStatusDot(item.status)}></span>
                        <span className="text-[10px] text-slate-400 capitalize">{item.status}</span>
                      </div>
                    )}
                  </a>
                ))}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
