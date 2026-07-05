"use client";

import { useState, useEffect, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { PageHeader, SearchInput, Pagination } from "@/components/UI";

interface VaultEntry {
  id: string;
  domain: string;
  username: string;
  password: string;
  hash: string;
  type: string;
  source: string;
  tags: string;
  expires_at: string;
  confirmed: boolean;
  agent_id: string;
  notes: string;
}

interface CredentialData {
  VaultEntries: VaultEntry[];
  VaultCount: number;
  AllTags: string[];
}


const CRED_TYPES = ["all", "password", "hash", "token", "key", "ntlm", "kerberos", "cleartext"];

const TYPE_COLORS: Record<string, string> = {
  cleartext: "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400",
  password: "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400",
  ntlm: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
  hash: "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400",
  token: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
  key: "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400",
  kerberos: "bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400",
  sha1: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400",
};

export default function CredentialsPage() {
  const [data, setData] = useState<CredentialData | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");
  const [confirmedFilter, setConfirmedFilter] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [showAddModal, setShowAddModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showBatchModal, setShowBatchModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState<string | null>(null);
  const [editTarget, setEditTarget] = useState<VaultEntry | null>(null);
  const [toast, setToast] = useState<{ text: string; type: string } | null>(null);
  const [showPasswords, setShowPasswords] = useState<Set<string>>(new Set());

  const [form, setForm] = useState({
    domain: "",
    username: "",
    password: "",
    hash: "",
    type: "cleartext",
    source: "",
    tags: "",
    notes: "",
  });

  const [batchTags, setBatchTags] = useState("");

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}?p=/credentials&format=json`);
      if (res.ok) {
        const result = await res.json();
        setData({
          VaultEntries: result.VaultEntries || result.vault_entries || [],
          VaultCount: result.VaultCount || result.vault_count || result.VaultEntries?.length || 0,
          AllTags: result.AllTags || result.all_tags || [],
        });
      }
    } catch {
      setData({ VaultEntries: [], VaultCount: 0, AllTags: [] });
    }
    setLoading(false);
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadData()); }, [loadData]);

  const showToastNotify = (text: string, type: string = "info") => {
    setToast({ text, type });
    setTimeout(() => setToast(null), 3000);
  };

  const postAPI = async (path: string, params: Record<string, string>) => {
    const body = new URLSearchParams(params);
    return fetch(`${API_BASE}?p=${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString(),
    });
  };

  const handleAdd = async () => {
    if (!form.username) return showToastNotify("Username is required", "error");
    try {
      const res = await postAPI("/credentials/add", {
        domain: form.domain,
        username: form.username,
        password: form.password,
        hash: form.hash,
        type: form.type,
        source: form.source || "manual",
        tags: form.tags,
        notes: form.notes,
      });
      if (res.ok) {
        showToastNotify("Credential added successfully", "success");
        setShowAddModal(false);
        resetForm();
        loadData();
      } else {
        showToastNotify(`Add failed: ${res.status}`, "error");
      }
    } catch (err) {
      showToastNotify(String(err), "error");
    }
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    try {
      const { apiPut } = await import("@/lib/api");
      const body = new URLSearchParams();
      if (form.tags) body.set("tags", form.tags);
      if (form.notes) body.set("notes", form.notes);
      await apiPut(`/credentials/${editTarget.id}`, body);
      showToastNotify("Credential updated", "success");
      setShowEditModal(false);
      setEditTarget(null);
      resetForm();
      loadData();
    } catch (err) {
      showToastNotify(String(err), "error");
    }
  };

  const handleDelete = async (id: string) => {
    try {
      const { apiDelete } = await import("@/lib/api");
      await apiDelete(`/credentials/${id}`);
      showToastNotify("Credential deleted", "success");
      setShowDeleteConfirm(null);
      loadData();
    } catch (err) {
      showToastNotify(String(err), "error");
    }
  };

  const handleToggleConfirm = async (entry: VaultEntry) => {
    try {
      await fetch(`${API_BASE}?p=/credentials/${entry.id}/confirm&format=json`, {
        method: "POST",
        credentials: "include",
      });
      loadData();
    } catch (e) { console.error("Credentials: toggle confirm failed", e); }
  };

  const handleBatchTags = async () => {
    if (!batchTags || selectedIds.size === 0) return;
    try {
      const { apiPostJson } = await import("@/lib/api");
      await apiPostJson("/credentials/batch/tags", {
        ids: Array.from(selectedIds).map((id) => Number(id)),
        tags: batchTags.split(",").map((t) => t.trim()).filter(Boolean),
      });
      showToastNotify(`Tags added to ${selectedIds.size} credentials`, "success");
      setShowBatchModal(false);
      setBatchTags("");
      setSelectedIds(new Set());
      loadData();
    } catch (err) {
      showToastNotify(String(err), "error");
    }
  };

  const exportCSV = () => {
    const headers = ["Type", "Domain", "Username", "Password", "Hash", "Source", "Tags", "Confirmed", "Notes"];
    const rows = filteredEntries.map(e => [
      e.type,
      e.domain || "",
      e.username,
      e.password || "",
      e.hash || "",
      e.source || "",
      e.tags || "",
      e.confirmed ? "Yes" : "No",
      e.notes || "",
    ]);
    const csv = [headers, ...rows].map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(",")).join("\n");
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `credentials_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
    showToastNotify("CSV exported", "success");
  };

  const openEdit = (entry: VaultEntry) => {
    setEditTarget(entry);
    setForm({
      domain: entry.domain || "",
      username: entry.username || "",
      password: entry.password || "",
      hash: entry.hash || "",
      type: entry.type || "cleartext",
      source: entry.source || "",
      tags: entry.tags || "",
      notes: entry.notes || "",
    });
    setShowEditModal(true);
  };

  const resetForm = () => {
    setForm({
      domain: "",
      username: "",
      password: "",
      hash: "",
      type: "cleartext",
      source: "",
      tags: "",
      notes: "",
    });
  };

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selectedIds.size === filteredEntries.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(filteredEntries.map(e => e.id)));
    }
  };

  const togglePasswordVisibility = (id: string) => {
    setShowPasswords(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const filteredEntries = data?.VaultEntries?.filter(entry => {
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      if (
        !entry.username.toLowerCase().includes(q) &&
        !entry.domain?.toLowerCase().includes(q) &&
        !entry.notes?.toLowerCase().includes(q)
      ) return false;
    }
    if (typeFilter !== "all" && entry.type !== typeFilter) return false;
    if (confirmedFilter === "true" && !entry.confirmed) return false;
    if (confirmedFilter === "false" && entry.confirmed) return false;
    return true;
  }) || [];

  const entries = data?.VaultEntries || [];
  const stats = {
    total: data?.VaultCount || 0,
    confirmed: entries.filter(e => e.confirmed).length || 0,
    unconfirmed: entries.filter(e => !e.confirmed).length || 0,
    byType: CRED_TYPES.slice(1).map(t => ({
      type: t,
      count: entries.filter(e => e.type === t).length || 0,
    })),
  };

  return (
    <>
      {toast && (
        <div className={`fixed top-4 right-4 z-50 px-4 py-3 rounded-xl text-sm font-medium shadow-lg ${
          toast.type === "success" ? "bg-emerald-600 text-white" :
          toast.type === "error" ? "bg-red-600 text-white" :
          "bg-blue-600 text-white"
        }`}>
          {toast.text}
        </div>
      )}

      <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title="Credentials" subtitle="Credential vault and management">
        {filteredEntries.length > 0 && (
          <button
            onClick={exportCSV}
            className="bg-emerald-600 hover:bg-emerald-500 text-white px-4 h-9 rounded-xl text-sm font-medium flex items-center gap-x-2 transition-colors"
          >
            <i className="fa-solid fa-download text-xs"></i>
            <span>Export CSV</span>
          </button>
        )}
        <button
          onClick={() => { resetForm(); setShowAddModal(true); }}
          className="bg-indigo-600 hover:bg-indigo-500 text-white px-4 h-9 rounded-xl text-sm font-medium flex items-center gap-x-2 transition-colors"
        >
          <i className="fa-solid fa-plus text-xs"></i>
          <span>Add Credential</span>
        </button>
      </PageHeader>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <div className="ui-card p-4">
          <div className="text-2xl font-bold">{stats.total}</div>
          <div className="text-xs text-gray-500 mt-1">Total Credentials</div>
        </div>
        <div className="ui-card p-4">
          <div className="text-2xl font-bold text-emerald-500">{stats.confirmed}</div>
          <div className="text-xs text-gray-500 mt-1">Confirmed</div>
        </div>
        <div className="ui-card p-4">
          <div className="text-2xl font-bold text-amber-500">{stats.unconfirmed}</div>
          <div className="text-xs text-gray-500 mt-1">Unconfirmed</div>
        </div>
        <div className="ui-card p-4">
          <div className="flex flex-wrap gap-1 mt-1">
            {stats.byType.map(s => s.count > 0 && (
              <span key={s.type} className={`px-2 py-0.5 text-[10px] rounded-full ${TYPE_COLORS[s.type] || "bg-slate-100 text-slate-600"}`}>
                {s.type}: {s.count}
              </span>
            ))}
          </div>
          <div className="text-xs text-gray-500 mt-1">By Type</div>
        </div>
      </div>

      <div className="ui-card p-4 mb-6">
        <div className="flex flex-wrap items-center gap-3">
          <SearchInput value={searchQuery} onChange={setSearchQuery} placeholder="Search username, domain, notes..." className="flex-1 min-w-[200px]" />
          <select
            value={typeFilter}
            onChange={e => setTypeFilter(e.target.value)}
            className="bg-slate-50 dark:bg-slate-700/50 border border-[var(--border)] text-xs rounded-xl px-3 py-2 focus:outline-none focus:border-indigo-500 dark:text-slate-100"
          >
            {CRED_TYPES.map(t => (
              <option key={t} value={t}>{t === "all" ? "All Types" : t.charAt(0).toUpperCase() + t.slice(1)}</option>
            ))}
          </select>
          <select
            value={confirmedFilter}
            onChange={e => setConfirmedFilter(e.target.value)}
            className="bg-slate-50 dark:bg-slate-700/50 border border-[var(--border)] text-xs rounded-xl px-3 py-2 focus:outline-none focus:border-indigo-500 dark:text-slate-100"
          >
            <option value="">All Status</option>
            <option value="true">Confirmed</option>
            <option value="false">Unconfirmed</option>
          </select>
          <button
            onClick={loadData}
            className="bg-indigo-600 hover:bg-indigo-500 text-white px-4 h-9 rounded-xl text-sm font-medium transition-colors"
          >
            <i className="fa-solid fa-filter mr-1"></i>Filter
          </button>
          <button
            onClick={() => { setSearchQuery(""); setTypeFilter("all"); setConfirmedFilter(""); }}
            className="bg-slate-100 hover:bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-300 dark:hover:bg-slate-600 px-4 h-9 rounded-xl text-sm font-medium transition-colors"
          >
            Clear
          </button>
          {selectedIds.size > 0 && (
            <button
              onClick={() => setShowBatchModal(true)}
              className="bg-amber-600 hover:bg-amber-500 text-white px-4 h-9 rounded-xl text-sm font-medium flex items-center gap-x-2 transition-colors"
            >
              <i className="fa-solid fa-tags text-xs"></i>
              <span>Batch Tags ({selectedIds.size})</span>
            </button>
          )}
        </div>
      </div>

      {data?.AllTags && data.AllTags.length > 0 && (
        <div className="ui-card p-4 mb-6">
          <div className="font-medium text-sm text-slate-700 dark:text-slate-200 flex items-center gap-2 mb-3">
            <i className="fa-solid fa-tags text-indigo-500"></i>
            <span>Tags</span>
          </div>
          <div className="flex flex-wrap gap-2">
            {data.AllTags.map(tag => (
              <button
                key={tag}
                onClick={() => setSearchQuery(tag)}
                className="px-3 py-1 text-xs rounded-full transition-colors bg-indigo-100 text-indigo-700 hover:bg-indigo-200 dark:bg-indigo-900/30 dark:text-indigo-300"
              >
                {tag}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="ui-card overflow-hidden mb-8">
        <div className="px-6 py-4 border-b border-[var(--border)] flex items-center justify-between">
          <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2">
            <i className="fa-solid fa-vault text-indigo-500"></i>
            <span>Credential Vault</span>
            {filteredEntries.length > 0 && (
              <span className="px-2 py-0.5 bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300 text-xs rounded-full font-mono">
                {filteredEntries.length}
              </span>
            )}
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-center text-slate-400">
            <i className="fa-solid fa-spinner fa-spin text-2xl mb-2"></i>
            <br />Loading...
          </div>
        ) : filteredEntries.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 dark:bg-slate-700/50 border-b border-[var(--border)]">
                <tr className="text-xs text-slate-500 dark:text-slate-400">
                  <th className="text-left py-3 px-2 font-normal">
                    <input
                      type="checkbox"
                      onChange={toggleSelectAll}
                      checked={selectedIds.size === filteredEntries.length && filteredEntries.length > 0}
                      className="rounded border-slate-300"
                    />
                  </th>
                  <th className="text-left py-3 px-4 font-normal">Type</th>
                  <th className="text-left py-3 px-4 font-normal">Username</th>
                  <th className="text-left py-3 px-4 font-normal">Password</th>
                  <th className="text-left py-3 px-4 font-normal">Domain</th>
                  <th className="text-left py-3 px-4 font-normal">Source</th>
                  <th className="text-left py-3 px-4 font-normal">Confirmed</th>
                  <th className="text-left py-3 px-4 font-normal">Tags</th>
                  <th className="text-center py-3 px-4 font-normal">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                {filteredEntries.map(entry => (
                  <tr key={entry.id} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                    <td className="py-3 px-2">
                      <input
                        type="checkbox"
                        checked={selectedIds.has(entry.id)}
                        onChange={() => toggleSelect(entry.id)}
                        className="rounded border-slate-300"
                      />
                    </td>
                    <td className="py-3 px-4">
                      <span className={`px-2 py-0.5 text-[10px] rounded-full font-medium ${TYPE_COLORS[entry.type] || "bg-slate-100 text-slate-600 dark:bg-slate-600 dark:text-slate-300"}`}>
                        {entry.type || "unknown"}
                      </span>
                    </td>
                    <td className="py-3 px-4 font-medium text-slate-900 dark:text-slate-100">
                      {entry.username}
                    </td>
                    <td className="py-3 px-4 font-mono text-xs">
                      {entry.password ? (
                        <div className="flex items-center gap-1">
                          <span className="text-slate-600 dark:text-slate-300">
                            {showPasswords.has(entry.id) ? entry.password : "????????"}
                          </span>
                          <button
                            onClick={() => togglePasswordVisibility(entry.id)}
                            className="text-slate-400 hover:text-slate-600 ml-1"
                          >
                            <i className={`fa-solid ${showPasswords.has(entry.id) ? "fa-eye-slash" : "fa-eye"} text-xs`}></i>
                          </button>
                          <button
                            onClick={() => { navigator.clipboard.writeText(entry.password); showToastNotify("Copied!", "success"); }}
                            className="text-slate-400 hover:text-slate-600"
                          >
                            <i className="fa-solid fa-copy text-xs"></i>
                          </button>
                        </div>
                      ) : (
                        <span className="text-slate-400">-</span>
                      )}
                    </td>
                    <td className="py-3 px-4 text-slate-600 dark:text-slate-300">{entry.domain || "无"}</td>
                    <td className="py-3 px-4 text-xs text-slate-500">
                      <i className={`fa-solid ${entry.source === "mimikatz" ? "fa-wand-magic-sparkles text-amber-500" : entry.source === "sam" ? "fa-database text-blue-500" : entry.source === "kerberoast" ? "fa-shield-halved text-orange-500" : "fa-pen text-slate-400"} mr-1`}></i>
                      {entry.source || "manual"}
                    </td>
                    <td className="py-3 px-4">
                      <button
                        onClick={() => handleToggleConfirm(entry)}
                        className={`inline-flex items-center gap-1 px-2 py-1 rounded text-xs cursor-pointer transition-colors ${
                          entry.confirmed
                            ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 hover:bg-emerald-200"
                            : "bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400 hover:bg-slate-200"
                        }`}
                      >
                        <i className={`fa-solid ${entry.confirmed ? "fa-check-circle" : "fa-circle-question"}`}></i>
                        {entry.confirmed ? "Confirmed" : "Unconfirmed"}
                      </button>
                    </td>
                    <td className="py-3 px-4">
                      {entry.tags ? (
                        <div className="flex flex-wrap gap-1">
                          {entry.tags.split(",").map(t => (
                            <span key={t} className="px-2 py-0.5 bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300 text-[10px] rounded-full">
                              {t.trim()}
                            </span>
                          ))}
                        </div>
                      ) : (
                        <span className="text-slate-400 text-xs">-</span>
                      )}
                    </td>
                    <td className="py-3 px-4 text-center whitespace-nowrap">
                      <button
                        onClick={() => openEdit(entry)}
                        className="text-xs px-2 py-1 text-indigo-500 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 rounded-lg transition-colors mr-1"
                        title="Edit"
                      >
                        <i className="fa-solid fa-edit"></i>
                      </button>
                      <button
                        onClick={() => setShowDeleteConfirm(entry.id)}
                        className="text-xs px-2 py-1 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/30 rounded-lg transition-colors"
                        title="Delete"
                      >
                        <i className="fa-solid fa-trash"></i>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="p-8 text-center text-slate-400 dark:text-slate-500">
            <i className="fa-solid fa-vault text-2xl mb-2"></i>
            <br />The vault is empty. Credentials will be auto-parsed from agent tasks.
          </div>
        )}
      </div>

      {showAddModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowAddModal(false)} />
          <div className="relative bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-lg mx-4 overflow-hidden">
            <div className="bg-gradient-to-r from-indigo-500 to-indigo-700 px-6 py-5">
              <h3 className="text-lg font-semibold text-white">Add Credential</h3>
            </div>
            <div className="px-6 py-5 space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Type *</label>
                  <select
                    value={form.type}
                    onChange={e => setForm({ ...form, type: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  >
                    {CRED_TYPES.filter(t => t !== "all").map(t => (
                      <option key={t} value={t}>{t.charAt(0).toUpperCase() + t.slice(1)}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Username *</label>
                  <input
                    type="text"
                    value={form.username}
                    onChange={e => setForm({ ...form, username: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Password</label>
                  <input
                    type="text"
                    value={form.password}
                    onChange={e => setForm({ ...form, password: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Domain</label>
                  <input
                    type="text"
                    value={form.domain}
                    onChange={e => setForm({ ...form, domain: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Source</label>
                  <input
                    type="text"
                    value={form.source}
                    onChange={e => setForm({ ...form, source: e.target.value })}
                    placeholder="manual, mimikatz, sam..."
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Hash (NTLM/SHA)</label>
                  <input
                    type="text"
                    value={form.hash}
                    onChange={e => setForm({ ...form, hash: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] font-mono text-xs rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
              </div>
              <div>
                <label className="text-xs text-slate-500 block mb-1">Tags (comma separated)</label>
                <input
                  type="text"
                  value={form.tags}
                  onChange={e => setForm({ ...form, tags: e.target.value })}
                  placeholder="admin, high-value"
                  className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                />
              </div>
              <div>
                <label className="text-xs text-slate-500 block mb-1">Notes</label>
                <textarea
                  value={form.notes}
                  onChange={e => setForm({ ...form, notes: e.target.value })}
                  rows={2}
                  className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100 resize-none"
                />
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  onClick={() => setShowAddModal(false)}
                  className="flex-1 px-4 py-2.5 bg-slate-100 hover:bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-300 rounded-xl text-sm font-medium transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={handleAdd}
                  className="flex-1 px-4 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm font-medium transition-colors"
                >
                  Add
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {showEditModal && editTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => { setShowEditModal(false); setEditTarget(null); }} />
          <div className="relative bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-lg mx-4 overflow-hidden">
            <div className="bg-gradient-to-r from-indigo-500 to-indigo-700 px-6 py-5">
              <h3 className="text-lg font-semibold text-white">Edit Credential</h3>
            </div>
            <div className="px-6 py-5 space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Type</label>
                  <select
                    value={form.type}
                    onChange={e => setForm({ ...form, type: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  >
                    {CRED_TYPES.filter(t => t !== "all").map(t => (
                      <option key={t} value={t}>{t}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Username *</label>
                  <input
                    type="text"
                    value={form.username}
                    onChange={e => setForm({ ...form, username: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Password</label>
                  <input
                    type="text"
                    value={form.password}
                    onChange={e => setForm({ ...form, password: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Domain</label>
                  <input
                    type="text"
                    value={form.domain}
                    onChange={e => setForm({ ...form, domain: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Source</label>
                  <input
                    type="text"
                    value={form.source}
                    onChange={e => setForm({ ...form, source: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Hash</label>
                  <input
                    type="text"
                    value={form.hash}
                    onChange={e => setForm({ ...form, hash: e.target.value })}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] font-mono text-xs rounded-xl px-3 py-2 dark:text-slate-100"
                  />
                </div>
              </div>
              <div>
                <label className="text-xs text-slate-500 block mb-1">Tags (comma separated)</label>
                <input
                  type="text"
                  value={form.tags}
                  onChange={e => setForm({ ...form, tags: e.target.value })}
                  className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                />
              </div>
              <div>
                <label className="text-xs text-slate-500 block mb-1">Notes</label>
                <textarea
                  value={form.notes}
                  onChange={e => setForm({ ...form, notes: e.target.value })}
                  rows={2}
                  className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100 resize-none"
                />
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  onClick={() => { setShowEditModal(false); setEditTarget(null); }}
                  className="flex-1 px-4 py-2.5 bg-slate-100 hover:bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-300 rounded-xl text-sm font-medium transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={handleEdit}
                  className="flex-1 px-4 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm font-medium transition-colors"
                >
                  Save
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {showBatchModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowBatchModal(false)} />
          <div className="relative bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-md mx-4 overflow-hidden">
            <div className="bg-gradient-to-r from-amber-500 to-amber-700 px-6 py-5">
              <h3 className="text-lg font-semibold text-white">Batch Add Tags</h3>
            </div>
            <div className="px-6 py-5 space-y-4">
              <p className="text-sm text-slate-500 dark:text-slate-400">
                Add tags to {selectedIds.size} selected credential(s)
              </p>
              <div>
                <label className="text-xs text-slate-500 block mb-1">Tags (comma separated)</label>
                <input
                  type="text"
                  value={batchTags}
                  onChange={e => setBatchTags(e.target.value)}
                  placeholder="high-value, production, dc"
                  className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2 dark:text-slate-100"
                />
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  onClick={() => setShowBatchModal(false)}
                  className="flex-1 px-4 py-2.5 bg-slate-100 hover:bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-300 rounded-xl text-sm font-medium transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={handleBatchTags}
                  className="flex-1 px-4 py-2.5 bg-amber-600 hover:bg-amber-700 text-white rounded-xl text-sm font-medium transition-colors"
                >
                  Add Tags
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {showDeleteConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowDeleteConfirm(null)} />
          <div className="relative bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-sm mx-4 overflow-hidden">
            <div className="px-6 py-6 text-center">
              <i className="fa-solid fa-triangle-exclamation text-red-500 text-3xl mb-3"></i>
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100 mb-2">Delete Credential?</h3>
              <p className="text-sm text-slate-500 dark:text-slate-400">This action cannot be undone.</p>
            </div>
            <div className="px-6 pb-6 flex gap-3">
              <button
                onClick={() => setShowDeleteConfirm(null)}
                className="flex-1 px-4 py-2.5 bg-slate-100 hover:bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-300 rounded-xl text-sm font-medium transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(showDeleteConfirm)}
                className="flex-1 px-4 py-2.5 bg-red-600 hover:bg-red-700 text-white rounded-xl text-sm font-medium transition-colors"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
      </div>
    </>
  );
}
