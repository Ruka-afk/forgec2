"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { downloadJSON } from "@/lib/download";
import { EmptyState, PageHeader, Spinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { AlertTriangle, Clock, Code, Copy, Download, FileCode, FileDown, FileEdit, FileWarning, List, PenTool, Plus, RotateCw, Save, Search, Send, Server, Shield, Trash2, X } from "lucide-react";

interface MalleableForm {
  enabled: boolean;
  status_code: number;
  content_type: string;
  headers_text: string;
  prepend: string;
  append: string;
}

interface AgentProfile {
  name: string;
  description: string;
  user_agent: string;
  beacon_uri: string;
  method: string;
  headers: Record<string, string>;
  sleep: number;
  jitter: number;
}

interface ActiveMalleableConfig {
  malleable_enabled: boolean;
  malleable_profile: string;
  status_code: number;
  content_type: string;
  headers: Record<string, string>;
  user_agent: string;
  jitter: number;
  interval: number;
  prepend: string;
  append: string;
}

const commonUAs = [
  { label: "Chrome 120 (Windows)", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36" },
  { label: "Edge 120 (Windows)", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 Edg/120.0.0.0" },
  { label: "Firefox 121 (Windows)", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0" },
  { label: "Cloudflare Health Check", value: "Mozilla/5.0 (compatible; Cloudflare-Health-Checks/1.0; +https://www.cloudflare.com/)" },
  { label: "GitHub Hookshot", value: "GitHub-Hookshot/abcd1234" },
  { label: "Office 365", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 OPR/106.0.0.0" },
  { label: "Microsoft Teams", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Teams/1.6.00.27573" },
  { label: "Slack", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 Slack/4.36.0" },
  { label: "Zoom", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Zoom/5.17.5" },
  { label: "Dropbox", value: "DropboxDesktopClient/187.4.6204 (Windows; 10.0; Win64; x64)" },
  { label: "Windows Update", value: "Windows-Update-Agent/10.0.19041.3636" },
  { label: "Safari (macOS)", value: "Mac OS X/10.15.7 (KHTML, like Gecko) Version/17.2 Safari/605.1.15" },
  { label: "Adobe Creative Cloud", value: "Creative Cloud/6.4.0.361 (Windows; x64)" },
  { label: "Linux Browser", value: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36" },
];

const emptyProfile = (): AgentProfile => ({
  name: "",
  description: "",
  user_agent: commonUAs[0].value,
  beacon_uri: "/api/v1/beacon",
  method: "POST",
  headers: { "Accept": "*/*" },
  sleep: 10,
  jitter: 20,
});

const emptyActiveConfig = (): ActiveMalleableConfig => ({
  malleable_enabled: false,
  malleable_profile: "",
  status_code: 200,
  content_type: "application/json",
  headers: {},
  user_agent: "",
  jitter: 0,
  interval: 0,
  prepend: "",
  append: "",
});

export default function ProfilesPage() {
  const [malleableForm, setMalleableForm] = useState<MalleableForm>({
    enabled: false, status_code: 200, content_type: "application/json",
    headers_text: "", prepend: "", append: "",
  });
  const [savingMalleable, setSavingMalleable] = useState(false);
  const [profiles, setProfiles] = useState<AgentProfile[]>([]);
  const [selectedIdx, setSelectedIdx] = useState<number>(-1);
  const selectedIdxRef = useRef(selectedIdx);
  selectedIdxRef.current = selectedIdx;
  const [editing, setEditing] = useState<AgentProfile>(emptyProfile());
  const [search, setSearch] = useState("");
  const [loadingProfiles, setLoadingProfiles] = useState(true);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [activeConfig, setActiveConfig] = useState<ActiveMalleableConfig>(emptyActiveConfig());
  const [loadingActiveConfig, setLoadingActiveConfig] = useState(true);
  const [reloading, setReloading] = useState(false);
  const [lastReload, setLastReload] = useState<string | null>(null);
  const [showPushModal, setShowPushModal] = useState(false);
  const [pushAgents, setPushAgents] = useState<{ id: string; hostname: string; ip: string }[]>([]);
  const [pushSelected, setPushSelected] = useState<string[]>([]);
  const [pushing, setPushing] = useState(false);
  const [loadingAgents, setLoadingAgents] = useState(false);

  const loadActiveConfig = useCallback(async () => {
    setLoadingActiveConfig(true);
    try {
      const data = await api.get<ActiveMalleableConfig>("/integrations/malleable");
      setActiveConfig({
        malleable_enabled: (data.malleable_enabled ?? false) as boolean,
        malleable_profile: (data.malleable_profile ?? "") as string,
        status_code: (data.status_code ?? 200) as number,
        content_type: (data.content_type ?? "application/json") as string,
        headers: (data.headers ?? {}) as Record<string, string>,
        user_agent: (data.user_agent ?? "") as string,
        jitter: (data.jitter ?? 0) as number,
        interval: (data.interval ?? 0) as number,
        prepend: (data.prepend ?? "") as string,
        append: (data.append ?? "") as string,
      });
    } catch {
    } finally {
      setLoadingActiveConfig(false);
    }
  }, []);

  const loadMalleableSettings = useCallback(async () => {
    try {
      const d = await api.get("/settings");
      setMalleableForm({
        enabled: (d.malleable_enabled ?? false) as boolean,
        status_code: (d.malleable_status ?? 200) as number,
        content_type: (d.malleable_ct ?? "application/json") as string,
        headers_text: "",
        prepend: (d.malleable_prepend ?? "") as string,
        append: (d.malleable_append ?? "") as string,
      });
    } catch {
      toast.error("Failed to load malleable settings");
    }
  }, []);

  const handleReloadConfig = useCallback(async () => {
    setReloading(true);
    try {
      const data = await api.post("/config/reload");
      if (!data.success) {
        toast.error((data.error as string) || "Failed to reload malleable config");
        return;
      }
      setLastReload(new Date().toLocaleTimeString());
      toast.success("Malleable config hot-reloaded successfully");
      await loadActiveConfig();
      await loadMalleableSettings();
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setReloading(false);
    }
  }, [loadActiveConfig, loadMalleableSettings]);

  const loadProfiles = useCallback(async () => {
    setLoadingProfiles(true);
    try {
      const d = await api.get("/api/generate/profiles");
      const list = (d.profiles || []) as AgentProfile[];
      setProfiles(list);
      if (list.length > 0 && selectedIdxRef.current < 0) {
        setSelectedIdx(0);
        setEditing({ ...list[0] });
      }
    } catch {
      toast.error("Failed to load agent profiles");
    } finally {
      setLoadingProfiles(false);
    }
  }, []);

  useEffect(() => {
    loadActiveConfig();
    loadMalleableSettings();
    loadProfiles();
  }, [loadActiveConfig, loadMalleableSettings, loadProfiles]);

  const selectProfile = (idx: number) => {
    setSelectedIdx(idx);
    setEditing({ ...profiles[idx] });
  };

  const filteredProfiles = profiles.filter((p) =>
    p.name.toLowerCase().includes(search.toLowerCase()) ||
    (p.description || "").toLowerCase().includes(search.toLowerCase())
  );

  const handleSaveMalleable = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingMalleable(true);
    try {
      await api.post("/settings/malleable", {
        enabled: String(malleableForm.enabled),
        status_code: String(malleableForm.status_code),
        content_type: malleableForm.content_type,
        headers_text: malleableForm.headers_text,
        prepend: malleableForm.prepend,
        append: malleableForm.append,
      });
      toast.success("Malleable C2 profile saved");
    } catch {
      toast.error("Failed to save malleable profile");
    } finally {
      setSavingMalleable(false);
    }
  };

  const handleSaveProfile = () => {
    downloadJSON(editing, `${editing.name || "profile"}.json`);
    toast.success("Profile exported as JSON \u2014 use Import to update on server");
  };

  const handleDeleteProfile = () => {
    if (selectedIdx < 0) return;
    const name = editing.name;
    setProfiles((prev) => prev.filter((_, i) => i !== selectedIdx));
    if (selectedIdx >= profiles.length - 1) {
      const newIdx = Math.max(0, profiles.length - 2);
      setSelectedIdx(newIdx >= 0 ? newIdx : -1);
      if (newIdx >= 0) setEditing({ ...profiles[newIdx] });
      else setEditing(emptyProfile());
    } else {
      const newIdx = selectedIdx;
      setSelectedIdx(newIdx);
      setEditing({ ...profiles[newIdx] });
    }
    toast.success(`Profile "${name}" removed from list (file not deleted on server)`);
  };

  const handleDuplicateProfile = () => {
    const copy: AgentProfile = {
      ...editing,
      name: editing.name + "_copy",
    };
    const idx = profiles.length;
    setProfiles((prev) => [...prev, copy]);
    setSelectedIdx(idx);
    setEditing(copy);
    toast.success("Profile duplicated");
  };

  const handleImportProfile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fd = new FormData();
    fd.append("profile", file);
    try {
      const d = await api.postFormData("/api/generate/profile/import", fd) as { success?: boolean; error?: string; profile?: AgentProfile };
      if (!d.success) {
        toast.error(d.error || "Failed to import malleable profile");
        return;
      }
      const imported: AgentProfile = d.profile!;
      setProfiles((prev) => {
        const idx = prev.findIndex((p) => p.name === imported.name);
        if (idx >= 0) {
          const next = [...prev];
          next[idx] = imported;
          return next;
        }
        return [...prev, imported];
      });
      setSelectedIdx(profiles.length);
      setEditing(imported);
      toast.success("Profile imported");
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      e.target.value = "";
    }
  };

  const updateHeader = (idx: number, key: string, value: string) => {
    const entries = Object.entries(editing.headers);
    if (key === "") {
      const [, ...rest] = entries;
      const newHeaders: Record<string, string> = {};
      for (const [k, v] of rest) newHeaders[k] = v;
      setEditing({ ...editing, headers: newHeaders });
      return;
    }
    const newHeaders: Record<string, string> = {};
    entries.forEach(([k, v], i) => {
      if (i === idx) newHeaders[key] = value;
      else newHeaders[k] = v;
    });
    setEditing({ ...editing, headers: newHeaders });
  };

  const addHeader = () => {
    setEditing({ ...editing, headers: { ...editing.headers, "": "" } });
  };

  const removeHeader = (idx: number) => {
    const entries = Object.entries(editing.headers).filter((_, i) => i !== idx);
    const newHeaders: Record<string, string> = {};
    for (const [k, v] of entries) newHeaders[k] = v;
    setEditing({ ...editing, headers: newHeaders });
  };

  const inputCls = "w-full rounded-lg border border-border bg-background px-3 py-2 text-sm placeholder:text-muted-foreground/50 focus:outline-none focus:ring-2 focus:ring-ring/30";
  const headerEntries = Object.entries(activeConfig.headers);

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title="Profile Editor" subtitle="Manage malleable C2 profiles for beacon traffic" />

      {/* Active Config Hot-Reload Card */}
      <Card className="overflow-hidden mb-6">
        <div className="bg-gradient-to-r from-emerald-600 to-emerald-800 px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><RotateCw className="w-4 h-4" /></div>
              <div>
                <h2 className="text-lg font-semibold text-white">Active Malleable Config</h2>
                <p className="text-xs text-emerald-200">Currently loaded in-memory configuration</p>
              </div>
            </div>
            <Button
              onClick={handleReloadConfig}
              disabled={reloading}
              className="h-10 px-5 bg-secondary/60 hover:bg-secondary/80 text-foreground rounded-xl text-sm font-medium transition-colors flex items-center gap-2 disabled:opacity-50"
            >
              {reloading ? <Spinner size="xs" /> : <RotateCw className="w-4 h-4" />}
              {reloading ? "Reloading..." : "Reload Config"}
            </Button>
          </div>
        </div>
        <CardContent className="p-5">
          {loadingActiveConfig ? (
            <div className="flex items-center justify-center py-6">
              <Spinner color="emerald" />
              <span className="ml-3 text-sm text-muted-foreground">Loading active config...</span>
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
                <div className="bg-secondary rounded-xl p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">Malleable Enabled</div>
                  <div className="flex items-center gap-2">
                    <span className={"inline-block w-2 h-2 rounded-full " + (activeConfig.malleable_enabled ? "bg-emerald-500" : "bg-muted-foreground")}></span>
                    <span className="text-sm font-medium">{activeConfig.malleable_enabled ? "Enabled" : "Disabled"}</span>
                  </div>
                </div>
                <div className="bg-secondary rounded-xl p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">Profile Name</div>
                  <div className="text-sm font-medium font-mono">{activeConfig.malleable_profile || "N/A"}</div>
                </div>
                <div className="bg-secondary rounded-xl p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">User-Agent</div>
                  <div className="text-sm font-mono truncate" title={activeConfig.user_agent}>{activeConfig.user_agent || "N/A"}</div>
                </div>
                <div className="bg-secondary rounded-xl p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">Beacon Interval / Jitter</div>
                  <div className="text-sm font-medium">{activeConfig.interval}s / {activeConfig.jitter}%</div>
                </div>
                <div className="bg-secondary rounded-xl p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">Status Code</div>
                  <div className="text-sm font-medium font-mono">{activeConfig.status_code}</div>
                </div>
                <div className="bg-secondary rounded-xl p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">Content-Type</div>
                  <div className="text-sm font-medium font-mono">{activeConfig.content_type}</div>
                </div>
              </div>

              <div className="bg-secondary rounded-xl p-3 border border-border mb-4">
                <div className="text-xs text-muted-foreground mb-2">Headers ({headerEntries.length})</div>
                {headerEntries.length === 0 ? (
                  <span className="text-xs text-muted-foreground italic">No custom headers configured</span>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                    {headerEntries.map(([key, value]) => (
                      <div key={key} className="flex items-center gap-2 text-xs font-mono">
                        <span className="text-indigo-500">{key}:</span>
                        <span className="text-muted-foreground truncate">{value}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-3">
                <div className="bg-secondary rounded-xl p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">Prepend</div>
                  <div className="text-xs font-mono text-muted-foreground truncate">{activeConfig.prepend || "(empty)"}</div>
                </div>
                <div className="bg-secondary rounded-xl p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">Append</div>
                  <div className="text-xs font-mono text-muted-foreground truncate">{activeConfig.append || "(empty)"}</div>
                </div>
              </div>

              {lastReload && (
                <div className="text-xs text-emerald-600 flex items-center gap-1">
                  <Clock className="w-4 h-4" />
                  Last reloaded at {lastReload}
                </div>
              )}

              <div className="p-3 bg-amber-50 dark:bg-amber-900/20 rounded-xl border border-amber-200 dark:border-amber-800 text-xs text-amber-700 dark:text-amber-400 mt-3">
                <AlertTriangle className="w-4 h-4" />
                Reloading reads config.yaml from disk and applies changes in-memory. Listeners are not restarted.
                Modified JWT secret takes effect immediately for new sessions.
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* Tab switcher */}
      <Tabs defaultValue="server">
        <TabsList className="mb-6">
          <TabsTrigger value="server" className="gap-2">
            <Server className="w-4 h-4" />Server Malleable Config
          </TabsTrigger>
          <TabsTrigger value="agents" className="gap-2">
            <PenTool className="w-4 h-4" />Agent Profiles
          </TabsTrigger>
        </TabsList>

      <TabsContent value="server">
        <Card className="overflow-hidden">
          <div className="bg-gradient-to-r from-violet-600 to-violet-800 px-6 py-4">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Shield className="w-4 h-4" /></div>
              <div><h2 className="text-lg font-semibold text-white">Malleable C2 Profile</h2><p className="text-xs text-violet-200">Customize beacon traffic characteristics</p></div>
            </div>
          </div>
          <CardContent className="p-4 sm:p-5">
            <form onSubmit={handleSaveMalleable} className="space-y-4">
              <div className="flex items-center gap-3">
                <Switch checked={malleableForm.enabled} onCheckedChange={(v) => setMalleableForm({ ...malleableForm, enabled: v })} />
                <span className="text-sm text-muted-foreground">{malleableForm.enabled ? "Enabled" : "Disabled"}</span>
                <span className="text-xs text-muted-foreground">Override default JSON response format</span>
              </div>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div>
                  <Label className="text-xs text-muted-foreground mb-1.5">HTTP Status Code</Label>
                  <Input aria-label="HTTP status code" name="input-1" type="number" min={100} max={599} value={malleableForm.status_code} onChange={(e) => setMalleableForm({ ...malleableForm, status_code: Number(e.target.value) })} className={inputCls} />
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground mb-1.5">Content-Type</Label>
                  <Input aria-label="application-json" name="application-json-2" type="text" placeholder="application/json" value={malleableForm.content_type} onChange={(e) => setMalleableForm({ ...malleableForm, content_type: e.target.value })} className={inputCls + " font-mono"} />
                </div>
              </div>
              <div>
                <Label className="text-xs text-muted-foreground mb-1.5">Custom Headers (one per line)</Label>
                <Textarea aria-label="Custom HTTP headers, one per line" name="textarea-3" rows={3} value={malleableForm.headers_text} onChange={(e) => setMalleableForm({ ...malleableForm, headers_text: e.target.value })} placeholder={"Server: nginx/1.24.0\nX-Powered-By: ASP.NET"} className="font-mono" />
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <Label className="text-xs text-muted-foreground mb-1.5">Prepend Content</Label>
                  <Textarea aria-label="Prepend content before response" name="textarea-4" rows={2} value={malleableForm.prepend} onChange={(e) => setMalleableForm({ ...malleableForm, prepend: e.target.value })} placeholder="<html><body><!--" className="font-mono" />
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground mb-1.5">Append Content</Label>
                  <Textarea aria-label="Append content after response" name="textarea-5" rows={2} value={malleableForm.append} onChange={(e) => setMalleableForm({ ...malleableForm, append: e.target.value })} placeholder="--></body></html>" className="font-mono" />
                </div>
              </div>
              <div className="p-3 bg-amber-50 dark:bg-amber-900/20 rounded-xl border border-amber-200 dark:border-amber-800 text-xs text-amber-700 dark:text-amber-400">
                <AlertTriangle className="w-4 h-4" />
                Enabling profile requires compatible agents. Prepend/append is for traffic camouflage only.
              </div>
              <Button type="submit" disabled={savingMalleable} className="h-11 px-6 bg-violet-600 hover:bg-violet-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
                <Save className="w-4 h-4" />Save Profile
              </Button>
            </form>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="agents">
        <div className="flex flex-col lg:flex-row gap-4">
          {/* Left sidebar - profile list */}
          <div className="w-full lg:w-72 shrink-0">
            <Card className="overflow-hidden">
              <div className="bg-gradient-to-r from-indigo-600 to-indigo-800 px-5 py-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <List className="w-4 h-4" />
                    <span className="text-sm font-semibold text-white">Profiles</span>
                    <span className="text-xs text-indigo-200 ml-1">({profiles.length})</span>
                  </div>
                  <div className="flex gap-1">
                    <Button onClick={() => fileInputRef.current?.click()} title="Import Profile" className="w-7 h-7 bg-secondary/50 hover:bg-secondary/70 rounded-xl flex items-center justify-center transition-colors" aria-label="Import profile" size="icon">
                      <FileDown className="w-4 h-4" />
                    </Button>
                    <Button onClick={() => {
                      const p = emptyProfile();
                      const idx = profiles.length;
                      setProfiles((prev) => [...prev, p]);
                      setSelectedIdx(idx);
                      setEditing(p);
                    }} title="New Profile" className="w-7 h-7 bg-secondary/50 hover:bg-secondary/70 rounded-xl flex items-center justify-center transition-colors" aria-label="New profile" size="icon">
                      <Plus className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              </div>
              <CardContent className="p-4 sm:p-5">
                <div className="relative mb-3">
                  <Search className="w-4 h-4" />
                  <Input aria-label="Filter by name..." name="filter-by-name-6"
                    type="text" placeholder="Filter by name..."
                    value={search} onChange={(e) => setSearch(e.target.value)}
                    className="w-full pl-9 pr-4 h-9 text-xs"
                  />
                </div>
                <div className="space-y-1 max-h-[500px] overflow-y-auto">
                  {loadingProfiles ? (
                    <div className="flex items-center justify-center py-16 sm:py-20"><Spinner size="sm" /></div>
                  ) : filteredProfiles.length === 0 ? (
                    <div className="text-center py-16 sm:py-20 text-xs text-muted-foreground">
                      <FileWarning className="w-4 h-4" />
                      {search ? "No matching profiles" : "No profiles loaded"}
                    </div>
                  ) : (
                    filteredProfiles.map((p) => {
                      const realIdx = profiles.indexOf(p);
                      return (
                        <Button
                          variant="ghost"
                          size="sm"
                          key={p.name}
                          onClick={() => selectProfile(realIdx)}
                          className={"w-full text-left px-3 py-2.5 rounded-xl text-xs transition-colors " + (selectedIdx === realIdx ? "bg-indigo-50 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300 border border-indigo-200 dark:border-indigo-800" : "hover:bg-secondary text-muted-foreground border border-transparent")}
                        >
                          <div className="flex items-center gap-2">
                            <FileCode className={`w-4 h-4 ${selectedIdx === realIdx ? "text-indigo-500" : "text-muted-foreground"}`} />
                            <span className="font-medium truncate">{p.name}</span>
                          </div>
                          {p.description && (
                            <p className="text-[10px] text-muted-foreground mt-0.5 truncate pl-6">{p.description}</p>
                          )}
                        </Button>
                      );
                    })
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
          {/* Right panel - profile editor */}
          <div className="flex-1 min-w-0">
            {selectedIdx < 0 ? (
                <EmptyState icon={FileEdit} title="Select a profile from the list or create a new one" />
            ) : (
              <div className="space-y-5">
                <Card className="overflow-hidden">
                  <div className="bg-gradient-to-r from-indigo-600 to-indigo-800 px-6 py-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><FileCode className="w-4 h-4" /></div>
                        <div>
                          <h2 className="text-lg font-semibold text-white">Editing: {editing.name || "untitled"}</h2>
                          <p className="text-xs text-indigo-200">{editing.description || "No description"}</p>
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <Button onClick={handleSaveProfile} className="h-9 px-4 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-xs font-medium transition-colors flex items-center gap-1.5">
                          <Download className="w-4 h-4" />Save (Export JSON)
                        </Button>
                        <Button onClick={handleDuplicateProfile} variant="secondary" className="h-9 px-4 rounded-xl text-xs font-medium transition-colors flex items-center gap-1.5">
                          <Copy className="w-4 h-4" />Duplicate
                        </Button>
                        <Button onClick={handleDeleteProfile} className="h-9 px-4 bg-destructive hover:bg-destructive/90 text-destructive-foreground rounded-xl text-xs font-medium transition-colors flex items-center gap-1.5">
                          <Trash2 className="w-4 h-4" />Delete
                        </Button>
                        <Button onClick={() => setShowPushModal(true)} className="h-9 px-4 rounded-xl text-xs font-medium transition-colors flex items-center gap-1.5">
                          <Send className="w-4 h-4" />Push to Agent
                        </Button>
                      </div>
                    </div>
                  </div>
<CardContent className="p-4 sm:p-5">
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                      <div className="space-y-4">
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">Profile Name</Label>
                          <Input aria-label="profile_name" name="input-7" type="text" value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} className={inputCls} placeholder="profile_name" />
                        </div>
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">Description</Label>
                          <Textarea aria-label="Brief description of this profile" name="textarea-8" rows={2} value={editing.description} onChange={(e) => setEditing({ ...editing, description: e.target.value })} placeholder="Brief description of this profile" />
                        </div>
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">User-Agent</Label>
                          <div className="flex gap-2">
                            <Input aria-label="User-Agent string" name="input-9" type="text" value={editing.user_agent} onChange={(e) => setEditing({ ...editing, user_agent: e.target.value })} className={inputCls + " font-mono text-xs"} />
                            <Select value="" onValueChange={(v) => { if (v) setEditing({ ...editing, user_agent: v }); }}>
                              <SelectTrigger className="shrink-0 w-[180px] h-11 text-xs">
                                <SelectValue placeholder="Common UAs..." />
                              </SelectTrigger>
                              <SelectContent>
                                {commonUAs.map((ua) => (
                                  <SelectItem key={ua.label} value={ua.value}>{ua.label}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                        </div>
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">Beacon URI</Label>
                          <Input aria-label="/api/v1/beacon" name="input-11" type="text" value={editing.beacon_uri} onChange={(e) => setEditing({ ...editing, beacon_uri: e.target.value })} className={inputCls + " font-mono"} placeholder="/api/v1/beacon" />
                        </div>
                      </div>
                      <div className="space-y-4">
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">HTTP Method</Label>
                          <Select value={editing.method} onValueChange={(v) => { if (v) setEditing({ ...editing, method: v }); }}>
                            <SelectTrigger className={inputCls}>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="GET">GET</SelectItem>
                              <SelectItem value="POST">POST</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        <div className="grid grid-cols-2 gap-4">
                          <div>
                            <Label className="text-xs text-muted-foreground mb-1.5">Sleep Interval (sec)</Label>
                            <Input aria-label="Sleep interval in seconds" name="input-13" type="number" min={0} max={86400} value={editing.sleep} onChange={(e) => setEditing({ ...editing, sleep: Number(e.target.value) })} className={inputCls} />
                          </div>
                          <div>
                            <Label className="text-xs text-muted-foreground mb-1.5">Jitter (%)</Label>
                            <Input aria-label="Jitter percentage" name="input-14" type="number" min={0} max={100} value={editing.jitter} onChange={(e) => setEditing({ ...editing, jitter: Number(e.target.value) })} className={inputCls} />
                          </div>
                        </div>
                        <div>
                          <div className="flex items-center justify-between mb-1.5">
                            <Label className="text-xs text-muted-foreground">Custom Headers</Label>
                            <Button type="button" onClick={addHeader} className="text-xs text-indigo-600 hover:underline flex items-center gap-1" variant="link" size="sm">
                              <Plus className="w-4 h-4" />Add
                            </Button>
                          </div>
                          <div className="space-y-2">
                            {Object.entries(editing.headers).map(([key, value], i) => (
                              <div key={i} className="flex gap-2 items-start">
                                <Input aria-label="Header name" name="header-name-15"
                                  type="text" placeholder="Header name"
                                  value={key} onChange={(e) => updateHeader(i, e.target.value, value)}
                                  className="w-2/5 h-9 text-xs font-mono"
                                />
                                <Input aria-label="Value" name="value-16"
                                  type="text" placeholder="Value"
                                  value={value} onChange={(e) => updateHeader(i, key, e.target.value)}
                                  className="flex-1 h-9 text-xs font-mono"
                                />
                                {Object.entries(editing.headers).length > 1 && (
                                   <Button onClick={() => removeHeader(i)} className="w-9 h-9 flex items-center justify-center text-destructive/50 hover:text-destructive hover:bg-destructive/10 rounded-xl transition-colors" variant="ghost" size="icon" aria-label="Remove header">
                                    <X className="w-4 h-4" />
                                  </Button>
                                )}
                              </div>
                            ))}
                          </div>
                        </div>
                      </div>
                    </div>
                  </CardContent>
                </Card>

                {/* JSON Preview */}
                <Card className="overflow-hidden">
                  <div className="bg-gradient-to-r from-muted to-secondary px-6 py-3">
                    <div className="flex items-center gap-2">
                      <Code className="w-4 h-4" />
                      <span className="text-sm font-semibold text-white">Profile JSON Preview</span>
                    </div>
                  </div>
                  <div className="p-4 bg-card">
                    <pre className="text-xs text-muted-foreground font-mono overflow-x-auto whitespace-pre-wrap">{JSON.stringify(editing, null, 2)}</pre>
                  </div>
                </Card>
              </div>
            )}
          </div>

          <input aria-label="Import profile JSON file" name="input-17" ref={fileInputRef} type="file" accept=".json,application/json" className="hidden" onChange={handleImportProfile} />
        </div>
      </TabsContent>
      </Tabs>

      <Dialog open={showPushModal} onOpenChange={setShowPushModal}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Push Profile: {editing.name}</DialogTitle>
          </DialogHeader>
          <div className="mb-3">
            <Button onClick={async () => {
              setLoadingAgents(true);
              try {
                const d = await api.get("/agents?size=500");
                const list = (d.agents || d.data || []) as Record<string, unknown>[];
                setPushAgents(list.map((a) => ({ id: String(a.id || ""), hostname: String(a.hostname || a.ip || ""), ip: String(a.ip || "") })));
              } catch { toast.error("Failed to load agents"); }
              setLoadingAgents(false);
            }} disabled={loadingAgents} variant="link" size="sm" className="text-xs text-indigo-500 hover:underline disabled:opacity-50">{loadingAgents ? "Loading..." : "Load agents"}</Button>
          </div>
          <div className="space-y-2 max-h-60 overflow-y-auto mb-4">
            {pushAgents.length === 0 ? (
              <p className="text-sm text-muted-foreground">Click &quot;Load agents&quot; to fetch agent list.</p>
            ) : (
              pushAgents.map(a => (
                <Label key={a.id} className="flex items-center gap-3 p-2 rounded-xl hover:bg-secondary transition-colors cursor-pointer">
                  <Checkbox aria-label={`Select ${a.hostname}`} name="input-18" checked={pushSelected.includes(a.id)} onCheckedChange={() => {
                    setPushSelected(prev => prev.includes(a.id) ? prev.filter(id => id !== a.id) : [...prev, a.id])
                  }} />
                  <span className="text-sm font-mono">{a.hostname}</span>
                  <span className="text-xs text-muted-foreground">{a.ip}</span>
                </Label>
              ))
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowPushModal(false)} className="h-9 px-4">Cancel</Button>
            <Button disabled={pushSelected.length === 0 || pushing} onClick={async () => {
              setPushing(true);
              let success = 0, fail = 0;
              for (const agentId of pushSelected) {
                try {
                  const d = await api.postJson(`/agents/${agentId}/profile-rotate`, {
                    beacon_uri: editing.beacon_uri,
                    beacon_method: editing.method,
                    user_agent: editing.user_agent,
                  });
                  if (d.success) success++; else fail++;
                } catch { fail++; }
              }
              setPushing(false);
              setShowPushModal(false);
              toast.success(`Profile push complete: ${success} success, ${fail} failed`);
              setPushSelected([]);
            }} className="h-9 px-4 rounded-xl text-sm disabled:opacity-50">
              {pushing ? "Pushing..." : `Push to ${pushSelected.length} agent(s)`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

