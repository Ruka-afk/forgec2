"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { PageHeader, SearchInput, Pagination } from "@/components/UI";

interface AuditLog {
  id?: string;
  ID?: string;
  timestamp?: string;
  Timestamp?: string;
  username?: string;
  Username?: string;
  action?: string;
  Action?: string;
  resource?: string;
  Resource?: string;
  target?: string;
  Target?: string;
  status?: string;
  Status?: string;
  details?: string;
  Details?: string;
  ip?: string;
  IP?: string;
  severity?: string;
  Severity?: string;
}

const SEVERITY_LEVELS = ["info", "warning", "error", "critical"] as const;
const ACTION_TYPES = ["login", "create", "update", "delete", "logout", "failed"] as const;

const ACTION_BADGES: Record<string, string> = {
  login: "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400",
  create: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
  update: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
  delete: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
  logout: "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300",
  failed: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
};

const SEVERITY_BADGES: Record<string, string> = {
  info: "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300",
  warning: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
  error: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
  critical: "bg-red-200 text-red-800 dark:bg-red-800 dark:text-red-200",
};



export default function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(50);
  const [totalPages, setTotalPages] = useState(1);
  const [search, setSearch] = useState("");
  const [userFilter, setUserFilter] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);

  const loadLogs = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        p: "/audit/logs",
        format: "json",
        page: String(page),
        pageSize: String(perPage),
      });
      if (search) params.set("search", search);
      if (userFilter) params.set("user", userFilter);
      if (actionFilter) params.set("action", actionFilter);
      const resp = await fetch(`${API_BASE}?${params}`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      setLogs(data.data || []);
      setTotal(data.total || 0);
      setTotalPages(Math.ceil((data.total || 0) / perPage));
    } catch {
      setLogs([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [page, perPage, search, userFilter, actionFilter]);

  useEffect(() => { Promise.resolve().then(() => loadLogs()); }, [loadLogs]);

  const applyFilters = () => { setPage(1); };
  const resetFilters = () => {
    setSearch("");
    setUserFilter("");
    setActionFilter("");
    setPage(1);
  };

  const handleExport = () => {
    const csv = logs.map((l) => {
      const time = l.timestamp || l.Timestamp || "";
      const user = l.username || l.Username || "";
      const ip = l.ip || l.IP || "";
      const action = l.action || l.Action || "";
      const resource = l.resource || l.Resource || "";
      const target = l.target || l.Target || "";
      const status = l.status || l.Status || "";
      const severity = l.severity || l.Severity || "";
      const details = (l.details || l.Details || "").replace(/,/g, ";");
      return `${time},${user},${ip},${action},${resource},${target},${status},${severity},${details}`;
    }).join("\n");
    const header = "Timestamp,User,IP,Action,Resource,Target,Status,Severity,Details\n";
    const blob = new Blob([header + csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "audit-logs.csv";
    a.click();
    URL.revokeObjectURL(url);
  };

  const getActionBadge = (action: string) => {
    const a = (action || "").toLowerCase();
    for (const [key, badge] of Object.entries(ACTION_BADGES)) {
      if (a.includes(key)) return badge;
    }
    return "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300";
  };

  const getSeverityBadge = (severity: string) => {
    const s = (severity || "").toLowerCase();
    return SEVERITY_BADGES[s] || "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300";
  };

  const getLogField = (log: AuditLog, field: keyof AuditLog | "severity") => {
    switch (field) {
      case "id": return String(log.id || log.ID || "");
      case "timestamp": return String(log.timestamp || log.Timestamp || "");
      case "username": return String(log.username || log.Username || "-");
      case "action": return String(log.action || log.Action || "-");
      case "resource": return String(log.resource || log.Resource || "-");
      case "target": return String(log.target || log.Target || "-");
      case "status": return String(log.status || log.Status || "-");
      case "details": return String(log.details || log.Details || "-");
      case "ip": return String(log.ip || log.IP || "-");
      case "severity": return String(log.severity || log.Severity || "info");
      default: return "";
    }
  };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title="安全审计日志" subtitle="记录所有系统操作和安全行为">
        <button onClick={handleExport} className="px-4 h-11 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm flex items-center gap-2 transition-colors shrink-0">
          <i className="fa-solid fa-download"></i>
          <span>导出 CSV</span>
        </button>
      </PageHeader>

      <div className="grid grid-cols-1 sm:grid-cols-1 gap-4 mb-4">
        <div className="ui-card p-5 shadow-sm">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-slate-500 uppercase tracking-wider">总记录数</div>
              <div className="text-3xl font-bold mt-2 text-slate-900 dark:text-slate-100">{total}</div>
            </div>
            <div className="w-12 h-12 bg-indigo-50 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center">
              <i className="fa-solid fa-file-text text-xl text-indigo-500"></i>
            </div>
          </div>
        </div>
      </div>

      <div className="ui-card p-4 mb-4 shadow-sm">
        <div className="flex flex-wrap items-center gap-3">
          <SearchInput value={search} onChange={setSearch} placeholder="搜索用户、资源、详情..." className="flex-1 min-w-[200px]" />
          <select className="bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 h-10 text-sm min-w-[140px]" value={actionFilter} onChange={(e) => setActionFilter(e.target.value)}>
            <option value="">所有操作</option>
            <option value="login">登录</option>
            <option value="logout">退出</option>
            <option value="create">创建</option>
            <option value="update">更新</option>
            <option value="delete">删除</option>
            <option value="failed">失败</option>
          </select>
          <select className="bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 h-10 text-sm min-w-[140px]" value={userFilter} onChange={(e) => setUserFilter(e.target.value)}>
            <option value="">所有用户</option>
          </select>
          <button onClick={applyFilters} className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm flex items-center gap-1.5 transition-colors">
            <i className="fa-solid fa-filter"></i>
            <span>应用</span>
          </button>
          <button onClick={resetFilters} className="px-4 h-10 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-xl text-sm transition-colors">
            重置
          </button>
        </div>
      </div>

      <div className="ui-card overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-slate-50 dark:bg-[var(--card-bg)] border-b border-[var(--border)]">
              <tr>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">时间</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">用户</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">IP</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">操作</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">严重度</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">资源</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">Target</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">状态</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-slate-500">详情</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
              {loading ? (
                <tr><td colSpan={9} className="py-16 text-center text-slate-400">
                  <i className="fa-solid fa-circle-notch fa-spin text-2xl text-indigo-500"></i>
                </td></tr>
              ) : !loading && logs.length === 0 ? (
                <tr><td colSpan={9} className="py-16 text-center text-slate-400 dark:text-slate-500">暂无审计记录</td></tr>
              ) : (
                logs.map((log, i) => {
                  const action = getLogField(log, "action");
                  const severity = getLogField(log, "severity");
                  return (
                    <tr key={getLogField(log, "id") || String(i)} onClick={() => setSelectedLog(log)} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors cursor-pointer">
                      <td className="py-3 px-4 text-xs text-slate-500 dark:text-slate-400 font-mono whitespace-nowrap">{getLogField(log, "timestamp")}</td>
                      <td className="py-3 px-4 text-sm font-medium text-[var(--text-secondary)]">{getLogField(log, "username")}</td>
                      <td className="py-3 px-4 text-xs text-slate-500 dark:text-slate-400 font-mono">{getLogField(log, "ip")}</td>
                      <td className="py-3 px-4">
                        <span className={`inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full ${getActionBadge(action)}`}>
                          {action}
                        </span>
                      </td>
                      <td className="py-3 px-4">
                        <span className={`inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full uppercase ${getSeverityBadge(severity)}`}>
                          {severity}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-xs text-slate-600 dark:text-slate-400 max-w-[200px] truncate">{getLogField(log, "resource")}</td>
                      <td className="py-3 px-4 text-xs text-slate-500 dark:text-slate-400 font-mono">{getLogField(log, "target")}</td>
                      <td className="py-3 px-4">
                        <span className={`inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full ${
                          (getLogField(log, "status") || "").toLowerCase().includes("fail")
                            ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                            : "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                        }`}>
                          {getLogField(log, "status")}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-xs text-slate-500 dark:text-slate-400 max-w-[300px] truncate">{getLogField(log, "details")}</td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        <Pagination page={page} pageSize={perPage} total={total} onPageChange={setPage} />
      </div>

      {selectedLog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" onClick={() => setSelectedLog(null)}>
          <div className="ui-card shadow-xl max-w-lg w-full max-h-[80vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)]">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">审计记录详情</h3>
              <button onClick={() => setSelectedLog(null)} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300">
                <i className="fa-solid fa-xmark"></i>
              </button>
            </div>
            <div className="p-6 space-y-4">
              {(["timestamp", "username", "ip", "action", "severity", "resource", "target", "status", "details"] as const).map(field => (
                <div key={field}>
                  <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">{field}</label>
                  <p className={`text-sm mt-0.5 ${field === "action" || field === "severity" || field === "status" ? "" : "text-[var(--text-secondary)]"}`}>
                    {field === "action" ? (
                      <span className={`inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full ${getActionBadge(getLogField(selectedLog, field))}`}>
                        {selectedLog.action || selectedLog.Action}
                      </span>
                    ) : field === "severity" ? (
                      <span className={`inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full uppercase ${getSeverityBadge(getLogField(selectedLog, field))}`}>
                        {selectedLog.severity || selectedLog.Severity || "info"}
                      </span>
                    ) : field === "status" ? (
                      <span className={`inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full ${
                        (getLogField(selectedLog, field)).toLowerCase().includes("fail")
                          ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                          : "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                      }`}>
                        {getLogField(selectedLog, field)}
                      </span>
                    ) : (
                      <span className="font-mono">{getLogField(selectedLog, field)}</span>
                    )}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
