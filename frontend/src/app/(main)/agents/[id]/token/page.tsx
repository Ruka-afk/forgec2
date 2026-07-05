"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams } from "next/navigation";
import { API_BASE } from "@/lib/constants";

interface Token {
  id?: number;
  ID?: string;
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
  CreatedAt?: string;
  created_at?: string;
  TokenType?: string;
  token_type?: string;
  Protocol?: string;
  protocol?: string;
  Note?: string;
  note?: string;
}

interface Process {
  pid: number;
  name: string;
  user?: string;
}


export default function AgentTokenPage() {
  const params = useParams();
  const agentId = params.id as string;
  const [tokens, setTokens] = useState<Token[]>([]);
  const [processes, setProcesses] = useState<Process[]>([]);
  const [loading, setLoading] = useState(true);
  const [stealPid, setStealPid] = useState("");
  const [makeUser, setMakeUser] = useState("");
  const [makeDomain, setMakeDomain] = useState("");
  const [makePass, setMakePass] = useState("");
  const [activeAction, setActiveAction] = useState<string | null>(null);
  const [toast, setToast] = useState<{ text: string; type: string } | null>(null);
  const [whoamiResult, setWhoamiResult] = useState<string | null>(null);
  const [tokenNotes, setTokenNotes] = useState<Record<string, string>>({});
  const [noteTargetId, setNoteTargetId] = useState<string | null>(null);

  const showToast = useCallback((text: string, type: string = "info") => {
    setToast({ text, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const loadTokens = useCallback(async () => {
    try {
      const r = await fetch(`${API_BASE}?p=/agents/${agentId}/token/list&format=json`);
      if (r.ok) {
        const data = await r.json();
        setTokens(data.Tokens || data.tokens || []);
      }
    } catch (e) { console.error("Token: list tokens failed", e); }
  }, [agentId]);

  const loadProcesses = useCallback(async () => {
    try {
      const r = await fetch(`${API_BASE}?p=/agents/${agentId}/token/list_procs`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "",
      });
      if (r.ok) {
        const data = await r.json();
        setProcesses(data.processes || data.data || data || []);
      }
    } catch (e) { console.error("Token: list processes failed", e); }
  }, [agentId]);

  useEffect(() => {
    Promise.resolve().then(() => {
      setLoading(true);
      Promise.all([loadTokens(), loadProcesses()]).finally(() => setLoading(false));
    });
  }, [loadTokens, loadProcesses]);

  const handleStealToken = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!stealPid) return;
    setActiveAction("steal");
    try {
      const body = new URLSearchParams();
      body.append("pid", stealPid);
      await fetch(`${API_BASE}?p=/agents/${agentId}/token/steal`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),      });
      setStealPid("");
      showToast("Token steal task dispatched", "success");
      await loadTokens();
    } catch {
      showToast("Failed to steal token", "error");
    } finally {
      setActiveAction(null);
    }
  };  const handleMakeToken = async (e: React.FormEvent) => {    e.preventDefault();
    if (!makeUser) return;
    setActiveAction("make");
    try {
      const body = new URLSearchParams();      body.append("username", makeUser);
      body.append("domain", makeDomain);
      body.append("password", makePass);
      await fetch(`${API_BASE}?p=/agents/${agentId}/token/make`, {        method: "POST",        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });      setMakeUser(""); setMakeDomain(""); setMakePass("");
      showToast("Token creation task dispatched", "success");
      await loadTokens();
    } catch {
      showToast("Failed to create token", "error");    } finally {
      setActiveAction(null);
    }
  };

  const handleRevert = async () => {
    setActiveAction("revert");
    try {
      await fetch(`${API_BASE}?p=/agents/${agentId}/token/revert`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "",
      });
      showToast("Token reverted to self", "success");
      setWhoamiResult(null);
      await loadTokens();
    } catch {
      showToast("Failed to revert token", "error");    } finally {
      setActiveAction(null);
    }
  };  const handleDrop = async (tokenId: string) => {
    setActiveAction(`drop-${tokenId}`);
    try {
      await fetch(`${API_BASE}?p=/agents/${agentId}/token/${tokenId}`, {
        method: "DELETE",
      });
      showToast("Token dropped", "success");
      await loadTokens();
    } catch {
      showToast("Failed to drop token", "error");
    } finally {
      setActiveAction(null);
    }
  };

  const handleImpersonate = async (tokenId: string) => {
    setActiveAction(`impersonate-${tokenId}`);    try {
      await fetch(`${API_BASE}?p=/agents/${agentId}/token/${tokenId}/impersonate`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "",
      });      showToast("Impersonation active", "success");
      await loadTokens();
    } catch {
      showToast("Failed to impersonate", "error");
    } finally {      setActiveAction(null);    }
  };

  const handleWhoami = async () => {
    setActiveAction("whoami");
    try {
      const r = await fetch(`${API_BASE}?p=/agents/${agentId}/token/whoami`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "",
      });
      if (r.ok) {
        const data = await r.json();
        setWhoamiResult(data.user || data.username || data.name || JSON.stringify(data));        showToast(`Identity: ${data.user || data.username || "unknown"}`, "info");
      } else {
        const text = await r.text();
        setWhoamiResult(`Error ${r.status}: ${text}`);
        showToast("Whoami failed", "error");
      }
    } catch (err) {
      setWhoamiResult(`Error: ${err}`);
      showToast("Whoami failed", "error");    } finally {      setActiveAction(null);    }
  };

  const handleNote = async (tokenId: string) => {
    const noteText = tokenNotes[tokenId];    setActiveAction(`note-${tokenId}`);
    try {
      const body = new URLSearchParams();
      body.append("note", noteText || "");
      await fetch(`${API_BASE}?p=/agents/${agentId}/token/${tokenId}/note`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      showToast(noteText ? "Note saved" : "Note cleared", "success");
      await loadTokens();
    } catch {
      showToast("Failed to save note", "error");
    } finally {
      setActiveAction(null);
      setNoteTargetId(null);
    }  };

  const getIntegrityBadge = (integrity: string) => {
    const colors: Record<string, string> = {
      System: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400 border border-red-200 dark:border-red-800",
      High: "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-400 border border-orange-200 dark:border-orange-800",
      Medium: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400 border border-yellow-200 dark:border-yellow-800",
      Low: "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300 border border-[var(--border)]",
    };
    return <span className={`px-2.5 py-1 text-xs font-semibold rounded-lg ${colors[integrity] || colors.Medium}`}>{integrity}</span>;
  };

  const getTokenTypeBadge = (source: string, tokenType?: string, protocol?: string) => {
    const type = tokenType || protocol || source;    const icons: Record<string, string> = {
      steal: "fa-user-ninja",
      make: "fa-plus-circle",
      named_pipe: "fa-circle-nodes",
      impersonate: "fa-user-secret",      create: "fa-key",
    };
    const icon = icons[type] || "fa-id-badge";
    const labels: Record<string, string> = {
      steal: "Named Pipe (Stolen)",
      make: "Logon (Created)",      named_pipe: "Named Pipe",
      impersonate: "Impersonation",
      create: "Logon Create",
    };
    const label = labels[type] || type;
    return (      <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-lg ${        source === "steal" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-400 border border-amber-200 dark:border-amber-800" :
        "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800"
      }`}>
        <i className={`fa-solid ${icon}`}></i>
        {label}
      </span>
    );
  };  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0 space-y-6">
      {toast && (
        <div className={`fixed top-4 right-4 z-50 px-4 py-3 rounded-xl text-sm font-medium shadow-lg ${          toast.type === "success" ? "bg-emerald-600 text-white" :          toast.type === "error" ? "bg-red-600 text-white" :          "bg-blue-600 text-white"
        }`}>
          {toast.text}
        </div>
      )}

      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">
            <i className="fa-solid fa-id-badge text-indigo-500 mr-2"></i>Token Operations
          </h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs mt-1">Manage tokens for agent {agentId.substring(0, 12)}</p>
        </div>
        <div className="flex items-center gap-2">          <button            onClick={handleWhoami}            disabled={activeAction === "whoami"}
            className="px-4 py-2 bg-slate-100 hover:bg-slate-200 disabled:opacity-50 text-slate-700 dark:bg-slate-700 dark:hover:bg-slate-600 dark:text-slate-200 rounded-xl text-xs font-medium transition-colors flex items-center gap-1.5"
          >
            {activeAction === "whoami" ? (
              <><i className="fa-solid fa-spinner fa-spin"></i> Checking...</>
            ) : (
              <><i className="fa-solid fa-user-check"></i> Whoami</>            )}
          </button>
          <button
            onClick={() => { setLoading(true); Promise.all([loadTokens(), loadProcesses()]).finally(() => setLoading(false)); }}
            className="px-4 py-2 bg-indigo-100 hover:bg-indigo-200 text-indigo-700 dark:bg-indigo-900/40 dark:hover:bg-indigo-900/60 dark:text-indigo-400 rounded-xl text-xs font-medium transition-colors flex items-center gap-1.5"
          >
            <i className="fa-solid fa-rotate"></i> Refresh
          </button>
        </div>      </div>

      {whoamiResult && (
        <div className={`border rounded-xl px-4 py-3 text-sm flex items-center gap-2 ${
          whoamiResult.startsWith("Error")            ? "border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-900/20 text-red-700 dark:text-red-400"
            : "border-indigo-200 bg-indigo-50 dark:border-indigo-800 dark:bg-indigo-900/20 text-indigo-700 dark:text-indigo-400"
        }`}>
          <i className={`fa-solid ${whoamiResult.startsWith("Error") ? "fa-circle-exclamation" : "fa-user-check"}`}></i>
          <span>Current context: <strong>{whoamiResult}</strong></span>
          <button onClick={() => setWhoamiResult(null)} className="ml-auto text-xs opacity-60 hover:opacity-100">
            <i className="fa-solid fa-xmark"></i>
          </button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="ui-card p-5 shadow-sm">
          <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-3">            <i className="fa-solid fa-user-ninja text-amber-500 mr-2"></i>Steal Token
          </h2>
          <form onSubmit={handleStealToken} className="space-y-3">            <div>              <label className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Target Process (PID)</label>
              <select value={stealPid} onChange={(e) => setStealPid(e.target.value)} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-3 h-10 cursor-pointer dark:text-slate-100">
                <option value="">Select process to steal from...</option>
                {processes.map((p) => (
                  <option key={p.pid} value={p.pid}>[{p.pid}] {p.name}{p.user ? ` (${p.user})` : ""}</option>
                ))}
              </select>
            </div>
            <button type="submit" disabled={!stealPid || activeAction === "steal"} className="w-full py-2.5 bg-amber-600 hover:bg-amber-700 disabled:opacity-50 text-white text-sm font-medium rounded-xl transition-colors flex items-center justify-center gap-1.5">
              {activeAction === "steal" ? <><i className="fa-solid fa-spinner fa-spin"></i>Stealing...</> : <><i className="fa-solid fa-user-ninja"></i>Steal Token</>}
            </button>
          </form>
        </div>

        <div className="ui-card p-5 shadow-sm">          <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-3">            <i className="fa-solid fa-plus-circle text-emerald-500 mr-2"></i>Make Token
          </h2>
          <form onSubmit={handleMakeToken} className="space-y-3">
            <div>              <label className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Username <span className="text-red-400">*</span></label>
              <input type="text" value={makeUser} onChange={(e) => setMakeUser(e.target.value)} placeholder="Target username" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-3 h-10 focus:outline-none focus:border-emerald-500 dark:text-slate-100" />
            </div>
            <div>
              <label className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Domain</label>
              <input type="text" value={makeDomain} onChange={(e) => setMakeDomain(e.target.value)} placeholder="Domain (optional)" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-3 h-10 focus:outline-none focus:border-emerald-500 dark:text-slate-100" />            </div>
            <div>
              <label className="text-xs text-slate-500 dark:text-slate-400 mb-1 block">Password</label>
              <input type="password" value={makePass} onChange={(e) => setMakePass(e.target.value)} placeholder="Password" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-3 h-10 focus:outline-none focus:border-emerald-500 dark:text-slate-100" />
            </div>
            <button type="submit" disabled={!makeUser || activeAction === "make"} className="w-full py-2.5 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white text-sm font-medium rounded-xl transition-colors flex items-center justify-center gap-1.5">
              {activeAction === "make" ? <><i className="fa-solid fa-spinner fa-spin"></i>Creating...</> : <><i className="fa-solid fa-plus"></i>Create Token</>}
            </button>
          </form>
        </div>        <div className="ui-card p-5 shadow-sm">
          <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-3">
            <i className="fa-solid fa-rotate-left text-indigo-500 mr-2"></i>Quick Actions          </h2>
          <div className="space-y-3">            <button onClick={handleRevert} disabled={activeAction === "revert"} className="w-full py-2.5 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-medium rounded-xl transition-colors flex items-center justify-center gap-1.5">
              {activeAction === "revert" ? <><i className="fa-solid fa-spinner fa-spin"></i>Reverting...</> : <><i className="fa-solid fa-rotate-left"></i>Revert to Self</>}
            </button>
            <button onClick={handleWhoami} disabled={activeAction === "whoami"} className="w-full py-2.5 bg-slate-600 hover:bg-slate-700 disabled:opacity-50 text-white text-sm font-medium rounded-xl transition-colors flex items-center justify-center gap-1.5">
              {activeAction === "whoami" ? <><i className="fa-solid fa-spinner fa-spin"></i>Querying...</> : <><i className="fa-solid fa-user-check"></i>Check Identity</>}
            </button>
            <div className="text-xs text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-700/50 rounded-xl p-3 flex items-start gap-2">
              <i className="fa-solid fa-info-circle text-indigo-500 mt-0.5"></i>
              <span>Impersonate which token is used by all agent operations. Use revert to return to original context.</span>
            </div>
          </div>        </div>
      </div>

      <div className="ui-card overflow-hidden">
        <div className="px-5 py-3 border-b border-[var(--border)] flex items-center justify-between">
          <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">
            <i className="fa-solid fa-list text-slate-400 mr-2"></i>Harvested Tokens ({tokens.length})
          </h2>          <div className="flex items-center gap-2">
            <div className="text-xs text-slate-400 dark:text-slate-500">
              <span className="inline-flex items-center gap-1.5">
                <span className="w-2 h-2 bg-amber-500 rounded-full animate-pulse"></span>                {tokens.filter(t => t.Active || t.active).length} active
              </span>
            </div>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-700/50 border-b border-[var(--border)]">
              <tr className="text-xs text-slate-500 dark:text-slate-400">
                <th className="text-left px-5 py-3 font-normal">User</th>                <th className="text-left px-4 py-3 font-normal">Integrity</th>                <th className="text-left px-4 py-3 font-normal">Type</th>
                <th className="text-left px-4 py-3 font-normal">Source</th>
                <th className="text-left px-4 py-3 font-normal">Process</th>                <th className="text-left px-4 py-3 font-normal">Status</th>
                <th className="text-left px-4 py-3 font-normal">Note</th>                <th className="text-left px-4 py-3 font-normal">Created</th>
                <th className="text-left px-4 py-3 font-normal">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-700">              {!loading && tokens.map((t) => {                const tid = t.id ? String(t.id) : t.ID || "";
                const domain = t.Domain || t.domain || "";                const username = t.Username || t.username || "";
                const integrity = t.Integrity || t.integrity || "Medium";
                const source = t.Source || t.source || "steal";
                const tokenType = t.TokenType || t.token_type;
                const protocol = t.Protocol || t.protocol;
                const pid = t.PID || t.pid;
                const procName = t.ProcessName || t.process_name;                const active = t.Active !== undefined ? t.Active : t.active;
                const createdAt = t.CreatedAt || t.created_at || "";
                const noteText = t.Note || t.note || tokenNotes[tid] || "";
                const isEditingNote = noteTargetId === tid;
                return (
                  <tr key={tid} className={`hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors ${active ? "bg-amber-50/30 dark:bg-amber-900/10" : ""}`}>
                    <td className="px-5 py-3">
                      <span className="font-semibold text-slate-900 dark:text-slate-100 text-sm">{domain ? `${domain}\\${username}` : username || "Unknown"}</span>
                    </td>
                    <td className="px-4 py-3">{getIntegrityBadge(integrity)}</td>
                    <td className="px-4 py-3">{getTokenTypeBadge(source, tokenType, protocol)}</td>                    <td className="px-4 py-3">
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${                        source === "steal" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" :
                        source === "make" || source === "create" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" :
                        "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300"
                      }`}>
                        {source}
                      </span>
                    </td>                    <td className="px-4 py-3 text-xs font-mono text-slate-500 dark:text-slate-400">{pid ? `[${pid}]` : ""} {procName || ""}</td>
                    <td className="px-4 py-3">
                      {active ? (
                        <span className="inline-flex items-center gap-1.5 text-xs text-amber-700 dark:text-amber-400 font-medium"><span className="w-2 h-2 bg-amber-500 rounded-full animate-pulse"></span>Active</span>
                      ) : (                        <span className="inline-flex items-center gap-1.5 text-xs text-slate-400 dark:text-slate-500"><span className="w-2 h-2 bg-slate-300 dark:bg-slate-600 rounded-full"></span>Inactive</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {isEditingNote ? (
                        <div className="flex items-center gap-1">                          <input
                            type="text"                            value={tokenNotes[tid] || ""}
                            onChange={(e) => setTokenNotes(prev => ({ ...prev, [tid]: e.target.value }))}
                            className="w-24 bg-[var(--card-bg)] border border-[var(--border)] rounded-lg px-2 py-1 text-xs dark:text-slate-100 focus:outline-none focus:border-indigo-500"
                            placeholder="Note..."
                            autoFocus
                          />
                          <button onClick={() => handleNote(tid)} className="text-emerald-500 hover:text-emerald-600 p-0.5">                            <i className="fa-solid fa-check text-xs"></i>                          </button>
                          <button onClick={() => setNoteTargetId(null)} className="text-slate-400 hover:text-slate-500 p-0.5">
                            <i className="fa-solid fa-xmark text-xs"></i>                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => { setNoteTargetId(tid); setTokenNotes(prev => ({ ...prev, [tid]: noteText })); }}
                          className="text-xs text-slate-400 dark:text-slate-500 hover:text-indigo-500 transition-colors flex items-center gap-1"
                          title="Edit note"
                        >                          <i className="fa-solid fa-pen-to-square"></i>                          {noteText ? <span className="truncate max-w-20">{noteText}</span> : <span className="text-slate-300 dark:text-slate-600">Add note</span>}
                        </button>
                      )}
                    </td>
                    <td className="px-4 py-3 text-xs font-mono text-slate-400 dark:text-slate-500">{createdAt ? new Date(createdAt).toLocaleString() : ""}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1">
                        <button onClick={() => handleImpersonate(tid)} disabled={!active || activeAction === `impersonate-${tid}`} className={`p-2 rounded-lg transition-colors flex items-center gap-1 ${active ? "text-amber-500 hover:text-amber-600 hover:bg-amber-50 dark:hover:bg-amber-900/20" : "text-slate-300 dark:text-slate-600 cursor-not-allowed"}`} title="Impersonate">                          {activeAction === `impersonate-${tid}` ? <i className="fa-solid fa-spinner fa-spin text-xs"></i> : <i className="fa-solid fa-user-secret text-xs"></i>}                        </button>
                        <button onClick={() => handleDrop(tid)} disabled={activeAction === `drop-${tid}`} className={`p-2 rounded-lg transition-colors flex items-center gap-1 text-red-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 ${activeAction === `drop-${tid}` ? "opacity-50" : ""}`} title="Drop Token">
                          <i className={`fa-solid ${activeAction === `drop-${tid}` ? "fa-spinner fa-spin" : "fa-trash"} text-xs`}></i>
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
              {loading && Array.from({ length: 3 }).map((_, i) => (<tr key={i}><td colSpan={8} className="py-3 px-4"><div className="h-8 bg-slate-100 dark:bg-slate-700 rounded animate-pulse"></div></td></tr>))}
              {!loading && tokens.length === 0 && (<tr><td colSpan={8} className="py-16 text-center text-slate-400"><i className="fa-solid fa-id-badge text-2xl mb-2 text-slate-300 dark:text-slate-600"></i><p className="text-sm">No tokens harvested yet</p></td></tr>)}
            </tbody>          </table>
        </div>      </div>
    </div>
  );
}