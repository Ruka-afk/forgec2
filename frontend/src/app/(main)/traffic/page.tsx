"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";
import { PageHeader, SearchInput, Pagination } from "@/components/UI";

interface TrafficEntry {
  id?: string;
  ID?: string;
  timestamp?: string;
  Timestamp?: string;
  method?: string;
  Method?: string;
  path?: string;
  Path?: string;
  source_ip?: string;
  SourceIP?: string;
  agent_id?: string;
  AgentID?: string;
  status_code?: number;
  StatusCode?: number;
  size?: number;
  Size?: number;
  latency?: number;
  Latency?: number;
  protocol?: string;
  Protocol?: string;
}

export default function TrafficPage() {
  const [entries, setEntries] = useState<TrafficEntry[]>([]);
  const [filteredEntries, setFilteredEntries] = useState<TrafficEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoScroll, setAutoScroll] = useState(false);
  const [sourceIpFilter, setSourceIpFilter] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const applyFilter = (data: TrafficEntry[], ip: string) => {
    if (!ip) return data;
    return data.filter(e => {
      const entryIp = e.source_ip || e.SourceIP || "";
      return entryIp.includes(ip);
    });
  };

  const loadTraffic = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/api/traffic&format=json`, { credentials: "include" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const newEntries = data.data || data.Entries || data.entries || data.Traffic || data.traffic || [];
      setEntries(newEntries);
      setFilteredEntries(applyFilter(newEntries, sourceIpFilter));
    } catch {
      setEntries([]);
      setFilteredEntries([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadTraffic()); }, [loadTraffic]);

  useEffect(() => {
    Promise.resolve().then(() => {
      if (autoRefresh) {
        intervalRef.current = setInterval(loadTraffic, 5000);
      } else {
        if (intervalRef.current) clearInterval(intervalRef.current);
      }
    });
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [autoRefresh, loadTraffic]);

  const applyFilterToEntries = useCallback((ip: string) => {
    setFilteredEntries(applyFilter(entries, ip));
  }, [entries]);

  useEffect(() => {
    Promise.resolve().then(() => {
      if (sourceIpFilter) {
        applyFilterToEntries(sourceIpFilter);
      } else {
        setFilteredEntries(entries);
      }
    });
  }, [sourceIpFilter, entries, applyFilterToEntries]);

  useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [filteredEntries, autoScroll]);

  const formatTime = (t: string) => {
    if (!t) return "-";
    try { return new Date(t).toLocaleTimeString(); } catch { return t; }
  };

  const clearLog = () => { setEntries([]); setFilteredEntries([]); };

  const sourceIps = [...new Set(entries.map(e => e.source_ip || e.SourceIP || "").filter(Boolean))];

  const totalRequests = entries.length;
  const beacons = entries.filter(e => { const p = e.protocol || e.Protocol || ""; return p.toLowerCase().includes("beacon"); }).length;
  const errors = entries.filter(e => { const s = e.status_code ?? e.StatusCode ?? 0; return s >= 400; }).length;
  const dataTransferred = entries.reduce((acc, e) => acc + (e.size ?? e.Size ?? 0), 0);

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(1) + " " + sizes[i];
  };

  const getMethodStyle = (method: string) => {
    const m = method.toUpperCase();
    if (m === "POST") return "bg-emerald-500 text-white";
    if (m === "GET") return "bg-blue-500 text-white";
    if (m === "BEACON") return "bg-purple-500 text-white";
    if (m === "PUT") return "bg-amber-500 text-white";
    if (m === "DELETE") return "bg-red-500 text-white";
    return "bg-slate-500 text-white";
  };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title="流量" subtitle="请求/响应日志 · C2 Beacon 通信记录">
        <div className="flex items-center gap-2 flex-wrap">
          <label className="flex items-center gap-x-2 text-sm text-slate-600 dark:text-slate-400 cursor-pointer">
            <input type="checkbox" checked={autoRefresh} onChange={e => setAutoRefresh(e.target.checked)} className="rounded border-[var(--border)] text-indigo-600 focus:ring-indigo-500" />
            自动刷新
          </label>
          <label className="flex items-center gap-x-2 text-sm text-slate-600 dark:text-slate-400 cursor-pointer">
            <input type="checkbox" checked={autoScroll} onChange={e => setAutoScroll(e.target.checked)} className="rounded border-[var(--border)] text-indigo-600 focus:ring-indigo-500" />
            自动滚动
          </label>
          <select value={sourceIpFilter} onChange={e => setSourceIpFilter(e.target.value)}
            className="ui-card px-3 py-2 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100">
            <option value="">所有 IP</option>
            {sourceIps.map(ip => (
              <option key={ip} value={ip}>{ip}</option>
            ))}
          </select>
          <button onClick={loadTraffic} className="px-4 h-11 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm font-medium transition-colors">
            <i className="fa-solid fa-sync mr-1"></i>
            <span>刷新</span>
          </button>
          <button onClick={clearLog} className="px-4 h-11 bg-red-600 hover:bg-red-700 text-white rounded-xl text-sm font-medium transition-colors">
            <i className="fa-solid fa-trash mr-1"></i>
            <span>清除</span>
          </button>
        </div>
      </PageHeader>

      <div className="ui-card p-4 mb-4">
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          <div className="text-center">
            <div className="text-xs text-slate-500 dark:text-slate-400 mb-1">总请求</div>
            <div className="text-xl font-bold text-indigo-600 dark:text-indigo-400">{totalRequests}</div>
          </div>
          <div className="text-center">
            <div className="text-xs text-slate-500 dark:text-slate-400 mb-1">Beacons</div>
            <div className="text-xl font-bold text-purple-600 dark:text-purple-400">{beacons}</div>
          </div>
          <div className="text-center">
            <div className="text-xs text-slate-500 dark:text-slate-400 mb-1">错误</div>
            <div className="text-xl font-bold text-red-600 dark:text-red-400">{errors}</div>
          </div>
          <div className="text-center">
            <div className="text-xs text-slate-500 dark:text-slate-400 mb-1">数据传输</div>
            <div className="text-xl font-bold text-emerald-600 dark:text-emerald-400">{formatBytes(dataTransferred)}</div>
          </div>
        </div>
      </div>

      <div className="ui-card overflow-hidden">
        <div className="bg-slate-50 dark:bg-[var(--card-bg)] border-b border-[var(--border)] px-6 py-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-x-3">
              <i className="fa-solid fa-network-wired text-indigo-500 dark:text-indigo-400"></i>
              <span className="text-sm font-medium text-[var(--text-secondary)]">Beacon 通信</span>
            </div>
            <span className="text-xs text-slate-500 dark:text-slate-400">显示 {filteredEntries.length} / {loading ? "..." : entries.length} 条</span>
          </div>
        </div>

        <div ref={containerRef} className="max-h-[500px] overflow-y-auto overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-[var(--card-bg)] border-b border-[var(--border)] sticky top-0">
              <tr className="text-xs text-slate-500 dark:text-slate-400">
                <th className="text-left py-3 px-4 font-medium min-w-[80px]">时间</th>
                <th className="text-left py-3 px-4 font-medium min-w-[80px]">Method</th>
                <th className="text-left py-3 px-4 font-medium min-w-[200px]">Path</th>
                <th className="text-left py-3 px-4 font-medium min-w-[120px]">Source IP</th>
                <th className="text-left py-3 px-4 font-medium min-w-[100px]">Agent</th>
                <th className="text-center py-3 px-4 font-medium min-w-[60px]">状态</th>
                <th className="text-right py-3 px-4 font-medium min-w-[60px]">Size</th>
                <th className="text-right py-3 px-4 font-medium min-w-[70px]">Latency</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-700 font-mono">
              {loading ? (
                [1, 2, 3, 4, 5].map(i => (
                  <tr key={i}>
                    {[1, 2, 3, 4, 5, 6, 7, 8].map(j => (
                      <td key={j} className="py-3 px-4"><div className="h-3 bg-slate-200 dark:bg-slate-700 rounded animate-pulse w-16"></div></td>
                    ))}
                  </tr>
                ))
              ) : filteredEntries.length > 0 ? (
                filteredEntries.map((e, i) => {
                  const id = e.id || e.ID || String(i);
                  const ts = e.timestamp || e.Timestamp || "";
                  const method = e.method || e.Method || "";
                  const path = e.path || e.Path || "";
                  const ip = e.source_ip || e.SourceIP || "";
                  const agent = e.agent_id || e.AgentID || "";
                  const status = e.status_code ?? e.StatusCode ?? 0;
                  const size = e.size ?? e.Size ?? 0;
                  const latency = e.latency ?? e.Latency ?? 0;

                  return (
                    <tr key={id} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                      <td className="py-3 px-4 text-xs text-slate-500 dark:text-slate-400">{formatTime(ts)}</td>
                      <td className="py-3 px-4">
                        <span className={`px-2 py-0.5 rounded text-xs font-bold ${getMethodStyle(method)}`}>{method}</span>
                      </td>
                      <td className="py-3 px-4 text-xs text-slate-600 dark:text-slate-300 max-w-[250px] truncate">{path}</td>
                      <td className="py-3 px-4 text-xs text-slate-500 dark:text-slate-400">{ip}</td>
                      <td className="py-3 px-4 text-xs text-slate-500 dark:text-slate-400">{agent ? agent.substring(0, 8) : "-"}</td>
                      <td className="py-3 px-4 text-center">
                        <span className={`text-xs font-medium ${status >= 400 ? "text-red-500 dark:text-red-400" : status >= 300 ? "text-amber-500 dark:text-amber-400" : "text-emerald-600 dark:text-emerald-400"}`}>{status}</span>
                      </td>
                      <td className="py-3 px-4 text-right text-xs text-slate-500 dark:text-slate-400">{size}B</td>
                      <td className="py-3 px-4 text-right text-xs text-slate-500 dark:text-slate-400">{latency}ms</td>
                    </tr>
                  );
                })
              ) : (
                <tr>
                  <td colSpan={8} className="py-16 text-center text-slate-400 dark:text-slate-500">
                    <i className="fa-solid fa-wifi text-2xl mb-2 text-slate-300 dark:text-slate-600"></i>
                    <p>没有流量数据</p>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
