"use client";

import { useEffect, useState, useCallback } from "react";
import { api, getCsrfToken } from "@/lib/api";
import { downloadFromResponse } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { ConfirmModal, EmptyState, PageHeader, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import { CardGridSkeleton } from "@/components/ui/skeletons";
import { Anchor, ArrowUp, Bug, CheckCircle, Clock, CloudUpload, Download, FileDown, Info, Key, LayoutGrid, Layers, Link, List, Play, Plus, Puzzle, Search, Star, Terminal, Trash2 } from "lucide-react";

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

const CATEGORIES = [
  { key: "", label: "All", icon: <Layers className="w-4 h-4" />, color: "bg-secondary text-muted-foreground" },
  { key: "post-exploitation", label: "Post-Exploitation", icon: <Terminal className="w-4 h-4" />, color: "bg-destructive/10 text-destructive" },
  { key: "reconnaissance", label: "Reconnaissance", icon: <Search className="w-4 h-4" />, color: "bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-400" },
  { key: "exploitation", label: "Exploitation", icon: <Bug className="w-4 h-4" />, color: "bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400" },
  { key: "credential", label: "Credential", icon: <Key className="w-4 h-4" />, color: "bg-primary/10 text-primary" },
  { key: "persistence", label: "Persistence", icon: <Anchor className="w-4 h-4" />, color: "bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400" },
];

export default function PluginsPage() {
  const { t } = useI18n();
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
  const [createForm, setCreateForm] = useState({ name: "", description: "", author: "", category: "", version: "1.0.0" });
  const [importFile, setImportFile] = useState<File | null>(null);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const loadPlugins = useCallback(async () => {
    setLoading(true);
    try {
      const apiData = await api.get<{ plugins?: Plugin[]; Plugins?: Plugin[] } | Plugin[]>("/api/plugins");
      const data = apiData as { plugins?: Plugin[]; Plugins?: Plugin[] };
      setPlugins(data.plugins || (Array.isArray(apiData) ? apiData : []));
    } catch {
      setPlugins([]);
      toast.error(t("plugins.toast.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { loadPlugins(); }, [loadPlugins]);

  const filtered = plugins.filter((p) => {
    const name = (p.name || "").toLowerCase();
    const desc = (p.description || "").toLowerCase();
    const author = (p.author || "").toLowerCase();
    const term = search.toLowerCase();
    const matchSearch = name.includes(term) || desc.includes(term) || author.includes(term);
    const matchCategory = !category || (p.category || "") === category;
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
      await api.post(`/api/plugins/${pluginId}/install`, {});
      toast.success(t("plugins.toast.installed"));
      loadPlugins();
    } catch { toast.error(t("plugins.toast.install_failed")); }
    finally { setAction(pluginId, null); }
  };

  const handleUninstall = async (pluginId: string) => {
    setAction(pluginId, "uninstalling");
    try {
      await api.post(`/api/plugins/${pluginId}/toggle`, { enabled: "false" });
      toast.success(t("plugins.toast.uninstalled"));
      loadPlugins();
    } catch { toast.error(t("plugins.toast.uninstall_failed")); }
    finally { setAction(pluginId, null); }
  };

  const handleDelete = (pluginId: string) => {
    setCfm({msg: "Delete this plugin permanently? This cannot be undone.", cb: async () => {
      setAction(pluginId, "deleting");
      try {
        await api.del(`/api/plugins/${pluginId}`);
        toast.success(t("plugins.toast.deleted"));
        loadPlugins();
      } catch { toast.error(t("plugins.toast.delete_failed")); }
      finally { setAction(pluginId, null); }
    }});
  };

  const handleToggle = async (pluginId: string, enabled: boolean) => {
    setAction(pluginId, "toggling");
    try {
      await api.post(`/api/plugins/${pluginId}/toggle`, { enabled: String(enabled) });
      toast.success(enabled ? t("plugins.toast.enabled") : t("plugins.toast.disabled"));
      loadPlugins();
    } catch { toast.error(t("plugins.toast.toggle_failed")); }
    finally { setAction(pluginId, null); }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const body: Record<string, string> = {};
      Object.entries(createForm).forEach(([k, v]) => { if (v) body[k] = v; });
      await api.post("/api/plugins", body);
      toast.success(t("plugins.toast.created"));
      setShowCreate(false);
      setCreateForm({ name: "", description: "", author: "", category: "", version: "1.0.0" });
      loadPlugins();
    } catch { toast.error(t("plugins.toast.create_failed")); }
  };

  const [executeLoading, setExecuteLoading] = useState(false);

  const handleExecute = async (pluginId: string) => {
    setExecuteResult(null);
    setExecuteLoading(true);
    try {
      const result = await api.post(`/api/plugins/${pluginId}/execute`);
      setExecuteResult(JSON.stringify(result, null, 2));
      toast.success(t("plugins.toast.executed"));
    } catch {
      setExecuteResult("Execution failed");
      toast.error(t("plugins.toast.execute_failed"));
    }
  };

  const handleExport = async (pluginId: string) => {
    try {
      const res = await fetch(`/api/plugins/${pluginId}/export`, { credentials: "include", headers: { "Accept": "application/octet-stream", "X-CSRF-Token": getCsrfToken() } });
      if (!res.ok) throw new Error("Export failed");
      await downloadFromResponse(res, `plugin_${pluginId}.json`);
      toast.success(t("plugins.toast.exported"));
    } catch { toast.error(t("plugins.toast.export_failed")); }
  };

  const handleImport = async () => {
    if (!importFile) return;
    try {
      const formData = new FormData();
      formData.append("file", importFile);
      await api.postFormData("/api/plugins/import?format=json", formData);
      toast.success(t("plugins.toast.imported"));
      setShowImport(false);
      setImportFile(null);
      loadPlugins();
    } catch { toast.error(t("plugins.toast.import_failed")); }
  };

  const handleUpdate = async (pluginId: string) => {
    setAction(pluginId, "updating");
    try {
      await api.post(`/api/plugins/${pluginId}/update`);
      toast.success(t("plugins.toast.updated"));
      loadPlugins();
    } catch { toast.error(t("plugins.toast.update_failed")); }
    finally { setAction(pluginId, null); }
  };

  const handleUpdateCheck = async () => {
    try {
      await api.post("/api/plugins/check-updates");
      toast.success(t("plugins.toast.update_check_done"));
      loadPlugins();
    } catch { toast.error(t("plugins.toast.update_check_failed")); }
  };

  const handleLoadReviews = async (plugin: Plugin) => {
    const pid = plugin.id || "";
    setReviewsPlugin(plugin);
    setShowReviews(true);
    try {
      const apiData = await api.get<{ reviews?: Review[]; Reviews?: Review[] } | Review[]>(`/api/plugins/${pid}/reviews`);
      const d = apiData as { reviews?: Review[]; Reviews?: Review[] };
      setReviews(d.reviews || (Array.isArray(apiData) ? apiData : []));
    } catch {
      setReviews([]);
      toast.error(t("plugins.toast.reviews_load_failed"));
    }
  };

  const handlePostReview = async (pluginId: string, rating: number, content: string) => {
    try {
      const body = { rating: String(rating), content };
      await api.post(`/api/plugins/${pluginId}/reviews`, body);
      toast.success(t("plugins.toast.review_posted"));
      if (reviewsPlugin) handleLoadReviews(reviewsPlugin);
    } catch { toast.error(t("plugins.toast.review_post_failed")); }
  };

  const handleRating = async (pluginId: string, rating: number) => {
    try {
      await api.post(`/api/plugins/${pluginId}/rating`, { rating: String(rating) });
      toast.success(t("plugins.toast.rating_submitted"));
      loadPlugins();
    } catch { toast.error(t("plugins.toast.rating_submit_failed")); }
  };

  const handleLoadDependencies = async (plugin: Plugin) => {
    const pid = plugin.id || "";
    try {
      const apiData = await api.get<{ dependencies?: string[]; Dependencies?: string[] } | string[]>(`/api/plugins/${pid}/dependencies`);
      const d = apiData as { dependencies?: string[]; Dependencies?: string[] };
      const deps = d.dependencies || (Array.isArray(apiData) ? apiData : []);
      setDetailPlugin({ ...plugin, Dependencies: Array.isArray(deps) ? deps : [] });
    } catch { toast.error(t("plugins.toast.load_deps_failed")); }
  };

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={<><Puzzle className="w-4 h-4" />{t("plugins.title")}</>} subtitle={t("plugins.subtitle", { count: String(plugins.length) })}>
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="outline" size="lg" onClick={handleUpdateCheck}>
            <CheckCircle className="w-4 h-4" /> {t("plugins.check_updates")}
          </Button>
          <Button onClick={() => setShowCreate(true)}>
            <Plus className="w-4 h-4" /> {t("plugins.create")}
          </Button>
          <Button variant="secondary" onClick={() => setShowImport(true)}>
            <FileDown className="w-4 h-4" /> {t("plugins.import")}
          </Button>
          <div className="flex bg-secondary rounded-xl p-0.5">
            <Button variant={viewMode === "grid" ? "secondary" : "ghost"} size="icon-sm" onClick={() => setViewMode("grid")} aria-label="Grid view">
              <LayoutGrid className="w-4 h-4" />
            </Button>
            <Button variant={viewMode === "list" ? "secondary" : "ghost"} size="icon-sm" onClick={() => setViewMode("list")} aria-label="List view">
              <List className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </PageHeader>

      <Card className="p-4 sm:p-5 mb-4">
        <div className="flex flex-col sm:flex-row gap-3">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none" />
            <Input aria-label={t("plugins.search_ph")} name="input-0" type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t("plugins.search_ph")} className="pl-9 h-10" />
          </div>
        </div>
      </Card>

      <div className="flex flex-wrap gap-2 mb-4">
        {CATEGORIES.map((cat) => {
          const count = cat.key === "" ? plugins.length : plugins.filter((p) => (p.category || "") === cat.key).length;
          return (
            <Button key={cat.key} variant="outline" size="sm" onClick={() => setCategory(cat.key)} className={`rounded-xl transition-all ${category === cat.key ? "ring-2 ring-primary/50 border-primary/30 " + cat.color : "text-muted-foreground"}`}>
              {cat.icon}
              {cat.label}
              <Badge variant="secondary" className="text-(--font-size-micro-sm)">{count}</Badge>
            </Button>
          );
        })}
      </div>

      {loading ? (
        <CardGridSkeleton count={6} columns={3} />
      ) : filtered.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">
          <EmptyState icon={Puzzle} title={t("plugins.empty")} />
        </div>
      ) : viewMode === "grid" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((p, i) => <PluginCard key={p.id || String(i)} plugin={p} actionState={actionStates[p.id || String(i)]} onInstall={handleInstall} onUninstall={handleUninstall} onDelete={handleDelete} onToggle={handleToggle} onDetail={() => { setDetailPlugin(p); handleLoadDependencies(p); }} onExecute={() => { setExecutePlugin(p); setShowExecute(true); setExecuteResult(null); }} onExport={() => handleExport(p.id || "")} onUpdate={() => handleUpdate(p.id || "")} onReviews={() => handleLoadReviews(p)} onRating={(r) => handleRating(p.id || "", r)} />)}
        </div>
      ) : (
        <div className="space-y-3">
          {filtered.map((p, i) => <PluginListItem key={p.id || String(i)} plugin={p} actionState={actionStates[p.id || String(i)]} onInstall={handleInstall} onUninstall={handleUninstall} onDelete={handleDelete} onToggle={handleToggle} onDetail={() => { setDetailPlugin(p); handleLoadDependencies(p); }} onExecute={() => { setExecutePlugin(p); setShowExecute(true); setExecuteResult(null); }} onExport={() => handleExport(p.id || "")} onUpdate={() => handleUpdate(p.id || "")} onReviews={() => handleLoadReviews(p)} onRating={(r) => handleRating(p.id || "", r)} />)}
        </div>
      )}

      {detailPlugin && <PluginDetailModal plugin={detailPlugin} open={!!detailPlugin} onOpenChange={(open) => { if (!open) setDetailPlugin(null); }} />}

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("plugins.create_title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreate} className="space-y-3">
            <Input aria-label={t("plugins.name_ph")} name="plugin-name-1" type="text" required placeholder={t("plugins.name_ph")} value={createForm.name} onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })} />
            <Textarea aria-label={t("plugins.desc_ph")} name="description-2" placeholder={t("plugins.desc_ph")} value={createForm.description} onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })} className="min-h-[5rem] resize-none" />
            <Input aria-label={t("plugins.author_ph")} name="author-3" type="text" placeholder={t("plugins.author_ph")} value={createForm.author} onChange={(e) => setCreateForm({ ...createForm, author: e.target.value })} />
            <Select value={createForm.category} onValueChange={(v) => setCreateForm({ ...createForm, category: v ?? "" })}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("plugins.select_category")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">{t("plugins.select_category")}</SelectItem>
                {CATEGORIES.slice(1).map((c) => <SelectItem key={c.key} value={c.key}>{c.label}</SelectItem>)}
              </SelectContent>
            </Select>
            <Input aria-label="Version (e.g. 1.0.0)" name="version-e-g-1-0-0-5" type="text" placeholder="1.0.0" value={createForm.version} onChange={(e) => setCreateForm({ ...createForm, version: e.target.value })} />
            <Button type="submit" className="w-full h-10">{t("plugins.create_title")}</Button>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showImport} onOpenChange={setShowImport}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("plugins.import_title")}</DialogTitle>
          </DialogHeader>
          <Card className="border-2 border-dashed hover:border-indigo-500 transition-colors">
            <CardContent className="p-4 sm:p-5 text-center">
              <CloudUpload className="w-4 h-4" />
              <p className="text-sm text-muted-foreground mb-3">{t("plugins.upload_desc")}</p>
              <input aria-label="Upload plugin file" name="input-6" type="file" accept=".json,.zip" onChange={(e) => setImportFile(e.target.files?.[0] || null)} className="text-sm text-muted-foreground file:mr-3 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-indigo-50 dark:file:bg-indigo-900/30 file:text-indigo-700 dark:file:text-indigo-400 hover:file:bg-indigo-100" />
              {importFile && <p className="text-xs text-muted-foreground mt-2">{importFile.name}</p>}
            </CardContent>
          </Card>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowImport(false)}>{t("plugins.cancel")}</Button>
            <Button onClick={handleImport}>{t("plugins.import")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={showExecute} onOpenChange={setShowExecute}>
        <DialogContent className="sm:max-w-lg max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t("plugins.execute_title")} {executePlugin?.Name || executePlugin?.name}</DialogTitle>
          </DialogHeader>
          <Button onClick={() => handleExecute(executePlugin?.ID || executePlugin?.id || "")} disabled={executeLoading} className="w-full mb-4">
            {executeLoading ? <><Spinner className="mr-2" />{t("plugins.running")}</> : <><Play className="w-4 h-4" />{t("plugins.run_plugin")}</>}
          </Button>
          {executeResult && (
            <div className="bg-card rounded-xl p-4 max-h-96 overflow-y-auto">
              <pre className="text-xs font-mono text-emerald-300 whitespace-pre-wrap">{executeResult}</pre>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {reviewsPlugin && (
        <ReviewsModal plugin={reviewsPlugin} reviews={reviews} open={showReviews} onOpenChange={setShowReviews} onPost={handlePostReview} />
      )}
      <ConfirmModal open={!!cfm} title={t("plugins.confirm_title")} message={cfm?.msg || ""} confirmText={t("plugins.btn_delete")} cancelText={t("plugins.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}

function ReviewsModal({ plugin, reviews, open, onOpenChange, onPost }: { plugin: Plugin; reviews: Review[]; open: boolean; onOpenChange: (open: boolean) => void; onPost: (id: string, rating: number, content: string) => void }) {
  const { t } = useI18n();
  const pid = plugin.id || "";
  const [rating, setRating] = useState(5);
  const [content, setContent] = useState("");

  const submit = () => {
    onPost(pid, rating, content);
    setContent("");
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("plugins.reviews_title")} {plugin.name}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 mb-4">
          <div className="flex items-center gap-1 mb-2">
            {[1, 2, 3, 4, 5].map((s) => (
              <Button key={s} variant="ghost" size="icon-xs" onClick={() => setRating(s)} className="text-lg" aria-label={`${s} star`}>
                <Star className={`w-4 h-4 ${s <= rating ? "text-amber-400 fill-amber-400" : "text-muted-foreground"}`} />
              </Button>
            ))}
          </div>
          <Textarea aria-label={t("plugins.review_ph")} name="textarea-7" value={content} onChange={(e) => setContent(e.target.value)} placeholder={t("plugins.review_ph")} className="min-h-[5rem] resize-none" />
          <Button onClick={submit} className="w-full">{t("plugins.submit_review")}</Button>
        </div>
        <div className="space-y-3">
          {reviews.length === 0 && <p className="text-sm text-muted-foreground text-center py-4">{t("plugins.no_reviews")}</p>}
          {reviews.map((r) => (
            <div key={r.id} className="bg-muted rounded-xl p-3">
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs font-medium text-muted-foreground">{r.user || t("plugins.anonymous")}</span>
                <div className="flex items-center gap-0.5">
                  {[1, 2, 3, 4, 5].map((s) => (
                    <Star key={s} className={`w-2.5 h-2.5 ${s <= (r.rating || 0) ? "text-amber-400 fill-amber-400" : "text-muted-foreground"}`} />
                  ))}
                </div>
              </div>
              <p className="text-xs text-muted-foreground">{r.content}</p>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
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
  const { t } = useI18n();
  const id = plugin.id || "";
  const name = plugin.name || t("plugins.unknown");
  const version = plugin.version || "1.0.0";
  const desc = plugin.description || "";
  const author = plugin.author || "-";
  const cat = plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const downloads = plugin.downloads || 0;
  const [hoverRating, setHoverRating] = useState(0);
  const catInfo = CATEGORIES.find((c) => c.key === cat);

  return (
    <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-all group">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center shrink-0">
            <Puzzle className="w-4 h-4 text-primary dark:text-indigo-400" />
          </div>
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-foreground truncate cursor-pointer hover:text-primary dark:hover:text-indigo-400 transition-colors" onClick={onDetail}>{name}</h3>
            <p className="text-xs text-muted-foreground">v{version} &middot; {author}</p>
          </div>
        </div>
        {updateAvail && (
          <Button size="xs" onClick={(e) => { e.stopPropagation(); onUpdate(); }} className="shrink-0 px-2 py-0.5 bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 text-(--font-size-micro-sm) font-medium rounded-xl hover:bg-amber-200 dark:hover:bg-amber-900/50 transition-colors" title={t("plugins.update_available")}>
            <ArrowUp className="w-4 h-4" />{t("plugins.update")}
          </Button>
        )}
      </div>

      <p className="text-xs text-muted-foreground mb-3 line-clamp-2 leading-relaxed">{desc || t("plugins.no_desc")}</p>

      <div className="flex items-center gap-2 mb-3 flex-wrap">
        {catInfo && (
          <span className={`text-(--font-size-micro-sm) px-2 py-0.5 rounded-lg ${catInfo.color}`}>
            <span className="mr-1">{catInfo.icon}</span>{catInfo.label}
          </span>
        )}
      </div>

      <div className="flex items-center gap-0.5 mb-1">
        {[1, 2, 3, 4, 5].map((s) => (
          <Button key={s} variant="ghost" size="icon-xs" onClick={() => onRating(s)} onMouseEnter={() => setHoverRating(s)} onMouseLeave={() => setHoverRating(0)} className="p-0.5" aria-label={`${s} star`}>
            <Star className={`w-2.5 h-2.5 transition-colors ${s <= (hoverRating || rating) ? "text-amber-400 fill-amber-400" : "text-muted-foreground"}`} />
          </Button>
        ))}
        <span className="text-(--font-size-micro-sm) text-muted-foreground ml-1">{(hoverRating || rating).toFixed(1)}</span>
        <Button variant="ghost" size="xs" onClick={onReviews} className="text-(--font-size-micro-sm) text-primary ml-1 hover:underline">{t("plugins.reviews")}</Button>
      </div>
      <div className="text-(--font-size-micro-sm) text-muted-foreground mb-3">{downloads.toLocaleString()} {t("plugins.downloads")}</div>

      {deps.length > 0 && (
        <div className="mb-3 text-(--font-size-micro-sm) text-muted-foreground">
          <Link className="w-4 h-4" />{t("plugins.deps")} {deps.join(", ")}
        </div>
      )}

        <div className="flex items-center justify-between pt-3 border-t border-border">
        {installed ? (
          <div className="flex items-center gap-2">
            <Switch checked={enabled} onCheckedChange={() => onToggle(id, !enabled)} disabled={actionState === "toggling"} />
            <span className="text-(--font-size-micro-sm) text-muted-foreground">{enabled ? t("plugins.enabled") : t("plugins.disabled")}</span>
          </div>
        ) : (
          <span className="text-(--font-size-micro-sm) text-muted-foreground">{t("plugins.not_installed")}</span>
        )}
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon-xs" onClick={onExecute} disabled={!installed} className="text-muted-foreground hover:text-emerald-500 hover:bg-emerald-50 dark:hover:bg-emerald-900/20 disabled:opacity-30" aria-label="Execute plugin"><Play className="w-4 h-4" /></Button>
          <Button variant="ghost" size="icon-xs" onClick={onExport} className="text-muted-foreground hover:text-primary hover:bg-primary/10 dark:hover:bg-primary/20" aria-label="Export plugin"><Download className="w-4 h-4" /></Button>
          {installed ? (
            <>
              <Button size="xs" onClick={() => onUninstall(id)} disabled={actionState === "uninstalling"} className="px-2 py-1 text-(--font-size-micro-sm) font-medium text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 hover:bg-amber-100 dark:hover:bg-amber-900/40 transition-colors disabled:opacity-50">{actionState === "uninstalling" ? <Spinner /> : t("plugins.uninstall")}</Button>
              <Button variant="ghost" size="icon-xs" onClick={() => onDelete(id)} disabled={actionState === "deleting"} className="text-muted-foreground hover:text-destructive hover:bg-destructive/10 disabled:opacity-50" aria-label={t("plugins.delete")}><Trash2 className="w-4 h-4" /></Button>
            </>
          ) : (
            <Button size="xs" onClick={() => onInstall(id)} disabled={actionState === "installing"} className="px-2.5 py-1 text-(--font-size-micro-sm) font-medium text-primary dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/20 hover:bg-indigo-100 dark:hover:bg-indigo-900/40 transition-colors disabled:opacity-50">{actionState === "installing" ? <><Spinner className="mr-1" />...</> : t("plugins.install")}</Button>
          )}
          <Button variant="ghost" size="icon-xs" onClick={onDetail} className="text-muted-foreground hover:text-primary hover:bg-indigo-50 dark:hover:bg-indigo-900/20" aria-label="View plugin details"><Info className="w-4 h-4" /></Button>
        </div>
      </div>
    </Card>
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
  const { t } = useI18n();
  const id = plugin.id || "";
  const name = plugin.name || t("plugins.unknown");
  const version = plugin.version || "1.0.0";
  const desc = plugin.description || "";
  const cat = plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const catInfo = CATEGORIES.find((c) => c.key === cat);

  return (
    <Card className="p-4 hover:shadow-lg dark:hover:shadow-black/30 transition-all">
      <div className="flex items-center gap-4">
        <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center shrink-0">
          <Puzzle className="w-4 h-4" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold text-foreground truncate cursor-pointer hover:text-primary dark:hover:text-indigo-400 transition-colors" onClick={onDetail}>{name}</h3>
            <span className="text-(--font-size-micro-sm) text-muted-foreground">v{version}</span>
            {catInfo && <span className={`text-(--font-size-micro-sm) px-1.5 py-0.5 rounded ${catInfo.color}`}>{catInfo.label}</span>}
            {updateAvail && <Button size="xs" onClick={onUpdate} className="text-(--font-size-micro-sm) px-1.5 py-0.5 rounded bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 hover:bg-amber-200"><ArrowUp className="w-4 h-4" />{t("plugins.update")}</Button>}
          </div>
          <p className="text-xs text-muted-foreground truncate">{desc || t("plugins.no_desc_short")}</p>
        </div>
        <div className="flex items-center gap-0.5 shrink-0">
          {[1, 2, 3, 4, 5].map((s) => <Button key={s} variant="ghost" size="icon-xs" onClick={() => onRating(s)} aria-label={`${s} star`}><Star className={`w-2.5 h-2.5 hover:text-amber-400 transition-colors ${s <= rating ? "text-amber-400 fill-amber-400" : "text-muted-foreground"}`} /></Button>)}
        <Button variant="ghost" size="xs" onClick={onReviews} className="text-(--font-size-micro-sm) text-primary ml-1 hover:underline">{t("plugins.reviews")}</Button>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button variant="ghost" size="icon-xs" onClick={onExecute} disabled={!installed} className="text-muted-foreground hover:text-emerald-500 hover:bg-emerald-50 dark:hover:bg-emerald-900/20 disabled:opacity-30" aria-label="Execute plugin"><Play className="w-4 h-4" /></Button>
          <Button variant="ghost" size="icon-xs" onClick={onExport} className="text-muted-foreground hover:text-primary hover:bg-primary/10 dark:hover:bg-primary/20" aria-label="Export plugin"><Download className="w-4 h-4" /></Button>
          {installed ? (
            <>
              <Switch checked={enabled} onCheckedChange={() => onToggle(id, !enabled)} disabled={actionState === "toggling"} className="shrink-0" />
              <Button size="xs" onClick={() => onUninstall(id)} disabled={actionState === "uninstalling"} className="px-2 py-1 text-(--font-size-micro-sm) font-medium text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 hover:bg-amber-100 dark:hover:bg-amber-900/40 transition-colors disabled:opacity-50">{actionState === "uninstalling" ? <Spinner /> : t("plugins.uninstall")}</Button>
              <Button variant="ghost" size="icon-xs" onClick={() => onDelete(id)} disabled={actionState === "deleting"} className="text-muted-foreground hover:text-destructive hover:bg-destructive/10 disabled:opacity-50" aria-label={t("plugins.delete")}><Trash2 className="w-4 h-4" /></Button>
            </>
          ) : (
            <Button onClick={() => onInstall(id)} disabled={actionState === "installing"} className="px-2.5 py-1 text-(--font-size-micro-sm) font-medium text-primary dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/20 hover:bg-indigo-100 dark:hover:bg-indigo-900/40 transition-colors disabled:opacity-50">{actionState === "installing" ? <><Spinner className="mr-1" />{t("plugins.installing")}</> : t("plugins.install")}</Button>
          )}
          <Button variant="ghost" size="icon-xs" onClick={onDetail} className="text-muted-foreground hover:text-primary hover:bg-indigo-50 dark:hover:bg-indigo-900/20" aria-label="View plugin details"><Info className="w-4 h-4" /></Button>
        </div>
      </div>
      {deps.length > 0 && (
        <div className="mt-2 ml-14 text-(--font-size-micro-sm) text-muted-foreground">
          <Link className="w-4 h-4" />{t("plugins.dependencies")} {deps.join(", ")}
        </div>
      )}
    </Card>
  );
}

function PluginDetailModal({ plugin, open, onOpenChange }: {
  plugin: Plugin;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useI18n();
  const name = plugin.name || t("plugins.unknown");
  const version = plugin.version || "1.0.0";
  const desc = plugin.description || t("plugins.no_desc");
  const author = plugin.author || "-";
  const cat = plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const readme = plugin.readme || "";
  const downloads = plugin.downloads || 0;
  const lastUpdated = plugin.last_updated || "-";

  const catInfo = CATEGORIES.find((c) => c.key === cat);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-hidden flex flex-col p-0">
        <DialogHeader className="px-6 py-4 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center">
              <Puzzle className="w-4 h-4 text-primary dark:text-indigo-400" />
            </div>
            <div>
              <DialogTitle>{name}</DialogTitle>
              <p className="text-xs text-muted-foreground">v{version} &middot; {author}</p>
            </div>
          </div>
        </DialogHeader>

        <div className="overflow-y-auto flex-1 px-6 py-4 space-y-5">
          <div className="flex items-center gap-3 flex-wrap">
            {catInfo && (
              <span className={`text-xs px-2.5 py-1 rounded-lg ${catInfo.color}`}>
                <span className="mr-1">{catInfo.icon}</span>{catInfo.label}
              </span>
            )}
            {installed ? (
              <Badge variant={enabled ? "success" : "secondary"} className="text-xs">
                {enabled ? <CheckCircle className="w-3 h-3 mr-1" /> : <span className="w-2 h-2 rounded-full bg-current inline-block mr-1" />}{enabled ? t("plugins.enabled") : t("plugins.disabled")}
              </Badge>
            ) : (
              <Badge variant="secondary" className="text-xs">{t("plugins.not_installed")}</Badge>
            )}
            {updateAvail && (
              <Badge variant="warning" className="text-xs">
                <ArrowUp className="w-4 h-4" />{t("plugins.update_available")}
              </Badge>
            )}
          </div>

          <div className="flex items-center gap-4">
            <div className="flex items-center gap-0.5">
              {[1, 2, 3, 4, 5].map((s) => (
                <Star key={s} className={`w-3 h-3 ${s <= rating ? "text-amber-400 fill-amber-400" : "text-muted-foreground"}`} />
              ))}
              <span className="text-xs text-muted-foreground ml-1">{rating.toFixed(1)}</span>
            </div>
            <span className="text-xs text-muted-foreground"><Download className="w-4 h-4" />{downloads.toLocaleString()} {t("plugins.downloads")}</span>
            <span className="text-xs text-muted-foreground"><Clock className="w-4 h-4" />{t("plugins.updated")}: {lastUpdated}</span>
          </div>

          <div>
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">{t("plugins.description")}</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">{desc}</p>
          </div>

          {deps.length > 0 && (
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">{t("plugins.dependencies")}</h3>
              <div className="flex flex-wrap gap-2">
                {deps.map((d, i) => (
                  <Badge key={i} variant="outline" className="font-mono">{d}</Badge>
                ))}
              </div>
            </div>
          )}

          {readme && (
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">{t("plugins.readme")}</h3>
              <div className="bg-muted border border-border rounded-xl p-4 text-xs text-muted-foreground whitespace-pre-wrap max-h-60 overflow-y-auto">{readme}</div>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}


