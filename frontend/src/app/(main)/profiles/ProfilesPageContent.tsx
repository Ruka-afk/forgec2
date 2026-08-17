"use client";
import { PageContainer } from "@/components/ui/page-container";
import { CardHeaderRow } from "@/components/ui/card-header-row";

import { useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadJSON } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { DataError } from "@/components/ui/data-state";
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
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { commonUAs, emptyProfile, type AgentProfile } from "./_components/types";
import { useProfilesData } from "./_components/useProfilesData";

export default function ProfilesPage({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  const {
    malleableForm,
    setMalleableForm,
    profiles,
    setProfiles,
    selectedIdx,
    setSelectedIdx,
    editing,
    setEditing,
    loadingProfiles,
    activeConfig,
    loadingActiveConfig,
    profilesError,
    loadActiveConfig,
    loadMalleableSettings,
    loadProfiles,
  } = useProfilesData();
  const [savingMalleable, setSavingMalleable] = useState(false);
  const [search, setSearch] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [reloading, setReloading] = useState(false);
  const [lastReload, setLastReload] = useState<string | null>(null);
  const [showPushModal, setShowPushModal] = useState(false);
  const [pushAgents, setPushAgents] = useState<{ id: string; hostname: string; ip: string }[]>([]);
  const [pushSelected, setPushSelected] = useState<string[]>([]);
  const [pushing, setPushing] = useState(false);
  const [loadingAgents, setLoadingAgents] = useState(false);

  const handleReloadConfig = useCallback(async () => {
    setReloading(true);
    try {
      const data = await api.post(paths.config.reload);
      if (!data.success) {
        toast.error((data.error as string) || t("profiles.toast.reload_failed"));
        return;
      }
      setLastReload(new Date().toLocaleTimeString());
      toast.success(t("profiles.toast.hot_reload_ok"));
      await loadActiveConfig();
      await loadMalleableSettings();
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setReloading(false);
    }
  }, [loadActiveConfig, loadMalleableSettings, t]);

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
      await api.post(paths.settings.malleable, {
        enabled: String(malleableForm.enabled),
        status_code: String(malleableForm.status_code),
        content_type: malleableForm.content_type,
        headers_text: malleableForm.headers_text,
        prepend: malleableForm.prepend,
        append: malleableForm.append,
      });
      toast.success(t("profiles.toast.profile_saved"));
    } catch {
      toast.error(t("profiles.toast.save_profile_failed"));
    } finally {
      setSavingMalleable(false);
    }
  };

  const handleSaveProfile = () => {
    downloadJSON(editing, `${editing.name || "profile"}.json`);
    toast.success(t("profiles.toast.profile_exported"));
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
    toast.success(t("profiles.toast.profile_removed", { name: name || "" }));
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
    toast.success(t("profiles.toast.profile_duplicated"));
  };

  const handleImportProfile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fd = new FormData();
    fd.append("profile", file);
    try {
      const d = await api.postFormData(paths.generate.profileImport, fd) as { success?: boolean; error?: string; profile?: AgentProfile };
      if (!d.success) {
        toast.error(d.error || t("profiles.toast.import_failed"));
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
      toast.success(t("profiles.toast.profile_imported"));
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

  const headerEntries = Object.entries(activeConfig.headers);

  return (
    <PageContainer embedded={embedded} title={!embedded ? t("profiles.title") : undefined} subtitle={!embedded ? t("profiles.subtitle") : undefined}>

      {/* Active Config Hot-Reload Card */}
      <Card className="overflow-hidden mb-6">
        <CardHeaderRow icon={RotateCw} tone="success" title={t("profiles.active_config")} description={t("profiles.active_config_desc")} action={
          <Button
            onClick={handleReloadConfig}
            disabled={reloading}
            className="px-5 bg-secondary/60 hover:bg-secondary/80 text-foreground text-sm font-medium transition-colors flex items-center gap-2 disabled:opacity-50"
          >
            {reloading ? <Spinner size="xs" /> : <RotateCw className="w-4 h-4" />}
            {reloading ? t("profiles.reloading") : t("profiles.reload_config")}
          </Button>
        } />
        <CardContent className="p-5">
          {loadingActiveConfig ? (
            <div className="flex items-center justify-center py-6">
              <Spinner color="emerald" />
              <span className="ml-3 text-sm text-muted-foreground">{t("profiles.loading_active")}</span>
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
                <div className="bg-secondary rounded-lg p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">{t("profiles.malleable_enabled")}</div>
                  <div className="flex items-center gap-2">
                    <span className={"inline-block w-2 h-2 rounded-full " + (activeConfig.malleable_enabled ? "bg-success" : "bg-muted-foreground")}></span>
                    <span className="text-sm font-medium">{activeConfig.malleable_enabled ? "Enabled" : "Disabled"}</span>
                  </div>
                </div>
                <div className="bg-secondary rounded-lg p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">{t("profiles.profile_name_label")}</div>
                  <div className="text-sm font-medium font-mono">{activeConfig.malleable_profile || "N/A"}</div>
                </div>
                <div className="bg-secondary rounded-lg p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">{t("profiles.user_agent_label")}</div>
                  <Tooltip>
                    <TooltipTrigger>
                      <div className="text-sm font-mono truncate">{activeConfig.user_agent || "N/A"}</div>
                    </TooltipTrigger>
                    <TooltipContent>{activeConfig.user_agent}</TooltipContent>
                  </Tooltip>
                </div>
                <div className="bg-secondary rounded-lg p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">{t("profiles.beacon_interval")}</div>
                  <div className="text-sm font-medium">{activeConfig.interval}s / {activeConfig.jitter}%</div>
                </div>
                <div className="bg-secondary rounded-lg p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">{t("profiles.status_code")}</div>
                  <div className="text-sm font-medium font-mono">{activeConfig.status_code}</div>
                </div>
                <div className="bg-secondary rounded-lg p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">{t("profiles.content_type")}</div>
                  <div className="text-sm font-medium font-mono">{activeConfig.content_type}</div>
                </div>
              </div>

              <div className="bg-secondary rounded-lg p-3 border border-border mb-4">
                <div className="text-xs text-muted-foreground mb-2">Headers ({headerEntries.length})</div>
                {headerEntries.length === 0 ? (
                  <span className="text-xs text-muted-foreground italic">{t("profiles.no_headers")}</span>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                    {headerEntries.map(([key, value]) => (
                      <div key={key} className="flex items-center gap-2 text-xs font-mono">
                        <span className="text-primary">{key}:</span>
                        <span className="text-muted-foreground truncate">{value}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-3">
                <div className="bg-secondary rounded-lg p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">{t("profiles.prepend_content")}</div>
                  <div className="text-xs font-mono text-muted-foreground truncate">{activeConfig.prepend || t("profiles.empty_value")}</div>
                </div>
                <div className="bg-secondary rounded-lg p-3 border border-border">
                  <div className="text-xs text-muted-foreground mb-1">{t("profiles.append_content")}</div>
                  <div className="text-xs font-mono text-muted-foreground truncate">{activeConfig.append || t("profiles.empty_value")}</div>
                </div>
              </div>

              {lastReload && (
                <div className="text-xs text-success flex items-center gap-1">
                  <Clock className="w-4 h-4" />
                  {t("profiles.last_reloaded", { time: lastReload })}
                </div>
              )}

              <div className="p-3 bg-warning/15 rounded-lg border border-warning/30 text-xs text-warning mt-3">
                <AlertTriangle className="w-4 h-4" />
                {t("profiles.reload_warning")}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* Tab switcher */}
      <Tabs defaultValue="server">
        <TabsList className="mb-6">
          <TabsTrigger value="server" className="gap-2">
            <Server className="w-4 h-4" />{t("profiles.tab_server")}
          </TabsTrigger>
          <TabsTrigger value="agents" className="gap-2">
            <PenTool className="w-4 h-4" />{t("profiles.tab_agents")}
          </TabsTrigger>
        </TabsList>

      <TabsContent value="server">
        <Card className="overflow-hidden">
          <CardHeaderRow icon={Shield} tone="violet" title={t("profiles.card_title")} description={t("profiles.card_desc")} />
          <CardContent className="p-4 sm:p-5">
            <form onSubmit={handleSaveMalleable} className="space-y-4">
              <div className="flex items-center gap-3">
                <Switch checked={malleableForm.enabled} onCheckedChange={(v) => setMalleableForm({ ...malleableForm, enabled: v })} />
                <span className="text-sm text-muted-foreground">{malleableForm.enabled ? t("profiles.enabled") : t("profiles.disabled")}</span>
                <span className="text-xs text-muted-foreground">{t("profiles.override_json_desc")}</span>
              </div>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div>
                  <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.http_status")}</Label>
                  <Input aria-label={t("profiles.http_status")} name="input-1" type="number" min={100} max={599} value={malleableForm.status_code} onChange={(e) => setMalleableForm({ ...malleableForm, status_code: Number(e.target.value) })} />
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.content_type")}</Label>
                  <Input aria-label="application-json" name="application-json-2" type="text" placeholder="application/json" value={malleableForm.content_type} onChange={(e) => setMalleableForm({ ...malleableForm, content_type: e.target.value })} className="font-mono" />
                </div>
              </div>
              <div>
                <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.custom_headers")}</Label>
                <Textarea aria-label={t("profiles.custom_headers")} name="textarea-3" rows={3} value={malleableForm.headers_text} onChange={(e) => setMalleableForm({ ...malleableForm, headers_text: e.target.value })} placeholder={"Server: nginx/1.24.0\nX-Powered-By: ASP.NET"} className="font-mono" />
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.prepend_label")}</Label>
                  <Textarea aria-label={t("profiles.prepend_content")} name="textarea-4" rows={2} value={malleableForm.prepend} onChange={(e) => setMalleableForm({ ...malleableForm, prepend: e.target.value })} placeholder="<html><body><!--" className="font-mono" />
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.append_label")}</Label>
                  <Textarea aria-label={t("profiles.append_content")} name="textarea-5" rows={2} value={malleableForm.append} onChange={(e) => setMalleableForm({ ...malleableForm, append: e.target.value })} placeholder="--></body></html>" className="font-mono" />
                </div>
              </div>
              <div className="p-3 bg-warning/15 rounded-lg border border-warning/30 text-xs text-warning">
                <AlertTriangle className="w-4 h-4" />
                {t("profiles.camouflage_warning")}
              </div>
              <Button type="submit" size="lg" disabled={savingMalleable} className="px-6 bg-primary hover:bg-primary/80 text-primary-foreground text-sm font-medium transition-colors disabled:opacity-50">
                <Save className="w-4 h-4" />{t("profiles.save_profile")}
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
              <div className="bg-primary/10 border-b border-primary/20 px-5 py-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <List className="w-4 h-4" />
                    <span className="text-sm font-semibold text-foreground">{t("profiles.list_title")}</span>
                    <span className="text-xs text-primary ml-1">({profiles.length})</span>
                  </div>
                  <div className="flex gap-1">
                    <Tooltip>
                      <TooltipTrigger render={<Button onClick={() => fileInputRef.current?.click()} className="w-7 h-7 bg-secondary/50 hover:bg-secondary/70 flex items-center justify-center transition-colors" aria-label={t("profiles.import_btn")} size="icon" />}>
                        <FileDown className="w-4 h-4" />
                      </TooltipTrigger>
                      <TooltipContent>{t("profiles.import_btn")}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger render={<Button onClick={() => {
                          const p = emptyProfile();
                          const idx = profiles.length;
                          setProfiles((prev) => [...prev, p]);
                          setSelectedIdx(idx);
                          setEditing(p);
                        }} className="w-7 h-7 bg-secondary/50 hover:bg-secondary/70 flex items-center justify-center transition-colors" aria-label={t("profiles.new_profile")} size="icon" />}>
                        <Plus className="w-4 h-4" />
                      </TooltipTrigger>
                      <TooltipContent>{t("profiles.new_profile")}</TooltipContent>
                    </Tooltip>
                  </div>
                </div>
              </div>
              <CardContent className="p-4 sm:p-5">
                <div className="relative mb-3">
                  <Search className="w-4 h-4" />
                  <Input aria-label={t("profiles.filter_ph")} name="filter-by-name-6"
                    type="text" placeholder={t("profiles.filter_ph")}
                    value={search} onChange={(e) => setSearch(e.target.value)}
                    className="w-full pl-9 pr-4 text-xs"
                  />
                </div>
                <div className="space-y-1 max-h-[500px] overflow-y-auto">
                  {loadingProfiles ? (
                    <div className="flex items-center justify-center py-16 sm:py-20"><Spinner size="sm" /></div>
                  ) : profilesError ? (
                    <DataError message={profilesError} onRetry={loadProfiles} className="py-10" />
                  ) : filteredProfiles.length === 0 ? (
                    <div className="text-center py-16 sm:py-20 text-xs text-muted-foreground">
                      <FileWarning className="w-4 h-4 mx-auto mb-2" />
                      {search ? t("profiles.no_match") : t("profiles.none_loaded")}
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
                          className={"w-full text-left px-3 py-2.5 rounded-lg text-xs transition-colors " + (selectedIdx === realIdx ? "bg-primary/10 dark:bg-primary/20 text-primary dark:text-primary border border-primary/30 dark:border-primary/40" : "hover:bg-secondary text-muted-foreground border border-transparent")}
                        >
                          <div className="flex items-center gap-2">
                            <FileCode className={`w-4 h-4 ${selectedIdx === realIdx ? "text-primary" : "text-muted-foreground"}`} />
                            <span className="font-medium truncate">{p.name}</span>
                          </div>
                          {p.description && (
                            <p className="text-(--fs-micro-sm) text-muted-foreground mt-0.5 truncate pl-6">{p.description}</p>
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
                <EmptyState icon={FileEdit} title={t("profiles.empty_title")} />
            ) : (
              <div className="space-y-5">
                <Card className="overflow-hidden">
                  <CardHeaderRow icon={FileCode} tone="primary" title={`Editing: ${editing.name || "untitled"}`} description={editing.description || "No description"} action={
                    <div className="flex gap-2">
                      <Button onClick={handleSaveProfile}                         className="h-9 px-4 text-xs font-medium transition-colors flex items-center gap-1.5">
                        <Download className="w-4 h-4" />Save (Export JSON)
                      </Button>
                      <Button onClick={handleDuplicateProfile} variant="secondary"                         className="h-9 px-4 text-xs font-medium transition-colors flex items-center gap-1.5">
                        <Copy className="w-4 h-4" />Duplicate
                      </Button>
                      <Button onClick={handleDeleteProfile} className="h-9 px-4 bg-destructive hover:bg-destructive/90 text-destructive-foreground text-xs font-medium transition-colors flex items-center gap-1.5">
                        <Trash2 className="w-4 h-4" />Delete
                      </Button>
                      <Button onClick={() => setShowPushModal(true)}                         className="h-9 px-4 text-xs font-medium transition-colors flex items-center gap-1.5">
                        <Send className="w-4 h-4" />Push to Agent
                        </Button>
                      </div>
                      } />
<CardContent className="p-4 sm:p-5">
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                      <div className="space-y-4">
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.profile_name_label")}</Label>
                          <Input aria-label={t("profiles.name_ph")} name="input-7" type="text" value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} placeholder={t("profiles.name_ph")} />
                        </div>
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.description")}</Label>
                          <Textarea aria-label={t("profiles.brief_desc")} name="textarea-8" rows={2} value={editing.description} onChange={(e) => setEditing({ ...editing, description: e.target.value })} placeholder={t("profiles.brief_desc")} />
                        </div>
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.user_agent_label")}</Label>
                          <div className="flex gap-2">
                            <Input aria-label={t("agents.config.ua")} name="input-9" type="text" value={editing.user_agent} onChange={(e) => setEditing({ ...editing, user_agent: e.target.value })} className="font-mono text-xs" />
                            <Select value="" onValueChange={(v) => { if (v) setEditing({ ...editing, user_agent: v }); }}>
                              <SelectTrigger className="shrink-0 w-[180px] h-9 text-xs">
                                <SelectValue placeholder={t("profiles.ua_ph")} />
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
                          <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.beacon_uri")}</Label>
                          <Input aria-label="/api/v1/beacon" name="input-11" type="text" value={editing.beacon_uri} onChange={(e) => setEditing({ ...editing, beacon_uri: e.target.value })} className="font-mono" placeholder="/api/v1/beacon" />
                        </div>
                      </div>
                      <div className="space-y-4">
                        <div>
                          <Label className="text-xs text-muted-foreground mb-1.5">{t("profiles.http_method")}</Label>
                          <Select value={editing.method} onValueChange={(v) => { if (v) setEditing({ ...editing, method: v }); }}>
                             <SelectTrigger>
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
                            <Label className="text-xs text-muted-foreground mb-1.5">{t("agents.config.sleep")}</Label>
                            <Input aria-label={t("agents.config.sleep")} name="input-13" type="number" min={0} max={86400} value={editing.sleep} onChange={(e) => setEditing({ ...editing, sleep: Number(e.target.value) })} />
                          </div>
                          <div>
                            <Label className="text-xs text-muted-foreground mb-1.5">{t("agents.config.jitter")}</Label>
                            <Input aria-label={t("agents.config.jitter")} name="input-14" type="number" min={0} max={100} value={editing.jitter} onChange={(e) => setEditing({ ...editing, jitter: Number(e.target.value) })} />
                          </div>
                        </div>
                        <div>
                          <div className="flex items-center justify-between mb-1.5">
                            <Label className="text-xs text-muted-foreground">{t("profiles.custom_headers")}</Label>
                            <Button type="button" onClick={addHeader} className="text-xs text-primary hover:underline flex items-center gap-1" variant="link" size="sm">
                              <Plus className="w-4 h-4" />{t("profiles.add_header")}
                            </Button>
                          </div>
                          <div className="space-y-2">
                            {Object.entries(editing.headers).map(([key, value], i) => (
                              <div key={key} className="flex gap-2 items-start">
                                <Input aria-label={t("agents.config.header_name")} name="header-name-15"
                                  type="text" placeholder={t("agents.config.header_name")}
                                  value={key} onChange={(e) => updateHeader(i, e.target.value, value)}
                                  className="w-2/5 text-xs font-mono"
                                />
                                <Input aria-label={t("agents.config.header_value")} name="value-16"
                                  type="text" placeholder={t("agents.config.header_value")}
                                  value={value} onChange={(e) => updateHeader(i, key, e.target.value)}
                                  className="flex-1 text-xs font-mono"
                                />
                                {Object.entries(editing.headers).length > 1 && (
                                   <Button onClick={() => removeHeader(i)} className="w-9 h-9 flex items-center justify-center text-destructive/50 hover:text-destructive hover:bg-destructive/10 transition-colors" variant="ghost" size="icon" aria-label={t("agents.config.remove_header")}>
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
                  <div className="bg-secondary/60 border-b border-border px-6 py-3">
                    <div className="flex items-center gap-2">
                      <Code className="w-4 h-4" />
                      <span className="text-sm font-semibold text-foreground">{t("profiles.json_preview")}</span>
                    </div>
                  </div>
                  <div className="p-4 bg-card">
                    <pre className="text-xs text-muted-foreground font-mono overflow-x-auto whitespace-pre-wrap">{JSON.stringify(editing, null, 2)}</pre>
                  </div>
                </Card>
              </div>
            )}
          </div>

          <Input aria-label={t("profiles.import_json")} name="input-17" ref={fileInputRef} type="file" accept=".json,application/json" className="hidden" onChange={handleImportProfile} />
        </div>
      </TabsContent>
      </Tabs>

      <Dialog open={showPushModal} onOpenChange={setShowPushModal}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("profiles.push_title")}: {editing.name}</DialogTitle>
          </DialogHeader>
          <div className="mb-3">
            <Button onClick={async () => {
              setLoadingAgents(true);
              try {
                const d = await api.get(paths.agents.list("page=1&pageSize=500"));
                const list = (d.agents || d.data || (Array.isArray(d) ? d : [])) as Record<string, unknown>[];
                setPushAgents(list.map((a) => ({ id: String(a.id || ""), hostname: String(a.hostname || a.ip || ""), ip: String(a.ip || "") })));
              } catch { toast.error(t("profiles.toast.load_agents_failed")); }
              setLoadingAgents(false);
            }} disabled={loadingAgents} variant="link" size="sm" className="text-xs text-primary hover:underline disabled:opacity-50">{loadingAgents ? t("common.loading") : t("profiles.load_agents")}</Button>
          </div>
          <div className="space-y-2 max-h-60 overflow-y-auto mb-4">
            {pushAgents.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("profiles.load_agents_hint")}</p>
            ) : (
              pushAgents.map(a => (
                <Label key={a.id} className="flex items-center gap-3 p-2 rounded-lg hover:bg-secondary transition-colors cursor-pointer">
                  <Checkbox aria-label={t("profiles.select_agent", { hostname: a.hostname })} name="input-18" checked={pushSelected.includes(a.id)} onCheckedChange={() => {
                    setPushSelected(prev => prev.includes(a.id) ? prev.filter(id => id !== a.id) : [...prev, a.id])
                  }} />
                  <span className="text-sm font-mono">{a.hostname}</span>
                  <span className="text-xs text-muted-foreground">{a.ip}</span>
                </Label>
              ))
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowPushModal(false)} className="h-9 px-4">{t("common.cancel")}</Button>
            <Button disabled={pushSelected.length === 0 || pushing} onClick={async () => {
              setPushing(true);
              let success = 0, fail = 0;
              for (const agentId of pushSelected) {
                try {
                  const d = await api.postJson(paths.agents.profileRotate(agentId), {
                    beacon_uri: editing.beacon_uri,
                    beacon_method: editing.method,
                    user_agent: editing.user_agent,
                  });
                  if (d.success) success++; else fail++;
                } catch { fail++; }
              }
              setPushing(false);
              setShowPushModal(false);
              toast.success(t("profiles.toast.push_complete", { success: String(success), fail: String(fail) }));
              setPushSelected([]);
            }} className="h-9 px-4 text-sm disabled:opacity-50">
              {pushing ? t("profiles.pushing") : t("profiles.push_to", { count: String(pushSelected.length) })}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageContainer>
  );
}

