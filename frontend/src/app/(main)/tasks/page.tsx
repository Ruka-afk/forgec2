"use client";

import { Suspense, useState, useEffect, useCallback, useRef } from "react";
import { useSearchParams } from "next/navigation";
import { API_BASE } from "@/lib/constants";
import { PageHeader, SearchInput, Pagination } from "@/components/UI";

interface Task {
  id: number;
  agent_id: string;
  type: string;
  command: string;
  status: string;
  result: string;
  error: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

interface Agent {
  id: string;
  hostname: string;
}

function TasksPage() {
  const searchParams = useSearchParams();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [agentFilter, setAgentFilter] = useState(searchParams.get("agent_id") || "");
  const [typeFilter, setTypeFilter] = useState("");
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());
  const [detailTask, setDetailTask] = useState<Task | null>(null);

  const getAgentName = useCallback((agentId: string) => {
    return agents.find((a) => a.id === agentId)?.hostname;
  }, [agents]);

  const loadAgents = useCallback(async () => {
    try {
      const r = await fetch(`${API_BASE}?p=/agents&page=1&pageSize=200&format=json`);
      if (r.ok) {
        const data = await r.json();
        setAgents((data.agents || data.Agents || data || []) as Agent[]);
      }
    } catch (e) { console.error("Tasks: load agents failed", e); }
  }, []);

  const loadTasks = useCallback(() => {
    setLoading(true);
    const params = new URLSearchParams({ page: String(page), pageSize: "50" });
    if (statusFilter) params.set("status", statusFilter);
    if (searchQuery) params.set("q", searchQuery);
    if (agentFilter) params.set("agentId", agentFilter);
    if (typeFilter) params.set("type", typeFilter);
    fetch(`${API_BASE}?p=/tasks&${params}&format=json`, { credentials: "include" })
      .then((r) => r.json())
      .then((data) => {
        setTasks((data.Tasks || data.tasks || data || []) as Task[]);
        setTotal(Number(data.Total) || 0);
      })
      .catch(() => { setTasks([]); setTotal(0); })
      .finally(() => setLoading(false));
  }, [page, statusFilter, searchQuery, agentFilter, typeFilter]);

  useEffect(() => { Promise.resolve().then(() => loadAgents()); }, [loadAgents]);
  useEffect(() => { Promise.resolve().then(() => loadTasks()); }, [loadTasks]);

  const handleServerExport = () => {
    window.open(`${API_BASE}?p=/tasks/export&format=json`, "_blank");
  };

  const handleExportCSV = () => {
    const headers = ["Time", "Agent", "Type", "Command", "Status", "Result", "Duration"];
    const rows = tasks.map((t) => [
      t.created_at || "",
      getAgentName(t.agent_id) || t.agent_id || String(t.id).substring(0, 8) || "",
      t.type || "",
      `"${(t.command || "").replace(/"/g, '""')}"`,
      t.status || "",
      `"${(t.result || "").replace(/"/g, '""').replace(/\n/g, " ")}"`,
      calcDuration(t.created_at, t.updated_at),
    ]);
    const csv = [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `tasks_export_${Date.now()}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const toggleRow = (id: number) => {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleCancel = async (task: Task) => {
    try {
      await fetch(`${API_BASE}?p=/agents/${task.agent_id}/tasks/${task.id}/cancel`, { method: "POST" });
      loadTasks();
    } catch (e) { console.error("Tasks: cancel task failed", e); }
  };

  const handleRerun = async (task: Task) => {
    try {
      await fetch(`${API_BASE}?p=/agents/${task.agent_id}/task/${task.id}/rerun`, { method: "POST" });
      loadTasks();
    } catch (e) { console.error("Tasks: rerun task failed", e); }
  };

  const getStatusBadge = (status: string): React.ReactNode => {
    if (status === "completed") return <span className="inline-flex items-center px-2.5 py-1 text-xs font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 rounded-lg"><i className="fa-solid fa-check mr-1 text-[10px]"></i>Done</span>;
    if (status === "failed") return <span className="inline-flex items-center px-2.5 py-1 text-xs font-medium bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400 rounded-lg"><i className="fa-solid fa-times mr-1 text-[10px]"></i>Failed</span>;
    if (status === "cancelled") return <span className="inline-flex items-center px-2.5 py-1 text-xs font-medium bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-[var(--text-secondary)] rounded-lg"><i className="fa-solid fa-ban mr-1 text-[10px]"></i>Cancelled</span>;
    return <span className="inline-flex items-center px-2.5 py-1 text-xs font-medium bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 rounded-lg"><i className="fa-solid fa-spinner fa-spin mr-1 text-[10px]"></i>Pending</span>;
  };

  const getTypeBadge = (type: string): React.ReactNode => {
    const colorMap: Record<string, string> = {
      shell: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
      screenshot: "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400",
      kill: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
    };
    const color = colorMap[type] || "bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-300";
    return <span className={`inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-lg ${color}`}>{type}</span>;
  };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title="Tasks" subtitle={`All task history \u00b7 ${total} total`}>
        <button onClick={handleServerExport} className="inline-flex items-center gap-2 px-4 py-2 bg-[var(--card-bg-secondary)] hover:bg-slate-200 text-[var(--text-secondary)] text-sm font-medium rounded-xl">
          <i className="fa-solid fa-download"></i> Server Export
        </button>
        <button onClick={handleExportCSV} className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-700 text-white text-sm font-medium rounded-xl transition-colors">
          <i className="fa-solid fa-file-csv"></i> Export CSV
        </button>
      </PageHeader>

      <div className="ui-card p-3 sm:p-4 mb-4 shadow-sm">
        <div className="flex flex-col sm:flex-row gap-3">
          <SearchInput value={searchQuery} onChange={(v) => { setSearchQuery(v); setPage(1); }} placeholder="Search command..." className="flex-1" />
          <select value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }} className="bg-[var(--card-bg-secondary)] border border-[var(--border)] text-sm rounded-xl px-3 h-10 cursor-pointer">
            <option value="">All Status</option>
            <option value="completed">Completed</option>
            <option value="pending">Pending</option>
            <option value="failed">Failed</option>
            <option value="cancelled">Cancelled</option>
          </select>
          <select value={agentFilter} onChange={(e) => { setAgentFilter(e.target.value); setPage(1); }} className="bg-[var(--card-bg-secondary)] border border-[var(--border)] text-sm rounded-xl px-3 h-10 cursor-pointer min-w-[150px]">
            <option value="">All Agents</option>
            {agents.map((a) => (
              <option key={a.id} value={a.id}>{a.hostname || a.id.substring(0, 8)}</option>
            ))}
          </select>
          <select value={typeFilter} onChange={(e) => { setTypeFilter(e.target.value); setPage(1); }} className="bg-[var(--card-bg-secondary)] border border-[var(--border)] text-sm rounded-xl px-3 h-10 cursor-pointer">
            <option value="">All Types</option>
            <option value="shell">shell</option>
            <option value="screenshot">screenshot</option>
            <option value="file">file</option>
            <option value="download">download</option>
            <option value="upload">upload</option>
            <option value="ps">ps</option>
            <option value="creds">creds</option>
          </select>
        </div>
      </div>

      <div className="ui-card sm:rounded-3xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-[var(--card-bg-secondary)] border-b border-[var(--border)] sticky top-0 z-10">
              <tr className="text-xs text-[var(--text-secondary)]">
                <th className="text-left py-3 px-4 font-normal min-w-[140px]">Time</th>
                <th className="text-left py-3 px-4 font-normal min-w-[120px]">Agent</th>
                <th className="text-left py-3 px-4 font-normal min-w-[100px]">Type</th>
                <th className="text-left py-3 px-4 font-normal min-w-[180px]">Command</th>
                <th className="text-left py-3 px-4 font-normal min-w-[200px]">Result</th>
                <th className="text-left py-3 px-4 font-normal min-w-[90px]">Duration</th>
                <th className="text-center py-3 px-4 font-normal min-w-[90px]">Status</th>
                <th className="text-center py-3 px-4 font-normal min-w-[120px]">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
              {!loading && tasks.map((task) => (
                <TaskRow key={task.id} task={task} expanded={expandedRows.has(task.id)} onToggle={() => toggleRow(task.id)} onDetail={() => setDetailTask(task)} onCancel={() => handleCancel(task)} onRerun={() => handleRerun(task)} getAgentName={getAgentName} getTypeBadge={getTypeBadge} getStatusBadge={getStatusBadge} />
              ))}
              {loading && Array.from({ length: 5 }).map((_, i) => (<tr key={i}><td colSpan={8} className="py-3 px-4"><div className="h-8 bg-[var(--card-bg-secondary)] rounded animate-pulse"></div></td></tr>))}
              {!loading && tasks.length === 0 && (<tr><td colSpan={8} className="py-20 text-center text-[var(--text-tertiary)]"><i className="fa-solid fa-inbox text-2xl mb-2"></i><p className="text-sm">No tasks yet</p></td></tr>)}
            </tbody>
          </table>
        </div>
      </div>

      <Pagination page={page} pageSize={50} total={total} onPageChange={setPage} />

      {detailTask && <TaskDetailModal task={detailTask} onClose={() => setDetailTask(null)} getAgentName={getAgentName} getStatusBadge={getStatusBadge} getTypeBadge={getTypeBadge} />}
    </div>
  );
}

export default function TasksPageWrapper() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center py-20 text-[var(--text-tertiary)]"><i className="fa-solid fa-circle-notch fa-spin mr-2"></i>Loading...</div>}>
      <TasksPage />
    </Suspense>
  );
}

function TaskRow({ task, expanded, onToggle, onDetail, onCancel, onRerun, getAgentName, getTypeBadge, getStatusBadge }: {
  task: Task;
  expanded: boolean;
  onToggle: () => void;
  onDetail: () => void;
  onCancel: () => void;
  onRerun: () => void;
  getAgentName: (id: string) => string | undefined;
  getTypeBadge: (t: string) => React.ReactNode;
  getStatusBadge: (s: string) => React.ReactNode;
}) {
  return (
    <>
      <tr className="hover:bg-[var(--card-bg-secondary)]/50 transition-colors cursor-pointer" onClick={onDetail}>
        <td className="py-3 px-4 font-mono text-xs text-[var(--text-secondary)] whitespace-nowrap">{task.created_at ? new Date(task.created_at).toLocaleString() : "-"}</td>
        <td className="py-3 px-4"><div className="font-medium text-[var(--text-primary)] text-sm">{getAgentName(task.agent_id) || task.agent_id?.substring(0, 8)}</div></td>
        <td className="py-3 px-4">{getTypeBadge(task.type)}</td>
        <td className="py-3 px-4 font-mono text-xs text-slate-600 dark:text-[var(--text-secondary)] max-w-xs truncate">{task.command || "-"}</td>
        <td className="py-3 px-4 max-w-sm" onClick={(e) => { e.stopPropagation(); onToggle(); }}>
          {task.result ? (
            <div>
              <span className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline">
                <i className={`fa-solid ${expanded ? "fa-chevron-down" : "fa-chevron-right"} mr-1 text-[10px]`}></i>
                {expanded ? "Hide" : "Expand"}
              </span>
            </div>
          ) : "-"}
        </td>
        <td className="py-3 px-4 text-xs text-[var(--text-secondary)] font-mono">{calcDuration(task.created_at, task.updated_at)}</td>
        <td className="py-3 px-4 text-center">{getStatusBadge(task.status)}</td>
        <td className="py-3 px-4 text-center" onClick={(e) => e.stopPropagation()}>
          <div className="flex items-center justify-center gap-1">
            {(task.status === "pending" || task.status === "running") && (
              <button onClick={onCancel} className="p-1.5 text-[var(--text-tertiary)] hover:text-red-500 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors" title="Cancel">
                <i className="fa-solid fa-ban text-xs"></i>
              </button>
            )}
            {(task.status === "completed" || task.status === "failed" || task.status === "cancelled") && (
              <button onClick={onRerun} className="p-1.5 text-[var(--text-tertiary)] hover:text-indigo-500 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors" title="Rerun">
                <i className="fa-solid fa-rotate-right text-xs"></i>
              </button>
            )}
          </div>
        </td>
      </tr>
      {expanded && task.result && (
        <tr>
          <td colSpan={8} className="px-4 py-3 bg-slate-900 dark:bg-slate-950">
            <div className="relative">
              <button onClick={onDetail} className="absolute top-2 right-2 text-xs text-[var(--text-tertiary)] hover:text-white bg-slate-700 px-2 py-1 rounded">
                <i className="fa-solid fa-expand mr-1"></i>Full View
              </button>
              <pre className="text-xs text-emerald-300 font-mono overflow-x-auto max-h-60 p-2 whitespace-pre-wrap break-all">{task.result}</pre>
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

function TaskDetailModal({ task, onClose, getAgentName, getStatusBadge, getTypeBadge }: {
  task: Task;
  onClose: () => void;
  getAgentName: (id: string) => string | undefined;
  getStatusBadge: (s: string) => React.ReactNode;
  getTypeBadge: (t: string) => React.ReactNode;
}) {
  const overlayRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    document.addEventListener("keydown", handleEsc);
    return () => document.removeEventListener("keydown", handleEsc);
  }, [onClose]);
  return (
    <div ref={overlayRef} className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" onClick={(e) => { if (e.target === overlayRef.current) onClose(); }}>
      <div className="ui-card shadow-2xl w-full max-w-3xl max-h-[85vh] overflow-hidden flex flex-col">
        <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)] shrink-0">
          <div className="flex items-center gap-3">
            {getTypeBadge(task.type)}
            {getStatusBadge(task.status)}
            <span className="text-xs font-mono text-[var(--text-secondary)]">{String(task.id).substring(0, 12)}</span>
          </div>
          <button onClick={onClose} className="p-2 text-[var(--text-tertiary)] hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-[var(--card-bg-secondary)] transition-colors">
            <i className="fa-solid fa-xmark text-lg"></i>
          </button>
        </div>
        <div className="px-6 py-4 border-b border-[var(--border)] grid grid-cols-2 sm:grid-cols-4 gap-4 text-xs shrink-0">
          <div><span className="text-[var(--text-secondary)]">Agent</span><p className="font-medium text-[var(--text-primary)] mt-0.5">{getAgentName(task.agent_id) || task.agent_id?.substring(0, 8)}</p></div>
          <div><span className="text-[var(--text-secondary)]">Created</span><p className="font-medium text-[var(--text-primary)] mt-0.5">{task.created_at ? new Date(task.created_at).toLocaleString() : "-"}</p></div>
          <div><span className="text-[var(--text-secondary)]">Duration</span><p className="font-medium text-[var(--text-primary)] mt-0.5">{calcDuration(task.created_at, task.updated_at)}</p></div>
          <div><span className="text-[var(--text-secondary)]">By</span><p className="font-medium text-[var(--text-primary)] mt-0.5">{task.created_by || "system"}</p></div>
        </div>
        <div className="overflow-y-auto flex-1 p-6">
          <div className="mb-4">
            <h3 className="text-xs font-medium text-[var(--text-secondary)] mb-2 uppercase tracking-wider">Command</h3>
            <code className="block bg-slate-50 dark:bg-slate-900 border border-[var(--border)] rounded-xl p-3 text-sm font-mono text-[var(--text-primary)]">{task.command || "-"}</code>
          </div>
          {task.result && (
            <div>
              <h3 className="text-xs font-medium text-[var(--text-secondary)] mb-2 uppercase tracking-wider">Output</h3>
              <pre className="bg-slate-900 dark:bg-slate-950 text-emerald-300 font-mono text-xs rounded-xl p-4 overflow-x-auto whitespace-pre-wrap break-all max-h-96">{task.result}</pre>
            </div>
          )}
          {task.error && (
            <div className="mt-4">
              <h3 className="text-xs font-medium text-[var(--text-secondary)] mb-2 uppercase tracking-wider">Error</h3>
              <pre className="bg-red-900/20 text-red-400 font-mono text-xs rounded-xl p-4 overflow-x-auto whitespace-pre-wrap">{task.error}</pre>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function calcDuration(created?: string, updated?: string): string {
  if (!created) return "-";
  const start = new Date(created).getTime();
  if (isNaN(start)) return "-";
  const end = updated ? new Date(updated).getTime() : Date.now();
  if (isNaN(end)) return "-";
  const ms = end - start;
  if (ms < 0) return "-";
  if (ms < 1000) return `${ms}ms`;
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  const remSec = sec % 60;
  if (min < 60) return `${min}m ${remSec}s`;
  const hr = Math.floor(min / 60);
  return `${hr}h ${min % 60}m`;
}
