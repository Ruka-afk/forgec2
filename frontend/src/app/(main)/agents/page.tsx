"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { useWS } from "@/lib/wsContext";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { API_BASE } from "@/lib/constants";
import { apiPostJson, apiDelete } from "@/lib/api";
import { ConfirmModal, PageHeader, SearchInput, Pagination } from "@/components/UI";

async function fetchBeacons(search = "", status = "", os = "", page = 1, pageSize = 20) {
  const p = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
  if (search) p.set("search", search);
  if (status) p.set("status", status);
  if (os) p.set("os", os);
  const r = await fetch(`${API_BASE}?p=/agents&${p.toString()}`, { credentials: "include" });
  if (!r.ok) throw new Error("Failed");
  return r.json();
}

interface Beacon {
  id?: string; ID?: string;
  hostname?: string; Hostname?: string;
  username?: string; Username?: string;
  ip?: string; IP?: string;
  os?: string; OS?: string;
  arch?: string; Arch?: string;
  status?: string; Status?: string;
  last_seen?: string; LastSeen?: string;
  integrity?: string; Integrity?: string;
  elevated?: boolean; Elevated?: boolean;
  notes?: string; Notes?: string;
  active_window?: string; ActiveWindow?: string;
  version?: string; Version?: string;
  parent_id?: string; ParentID?: string;
}

function normalize(b: Beacon): Beacon {
  return {
    id: b.id || b.ID,
    hostname: b.hostname || b.Hostname,
    username: b.username || b.Username,
    ip: b.ip || b.IP,
    os: b.os || b.OS,
    arch: b.arch || b.Arch,
    status: b.status || b.Status,
    last_seen: b.last_seen || b.LastSeen,
    integrity: b.integrity || b.Integrity,
    elevated: b.elevated ?? b.Elevated,
    notes: b.notes || b.Notes,
    active_window: b.active_window || b.ActiveWindow,
    version: b.version || b.Version,
    parent_id: b.parent_id || b.ParentID || "",
  };
}

export default function BeaconsPage() {
  const router = useRouter();
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [osFilter, setOsFilter] = useState("");
  const [beacons, setBeacons] = useState<Beacon[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [confirm, setConfirm] = useState<{ type: "kill" | "delete" | "batch-delete"; id?: string; hostname?: string } | null>(null);
  const [actionMsg, setActionMsg] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<"hostname" | "username" | "os" | "ip" | "last_seen" | "status">("last_seen");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [linkedFilter, setLinkedFilter] = useState<"" | "direct" | "chained">("");
  const [taskCountMap, setTaskCountMap] = useState<Record<string, number>>({});
  const { subscribe } = useWS();

  const loadBeacons = useCallback(() => {
    setLoading(true);
    fetchBeacons(search, statusFilter, osFilter, page, 20)
      .then((data) => {
        const raw: Beacon[] = data.Beacons || data.Agents || data.data || data || [];
        const list = raw.map(normalize);
        setBeacons(list);
        setTotal(Number(data.Total) || list.length);
      })
      .catch(() => {
        setBeacons([]);
        setTotal(0);
      })
      .finally(() => setLoading(false));
  }, [search, statusFilter, osFilter, page]);

  useEffect(() => { loadBeacons(); }, [loadBeacons]);

  useEffect(() => {
    fetch(`${API_BASE}?p=/tasks&page=1&pageSize=1000&format=json`, { credentials: "include" })
      .then((r) => r.json())
      .then((d) => {
        const tasks = d.Tasks || d.tasks || d.data || [];
        const map: Record<string, number> = {};
        (tasks as { agent_id?: string; AgentID?: string }[]).forEach((t) => {
          const aid = t.agent_id || t.AgentID || "";
          if (aid) map[aid] = (map[aid] || 0) + 1;
        });
        setTaskCountMap(map);
      })
      .catch(() => setTaskCountMap({}));
  }, []);

  // WebSocket real-time updates for agent online/offline events
  useEffect(() => {
    const unsub = subscribe((msg) => {
      if (msg.type === "agent_online" || msg.type === "agent_offline") {
        loadBeacons();
      } else if (msg.type === "agent_data_update" && msg.agent_id) {
        const aid = String(msg.agent_id);
        setBeacons((prev) => prev.map((b) =>
          (b.id || "") === aid ? { ...b, ...(msg.data as Partial<Beacon>) } : b
        ));
      }
    });
    return () => unsub();
  }, [subscribe, loadBeacons]);

  const toggleSort = (key: typeof sortKey) => {
    if (sortKey === key) setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const filteredBeacons = useMemo(() => {
    if (!linkedFilter) return beacons;
    return beacons.filter((b) => {
      const pid = b.parent_id || "";
      if (linkedFilter === "direct") return !pid;
      if (linkedFilter === "chained") return !!pid;
      return true;
    });
  }, [beacons, linkedFilter]);

  const sortedBeacons = useMemo(() => {
    const list = [...filteredBeacons];
    const dir = sortDir === "asc" ? 1 : -1;
    list.sort((a, b) => {
      const av = String(a[sortKey] || "");
      const bv = String(b[sortKey] || "");
      if (sortKey === "last_seen") {
        const at = av ? new Date(av).getTime() : 0;
        const bt = bv ? new Date(bv).getTime() : 0;
        return (at - bt) * dir;
      }
      return av.localeCompare(bv) * dir;
    });
    return list;
  }, [filteredBeacons, sortKey, sortDir]);

  const sortIcon = (key: typeof sortKey) => {
    if (sortKey !== key) return "fa-sort text-slate-300";
    return sortDir === "asc" ? "fa-sort-up text-indigo-500" : "fa-sort-down text-indigo-500";
  };

  const avatarInitial = (hostname: string) => (hostname || "?").charAt(0).toUpperCase();
  const avatarColor = (hostname: string) => {
    const colors = ["bg-indigo-500", "bg-emerald-500", "bg-amber-500", "bg-rose-500", "bg-cyan-500"];
    let h = 0;
    for (let i = 0; i < hostname.length; i++) h = hostname.charCodeAt(i) + ((h << 5) - h);
    return colors[Math.abs(h) % colors.length];
  };

  const toggleSelect = (id: string, checked: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id); else next.delete(id);
      return next;
    });
  };

  const toggleSelectAll = (checked: boolean) => {
    if (checked) setSelected(new Set(beacons.map((b) => b.id || "").filter(Boolean)));
    else setSelected(new Set());
  };

  const runBatch = async (payload: Record<string, unknown>) => {
    try {
      const data = await apiPostJson<{ success?: boolean; tasks_created?: number; error?: string }>("/agents/batch", payload);
      if (data.success) {
        setActionMsg(`Sent to ${data.tasks_created ?? selected.size} agents`);
        setSelected(new Set());
        loadBeacons();
      } else {
        setActionMsg(data.error || "Batch command failed");
      }
    } catch {
      setActionMsg("Batch command failed");
    }
  };

  const batchShell = () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    const cmd = prompt(`Execute on ${ids.length} agents:`, "whoami");
    if (!cmd) return;
    runBatch({ agent_ids: ids, command: cmd, shell: "cmd.exe", task_type: "shell" });
  };

  const batchScreenshot = () => {
    const ids = Array.from(selected);
    if (!ids.length || !window.confirm(`Screenshot ${ids.length} agents?`)) return;
    runBatch({ agent_ids: ids, task_type: "screenshot" });
  };

  const batchSleep = () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    const interval = prompt("Sleep interval (seconds):", "30");
    if (!interval) return;
    const jitter = prompt("Jitter (0-100):", "20");
    if (!jitter) return;
    runBatch({ agent_ids: ids, task_type: "sleep", args: `${interval},${jitter}` });
  };

  const batchDelete = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    try {
      const data = await apiPostJson<{ success?: boolean; deleted?: number; error?: string }>("/agents/batch/delete", { agent_ids: ids });
      if (data.success) {
        setActionMsg(`Deleted ${data.deleted ?? 0} agents`);
        setSelected(new Set());
        loadBeacons();
      } else {
        setActionMsg(data.error || "Delete failed");
      }
    } catch {
      setActionMsg("Delete failed");
    }
    setConfirm(null);
  };

  const killAgent = async (id: string) => {
    try {
      await apiPostJson(`/agents/${id}/kill`, {});
      setActionMsg("Kill command sent");
      loadBeacons();
    } catch {
      setActionMsg("Kill failed");
    }
    setConfirm(null);
  };

  const deleteAgent = async (id: string) => {
    try {
      await apiDelete(`/agents/${id}`);
      setActionMsg("Agent deleted");
      loadBeacons();
    } catch {
      setActionMsg("Delete failed");
    }
    setConfirm(null);
  };

  const onlineCount = beacons.filter((b) => b.status === "online").length;
  const staleCount = beacons.filter((b) => b.status === "stale").length;
  const offlineCount = beacons.filter((b) => b.status === "offline").length;

  return (
    <div>
      <PageHeader
        title="Beacons"
        subtitle={`${total} total \u00b7 ${onlineCount} online${staleCount > 0 ? `, ${staleCount} stale` : ""}${offlineCount > 0 ? `, ${offlineCount} offline` : ""}`}
      >
        <button
          onClick={() => { setPage(1); loadBeacons(); }}
          className="w-11 h-11 sm:px-4 sm:h-10 bg-[var(--card-bg)] border border-[var(--border)] hover:bg-[var(--card-bg-secondary)] rounded-xl sm:rounded-2xl flex items-center justify-center gap-2 touch-target"
        >
          <i className="fa-solid fa-sync text-[var(--text-secondary)]"></i>
          <span className="hidden sm:inline text-[var(--text-primary)] text-sm">Refresh</span>
        </button>
        <Link
          href="/generate"
          className="inline-flex items-center justify-center gap-x-2 bg-indigo-600 hover:bg-indigo-700 transition-colors px-4 sm:px-5 h-11 sm:h-10 rounded-xl sm:rounded-2xl text-sm font-medium text-white touch-target"
        >
          <i className="fa-solid fa-plus"></i>
          <span className="hidden sm:inline">Generate Implant</span>
          <span className="sm:hidden">New</span>
        </Link>
      </PageHeader>

      {actionMsg && (
        <div className="mb-3 px-4 py-2 bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800 rounded-xl text-sm text-indigo-700 dark:text-indigo-300 flex items-center justify-between">
          <span>{actionMsg}</span>
          <button onClick={() => setActionMsg(null)} className="text-indigo-400 hover:text-indigo-600"><i className="fa-solid fa-xmark"></i></button>
        </div>
      )}

      {selected.size > 0 && (
        <div className="mb-3 px-4 py-3 bg-[var(--sidebar-bg)] border border-slate-700 rounded-2xl flex flex-wrap items-center gap-2 shadow-sm">
          <span className="text-sm text-white font-medium mr-2">{selected.size} selected</span>
          <button onClick={batchShell} className="px-3 py-1.5 text-xs bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg"><i className="fa-solid fa-terminal mr-1"></i>Shell</button>
          <button onClick={batchScreenshot} className="px-3 py-1.5 text-xs bg-slate-600 hover:bg-slate-500 text-white rounded-lg"><i className="fa-solid fa-camera mr-1"></i>Screenshot</button>
          <button onClick={batchSleep} className="px-3 py-1.5 text-xs bg-slate-600 hover:bg-slate-500 text-white rounded-lg"><i className="fa-solid fa-clock mr-1"></i>Sleep</button>
          <button onClick={() => setConfirm({ type: "batch-delete" })} className="px-3 py-1.5 text-xs bg-red-600 hover:bg-red-700 text-white rounded-lg"><i className="fa-solid fa-trash mr-1"></i>Delete</button>
          <button onClick={() => setSelected(new Set())} className="ml-auto px-3 py-1.5 text-xs text-[var(--text-tertiary)] hover:text-white">Clear</button>
        </div>
      )}

      {/* Filters */}
      <div className="ui-card p-3 sm:p-4 mb-4 shadow-sm">
        <div className="flex flex-col sm:flex-row gap-3">
          <SearchInput value={search} onChange={(v) => { setSearch(v); setPage(1); }} placeholder="Search hostname, user, IP..." className="flex-1" />
          <div className="flex gap-2 flex-wrap">
            <select
              value={statusFilter}
              onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
              className="flex-1 sm:flex-none bg-[var(--card-bg-secondary)] border border-[var(--border)] focus:border-indigo-500 text-sm rounded-xl px-3 h-11 cursor-pointer touch-target"
            >
              <option value="">All Status</option>
              <option value="online">Online</option>
              <option value="stale">Stale</option>
              <option value="offline">Offline</option>
            </select>
            <select
              value={osFilter}
              onChange={(e) => { setOsFilter(e.target.value); setPage(1); }}
              className="flex-1 sm:flex-none bg-[var(--card-bg-secondary)] border border-[var(--border)] focus:border-indigo-500 text-sm rounded-xl px-3 h-11 cursor-pointer touch-target"
            >
              <option value="">All OS</option>
              <option value="windows">Windows</option>
              <option value="linux">Linux</option>
              <option value="darwin">macOS</option>
            </select>
            <select
              value={linkedFilter}
              onChange={(e) => { setLinkedFilter(e.target.value as "" | "direct" | "chained"); setPage(1); }}
              className="flex-1 sm:flex-none bg-[var(--card-bg-secondary)] border border-[var(--border)] focus:border-indigo-500 text-sm rounded-xl px-3 h-11 cursor-pointer touch-target"
            >
              <option value="">All Links</option>
              <option value="direct">Direct C2</option>
              <option value="chained">P2P Chained</option>
            </select>
          </div>
        </div>
      </div>

      {/* Table */}
      <div className="ui-card sm:rounded-3xl overflow-x-auto shadow-sm">
        <table className="w-full text-sm min-w-[750px] table-responsive">
          <thead className="border-b border-[var(--border)] bg-slate-50 dark:bg-slate-800">
            <tr className="text-xs text-[var(--text-secondary)] font-semibold uppercase tracking-wider">
              <th className="text-left py-3 px-4 sm:py-3.5 sm:px-5 w-10">
                <input
                  type="checkbox"
                  className="w-5 h-5 accent-indigo-600 rounded cursor-pointer"
                  checked={beacons.length > 0 && selected.size === beacons.length}
                  onChange={(e) => toggleSelectAll(e.target.checked)}
                />
              </th>
              <th className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("hostname")}>
                Hostname <i className={`fa-solid ${sortIcon("hostname")} ml-1 text-[10px]`}></i>
              </th>
              <th className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("username")}>
                User <i className={`fa-solid ${sortIcon("username")} ml-1 text-[10px]`}></i>
              </th>
              <th className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("os")}>
                OS <i className={`fa-solid ${sortIcon("os")} ml-1 text-[10px]`}></i>
              </th>
              <th className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("ip")}>
                IP <i className={`fa-solid ${sortIcon("ip")} ml-1 text-[10px]`}></i>
              </th>
              <th className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("last_seen")}>
                Last Seen <i className={`fa-solid ${sortIcon("last_seen")} ml-1 text-[10px]`}></i>
              </th>
              <th className="text-left py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">Window</th>
              <th className="text-center py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">Tasks</th>
              <th className="text-center py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("status")}>
                Status <i className={`fa-solid ${sortIcon("status")} ml-1 text-[10px]`}></i>
              </th>
              <th className="text-right py-3 px-4 sm:py-3.5 sm:px-5">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
            {loading && Array.from({ length: 6 }).map((_, i) => (
              <tr key={`skel-${i}`}>
                {Array.from({ length: 10 }).map((_, j) => (
                  <td key={j} className="py-3 px-3 sm:py-3.5 sm:px-4">
                    <div className="h-4 bg-slate-200 dark:bg-slate-700 rounded animate-pulse w-3/4"></div>
                  </td>
                ))}
              </tr>
            ))}
            {!loading && sortedBeacons.map((beacon) => {
              const id = beacon.id || "";
              const hostname = beacon.hostname || "-";
              const username = beacon.username || "-";
              const ip = beacon.ip || "-";
              const os = beacon.os || "";
              const arch = beacon.arch || "";
              const status = beacon.status || "offline";
              const lastSeen = beacon.last_seen || "";
              const integrity = beacon.integrity || "";
              const elevated = beacon.elevated || false;
              const notes = beacon.notes || "";
              const activeWindow = beacon.active_window || "";

              const borderLeft = status === "online" ? "border-l-2 border-l-emerald-500" :
                status === "stale" ? "border-l-2 border-l-amber-500" : "border-l-2 border-l-red-500";
              const osIcon = os.toLowerCase() === "windows" ? "fa-brands fa-windows" :
                os.toLowerCase() === "linux" ? "fa-brands fa-linux" : "fa-brands fa-apple";
              const statusBg = status === "online" ? "bg-emerald-50 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400" :
                status === "stale" ? "bg-amber-50 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400" :
                "bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-400";
              const statusLabel = status === "online" ? "Online" : status === "stale" ? "Stale" : "Offline";

              return (
                <tr
                  key={id}
                  className={`hover:bg-indigo-50/50 dark:hover:bg-indigo-900/20 transition-colors cursor-pointer ${borderLeft} even:bg-slate-50/50 dark:even:bg-slate-800/30`}
                  onClick={() => router.push(`/agents/${id}`)}
                >
                  <td className="py-3 px-4 sm:py-3.5 sm:px-5" data-label="">
                    <input
                      type="checkbox"
                      className="w-5 h-5 accent-indigo-600 rounded cursor-pointer"
                      checked={selected.has(id)}
                      onChange={(e) => toggleSelect(id, e.target.checked)}
                      onClick={(e) => e.stopPropagation()}
                    />
                  </td>
                  <td className="py-3 px-3 sm:py-3.5 sm:px-4" data-label="Hostname">
                    <div className="flex items-center gap-2">
                      <span className={`w-7 h-7 rounded-lg ${avatarColor(hostname)} flex items-center justify-center text-white text-xs font-bold shrink-0`}>
                        {avatarInitial(hostname)}
                      </span>
                      <div className="min-w-0">
                        <span className="font-semibold text-indigo-600 dark:text-indigo-400 hover:underline">{hostname}</span>
                        {notes && <span className="ml-1.5" title={notes}><i className="fa-solid fa-note-sticky text-amber-500 text-xs"></i></span>}
                        {beacon.parent_id && (
                          <span className="ml-1 text-[10px] text-purple-600" title="P2P chained">
                            <i className="fa-solid fa-link"></i>
                          </span>
                        )}
                      </div>
                    </div>
                  </td>
                  <td className="py-3 px-3 sm:py-3.5 sm:px-4 text-[var(--text-secondary)] text-xs font-mono font-medium" data-label="User">{username}</td>
                  <td className="py-3 px-3 sm:py-3.5 sm:px-4" data-label="OS">
                    <div className="flex items-center gap-1 flex-wrap">
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 text-[11px] font-medium bg-[var(--card-bg-secondary)] rounded-lg whitespace-nowrap">
                        <i className={`${osIcon} text-[var(--text-secondary)] text-[10px]`}></i>
                        {os}{arch ? ` ${arch}` : ""}
                      </span>
                      {integrity && (
                        <span className={`text-[10px] px-1.5 py-0.5 rounded-md font-semibold ${
                          integrity === "System" ? "bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-400" :
                          integrity === "High" ? "bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-400" :
                          integrity === "Medium" ? "bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-400" :
                          integrity === "Low" ? "bg-orange-100 dark:bg-orange-900/40 text-orange-700 dark:text-orange-400" :
                          "bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-400"
                        }`}>{integrity}</span>
                      )}
                      {elevated && <span className="text-[10px] px-1.5 py-0.5 bg-red-100 dark:bg-red-900/40 text-red-600 dark:text-red-400 rounded-md font-bold" title="Elevated"><i className="fa-solid fa-shield-halved"></i></span>}
                    </div>
                  </td>
                  <td className="py-3 px-3 sm:py-3.5 sm:px-4" data-label="IP">
                    <span className="font-mono text-xs text-[var(--text-secondary)]">{ip}</span>
                  </td>
                  <td className="py-3 px-3 sm:py-3.5 sm:px-4 text-xs text-slate-600 dark:text-[var(--text-secondary)] font-mono whitespace-nowrap" data-label="Last Seen">
                    {lastSeen ? new Date(lastSeen).toLocaleTimeString() : "-"}
                  </td>
                  <td className="py-3 px-3 sm:py-3.5 sm:px-4 text-xs text-[var(--text-primary)] max-w-[140px] max-sm:hidden" data-label="Window">
                    {activeWindow ? (
                      <span className="inline-flex items-center gap-1 truncate" title={activeWindow}>
                        <i className="fa-solid fa-window-maximize text-[var(--text-tertiary)] text-[10px] shrink-0"></i>
                        <span className="truncate">{activeWindow}</span>
                      </span>
                    ) : <span className="text-[var(--text-tertiary)] dark:text-[var(--text-secondary)]"></span>}
                  </td>
                  <td className="py-3 px-3 sm:py-3.5 sm:px-4 text-center max-sm:hidden" data-label="Tasks">
                    <span className="text-xs font-mono text-[var(--text-primary)]">{taskCountMap[id] ?? 0}</span>
                  </td>
                  <td className="py-3 px-3 sm:py-3.5 sm:px-4 text-center" data-label="Status">
                    <span className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-lg text-[11px] font-semibold ${statusBg}`}>
                      <span className={`w-1.5 h-1.5 rounded-full ${status === "online" ? "bg-emerald-500" : status === "stale" ? "bg-amber-500" : "bg-red-500"} ${status === "online" ? "animate-pulse" : ""}`}></span>
                      {statusLabel}
                    </span>
                  </td>
                  <td className="py-3 px-4 sm:py-3.5 sm:px-5 text-right" data-label="Actions">
                    <div className="flex items-center justify-end gap-1">
                      <Link
                        href={`/agents/${id}`}
                        className="w-9 h-9 sm:w-8 sm:h-8 flex items-center justify-center bg-[var(--card-bg-secondary)] hover:bg-indigo-100 dark:hover:bg-indigo-900/40 hover:text-indigo-700 dark:hover:text-indigo-400 rounded-lg transition-all text-[var(--text-secondary)] touch-target"
                        title="Details"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <i className="fa-solid fa-eye text-sm"></i>
                      </Link>
                      <Link
                        href={`/agents/${id}/files`}
                        className="w-9 h-9 sm:w-8 sm:h-8 flex items-center justify-center bg-[var(--card-bg-secondary)] hover:bg-blue-100 dark:hover:bg-blue-900/40 hover:text-blue-700 dark:hover:text-blue-400 rounded-lg transition-all text-[var(--text-secondary)] touch-target"
                        title="Files"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <i className="fa-solid fa-folder-open text-sm"></i>
                      </Link>
                      <Link
                        href={`/agents/${id}/shell`}
                        className="w-9 h-9 sm:w-8 sm:h-8 flex items-center justify-center bg-[var(--card-bg-secondary)] hover:bg-emerald-100 dark:hover:bg-emerald-900/40 hover:text-emerald-700 dark:hover:text-emerald-400 rounded-lg transition-all text-[var(--text-secondary)] touch-target"
                        title="Shell"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <i className="fa-solid fa-terminal text-sm"></i>
                      </Link>
                      <button
                        className="w-9 h-9 sm:w-8 sm:h-8 flex items-center justify-center bg-[var(--card-bg-secondary)] hover:bg-orange-100 dark:hover:bg-orange-900/40 hover:text-orange-700 dark:hover:text-orange-400 rounded-lg transition-all text-[var(--text-secondary)] touch-target"
                        title="Disconnect"
                        onClick={(e) => { e.stopPropagation(); setConfirm({ type: "kill", id, hostname }); }}
                      >
                        <i className="fa-solid fa-power-off text-sm"></i>
                      </button>
                      <button
                        className="w-9 h-9 sm:w-8 sm:h-8 flex items-center justify-center bg-[var(--card-bg-secondary)] hover:bg-red-100 dark:hover:bg-red-900/40 hover:text-red-700 dark:hover:text-red-400 rounded-lg transition-all text-[var(--text-secondary)] touch-target"
                        title="Delete"
                        onClick={(e) => { e.stopPropagation(); setConfirm({ type: "delete", id, hostname }); }}
                      >
                        <i className="fa-solid fa-trash text-sm"></i>
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })}
            {!loading && beacons.length === 0 && (
              <tr>
                <td colSpan={9} className="py-16 sm:py-20">
                  <div className="text-center">
                    <i className="fa-solid fa-satellite-dish text-5xl sm:text-6xl text-slate-300 dark:text-slate-600 mb-4"></i>
                    <h3 className="text-base sm:text-lg font-medium text-[var(--text-secondary)] mb-2">No beacons connected</h3>
                    <p className="text-[var(--text-secondary)] mb-4 text-sm">
                      {search || statusFilter || osFilter
                        ? "No beacons match the current filters."
                        : "Deploy your first implant to establish a beacon connection."}
                    </p>
                    {!search && !statusFilter && !osFilter && (
                      <Link href="/generate" className="inline-flex items-center gap-2 px-4 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl transition-colors touch-target">
                        <i className="fa-solid fa-plus"></i>
                        <span>Generate Implant</span>
                      </Link>
                    )}
                  </div>
                </td>
              </tr>
            )}
          </tbody>
        </table>
        <div className="sm:hidden px-4 py-2 text-center text-xs text-[var(--text-tertiary)] dark:text-[var(--text-secondary)] border-t border-[var(--border)] bg-slate-50 dark:bg-slate-800/50">
          <i className="fa-solid fa-arrows-left-right mr-1"></i> Swipe for more columns
        </div>

        <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} />
      </div>

      <ConfirmModal
        open={confirm?.type === "kill"}
        title="Disconnect Agent"
        message={`Send kill command to ${confirm?.hostname || confirm?.id}? Agent will exit on next beacon.`}
        confirmText="Disconnect"
        danger
        onConfirm={() => confirm?.id && killAgent(confirm.id)}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "delete"}
        title="Delete Agent"
        message={`Permanently delete ${confirm?.hostname || confirm?.id} and all task history?`}
        confirmText="Delete"
        danger
        onConfirm={() => confirm?.id && deleteAgent(confirm.id)}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "batch-delete"}
        title="Batch Delete"
        message={`Delete ${selected.size} selected agents and their task history?`}
        confirmText="Delete All"
        danger
        onConfirm={batchDelete}
        onCancel={() => setConfirm(null)}
      />
    </div>
  );
}
