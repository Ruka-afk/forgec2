"use client";

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useWS } from "@/lib/wsContext";
import { downloadText } from "@/lib/download";
import Link from "next/link";
import { api } from "@/lib/api";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { DataError } from "@/components/ui/data-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ConfirmModal, PageHeader, Pagination } from "@/components/UI";
import { AgentFilters } from "./_components/AgentFilters";
import { AgentBulkBar } from "./_components/AgentBulkBar";
import { AgentRow } from "./_components/AgentRow";
import { AgentGrid } from "./_components/AgentGrid";
import { BulkCommandModal } from "./_components/BulkCommandModal";
import { QuickSleepModal } from "./_components/QuickSleepModal";
import { NotesEditModal } from "./_components/NotesEditModal";
import { BatchSleepModal } from "./_components/BatchSleepModal";
import { useAgentFilters } from "./_components/useAgentFilters";
import { useAgentSelection } from "./_components/useAgentSelection";
import { useAgentData } from "./_components/useAgentData";
import { useAgentModals } from "./_components/useAgentModals";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import AgentDetailPage from "./[id]/AgentDetailPage";
import type { Beacon, BulkResult } from "./_components/types";
import { copyToClipboard } from "./_components/types";
import { useVirtualWindow } from "@/lib/hooks/useVirtualWindow";

export type { Beacon };
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { useI18n } from "@/lib/i18n";
import { ArrowDown, ArrowLeftRight, ArrowUp, ArrowUpDown, Download, Grip, History, ListChecks, ListOrdered, Pause, Play, Plus, Radio, RefreshCw } from "lucide-react";



export default function AgentsPageContent() {
  const { t } = useI18n();
  const { subscribe } = useWS();

  const {
    beacons, setBeacons, loading, total, error, setError,
    allTags, tagsByAgent, taskCountMap, agentLocks, setAgentLocks,
    loadBeacons: loadBeaconsRaw, loadLocks,
  } = useAgentData(t);

  const {
    confirm, setConfirm, cmdModalOpen, cmdType, cmdText, setCmdType, setCmdText,
    closeCmdModal, openCmdModal,
    quickSleepAgent, closeQuickSleep, openQuickSleep, sleepInterval, sleepJitter, setSleepInterval, setSleepJitter,
    editingNotesId, closeNotesEdit, openNotesEdit, editNotesText, setNotesText,
    screenshotConfirmOpen, setScreenshotConfirm,
    batchSleepOpen, closeBatchSleep, openBatchSleep, batchSleepInterval, batchSleepJitter, setBatchSleepInterval, setBatchSleepJitter,
  } = useAgentModals();

  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const handleCloseDetail = useCallback(() => setSelectedAgentId(null), []);
  const handleSelectAgent = useCallback((id: string) => setSelectedAgentId(id), []);

  const [bulkMode, setBulkMode] = useState(false);
  const [actionMsg, setActionMsg] = useState<string | null>(null);
  const [showResults, setShowResults] = useState(false);
  const [bulkResults, setBulkResults] = useState<BulkResult[]>([]);
  const [copiedField, setCopiedField] = useState("");

  const {
    searchInput, setSearchInput, searchQuery,
    statusFilter, setStatusFilter, osFilter, setOsFilter,
    page, setPage, sortKey, sortDir, toggleSort,
    linkedFilter, setLinkedFilter, tagFilter, setTagFilter,
    autoRefresh, setAutoRefresh, viewMode, setViewMode,
    visibleCols, setVisibleCols,
    sortedBeacons,
  } = useAgentFilters(beacons);
  const { selected, setSelected, toggleSelect, toggleSelectAll } = useAgentSelection(beacons);

  const AGENT_ROW_H = 56;
  const {
    scrollRef: agentScrollRef,
    onScroll: onAgentScroll,
    virtualized: agentVirtualized,
    start: agentVirtStart,
    end: agentVirtEnd,
    offsetTop: agentOffsetTop,
    totalHeight: agentTotalHeight,
  } = useVirtualWindow({ count: sortedBeacons.length, rowHeight: AGENT_ROW_H, threshold: 25 });

  const visibleBeacons = useMemo(
    () => (agentVirtualized ? sortedBeacons.slice(agentVirtStart, agentVirtEnd) : sortedBeacons),
    [sortedBeacons, agentVirtualized, agentVirtStart, agentVirtEnd],
  );

  const loadBeacons = useCallback(() => {
    loadBeaconsRaw(searchQuery, statusFilter, osFilter, page, 20, tagFilter);
  }, [loadBeaconsRaw, searchQuery, statusFilter, osFilter, page, tagFilter]);

  const loadBeaconsRef = useRef(loadBeacons);
  loadBeaconsRef.current = loadBeacons;

  useEffect(() => { loadBeacons(); }, [loadBeacons]);
  useEffect(() => { loadLocks(); }, [loadLocks]);

  useVisibleInterval(() => loadBeacons(), autoRefresh ? 30000 : 0);

  useEffect(() => {
    if (!actionMsg) return;
    const timer = setTimeout(() => setActionMsg(null), 5000);
    return () => clearTimeout(timer);
  }, [actionMsg]);

  useEffect(() => {
    if (!showResults) return;
      api.get("/api/v1/bulk/results?page=1&pageSize=10")
      .then((d) => {
        const results = (d.results || d.data || []) as BulkResult[];
        setBulkResults(results);
      })
      .catch(() => { setActionMsg(t("agents.bulk_results_failed")); });
  }, [showResults, t]);

  useEffect(() => {
    const unsub = subscribe((msg) => {
      if (msg.type === "agent_online" || msg.type === "agent_offline") {
        loadBeaconsRef.current();
      } else if (msg.type === "agent_data_update" && msg.agent_id) {
        const aid = String(msg.agent_id);
        setBeacons((prev) => prev.map((b) =>
          (b.id || "") === aid ? { ...b, ...(msg.data as Partial<Beacon>) } : b
        ));
      } else if (msg.type === "agent_locked" && msg.agent_id) {
        setAgentLocks((prev) => ({ ...prev, [String(msg.agent_id)]: String(msg.username) }));
      } else if (msg.type === "agent_unlocked" && msg.agent_id) {
        setAgentLocks((prev) => {
          const next = { ...prev };
          delete next[String(msg.agent_id)];
          return next;
        });
      }
    });
    return () => unsub();
  }, [subscribe]); // eslint-disable-line react-hooks/exhaustive-deps

  const sortIcon = (field: typeof sortKey) => {
    if (sortKey !== field) return <ArrowUpDown className="w-3 h-3 text-muted-foreground" />;
    return sortDir === "asc" ? <ArrowUp className="w-3 h-3 text-indigo-500" /> : <ArrowDown className="w-3 h-3 text-indigo-500" />;
  };

  const handleRowConfirm = useCallback(
    (type: "kill" | "delete" | "batch-delete" | "bulk-kill" | "bulk-uninstall", id: string, hostname: string) => {
      setConfirm({ type, id, hostname });
    },
    [setConfirm],
  );

  const handleRowCopy = useCallback(
    (field: string, value: string) => copyToClipboard(value, field, setCopiedField),
    [],
  );

  const runBatch = async (payload: Record<string, unknown>) => {
    try {
      const data = await api.postJson<{ success?: boolean; tasks_created?: number; error?: string }>("/agents/batch", payload);
      if (data.success) {
        setActionMsg(t("agents.batch_result").replace("{count}", String(data.tasks_created ?? selected.size)));
        setSelected(new Set());
        loadBeacons();
      } else {
        setActionMsg(data.error || t("agents.batch_failed"));
      }
    } catch {
      setActionMsg(t("agents.batch_failed"));
    }
  };

  const bulkTask = async (agentIds: string[], type: string, command: string) => {
    try {
      const data = await api.postJson<{ success?: boolean; task_count?: number; failed?: string[]; error?: string }>("/agents/bulk/task", { agent_ids: agentIds, type, command });
      if (data.success) {
        let bulkMsg = t("agents.bulk_type_result").replace("{type}", type).replace("{count}", String(data.task_count ?? agentIds.length));
        if (data.failed?.length) bulkMsg += ` (${data.failed.length} ${t("agents.failed_suffix")})`;
        setActionMsg(bulkMsg);
        setSelected(new Set());
        loadBeacons();
      } else {
        setActionMsg(data.error || t("agents.bulk_task_failed"));
      }
    } catch {
      setActionMsg(t("agents.bulk_task_failed"));
    }
  };

  const batchScreenshot = () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    setScreenshotConfirm(true);
  };

  const doBatchScreenshot = () => {
    setScreenshotConfirm(false);
    runBatch({ agent_ids: Array.from(selected), task_type: "screenshot" });
  };

  const batchSleep = () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    openBatchSleep("30", "20");
  };

  const doBatchSleep = () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    runBatch({ agent_ids: ids, task_type: "sleep", args: `${batchSleepInterval},${batchSleepJitter}` });
    closeBatchSleep();
  };

  const batchDelete = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    try {
      const data = await api.postJson<{ success?: boolean; deleted?: number; error?: string }>("/agents/batch/delete", { agent_ids: ids });
      if (data.success) {
        setActionMsg(t("agents.delete_result").replace("{count}", String(data.deleted ?? 0)));
        setSelected(new Set());
        loadBeacons();
      } else {
        setActionMsg(data.error || t("agents.delete_failed"));
      }
    } catch {
      setActionMsg(t("agents.delete_failed"));
    }
    setConfirm(null);
  };

  const killAgent = async (id: string) => {
    try {
      await api.postJson(`/agents/${id}/kill`, {});
      setActionMsg(t("agents.kill_sent"));
      loadBeacons();
    } catch {
      setActionMsg(t("agents.kill_failed"));
    }
    setConfirm(null);
  };

  const deleteAgent = async (id: string) => {
    try {
      await api.del(`/agents/${id}`);
      setActionMsg(t("agents.deleted"));
      loadBeacons();
    } catch {
      setActionMsg(t("agents.delete_failed"));
    }
    setConfirm(null);
  };

  const bulkKill = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    await bulkTask(ids, "kill", "");
    setConfirm(null);
  };

  const bulkUninstall = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    await bulkTask(ids, "uninstall", "");
    setConfirm(null);
  };

  const handleCmdSubmit = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    await bulkTask(ids, cmdType, cmdText);
    closeCmdModal();
  };

  const sendScreenshot = async (id: string) => {
    try {
      await api.post(`/agents/${id}/screenshot`);
      setActionMsg(t("agents.screenshot_sent"));
    } catch { setActionMsg(t("agents.screenshot_failed")); }
  };

  const submitQuickSleep = async () => {
    if (!quickSleepAgent) return;
    try {
      await api.postJson(`/agents/${quickSleepAgent.id}/set_sleep`, { interval: Number(sleepInterval), jitter: Number(sleepJitter) });
      setActionMsg(t("agents.sleep_updated").replace("{name}", quickSleepAgent.hostname));
      closeQuickSleep();
      loadBeacons();
    } catch { setActionMsg(t("agents.sleep_failed")); }
  };

  const submitNotes = async () => {
    if (!editingNotesId) return;
    try {
      await api.postJson(`/agents/${editingNotesId}/note`, { notes: editNotesText });
      setActionMsg(t("agents.notes_updated"));
      closeNotesEdit();
      loadBeacons();
    } catch { setActionMsg(t("agents.notes_failed")); }
  };

  const exportCSV = () => {
    const headers = ["Hostname", "User", "OS", "IP", "Status", "Last Seen", "Version", "Active Window", "Notes"];
    const rows = sortedBeacons.map((b) => [
      b.hostname || "", b.username || "", b.os || "", b.ip || "",
      b.status || "", b.last_seen || "", b.version || "", b.active_window || "", b.notes || "",
    ]);
    const csv = [headers.join(","), ...rows.map((r) => r.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(","))].join("\n");
    downloadText(csv, `agents-${new Date().toISOString().slice(0, 10)}.csv`, "text/csv");
  };

  const counts = useMemo(() => {
    let online = 0, stale = 0, offline = 0, windows = 0, linux = 0, darwin = 0;
    for (const b of beacons) {
      if (b.status === "online") online++;
      else if (b.status === "stale") stale++;
      else if (b.status === "offline") offline++;
      const os = (b.os || "").toLowerCase();
      if (os.includes("windows")) windows++;
      else if (os.includes("linux")) linux++;
      else if (os.includes("darwin")) darwin++;
    }
    return { onlineCount: online, staleCount: stale, offlineCount: offline, windowsCount: windows, linuxCount: linux, darwinCount: darwin };
  }, [beacons]);

  const { onlineCount, staleCount, offlineCount, windowsCount, linuxCount, darwinCount } = counts;

  const emptyColSpan = Object.values(visibleCols).filter(Boolean).length + 2;

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      {error && (
        <DataError
          message={error}
          onRetry={() => { setError(null); loadBeacons(); }}
          onDismiss={() => setError(null)}
          className="mb-4"
        />
      )}
      <PageHeader
        title={t("agents.title")}
        subtitle={`${total} ${t("agents.total_label")} · ${onlineCount} ${t("agents.online_label")}${staleCount > 0 ? `, ${staleCount} ${t("agents.stale_label")}` : ""}${offlineCount > 0 ? `, ${offlineCount} ${t("agents.offline_label")}` : ""}`}
      >
        <Button
          variant="outline"
          onClick={() => { setBulkMode((p) => !p); if (!bulkMode) setSelected(new Set()); }}
          className={`h-9 sm:h-10 px-3 rounded-xl gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
            bulkMode
              ? "bg-primary text-primary-foreground border-primary hover:bg-primary/80"
              : "text-muted-foreground"
          }`}
        >
          <ListChecks className="w-4 h-4" />
          <span className="hidden sm:inline text-sm">{t("agents.bulk_ops")}</span>
        </Button>
        <Button
          variant="outline"
          onClick={() => { setShowResults((p) => !p); if (!showResults) setBulkMode(true); }}
          className={`h-9 sm:h-10 px-3 rounded-xl gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
            showResults
              ? "bg-primary text-primary-foreground border-primary hover:bg-primary/80"
              : "text-muted-foreground"
          }`}
          title={t("agents.bulk_results_title")}
        >
          <History className="w-4 h-4" />
          <span className="hidden sm:inline text-sm">{t("agents.results")}</span>
        </Button>
        <Button
          variant="outline"
          onClick={exportCSV}
          className="h-9 sm:h-10 px-3 rounded-xl gap-2 min-w-[2.75rem] min-h-[2.75rem]"
          title={t("agents.export_csv_title")}
        >
          <Download className="w-4 h-4" />
          <span className="hidden sm:inline text-foreground text-sm">{t("agents.export")}</span>
        </Button>
        <Button
          variant="outline"
          onClick={() => setAutoRefresh((p) => !p)}
          className={`h-9 sm:h-10 px-3 rounded-xl gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
            autoRefresh
              ? "bg-emerald-600 border-emerald-600 text-white hover:bg-emerald-700"
              : "text-muted-foreground"
          }`}
          title={autoRefresh ? t("agents.auto_refresh_on") : t("agents.auto_refresh_off")}
        >
          {autoRefresh ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
          <span className="hidden sm:inline text-sm">{autoRefresh ? t("agents.live") : t("agents.auto")}</span>
        </Button>
        <Button
          variant="outline"
          onClick={() => setViewMode((p) => p === "table" ? "grid" : "table")}
          className="h-9 sm:h-10 px-3 rounded-xl gap-2 min-w-[2.75rem] min-h-[2.75rem]"
          title={viewMode === "table" ? t("agents.switch_grid") : t("agents.switch_table")}
        >
          {viewMode === "table" ? <Grip className="w-4 h-4" /> : <ListOrdered className="w-4 h-4" />}
        </Button>
        <Button
          variant="outline"
          onClick={() => { setPage(1); loadBeacons(); }}
          className="h-9 sm:h-10 px-3 rounded-xl gap-2 min-w-[2.75rem] min-h-[2.75rem]"
        >
          <RefreshCw className="w-4 h-4" />
          <span className="hidden sm:inline text-foreground text-sm">{t("agents.refresh")}</span>
        </Button>
        <Button render={<Link href="/generate" />}>
          <Plus className="w-4 h-4" />
          <span className="hidden sm:inline">{t("agents.generate_implant")}</span>
          <span className="sm:hidden">{t("agents.new")}</span>
        </Button>
      </PageHeader>

      <AgentBulkBar
        selected={selected}
        bulkMode={bulkMode}
        bulkResults={bulkResults}
        showResults={showResults}
        setShowResults={setShowResults}
        onBulkShell={openCmdModal}
        onBulkScreenshot={batchScreenshot}
        onBulkSleep={batchSleep}
        onBulkKill={() => setConfirm({ type: "bulk-kill" })}
        onBulkUninstall={() => setConfirm({ type: "bulk-uninstall" })}
        onBulkDelete={() => setConfirm({ type: "batch-delete" })}
        onClearSelection={() => setSelected(new Set())}
        actionMsg={actionMsg}
        dismissActionMsg={() => setActionMsg(null)}
      />

      <AgentFilters
        searchInput={searchInput}
        setSearchInput={setSearchInput}
        statusFilter={statusFilter}
        setStatusFilter={(v) => { setStatusFilter(v); setPage(1); }}
        osFilter={osFilter}
        setOsFilter={(v) => { setOsFilter(v); setPage(1); }}
        tagFilter={tagFilter}
        setTagFilter={(v) => { setTagFilter(v); setPage(1); }}
        linkedFilter={linkedFilter}
        setLinkedFilter={(v) => { setLinkedFilter(v as "" | "direct" | "chained"); setPage(1); }}
        allTags={allTags}
        visibleCols={visibleCols}
        setVisibleCols={setVisibleCols}
        onlineCount={onlineCount}
        windowsCount={windowsCount}
        linuxCount={linuxCount}
        darwinCount={darwinCount}
      />

      {viewMode === "table" && (
      <Card className="sm:rounded-2xl overflow-hidden">
        <div
          ref={agentScrollRef}
          onScroll={onAgentScroll}
          className={agentVirtualized ? "overflow-auto max-h-[min(70vh,720px)]" : "overflow-auto"}
        >
        <Table className="text-sm min-w-[850px]">
          <TableHeader className="border-b border-border bg-muted sticky top-0 z-10">
            <TableRow className="text-xs text-muted-foreground font-semibold uppercase tracking-wider hover:bg-transparent">
              <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5 w-10">
                <Checkbox aria-label={t("agents.select_all")} name="input-4"
                  checked={beacons.length > 0 && beacons.every((b) => b.id && selected.has(b.id))}
                  onCheckedChange={(v) => toggleSelectAll(v === true)}
                />
              </TableHead>
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("hostname")}>
                {t("agents.col_hostname")} {sortIcon("hostname")}
              </TableHead>
              {visibleCols.username && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("username")}>
                {t("agents.col_user")} {sortIcon("username")}
              </TableHead>
              )}
              {visibleCols.os && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("os")}>
                {t("agents.col_os")} {sortIcon("os")}
              </TableHead>
              )}
              {visibleCols.ip && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("ip")}>
                {t("agents.col_ip")} {sortIcon("ip")}
              </TableHead>
              )}
              {visibleCols.last_seen && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("last_seen")}>
                {t("agents.col_last_seen")} {sortIcon("last_seen")}
              </TableHead>
              )}
              {visibleCols.window && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_window")}</TableHead>
              )}
              {visibleCols.lock && (
              <TableHead className="text-center py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_lock")}</TableHead>
              )}
              {visibleCols.tasks && (
              <TableHead className="text-center py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_tasks")}</TableHead>
              )}
              {visibleCols.version && <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_version")}</TableHead>}
              {visibleCols.status && (
              <TableHead className="text-center py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" onClick={() => toggleSort("status")}>
                {t("agents.col_status")} {sortIcon("status")}
              </TableHead>
              )}
              <TableHead className="text-right py-3 px-4 sm:py-3.5 sm:px-5">{t("agents.col_actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="divide-y divide-border">
            {loading && Array.from({ length: 6 }).map((_, i) => (
              <TableRow key={`skel-${i}`}>
                {Array.from({ length: 12 }).map((_, j) => (
                  <TableCell key={j} className="py-3 px-3 sm:py-3.5 sm:px-4">
                    <Skeleton className="h-4 w-3/4" />
                  </TableCell>
                ))}
              </TableRow>
            ))}
            {!loading && agentVirtualized && agentOffsetTop > 0 && (
              <TableRow aria-hidden className="hover:bg-transparent">
                <TableCell colSpan={emptyColSpan} style={{ height: agentOffsetTop, padding: 0, border: 0 }} />
              </TableRow>
            )}
            {!loading && visibleBeacons.map((beacon) => (
              <AgentRow
                key={beacon.id || ""}
                beacon={beacon}
                isSelected={selected.has(beacon.id || "")}
                onToggleSelect={toggleSelect}
                onScreenshot={sendScreenshot}
                onQuickSleep={openQuickSleep}
                onNotes={openNotesEdit}
                onConfirm={handleRowConfirm}
                onSelect={handleSelectAgent}
                taskCount={taskCountMap[beacon.id || ""] ?? 0}
                lockUser={agentLocks[beacon.id || ""] || null}
                visibleCols={visibleCols}
                copiedField={copiedField}
                onCopy={handleRowCopy}
              />
            ))}
            {!loading && agentVirtualized && agentTotalHeight - agentOffsetTop - visibleBeacons.length * AGENT_ROW_H > 0 && (
              <TableRow aria-hidden className="hover:bg-transparent">
                <TableCell colSpan={emptyColSpan} style={{ height: agentTotalHeight - agentOffsetTop - visibleBeacons.length * AGENT_ROW_H, padding: 0, border: 0 }} />
              </TableRow>
            )}
            {!loading && beacons.length === 0 && (
              <TableRow>
                <TableCell colSpan={emptyColSpan} className="py-16 sm:py-20">
                  <div className="text-center">
                    <Radio className="w-4 h-4" />
                    <h3 className="text-base sm:text-lg font-semibold text-muted-foreground mb-2">{t("agents.no_beacons")}</h3>
                    <p className="text-muted-foreground mb-4 text-sm">
                      {statusFilter || osFilter
                        ? t("agents.no_beacons_filtered")
                        : t("agents.no_beacons_hint")}
                    </p>
                    {!statusFilter && !osFilter && (
                      <Button render={<Link href="/generate" />}>
                        <Plus className="w-4 h-4" />
                        <span>{t("agents.generate_implant")}</span>
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        </div>
        <div className="sm:hidden px-4 py-2 text-center text-xs text-muted-foreground border-t border-border bg-muted">
          <ArrowLeftRight className="w-4 h-4" /> {t("agents.swipe_hint")}
        </div>

        <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} />
      </Card>
      )}

      {viewMode === "grid" && !loading && (
        <>
          <AgentGrid
            beacons={sortedBeacons}
            tagsByAgent={tagsByAgent}
            taskCountMap={taskCountMap}
            onSelect={handleSelectAgent}
          />
          <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} />
        </>
      )}

      <ConfirmModal
        open={confirm?.type === "kill"}
        title={t("agents.confirm_kill_title")}
        message={t("agents.confirm_kill_msg").replace("{name}", confirm?.hostname || confirm?.id || "")}
        confirmText={t("agents.confirm_kill_btn")}
        danger
        onConfirm={() => confirm?.id && killAgent(confirm.id)}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "delete"}
        title={t("agents.confirm_delete_title")}
        message={t("agents.confirm_delete_msg").replace("{name}", confirm?.hostname || confirm?.id || "")}
        confirmText={t("agents.confirm_delete_btn")}
        danger
        onConfirm={() => confirm?.id && deleteAgent(confirm.id)}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "batch-delete"}
        title={t("agents.confirm_batch_delete_title")}
        message={t("agents.confirm_batch_delete_msg").replace("{count}", String(selected.size))}
        confirmText={t("agents.confirm_batch_delete_btn")}
        danger
        onConfirm={batchDelete}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "bulk-kill"}
        title={t("agents.confirm_bulk_kill_title")}
        message={t("agents.confirm_bulk_kill_msg").replace("{count}", String(selected.size))}
        confirmText={t("agents.confirm_bulk_kill_btn")}
        danger
        onConfirm={bulkKill}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "bulk-uninstall"}
        title={t("agents.confirm_bulk_uninstall_title")}
        message={t("agents.confirm_bulk_uninstall_msg").replace("{count}", String(selected.size))}
        confirmText={t("agents.confirm_bulk_uninstall_btn")}
        danger
        onConfirm={bulkUninstall}
        onCancel={() => setConfirm(null)}
      />

      <BulkCommandModal
        open={cmdModalOpen}
        selectedCount={selected.size}
        cmdType={cmdType}
        setCmdType={setCmdType}
        cmdText={cmdText}
        setCmdText={setCmdText}
        onSubmit={handleCmdSubmit}
        onClose={closeCmdModal}
      />

      {quickSleepAgent && (
        <QuickSleepModal
          agent={quickSleepAgent}
          sleepInterval={sleepInterval}
          setSleepInterval={setSleepInterval}
          sleepJitter={sleepJitter}
          setSleepJitter={setSleepJitter}
          onSubmit={submitQuickSleep}
          onClose={closeQuickSleep}
        />
      )}

      {editingNotesId && (
        <NotesEditModal
          notesText={editNotesText}
          setNotesText={setNotesText}
          onSubmit={submitNotes}
          onClose={closeNotesEdit}
        />
      )}

      <ConfirmModal
        open={screenshotConfirmOpen}
        title={t("agents.confirm_screenshot_title")}
        message={t("agents.confirm_screenshot_msg").replace("{count}", String(selected.size))}
        confirmText={t("agents.confirm_screenshot_btn")}
        danger={false}
        onConfirm={doBatchScreenshot}
        onCancel={() => setScreenshotConfirm(false)}
      />

      {batchSleepOpen && (
        <BatchSleepModal
          agentCount={selected.size}
          interval={batchSleepInterval}
          setInterval={setBatchSleepInterval}
          jitter={batchSleepJitter}
          setJitter={setBatchSleepJitter}
          onSubmit={doBatchSleep}
          onClose={closeBatchSleep}
        />
      )}

      <Sheet open={!!selectedAgentId} onOpenChange={(open) => { if (!open) setSelectedAgentId(null); }}>
        <SheetContent side="right" className="w-full sm:w-[800px] lg:w-[1000px] sm:max-w-none p-0 overflow-auto" showCloseButton={false}>
          {selectedAgentId && <AgentDetailPage agentId={selectedAgentId} onClose={handleCloseDetail} />}
        </SheetContent>
      </Sheet>
    </div>
  );
}

