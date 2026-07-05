"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { apiGet, apiSend, apiDelete } from "@/lib/api";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal } from "@/components/UI";

interface Plugin {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Version?: string;
  version?: string;
  Description?: string;
  description?: string;
  Author?: string;
  author?: string;
  Category?: string;
  category?: string;
  PluginType?: string;
  plugin_type?: string;
  Enabled?: boolean;
  enabled?: boolean;
  Rating?: number;
  rating?: number;
  Dependencies?: string[];
  dependencies?: string[];
  Installed?: boolean;
  installed?: string;
  UpdateAvailable?: boolean;
  update_available?: boolean;
  LastUpdated?: string;
  last_updated?: string;
  Icon?: string;
  icon?: string;
  Readme?: string;
  readme?: string;
  Size?: number;
  size?: number;
  Downloads?: number;
  downloads?: number;
}

interface Review {
  id?: string;
  user?: string;
  rating?: number;
  content?: string;
  created_at?: string;
}

interface ToastNotification {
  msg: string;
  type: "success" | "error" | "info";
}


const CATEGORIES = [
  { key: "", label: "All", icon: "fa-layer-group", color: "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300" },
  { key: "post-exploitation", label: "Post-Exploitation", icon: "fa-terminal", color: "bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400" },
  { key: "reconnaissance", label: "Reconnaissance", icon: "fa-search", color: "bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-400" },
  { key: "exploitation", label: "Exploitation", icon: "fa-bug", color: "bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400" },
  { key: "credential", label: "Credential", icon: "fa-key", color: "bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400" },
  { key: "persistence", label: "Persistence", icon: "fa-anchor", color: "bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400" },
];

export default function PluginsPage() {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("");
  const [loading, setLoading] = useState(true);
  const [detailPlugin, setDetailPlugin] = useState<Plugin | null>(null);
  const [actionStates, setActionStates] = useState<Record<string, string>>({});
  const [viewMode, setViewMode] = useState<"grid" | "list">("grid");
  const [showCreate, setShowCreate] = useState(false);
  const [showImport, setShowImport] = useState(false);
  const [showExecute, setShowExecute] = useState(false);
  const [executePlugin, setExecutePlugin] = useState<Plugin | null>(null);
  const [executeResult, setExecuteResult] = useState<string | null>(null);
  const [showReviews, setShowReviews] = useState(false);
  const [reviewsPlugin, setReviewsPlugin] = useState<Plugin | null>(null);
  const [reviews, setReviews] = useState<Review[]>([]);
  const [toasts, setToasts] = useState<ToastNotification[]>([]);
  const [createForm, setCreateForm] = useState({ name: "", description: "", author: "", category: "", version: "1.0.0" });
  const [importFile, setImportFile] = useState<File | null>(null);
  const overlayRef = useRef<HTMLDivElement>(null);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const showToast = useCallback((msg: string, type: "success" | "error" | "info" = "info") => {
    const id = Date.now().toString();
    setToasts((prev) => [...prev, { msg, type }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.msg !== msg || t.type !== type));
    }, 3000);
  }, []);

  const loadPlugins = useCallback(async () => {
    setLoading(true);
    try {
      const apiData = await apiGet<{ plugins?: Plugin[]; Plugins?: Plugin[] } | Plugin[]>("/api/plugins");
      const data = apiData as { plugins?: Plugin[]; Plugins?: Plugin[] };
      setPlugins(data.plugins || data.Plugins || (Array.isArray(apiData) ? apiData : []));
    } catch {
      setPlugins([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadPlugins()); }, [loadPlugins]);

  const filtered = plugins.filter((p) => {
    const name = (p.Name || p.name || "").toLowerCase();
    const desc = (p.Description || p.description || "").toLowerCase();
    const author = (p.Author || p.author || "").toLowerCase();
    const term = search.toLowerCase();
    const matchSearch = name.includes(term) || desc.includes(term) || author.includes(term);
    const matchCategory = !category || (p.Category || p.category || "") === category;
    return matchSearch && matchCategory;
  });

  const setAction = (id: string, action: string | null) => {
    setActionStates((s) => {
      const n = { ...s };
      if (action === null) delete n[id]; else n[id] = action;
      return n;
    });
  };

  const handleInstall = async (pluginId: string) => {
    setAction(pluginId, "installing");
    try {
      const body = new URLSearchParams({ id: pluginId });
      await apiSend(`/api/plugins/install`, "POST", body);
      showToast("Plugin installed successfully", "success");
      loadPlugins();
    } catch { showToast("Failed to install plugin", "error"); }
    finally { setAction(pluginId, null); }
  };

  const handleUninstall = async (pluginId: string) => {
    setAction(pluginId, "uninstalling");
    try {
      await apiSend(`/api/plugins/${pluginId}/toggle`, "POST", new URLSearchParams({ enabled: "false" }));
      showToast("Plugin uninstalled", "success");
      loadPlugins();
    } catch { showToast("Failed to uninstall plugin", "error"); }
    finally { setAction(pluginId, null); }
  };

  const handleDelete = (pluginId: string) => {
    setCfm({msg: "Delete this plugin permanently? This cannot be undone.", cb: async () => {
      setAction(pluginId, "deleting");
      try {
        await apiDelete(`/api/plugins/${pluginId}`);
        showToast("Plugin deleted", "success");
        loadPlugins();
      } catch { showToast("Failed to delete plugin", "error"); }
      finally { setAction(pluginId, null); }
    }});
  };

  const handleToggle = async (pluginId: string, enabled: boolean) => {
    setAction(pluginId, "toggling");
    try {
      await apiSend(`/api/plugins/${pluginId}/toggle`, "POST", new URLSearchParams({ enabled: String(enabled) }));
      showToast(enabled ? "Plugin enabled" : "Plugin disabled", "success");
      loadPlugins();
    } catch { showToast("Failed to toggle plugin", "error"); }
    finally { setAction(pluginId, null); }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const body = new URLSearchParams();
      Object.entries(createForm).forEach(([k, v]) => { if (v) body.append(k, v); });
      await apiSend("/api/plugins", "POST", body);
      showToast("Plugin created", "success");
      setShowCreate(false);
      setCreateForm({ name: "", description: "", author: "", category: "", version: "1.0.0" });
      loadPlugins();
    } catch { showToast("Failed to create plugin", "error"); }
  };

  const handleExecute = async (pluginId: string) => {
    setExecuteResult(null);
    setExecuteLoading(true);
    try {
      const result = await apiSend(`/api/plugins/${pluginId}/execute`, "POST");
      setExecuteResult(JSON.stringify(result, null, 2));
      showToast("Plugin executed", "success");
    } catch {
      setExecuteResult("Execution failed");
      showToast("Failed to execute plugin", "error");
    }
  };

  const [executeLoading, setExecuteLoading] = useState(false);

  const handleExport = async (pluginId: string) => {
    try {
      const result = await fetch(`${API_BASE}?p=/api/plugins/${pluginId}/export&format=json`);
      if (!result.ok) throw new Error(`HTTP ${result.status}`);
      const blob = await result.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `plugin_${pluginId}.json`;
      a.click();
      URL.revokeObjectURL(url);
      showToast("Plugin exported", "success");
    } catch { showToast("Failed to export plugin", "error"); }
  };

  const handleImport = async () => {
    if (!importFile) return;
    try {
      const formData = new FormData();
      formData.append("file", importFile);
      await fetch(`${API_BASE}?p=/api/plugins/import&format=json`, {
        method: "POST",
        body: formData,
      });
      showToast("Plugin imported", "success");
      setShowImport(false);
      setImportFile(null);
      loadPlugins();
    } catch { showToast("Failed to import plugin", "error"); }
  };

  const handleUpdate = async (pluginId: string) => {
    setAction(pluginId, "updating");
    try {
      await apiSend(`/api/plugins/${pluginId}/update`, "POST");
      showToast("Plugin updated", "success");
      loadPlugins();
    } catch { showToast("Failed to update plugin", "error"); }
    finally { setAction(pluginId, null); }
  };

  const handleUpdateCheck = async () => {
    try {
      await apiSend("/api/plugins/check-updates", "POST");
      showToast("Update check complete", "success");
      loadPlugins();
    } catch { showToast("Update check failed", "error"); }
  };

  const handleLoadReviews = async (plugin: Plugin) => {
    const pid = plugin.ID || plugin.id || "";
    setReviewsPlugin(plugin);
    setShowReviews(true);
    try {
      const apiData = await apiGet<{ reviews?: Review[]; Reviews?: Review[] } | Review[]>(`/api/plugins/${pid}/reviews`);
      const d = apiData as { reviews?: Review[]; Reviews?: Review[] };
      setReviews(d.reviews || d.Reviews || (Array.isArray(apiData) ? apiData : []));
    } catch { setReviews([]); }
  };

  const handlePostReview = async (pluginId: string, rating: number, content: string) => {
    try {
      const body = new URLSearchParams({ rating: String(rating), content });
      await apiSend(`/api/plugins/${pluginId}/reviews`, "POST", body);
      showToast("Review posted", "success");
      if (reviewsPlugin) handleLoadReviews(reviewsPlugin);
    } catch { showToast("Failed to post review", "error"); }
  };

  const handleRating = async (pluginId: string, rating: number) => {
    try {
      await apiSend(`/api/plugins/${pluginId}/rating`, "POST", new URLSearchParams({ rating: String(rating) }));
      showToast("Rating submitted", "success");
      loadPlugins();
    } catch { showToast("Failed to submit rating", "error"); }
  };

  const handleLoadDependencies = async (plugin: Plugin) => {
    const pid = plugin.ID || plugin.id || "";
    try {
      const apiData = await apiGet<{ dependencies?: string[]; Dependencies?: string[] } | string[]>(`/api/plugins/${pid}/dependencies`);
      const d = apiData as { dependencies?: string[]; Dependencies?: string[] };
      const deps = d.dependencies || d.Dependencies || (Array.isArray(apiData) ? apiData : []);
      setDetailPlugin({ ...plugin, Dependencies: Array.isArray(deps) ? deps : [] });
    } catch (e) { console.error("Plugins: load dependencies failed", e); }
  };

  const currentCat = CATEGORIES.find((c) => c.key === category);

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      {toasts.length > 0 && (
        <div className="fixed top-4 right-4 z-[100] space-y-2">
          {toasts.map((t, i) => (
            <div key={i} className={`px-4 py-3 rounded-2xl shadow-lg text-sm font-medium text-white ${t.type === "success" ? "bg-emerald-600" : t.type === "error" ? "bg-red-600" : "bg-indigo-600"} animate-[fadeIn_0.2s_ease-out]`}>
              <i className={`fa-solid ${t.type === "success" ? "fa-check-circle" : t.type === "error" ? "fa-exclamation-circle" : "fa-info-circle"} mr-2`}></i>
              {t.msg}
            </div>
          ))}
        </div>
      )}

      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">
            <i className="fa-solid fa-puzzle-piece text-indigo-500 mr-2"></i>Plugins
          </h1>
          <p className="text-xs sm:text-sm text-slate-500 dark:text-slate-400 mt-1">
            {plugins.length} plugins available &middot; Manage, install, and configure
          </p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <button onClick={handleUpdateCheck} className="inline-flex items-center gap-1.5 px-3 py-2 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 text-xs font-medium rounded-2xl transition-colors">
            <i className="fa-solid fa-circle-check"></i> Check Updates
          </button>
          <button onClick={() => setShowCreate(true)} className="inline-flex items-center gap-1.5 px-3 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-medium rounded-2xl transition-colors">
            <i className="fa-solid fa-plus"></i> Create
          </button>
          <button onClick={() => setShowImport(true)} className="inline-flex items-center gap-1.5 px-3 py-2 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 text-xs font-medium rounded-2xl transition-colors">
            <i className="fa-solid fa-file-import"></i> Import
          </button>
          <div className="flex bg-slate-100 dark:bg-slate-700 rounded-2xl p-0.5">
            <button onClick={() => setViewMode("grid")} className={`p-2 rounded-lg transition-colors ${viewMode === "grid" ? "bg-white dark:bg-slate-600 shadow-sm" : "text-slate-500"}`}>
              <i className="fa-solid fa-grid-2 text-sm"></i>
            </button>
            <button onClick={() => setViewMode("list")} className={`p-2 rounded-lg transition-colors ${viewMode === "list" ? "bg-white dark:bg-slate-600 shadow-sm" : "text-slate-500"}`}>
              <i className="fa-solid fa-list text-sm"></i>
            </button>
          </div>
        </div>
      </div>

      <div className="ui-card p-3 sm:p-4 mb-4 shadow-sm">
        <div className="flex flex-col sm:flex-row gap-3">
          <div className="relative flex-1">
            <i className="fa-solid fa-search absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 dark:text-slate-500 text-sm"></i>
            <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search plugins by name, description, author..." className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-2xl pl-9 pr-4 h-10 focus:outline-none focus:border-indigo-500" />
          </div>
        </div>
      </div>

      <div className="flex flex-wrap gap-2 mb-4">
        {CATEGORIES.map((cat) => {
          const count = cat.key === "" ? plugins.length : plugins.filter((p) => (p.Category || p.category || "") === cat.key).length;
          return (
            <button key={cat.key} onClick={() => setCategory(cat.key)} className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-2xl border transition-all ${category === cat.key ? "ring-2 ring-indigo-500 border-indigo-300 dark:border-indigo-700 " + cat.color : "border-[var(--border)] text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-700/50"}`}>
              <i className={`fa-solid ${cat.icon}`}></i>
              {cat.label}
              <span className="ml-1 px-1.5 py-0.5 bg-white/50 dark:bg-slate-800/50 rounded-md text-[10px]">{count}</span>
            </button>
          );
        })}
      </div>

      {loading ? (
        <div className={viewMode === "grid" ? "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4" : "space-y-3"}>
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <div key={i} className="ui-card p-5 shadow-sm">
              <div className="flex gap-3 mb-3">
                <div className="w-10 h-10 bg-slate-100 dark:bg-slate-700 rounded-2xl animate-pulse shrink-0"></div>
                <div className="flex-1 space-y-2">
                  <div className="h-4 bg-slate-100 dark:bg-slate-700 rounded w-3/4 animate-pulse"></div>
                  <div className="h-3 bg-slate-100 dark:bg-slate-700 rounded w-1/2 animate-pulse"></div>
                </div>
              </div>
              <div className="h-3 bg-slate-100 dark:bg-slate-700 rounded w-full animate-pulse mb-2"></div>
              <div className="h-3 bg-slate-100 dark:bg-slate-700 rounded w-2/3 animate-pulse"></div>
            </div>
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="text-center py-16 text-slate-400">
          <i className="fa-solid fa-puzzle-piece text-4xl mb-3"></i>
          <p className="text-sm">No plugins found matching your criteria</p>
        </div>
      ) : viewMode === "grid" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((p, i) => <PluginCard key={p.ID || p.id || String(i)} plugin={p} actionState={actionStates[p.ID || p.id || String(i)]} onInstall={handleInstall} onUninstall={handleUninstall} onDelete={handleDelete} onToggle={handleToggle} onDetail={() => { setDetailPlugin(p); handleLoadDependencies(p); }} onExecute={() => { setExecutePlugin(p); setShowExecute(true); setExecuteResult(null); }} onExport={() => handleExport(p.ID || p.id || "")} onUpdate={() => handleUpdate(p.ID || p.id || "")} onReviews={() => handleLoadReviews(p)} onRating={(r) => handleRating(p.ID || p.id || "", r)} />)}
        </div>
      ) : (
        <div className="space-y-3">
          {filtered.map((p, i) => <PluginListItem key={p.ID || p.id || String(i)} plugin={p} actionState={actionStates[p.ID || p.id || String(i)]} onInstall={handleInstall} onUninstall={handleUninstall} onDelete={handleDelete} onToggle={handleToggle} onDetail={() => { setDetailPlugin(p); handleLoadDependencies(p); }} onExecute={() => { setExecutePlugin(p); setShowExecute(true); setExecuteResult(null); }} onExport={() => handleExport(p.ID || p.id || "")} onUpdate={() => handleUpdate(p.ID || p.id || "")} onReviews={() => handleLoadReviews(p)} onRating={(r) => handleRating(p.ID || p.id || "", r)} />)}
        </div>
      )}

      {detailPlugin && <PluginDetailModal plugin={detailPlugin} onClose={() => setDetailPlugin(null)} overlayRef={overlayRef} />}

      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" onClick={() => setShowCreate(false)}>
          <div className="bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-md p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Create Plugin</h3>
              <button onClick={() => setShowCreate(false)} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700"><i className="fa-solid fa-xmark"></i></button>
            </div>
            <form onSubmit={handleCreate} className="space-y-3">
              <input type="text" required placeholder="Plugin name" value={createForm.name} onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm focus:outline-none focus:border-indigo-500" />
              <textarea placeholder="Description" value={createForm.description} onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 py-2 text-sm focus:outline-none focus:border-indigo-500 resize-none h-20" />
              <input type="text" placeholder="Author" value={createForm.author} onChange={(e) => setCreateForm({ ...createForm, author: e.target.value })} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm focus:outline-none focus:border-indigo-500" />
              <select value={createForm.category} onChange={(e) => setCreateForm({ ...createForm, category: e.target.value })} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm focus:outline-none focus:border-indigo-500">
                <option value="">Select category</option>
                {CATEGORIES.slice(1).map((c) => <option key={c.key} value={c.key}>{c.label}</option>)}
              </select>
              <input type="text" placeholder="Version (e.g. 1.0.0)" value={createForm.version} onChange={(e) => setCreateForm({ ...createForm, version: e.target.value })} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm focus:outline-none focus:border-indigo-500" />
              <button type="submit" className="w-full h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl text-sm font-medium transition-colors">Create Plugin</button>
            </form>
          </div>
        </div>
      )}

      {showImport && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" onClick={() => setShowImport(false)}>
          <div className="bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-md p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Import Plugin</h3>
              <button onClick={() => setShowImport(false)} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700"><i className="fa-solid fa-xmark"></i></button>
            </div>
            <div className="border-2 border-dashed border-[var(--border)] rounded-2xl p-8 text-center mb-4 hover:border-indigo-500 transition-colors">
              <i className="fa-solid fa-cloud-arrow-up text-3xl text-slate-400 mb-2"></i>
              <p className="text-sm text-slate-500 dark:text-slate-400 mb-3">Upload a plugin JSON file</p>
              <input type="file" accept=".json,.zip" onChange={(e) => setImportFile(e.target.files?.[0] || null)} className="text-sm text-slate-600 dark:text-slate-300 file:mr-3 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-indigo-50 dark:file:bg-indigo-900/30 file:text-indigo-700 dark:file:text-indigo-400 hover:file:bg-indigo-100" />
              {importFile && <p className="text-xs text-slate-500 mt-2">{importFile.name}</p>}
            </div>
            <div className="flex gap-3">
              <button onClick={() => setShowImport(false)} className="flex-1 h-10 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 text-slate-600 dark:text-slate-300 rounded-2xl text-sm font-medium transition-colors">Cancel</button>
              <button onClick={handleImport} className="flex-1 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl text-sm font-medium transition-colors">Import</button>
            </div>
          </div>
        </div>
      )}

      {showExecute && executePlugin && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" onClick={() => setShowExecute(false)}>
          <div className="bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-lg p-6 max-h-[80vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Execute: {executePlugin.Name || executePlugin.name}</h3>
              <button onClick={() => setShowExecute(false)} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700"><i className="fa-solid fa-xmark"></i></button>
            </div>
            <button onClick={() => handleExecute(executePlugin.ID || executePlugin.id || "")} disabled={executeLoading} className="w-full h-10 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white rounded-2xl text-sm font-medium transition-colors mb-4">
              {executeLoading ? <><i className="fa-solid fa-spinner fa-spin mr-2"></i>Running...</> : <><i className="fa-solid fa-play mr-2"></i>Run Plugin</>}
            </button>
            {executeResult && (
              <div className="bg-slate-900 rounded-2xl p-4 max-h-96 overflow-y-auto">
                <pre className="text-xs font-mono text-emerald-300 whitespace-pre-wrap">{executeResult}</pre>
              </div>
            )}
          </div>
        </div>
      )}

      {showReviews && reviewsPlugin && (
        <ReviewsModal plugin={reviewsPlugin} reviews={reviews} onClose={() => setShowReviews(false)} onPost={handlePostReview} />
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Delete" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}

function ReviewsModal({ plugin, reviews, onClose, onPost }: { plugin: Plugin; reviews: Review[]; onClose: () => void; onPost: (id: string, rating: number, content: string) => void }) {
  const pid = plugin.ID || plugin.id || "";
  const [rating, setRating] = useState(5);
  const [content, setContent] = useState("");

  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    document.addEventListener("keydown", handleEsc);
    return () => document.removeEventListener("keydown", handleEsc);
  }, [onClose]);

  const submit = () => {
    onPost(pid, rating, content);
    setContent("");
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" onClick={onClose}>
      <div className="bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-md p-6 max-h-[80vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Reviews: {plugin.Name || plugin.name}</h3>
          <button onClick={onClose} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700"><i className="fa-solid fa-xmark"></i></button>
        </div>
        <div className="space-y-3 mb-4">
          <div className="flex items-center gap-1 mb-2">
            {[1, 2, 3, 4, 5].map((s) => (
              <button key={s} onClick={() => setRating(s)} className="text-lg">
                <i className={`fa-solid fa-star ${s <= rating ? "text-amber-400" : "text-slate-200 dark:text-slate-600"}`}></i>
              </button>
            ))}
          </div>
          <textarea value={content} onChange={(e) => setContent(e.target.value)} placeholder="Write your review..." className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 py-2 text-sm focus:outline-none focus:border-indigo-500 resize-none h-20" />
          <button onClick={submit} className="w-full h-9 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl text-sm font-medium transition-colors">Submit Review</button>
        </div>
        <div className="space-y-3">
          {reviews.length === 0 && <p className="text-sm text-slate-400 text-center py-4">No reviews yet</p>}
          {reviews.map((r) => (
            <div key={r.id} className="bg-slate-50 dark:bg-slate-700/50 rounded-2xl p-3">
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs font-medium text-[var(--text-secondary)]">{r.user || "Anonymous"}</span>
                <div className="flex items-center gap-0.5">
                  {[1, 2, 3, 4, 5].map((s) => (
                    <i key={s} className={`fa-solid fa-star text-[10px] ${s <= (r.rating || 0) ? "text-amber-400" : "text-slate-200 dark:text-slate-600"}`}></i>
                  ))}
                </div>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400">{r.content}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function PluginCard({ plugin, actionState, onInstall, onUninstall, onDelete, onToggle, onDetail, onExecute, onExport, onUpdate, onReviews, onRating }: {
  plugin: Plugin;
  actionState?: string;
  onInstall: (id: string) => void;
  onUninstall: (id: string) => void;
  onDelete: (id: string) => void;
  onToggle: (id: string, enabled: boolean) => void;
  onDetail: () => void;
  onExecute: () => void;
  onExport: () => void;
  onUpdate: () => void;
  onReviews: () => void;
  onRating: (r: number) => void;
}) {
  const id = plugin.ID || plugin.id || "";
  const name = plugin.Name || plugin.name || "Unknown";
  const version = plugin.Version || plugin.version || "1.0.0";
  const desc = plugin.Description || plugin.description || "";
  const author = plugin.Author || plugin.author || "-";
  const cat = plugin.Category || plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.Dependencies || plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const icon = plugin.Icon || plugin.icon || "fa-puzzle-piece";
  const downloads = plugin.Downloads || plugin.downloads || 0;
  const [hoverRating, setHoverRating] = useState(0);
  const catInfo = CATEGORIES.find((c) => c.key === cat);

  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-all group">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-2xl flex items-center justify-center shrink-0">
            <i className={`fa-solid ${icon} text-indigo-600 dark:text-indigo-400`}></i>
          </div>
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100 truncate cursor-pointer hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors" onClick={onDetail}>{name}</h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">v{version} &middot; {author}</p>
          </div>
        </div>
        {updateAvail && (
          <button onClick={(e) => { e.stopPropagation(); onUpdate(); }} className="shrink-0 px-2 py-0.5 bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 text-[10px] font-medium rounded-lg hover:bg-amber-200 dark:hover:bg-amber-900/50 transition-colors" title="Update available - click to update">
            <i className="fa-solid fa-arrow-up mr-0.5"></i>Update
          </button>
        )}
      </div>

      <p className="text-xs text-slate-500 dark:text-slate-400 mb-3 line-clamp-2 leading-relaxed">{desc || "No description available"}</p>

      <div className="flex items-center gap-2 mb-3 flex-wrap">
        {catInfo && (
          <span className={`text-[10px] px-2 py-0.5 rounded-lg ${catInfo.color}`}>
            <i className={`fa-solid ${catInfo.icon} mr-1`}></i>{catInfo.label}
          </span>
        )}
      </div>

      <div className="flex items-center gap-0.5 mb-1">
        {[1, 2, 3, 4, 5].map((s) => (
          <button key={s} onClick={() => onRating(s)} onMouseEnter={() => setHoverRating(s)} onMouseLeave={() => setHoverRating(0)} className="p-0.5">
            <i className={`fa-solid fa-star text-[10px] transition-colors ${s <= (hoverRating || rating) ? "text-amber-400" : "text-slate-200 dark:text-slate-600"}`}></i>
          </button>
        ))}
        <span className="text-[10px] text-slate-400 ml-1">{(hoverRating || rating).toFixed(1)}</span>
        <button onClick={onReviews} className="text-[10px] text-indigo-500 ml-1 hover:underline">Reviews</button>
      </div>
      <div className="text-[10px] text-slate-400 mb-3">{downloads.toLocaleString()} downloads</div>

      {deps.length > 0 && (
        <div className="mb-3 text-[10px] text-slate-400">
          <i className="fa-solid fa-link mr-1"></i>Deps: {deps.join(", ")}
        </div>
      )}

      <div className="flex items-center justify-between pt-3 border-t border-slate-100 dark:border-slate-700">
        {installed ? (
          <div className="flex items-center gap-2">
            <button onClick={() => onToggle(id, !enabled)} disabled={actionState === "toggling"} className={`w-10 h-5 rounded-full transition-colors disabled:opacity-50 ${enabled ? "bg-indigo-600" : "bg-slate-300 dark:bg-slate-600"}`}>
              <div className={`w-4 h-4 bg-white rounded-full transition-transform mt-0.5 ${enabled ? "translate-x-5" : "translate-x-0.5"}`}></div>
            </button>
            <span className="text-[10px] text-slate-500">{enabled ? "Enabled" : "Disabled"}</span>
          </div>
        ) : (
          <span className="text-[10px] text-slate-400">Not installed</span>
        )}
        <div className="flex items-center gap-1">
          <button onClick={onExecute} disabled={!installed} className="p-1.5 text-slate-400 hover:text-emerald-500 rounded-lg hover:bg-emerald-50 dark:hover:bg-emerald-900/20 transition-colors disabled:opacity-30" title="Execute"><i className="fa-solid fa-play text-xs"></i></button>
          <button onClick={onExport} className="p-1.5 text-slate-400 hover:text-blue-500 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors" title="Export"><i className="fa-solid fa-download text-xs"></i></button>
          {installed ? (
            <>
              <button onClick={() => onUninstall(id)} disabled={actionState === "uninstalling"} className="px-2 py-1 text-[10px] font-medium text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 hover:bg-amber-100 dark:hover:bg-amber-900/40 rounded-lg transition-colors disabled:opacity-50">{actionState === "uninstalling" ? <i className="fa-solid fa-spinner fa-spin"></i> : "Uninstall"}</button>
              <button onClick={() => onDelete(id)} disabled={actionState === "deleting"} className="p-1.5 text-slate-400 hover:text-red-500 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors" title="Delete"><i className="fa-solid fa-trash text-xs"></i></button>
            </>
          ) : (
            <button onClick={() => onInstall(id)} disabled={actionState === "installing"} className="px-2.5 py-1 text-[10px] font-medium text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/20 hover:bg-indigo-100 dark:hover:bg-indigo-900/40 rounded-lg transition-colors disabled:opacity-50">{actionState === "installing" ? <><i className="fa-solid fa-spinner fa-spin mr-1"></i>...</> : "Install"}</button>
          )}
          <button onClick={onDetail} className="p-1.5 text-slate-400 hover:text-indigo-500 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors" title="Details"><i className="fa-solid fa-circle-info text-xs"></i></button>
        </div>
      </div>
    </div>
  );
}

function PluginListItem({ plugin, actionState, onInstall, onUninstall, onDelete, onToggle, onDetail, onExecute, onExport, onUpdate, onReviews, onRating }: {
  plugin: Plugin;
  actionState?: string;
  onInstall: (id: string) => void;
  onUninstall: (id: string) => void;
  onDelete: (id: string) => void;
  onToggle: (id: string, enabled: boolean) => void;
  onDetail: () => void;
  onExecute: () => void;
  onExport: () => void;
  onUpdate: () => void;
  onReviews: () => void;
  onRating: (r: number) => void;
}) {
  const id = plugin.ID || plugin.id || "";
  const name = plugin.Name || plugin.name || "Unknown";
  const version = plugin.Version || plugin.version || "1.0.0";
  const desc = plugin.Description || plugin.description || "";
  const author = plugin.Author || plugin.author || "-";
  const cat = plugin.Category || plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.Dependencies || plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const catInfo = CATEGORIES.find((c) => c.key === cat);

  return (
    <div className="ui-card p-4 shadow-sm hover:shadow-md transition-all">
      <div className="flex items-center gap-4">
        <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-2xl flex items-center justify-center shrink-0">
          <i className="fa-solid fa-puzzle-piece text-indigo-600 dark:text-indigo-400"></i>
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100 truncate cursor-pointer hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors" onClick={onDetail}>{name}</h3>
            <span className="text-[10px] text-slate-400">v{version}</span>
            {catInfo && <span className={`text-[10px] px-1.5 py-0.5 rounded ${catInfo.color}`}>{catInfo.label}</span>}
            {updateAvail && <button onClick={onUpdate} className="text-[10px] px-1.5 py-0.5 rounded bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 hover:bg-amber-200"><i className="fa-solid fa-arrow-up mr-0.5"></i>Update</button>}
          </div>
          <p className="text-xs text-slate-500 dark:text-slate-400 truncate">{desc || "No description"}</p>
        </div>
        <div className="flex items-center gap-0.5 shrink-0">
          {[1, 2, 3, 4, 5].map((s) => <button key={s} onClick={() => onRating(s)}><i className={`fa-solid fa-star text-[10px] hover:text-amber-400 ${s <= rating ? "text-amber-400" : "text-slate-200 dark:text-slate-600"}`}></i></button>)}
          <button onClick={onReviews} className="text-[10px] text-indigo-500 ml-1 hover:underline">Reviews</button>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <button onClick={onExecute} disabled={!installed} className="p-1.5 text-slate-400 hover:text-emerald-500 rounded-lg hover:bg-emerald-50 dark:hover:bg-emerald-900/20 transition-colors disabled:opacity-30" title="Execute"><i className="fa-solid fa-play text-xs"></i></button>
          <button onClick={onExport} className="p-1.5 text-slate-400 hover:text-blue-500 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors" title="Export"><i className="fa-solid fa-download text-xs"></i></button>
          {installed ? (
            <>
              <button onClick={() => onToggle(id, !enabled)} disabled={actionState === "toggling"} className={`w-10 h-5 rounded-full transition-colors disabled:opacity-50 shrink-0 ${enabled ? "bg-indigo-600" : "bg-slate-300 dark:bg-slate-600"}`}>
                <div className={`w-4 h-4 bg-white rounded-full transition-transform mt-0.5 ${enabled ? "translate-x-5" : "translate-x-0.5"}`}></div>
              </button>
              <button onClick={() => onUninstall(id)} disabled={actionState === "uninstalling"} className="px-2 py-1 text-[10px] font-medium text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 hover:bg-amber-100 dark:hover:bg-amber-900/40 rounded-lg transition-colors disabled:opacity-50">{actionState === "uninstalling" ? <i className="fa-solid fa-spinner fa-spin"></i> : "Uninstall"}</button>
              <button onClick={() => onDelete(id)} disabled={actionState === "deleting"} className="p-1.5 text-slate-400 hover:text-red-500 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors" title="Delete"><i className="fa-solid fa-trash text-xs"></i></button>
            </>
          ) : (
            <button onClick={() => onInstall(id)} disabled={actionState === "installing"} className="px-3 py-1.5 text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg transition-colors disabled:opacity-50">{actionState === "installing" ? <><i className="fa-solid fa-spinner fa-spin mr-1"></i>Installing...</> : "Install"}</button>
          )}
          <button onClick={onDetail} className="p-1.5 text-slate-400 hover:text-indigo-500 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors"><i className="fa-solid fa-circle-info text-xs"></i></button>
        </div>
      </div>
      {deps.length > 0 && (
        <div className="mt-2 ml-14 text-[10px] text-slate-400">
          <i className="fa-solid fa-link mr-1"></i>Dependencies: {deps.join(", ")}
        </div>
      )}
    </div>
  );
}

function PluginDetailModal({ plugin, onClose, overlayRef }: {
  plugin: Plugin;
  onClose: () => void;
  overlayRef: React.RefObject<HTMLDivElement | null>;
}) {
  const id = plugin.ID || plugin.id || "";
  const name = plugin.Name || plugin.name || "Unknown";
  const version = plugin.Version || plugin.version || "1.0.0";
  const desc = plugin.Description || plugin.description || "No description available";
  const author = plugin.Author || plugin.author || "-";
  const cat = plugin.Category || plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.Dependencies || plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const readme = plugin.Readme || plugin.readme || "";
  const downloads = plugin.Downloads || plugin.downloads || 0;
  const lastUpdated = plugin.LastUpdated || plugin.last_updated || "-";
  const icon = plugin.Icon || plugin.icon || "fa-puzzle-piece";

  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    document.addEventListener("keydown", handleEsc);
    return () => document.removeEventListener("keydown", handleEsc);
  }, [onClose]);

  const catInfo = CATEGORIES.find((c) => c.key === cat);

  return (
    <div ref={overlayRef} className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" onClick={(e) => { if (e.target === overlayRef.current) onClose(); }}>
      <div className="ui-card shadow-2xl w-full max-w-2xl max-h-[85vh] overflow-hidden flex flex-col">
        <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)] shrink-0">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-2xl flex items-center justify-center">
              <i className={`fa-solid ${icon} text-indigo-600 dark:text-indigo-400`}></i>
            </div>
            <div>
              <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">{name}</h2>
              <p className="text-xs text-slate-500 dark:text-slate-400">v{version} &middot; {author}</p>
            </div>
          </div>
          <button onClick={onClose} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors">
            <i className="fa-solid fa-xmark text-lg"></i>
          </button>
        </div>

        <div className="overflow-y-auto flex-1 p-6 space-y-5">
          <div className="flex items-center gap-3 flex-wrap">
            {catInfo && (
              <span className={`text-xs px-2.5 py-1 rounded-lg ${catInfo.color}`}>
                <i className={`fa-solid ${catInfo.icon} mr-1`}></i>{catInfo.label}
              </span>
            )}
            {installed ? (
              <span className={`text-xs px-2.5 py-1 rounded-lg ${enabled ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300"}`}>
                <i className={`fa-solid ${enabled ? "fa-check-circle" : "fa-circle"} mr-1`}></i>{enabled ? "Enabled" : "Disabled"}
              </span>
            ) : (
              <span className="text-xs px-2.5 py-1 rounded-lg bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400">Not Installed</span>
            )}
            {updateAvail && (
              <span className="text-xs px-2.5 py-1 rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
                <i className="fa-solid fa-arrow-up mr-1"></i>Update Available
              </span>
            )}
          </div>

          <div className="flex items-center gap-4">
            <div className="flex items-center gap-0.5">
              {[1, 2, 3, 4, 5].map((s) => (
                <i key={s} className={`fa-solid fa-star text-xs ${s <= rating ? "text-amber-400" : "text-slate-200 dark:text-slate-600"}`}></i>
              ))}
              <span className="text-xs text-slate-500 ml-1">{rating.toFixed(1)}</span>
            </div>
            <span className="text-xs text-slate-400"><i className="fa-solid fa-download mr-1"></i>{downloads.toLocaleString()} downloads</span>
            <span className="text-xs text-slate-400"><i className="fa-solid fa-clock mr-1"></i>Updated: {lastUpdated}</span>
          </div>

          <div>
            <h3 className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-2">Description</h3>
            <p className="text-sm text-[var(--text-secondary)] leading-relaxed">{desc}</p>
          </div>

          {deps.length > 0 && (
            <div>
              <h3 className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-2">Dependencies</h3>
              <div className="flex flex-wrap gap-2">
                {deps.map((d, i) => (
                  <span key={i} className="text-xs px-2.5 py-1 bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-lg border border-[var(--border)] font-mono">{d}</span>
                ))}
              </div>
            </div>
          )}

          {readme && (
            <div>
              <h3 className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-2">Readme</h3>
              <div className="bg-slate-50 dark:bg-slate-900 border border-[var(--border)] rounded-2xl p-4 text-xs text-[var(--text-secondary)] whitespace-pre-wrap max-h-60 overflow-y-auto">{readme}</div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
