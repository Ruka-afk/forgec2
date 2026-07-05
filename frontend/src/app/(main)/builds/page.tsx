"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";

interface BuildLog {
  id?: string;
  ID?: string;
  created_at?: string;
  CreatedAt?: string;
  platform?: string;
  Platform?: string;
  format?: string;
  Format?: string;
  c2_url?: string;
  C2URL?: string;
  filename?: string;
  Filename?: string;
  user?: string;
  User?: string;
  status?: string;
  Status?: string;
  error?: string;
  Error?: string;
  stdout?: string;
  Stdout?: string;
  stderr?: string;
  Stderr?: string;
  started_at?: string;
  StartedAt?: string;
  completed_at?: string;
  CompletedAt?: string;
  duration?: string;
  Duration?: string;
  artifact_path?: string;
  ArtifactPath?: string;
}

const PLATFORMS = ["all", "windows", "linux", "macos"];

function getDuration(start?: string, end?: string): string {
  if (!start) return "-";
  try {
    const s = new Date(start).getTime();
    const e = end ? new Date(end).getTime() : Date.now();
    const diff = Math.max(0, e - s);
    const min = Math.floor(diff / 60000);
    const sec = Math.floor((diff % 60000) / 1000);
    if (min > 0) return `${min}m ${sec}s`;
    return `${sec}s`;
  } catch { return "-"; }
}

function getStatusInfo(status: string) {
  switch (status) {
    case "success": return { icon: "fa-check-circle", color: "text-emerald-500", bg: "bg-emerald-500", label: "成功" };
    case "failed": return { icon: "fa-times-circle", color: "text-red-500", bg: "bg-red-500", label: "失败" };
    case "building": return { icon: "fa-circle-notch fa-spin", color: "text-blue-500", bg: "bg-blue-500", label: "构建中" };
    default: return { icon: "fa-clock", color: "text-slate-400", bg: "bg-slate-400", label: "等待" };
  }
}

export default function BuildsPage() {
  const [builds, setBuilds] = useState<BuildLog[]>([]);
  const [total, setTotal] = useState(0);
  const [successCount, setSuccessCount] = useState(0);
  const [failedCount, setFailedCount] = useState(0);
  const [avgDuration, setAvgDuration] = useState(0);
  const [loading, setLoading] = useState(true);
  const [filterPlatform, setFilterPlatform] = useState("all");
  const [filterStatus, setFilterStatus] = useState("");
  const [expandedBuild, setExpandedBuild] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadBuilds = useCallback(async () => {
    try {
      const params = new URLSearchParams({ format: "json" });
      if (filterPlatform !== "all") params.set("platform", filterPlatform);
      if (filterStatus) params.set("status", filterStatus);
      const res = await fetch(`${API_BASE}?p=/builds&${params}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const logs: BuildLog[] = data.logs || data.Logs || [];
      setBuilds(logs);
      setTotal(data.total ?? data.Total ?? logs.length);
      const success = data.success_count ?? data.SuccessCount ?? logs.filter((l: BuildLog) => (l.Status || l.status) === "success").length;
      const failed = data.failed_count ?? data.FailedCount ?? logs.filter((l: BuildLog) => (l.Status || l.status) === "failed").length;
      setSuccessCount(success);
      setFailedCount(failed);
      const durations = logs.filter((l: BuildLog) => l.Duration || l.duration).map((l: BuildLog) => {
        const d = (l.Duration || l.duration || "0").replace("s", "");
        return parseFloat(d) || 0;
      });
      setAvgDuration(durations.length > 0 ? Math.round(durations.reduce((a: number, b: number) => a + b, 0) / durations.length) : 0);
    } catch {
      setBuilds([]);
      setTotal(0);
      setSuccessCount(0);
      setFailedCount(0);
      setAvgDuration(0);
    }
    setLoading(false);
  }, [filterPlatform, filterStatus]);

  useEffect(() => {
    Promise.resolve().then(() => {
      loadBuilds();
      intervalRef.current = setInterval(loadBuilds, 5000);
    });
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [loadBuilds]);

  const formatTime = (t: string) => {
    if (!t) return "-";
    try { return new Date(t).toLocaleString(); } catch { return t; }
  };

  const handleDownload = (build: BuildLog) => {
    const artifactPath = build.ArtifactPath || build.artifact_path;
    const buildId = build.ID || build.id;
    if (artifactPath) {
      window.open(`${API_BASE}?p=/builds/${buildId || ""}/download`, "_blank");
    } else if (buildId) {
      window.open(`${API_BASE}?p=/builds/${buildId}/download&format=zip`, "_blank");
    }
  };

  const platformIcon = (p: string) => {
    switch (p) {
      case "windows": return "fa-brands fa-windows";
      case "linux": return "fa-brands fa-linux";
      case "macos": return "fa-brands fa-apple";
      default: return "fa-solid fa-microchip";
    }
  };

  const platformColor = (p: string) => {
    switch (p) {
      case "windows": return "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300";
      case "linux": return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
      case "macos": return "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300";
      default: return "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300";
    }
  };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">
            <i className="fa-solid fa-hammer text-indigo-500 mr-2"></i>构建日志
          </h1>
          <p className="text-xs sm:text-sm text-slate-500 dark:text-slate-400 mt-1">
            Implant 构建记录 · 成功 {successCount} · 失败 {failedCount} · 平均耗时 {avgDuration}s
          </p>
        </div>
        <div className="flex items-center gap-2">
          <a href="/generate" className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl text-sm font-medium flex items-center gap-x-2 transition-colors">
            <i className="fa-solid fa-plus"></i> 新建构建
          </a>
          <button onClick={loadBuilds} className="px-4 h-10 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-2xl text-sm font-medium flex items-center gap-x-2 transition-colors">
            <i className="fa-solid fa-sync"></i> 刷新
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-4 gap-4 mb-4">
        <div className="ui-card p-5 shadow-sm">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">总构建</div>
              <div className="text-3xl font-bold mt-2 text-indigo-600 dark:text-indigo-400 tabular-nums">{total}</div>
            </div>
            <div className="w-12 h-12 bg-indigo-50 dark:bg-indigo-900/30 rounded-2xl flex items-center justify-center">
              <i className="fa-solid fa-hammer text-xl text-indigo-400"></i>
            </div>
          </div>
        </div>
        <div className="ui-card p-5 shadow-sm">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">成功</div>
              <div className="text-3xl font-bold mt-2 text-emerald-600 dark:text-emerald-400 tabular-nums">{successCount}</div>
            </div>
            <div className="w-12 h-12 bg-emerald-50 dark:bg-emerald-900/30 rounded-2xl flex items-center justify-center">
              <i className="fa-solid fa-check-circle text-xl text-emerald-400"></i>
            </div>
          </div>
        </div>
        <div className="ui-card p-5 shadow-sm">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">失败</div>
              <div className="text-3xl font-bold mt-2 text-red-600 dark:text-red-400 tabular-nums">{failedCount}</div>
            </div>
            <div className="w-12 h-12 bg-red-50 dark:bg-red-900/30 rounded-2xl flex items-center justify-center">
              <i className="fa-solid fa-exclamation-circle text-xl text-red-400"></i>
            </div>
          </div>
        </div>
        <div className="ui-card p-5 shadow-sm">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">成功率</div>
              <div className="text-3xl font-bold mt-2 text-amber-600 dark:text-amber-400 tabular-nums">{total > 0 ? Math.round((successCount / total) * 100) : 0}%</div>
            </div>
            <div className="w-12 h-12 bg-amber-50 dark:bg-amber-900/30 rounded-2xl flex items-center justify-center">
              <i className="fa-solid fa-chart-line text-xl text-amber-400"></i>
            </div>
          </div>
        </div>
      </div>

      <div className="ui-card p-3 sm:p-4 mb-4 shadow-sm">
        <div className="flex flex-wrap items-center gap-3">
          <i className="fa-solid fa-filter text-indigo-500 text-sm"></i>
          <span className="text-sm font-semibold text-[var(--text-secondary)]">平台</span>
          <div className="flex flex-wrap gap-2">
            {PLATFORMS.map((p) => (
              <button key={p} onClick={() => setFilterPlatform(p)}
                className={`px-3 h-9 rounded-2xl text-xs font-medium transition-colors flex items-center gap-1.5 ${filterPlatform === p ? "bg-indigo-600 text-white" : "bg-slate-100 dark:bg-slate-700 text-[var(--text-secondary)] hover:bg-slate-200 dark:hover:bg-slate-600"}`}>
                <i className={`${platformIcon(p)} text-xs`}></i>
                {p === "all" ? "全部" : p.charAt(0).toUpperCase() + p.slice(1)}
              </button>
            ))}
          </div>
          <span className="text-sm font-semibold text-[var(--text-secondary)] ml-2">状态</span>
          <select value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}
            className="bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-2xl px-3 h-9 focus:outline-none focus:border-indigo-500 dark:text-slate-100">
            <option value="">全部</option>
            <option value="success">成功</option>
            <option value="failed">失败</option>
            <option value="building">构建中</option>
          </select>
          {(filterStatus || filterPlatform !== "all") && (
            <button onClick={() => { setFilterStatus(""); setFilterPlatform("all"); }} className="px-3 h-9 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-slate-600 dark:text-slate-300 rounded-2xl text-xs font-medium flex items-center transition-colors">
              <i className="fa-solid fa-times mr-1"></i> 清除
            </button>
          )}
        </div>
      </div>

      <div className="space-y-3">
        {loading ? (
          [1, 2, 3].map((i) => (
            <div key={i} className="ui-card p-5 animate-pulse">
              <div className="flex items-center gap-4">
                <div className="w-10 h-10 bg-slate-200 dark:bg-slate-700 rounded-2xl"></div>
                <div className="flex-1 space-y-2">
                  <div className="h-3 bg-slate-200 dark:bg-slate-700 rounded w-40"></div>
                  <div className="h-2 bg-slate-200 dark:bg-slate-700 rounded w-60"></div>
                </div>
              </div>
            </div>
          ))
        ) : builds.length === 0 ? (
          <div className="ui-card p-12 shadow-sm text-center">
            <i className="fa-solid fa-hammer text-5xl mb-4 text-slate-300 dark:text-slate-600"></i>
            <p className="text-sm font-medium text-slate-600 dark:text-slate-400">暂无构建记录</p>
            <p className="text-xs mt-2 text-slate-400 dark:text-slate-500">
              <a href="/generate" className="text-indigo-600 dark:text-indigo-400 hover:underline">前往生成页面</a> 生成新的 Implant 即可
            </p>
          </div>
        ) : (
          builds.map((build, idx) => {
            const id = build.ID || build.id || String(idx);
            const status = build.Status || build.status || "unknown";
            const platform = build.Platform || build.platform || "";
            const info = getStatusInfo(status);
            const startedAt = build.StartedAt || build.started_at || build.CreatedAt || build.created_at || "";
            const completedAt = build.CompletedAt || build.completed_at || "";
            const stderr = build.Stderr || build.stderr || "";
            const stdout = build.Stdout || build.stdout || "";
            const error = build.Error || build.error || "";
            const isExpanded = expandedBuild === id;
            const isBuilding = status === "building";

            return (
              <div key={id} className="ui-card overflow-hidden">
                <div className="p-4 sm:p-5">
                  <div className="flex items-center gap-4">
                    <div className={`w-10 h-10 rounded-2xl flex items-center justify-center ${status === "success" ? "bg-emerald-50 dark:bg-emerald-900/20" : status === "failed" ? "bg-red-50 dark:bg-red-900/20" : status === "building" ? "bg-blue-50 dark:bg-blue-900/20" : "bg-slate-50 dark:bg-slate-700"}`}>
                      <i className={`fa-solid ${info.color} ${isBuilding ? "fa-spinner fa-spin" : info.icon}`}></i>
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">#{String(id).substring(0, 8)}</span>
                        <span className={`px-2 py-0.5 text-[10px] rounded-lg font-medium inline-flex items-center gap-1 ${platformColor(platform)}`}>
                          <i className={`${platformIcon(platform)}`}></i> {platform || "unknown"}
                        </span>
                        <span className="px-2 py-0.5 text-[10px] rounded-lg bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300">{build.Format || build.format || "-"}</span>
                        <span className={`px-2 py-0.5 text-[10px] rounded-full font-medium ${status === "success" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : status === "failed" ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400" : "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"}`}>
                          <span className={`w-1.5 h-1.5 rounded-full inline-block mr-1 ${info.bg} ${isBuilding ? "animate-pulse" : ""}`}></span>
                          {info.label}
                        </span>
                      </div>
                      <div className="flex items-center gap-4 mt-1.5 text-xs text-slate-500 dark:text-slate-400">
                        <span><i className="fa-solid fa-calendar mr-1"></i>{formatTime(startedAt)}</span>
                        <span><i className="fa-solid fa-stopwatch mr-1"></i>{getDuration(startedAt, completedAt || (isBuilding ? undefined : completedAt))}</span>
                        {build.User || build.user ? <span><i className="fa-solid fa-user mr-1"></i>{build.User || build.user}</span> : null}
                        {build.Filename || build.filename ? <span className="truncate max-w-[200px]"><i className="fa-solid fa-file mr-1"></i>{build.Filename || build.filename}</span> : null}
                      </div>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <button onClick={() => setExpandedBuild(isExpanded ? null : id)}
                        className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors" title="日志">
                        <i className="fa-solid fa-terminal"></i>
                      </button>
                      {(status === "success" || build.ArtifactPath || build.artifact_path) && (
                        <button onClick={() => handleDownload(build)}
                          className="p-2 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 rounded-lg transition-colors" title="下载产物">
                          <i className="fa-solid fa-download"></i>
                        </button>
                      )}
                    </div>
                  </div>
                </div>

                {isExpanded && (
                  <div className="border-t border-[var(--border)]">
                    {(status === "failed" || error || stderr) && (
                      <div className="p-4 bg-red-50/50 dark:bg-red-900/10 border-b border-red-100 dark:border-red-900/20">
                        <div className="flex items-center gap-2 mb-2">
                          <i className="fa-solid fa-triangle-exclamation text-red-500 text-xs"></i>
                          <span className="text-xs font-semibold text-red-700 dark:text-red-400">错误信息</span>
                        </div>
                        <pre className="text-xs font-mono text-red-700 dark:text-red-300 bg-red-900/5 dark:bg-red-900/20 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap break-all">{error || stderr || "未知错误"}</pre>
                      </div>
                    )}
                    <div className="bg-slate-900 dark:bg-black p-4 max-h-[400px] overflow-y-auto">
                      <div className="flex items-center justify-between mb-3">
                        <span className="text-xs font-semibold text-slate-400 flex items-center gap-1.5">
                          <i className="fa-solid fa-terminal"></i> 构建日志
                        </span>
                        <span className="text-[10px] text-slate-500 font-mono">{stdout.split("\n").length} 行</span>
                      </div>
                      <pre className="text-xs font-mono text-slate-300 whitespace-pre-wrap leading-relaxed">
                        {stdout || <span className="text-slate-500 italic">暂无构建输出</span>}
                      </pre>
                    </div>
                    {build.c2_url || build.C2URL ? (
                      <div className="px-4 py-3 bg-slate-50 dark:bg-slate-700/30 border-t border-[var(--border)] flex items-center justify-between">
                        <span className="text-xs text-slate-500 font-mono truncate">{build.C2URL || build.c2_url}</span>
                      </div>
                    ) : null}
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
