"use client";

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useRouter } from "next/navigation";
import { useWS } from "@/lib/wsContext";
import { downloadText } from "@/lib/download";
import Link from "next/link";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { DataError } from "@/components/ui/data-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PageContainer } from "@/components/ui/page-container";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { Pagination } from "@/components/ui/pagination";
import { AgentFilters } from "./_components/AgentFilters";
import { AgentBulkBar } from "./_components/AgentBulkBar";
import { AgentRow } from "./_components/AgentRow";
import { AgentGrid } from "./_components/AgentGrid";
import { BulkCommandModal } from "./_components/BulkCommandModal";
import { QuickSleepModal } from "./_components/QuickSleepModal";
import { NotesEditModal } from "./_components/NotesEditModal";
import { BatchSleepModal } from "./_components/BatchSleepModal";
import { AgentContextMenu } from "./_components/AgentContextMenu";
import { useAgentFilters } from "./_components/useAgentFilters";
import { useAgentSelection } from "./_components/useAgentSelection";
import { useAgentData } from "./_components/useAgentData";
import { useAgentModals } from "./_components/useAgentModals";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import AgentDetailPage from "./[id]/AgentDetailPage";
import type { Beacon, BulkResult } from "./_components/types";
import type { AgentMenuAction, AgentMenuPoint } from "./_components/agent-menu-actions";
import { useVirtualWindow } from "@/lib/hooks/useVirtualWindow";
import { useInteractStore } from "@/lib/interact-store";
import { toast } from "sonner";

export type { Beacon };
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { useI18n } from "@/lib/i18n";
import { ArrowDown, ArrowLeftRight, ArrowUp, ArrowUpDown, Download, Grip, History, ListChecks, ListOrdered, Pause, Play, Plus, Radio, RefreshCw } from "lucide-react";



export default function AgentsPageContent() {
  const { t } = useI18n();
  const router = useRouter();
  const { subscribe } = useWS();
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

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
  const [menuPoint, setMenuPoint] = useState<AgentMenuPoint | null>(null);
  const handleCloseDetail = useCallback(() => setSelectedAgentId(null), []);
  const handleSelectAgent = useCallback((id: string) => setSelectedAgentId(id), []);

  const [bulkMode, setBulkMode] = useState(false);
  const [actionMsg, setActionMsg] = useState<string | null>(null);
  const [showResults, setShowResults] = useState(false);
  const [bulkResults, setBulkResults] = useState<BulkResult[]>([]);
  const [exporting, setExporting] = useState(false);

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

  // Keep the bulk selection honest: prune ids that left the current list
  // (page change, filter applied, agent deleted) so the bulk bar never
  // claims invisible rows.
  const visibleIds = useMemo(() => new Set(sortedBeacons.map((b) => b.id || "").filter(Boolean)), [sortedBeacons]);
  useEffect(() => {
    setSelected((prev) => {
      let pruned = false;
      const next = new Set<string>();
      for (const id of prev) {
        if (visibleIds.has(id)) next.add(id);
        else pruned = true;
      }
      return pruned ? next : prev;
    });
  }, [visibleIds, setSelected]);

  const AGENT_ROW_H = 56;
  // Hardcoded 56px was ~40% taller than the real rows (py-1 + content), so
  // spacer rows drifted from actual layout while scrolling. Measure the
  // first rendered row instead.
  const [rowHeight, setRowHeight] = useState(AGENT_ROW_H);
  const rowHeightRef = useRef(AGENT_ROW_H);
  const {
    scrollRef: agentScrollRef,
    onScroll: onAgentScroll,
    virtualized: agentVirtualized,
    start: agentVirtStart,
    end: agentVirtEnd,
    offsetTop: agentOffsetTop,
    totalHeight: agentTotalHeight,
  } = useVirtualWindow({ count: sortedBeacons.length, rowHeight, threshold: 25 });

  useEffect(() => {
    if (!agentVirtualized) return;
    const el = agentScrollRef.current;
    if (!el) return;
    const probe = () => {
      const tr = el.querySelector<HTMLElement>("tbody tr[data-agent-id]");
      if (tr && tr.offsetHeight > 0 && Math.abs(tr.offsetHeight - rowHeightRef.current) > 1) {
        rowHeightRef.current = tr.offsetHeight;
        setRowHeight(tr.offsetHeight);
      }
    };
    probe();
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(probe);
    ro.observe(el);
    return () => ro.disconnect();
  }, [agentVirtualized, agentScrollRef]);

  const visibleBeacons = useMemo(
    () => (agentVirtualized ? sortedBeacons.slice(agentVirtStart, agentVirtEnd) : sortedBeacons),
    [sortedBeacons, agentVirtualized, agentVirtStart, agentVirtEnd],
  );

  const loadBeacons = useCallback(() => {
    loadBeaconsRaw(searchQuery, statusFilter, osFilter, page, 50, tagFilter, linkedFilter, sortKey, sortDir);
  }, [loadBeaconsRaw, searchQuery, statusFilter, osFilter, page, tagFilter, linkedFilter, sortKey, sortDir]);

  const loadBeaconsRef = useRef(loadBeacons);
  loadBeaconsRef.current = loadBeacons;
  const onlineReloadTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

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
      api.get(paths.agents.bulkResults())
      .then((d) => {
        if (!mountedRef.current) return;
        const results = (d.results || d.data || []) as BulkResult[];
        setBulkResults(results);
      })
      .catch(() => { if (mountedRef.current) setActionMsg(t("agents.bulk_results_failed")); });
  }, [showResults, t]);

  useEffect(() => {
    const unsub = subscribe((msg) => {
      if (msg.type === "agent_online" || msg.type === "agent_offline") {
        if (onlineReloadTimer.current) clearTimeout(onlineReloadTimer.current);
        onlineReloadTimer.current = setTimeout(() => loadBeaconsRef.current(), 500);
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
    return () => {
      if (onlineReloadTimer.current) clearTimeout(onlineReloadTimer.current);
      unsub();
    };
    // loadBeaconsRef always points at latest loadBeacons — intentional stable subscribe
  }, [subscribe, setAgentLocks, setBeacons]);

  const sortIcon = (field: typeof sortKey) => {
    if (sortKey !== field) return <ArrowUpDown className="w-3 h-3 text-muted-foreground" />;
    return sortDir === "asc" ? <ArrowUp className="w-3 h-3 text-primary" /> : <ArrowDown className="w-3 h-3 text-primary" />;
  };

  const handleSortKeyDown = useCallback(
    (field: typeof sortKey) => (e: React.KeyboardEvent) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        toggleSort(field);
      }
    },
    [toggleSort],
  );

  const runBatch = async (payload: Record<string, unknown>) => {
    try {
      const data = await api.postJson<{ success?: boolean; tasks_created?: number; error?: string }>(paths.agents.batch, payload);
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
      const data = await api.postJson<{ success?: boolean; tasks_created?: number; failed?: number; error?: string }>(paths.agents.bulkTask, { agent_ids: agentIds, task_type: type, command });
      if (data.success) {
        let bulkMsg = t("agents.bulk_type_result").replace("{type}", type).replace("{count}", String(data.tasks_created ?? agentIds.length));
        if (data.failed) bulkMsg += ` (${data.failed} ${t("agents.failed_suffix")})`;
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
    const interval = Number(batchSleepInterval);
    const jitter = Number(batchSleepJitter);
    if (!Number.isFinite(interval) || interval < 1 || interval > 86400 || !Number.isFinite(jitter) || jitter < 0 || jitter > 100) {
      setActionMsg(t("agents.sleep_invalid"));
      return;
    }
    runBatch({ agent_ids: ids, task_type: "sleep", args: `${interval},${jitter}` });
    closeBatchSleep();
  };

  const batchDelete = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    try {
      const data = await api.postJson<{ success?: boolean; deleted?: number; error?: string }>(paths.agents.batchDelete, { agent_ids: ids });
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
      await api.postJson(paths.agents.kill(id), {});
      setActionMsg(t("agents.kill_sent"));
      loadBeacons();
    } catch {
      setActionMsg(t("agents.kill_failed"));
    }
    setConfirm(null);
  };

  const deleteAgent = async (id: string) => {
    try {
      await api.del(paths.agents.one(id));
      setActionMsg(t("agents.deleted"));
      loadBeacons();
    } catch {
      setActionMsg(t("agents.delete_failed"));
    }
    setConfirm(null);
  };

  const uninstallAgent = async (id: string) => {
    try {
      await api.postJson(paths.agents.uninstall(id), {});
      setActionMsg(t("agents.detail_uninstall_sent"));
      loadBeacons();
    } catch {
      setActionMsg(t("agents.detail_uninstall_failed"));
    }
    setConfirm(null);
  };

  const handleMenuAction = useCallback((action: AgentMenuAction, point: AgentMenuPoint) => {
    const id = point.beacon.id || "";
    switch (action) {
      case "interact":
      case "socks":
        useInteractStore.getState().open(id, { beacon: point.beacon });
        break;
      case "details":
        setSelectedAgentId(id);
        break;
      case "screenshot":
        api.post(paths.agents.screenshotTask(id))
          .then((d) => {
            const taskId = Number((d as { task_id?: number }).task_id);
            const queued = Number.isFinite(taskId) && taskId > 0;
            if (queued) useInteractStore.getState().revealTask(id, taskId);
            toast.success(t("agents.detail_action_queued", { label: t("agents.screenshot") }));
          })
          .catch(() => toast.error(t("agents.screenshot_failed")));
        break;
      case "sleep":
        openQuickSleep(point.beacon);
        break;
      case "notes":
        openNotesEdit(point.beacon);
        break;
      case "files":
        router.push(`/agents/${id}/files`);
        break;
      case "tokens":
        router.push(`/agents/${id}/token`);
        break;
      case "screen":
        router.push(`/agents/${id}/screen`);
        break;
      case "copy_id":
        navigator.clipboard.writeText(id)
          .then(() => toast.success(t("agents.detail_copied")))
          .catch(() => toast.error(t("agents.detail_copy_failed")));
        break;
      case "kill":
        setConfirm({ type: "kill", id, hostname: point.beacon.hostname || id });
        break;
      case "uninstall":
        setConfirm({ type: "uninstall", id, hostname: point.beacon.hostname || id });
        break;
      case "delete":
        setConfirm({ type: "delete", id, hostname: point.beacon.hostname || id });
        break;
    }
    setMenuPoint(null);
  }, [t, openQuickSleep, openNotesEdit, router, setConfirm]);

  const handleQuickNav = useCallback((beacon: Beacon, view: "shell" | "files" | "screen") => {
    const id = beacon.id || "";
    if (!id) return;
    router.push(`/agents/${id}/${view}`);
  }, [router]);

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

  const submitQuickSleep = async () => {
    if (!quickSleepAgent) return;
    const interval = Number(sleepInterval);
    const jitter = Number(sleepJitter);
    if (!Number.isFinite(interval) || interval < 1 || interval > 86400 || !Number.isFinite(jitter) || jitter < 0 || jitter > 100) {
      setActionMsg(t("agents.sleep_invalid"));
      return;
    }
    try {
      await api.postJson(paths.agents.setSleep(quickSleepAgent.id), { interval, jitter });
      setActionMsg(t("agents.sleep_updated").replace("{name}", quickSleepAgent.hostname));
      closeQuickSleep();
      loadBeacons();
    } catch { setActionMsg(t("agents.sleep_failed")); }
  };

  const submitNotes = async () => {
    if (!editingNotesId) return;
    try {
      await api.postJson(paths.agents.note(editingNotesId), { notes: editNotesText });
      setActionMsg(t("agents.notes_updated"));
      closeNotesEdit();
      loadBeacons();
    } catch { setActionMsg(t("agents.notes_failed")); }
  };

  const exportCSV = async () => {
    if (exporting) return;
    setExporting(true);
    try {
      // Export respects the active filters but spans ALL pages, not just
      // the current 50-row slice.
      const all: Beacon[] = [];
      const PAGE = 100;
      for (let p = 1; p <= 1000; p++) {
        const q = new URLSearchParams({ page: String(p), page_size: String(PAGE), group: "host" });
        if (searchQuery) q.set("search", searchQuery);
        if (statusFilter) q.set("status", statusFilter);
        if (osFilter) q.set("os", osFilter);
        if (tagFilter) q.set("tag_id", tagFilter);
        if (linkedFilter) q.set("linked", linkedFilter);
        q.set("sort_key", sortKey);
        q.set("sort_dir", sortDir);
        const d = await api.get<{ agents?: Beacon[]; total?: number | string }>(paths.agents.list(q.toString()), { unwrap: false });
        const pageList = (d.agents || []) as Beacon[];
        all.push(...pageList);
        if (pageList.length < PAGE) break;
      }
      if (all.length === 0) {
        toast.info(t("agents.no_beacons"));
        return;
      }
      const headers = ["Hostname", "User", "OS", "IP", "Status", "Last Seen", "Version", "Active Window", "Notes"];
      const rows = all.map((b) => [
        b.hostname || "", b.username || "", b.os || "", b.ip || "",
        b.status || "", b.last_seen || "", b.version || "", b.active_window || "", b.notes || "",
      ]);
      const csv = [headers.join(","), ...rows.map((r) => r.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(","))].join("\n");
      downloadText(csv, `agents-${new Date().toISOString().slice(0, 10)}.csv`, "text/csv");
      toast.success(t("agents.export_done").replace("{count}", String(all.length)));
    } catch {
      toast.error(t("agents.export_failed"));
    } finally {
      setExporting(false);
    }
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
  const allVisibleSelected = beacons.length > 0 && beacons.every((b) => b.id && selected.has(b.id));
  const someVisibleSelected = selected.size > 0 && !allVisibleSelected;

  return (
    <PageContainer
      title={t("agents.title")}
      subtitle={`${total} ${t("agents.total_label")} · ${onlineCount} ${t("agents.online_label")}${staleCount > 0 ? `, ${staleCount} ${t("agents.stale_label")}` : ""}${offlineCount > 0 ? `, ${offlineCount} ${t("agents.offline_label")}` : ""}`}
      className="animate-fade-slide-up"
      actions={<>
        <Button
          variant="outline"
          onClick={() => { setBulkMode((p) => !p); if (!bulkMode) setSelected(new Set()); }}
          className={`h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
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
          className={`h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
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
          disabled={exporting}
          className="h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem]"
          title={t("agents.export_csv_title")}
        >
          <Download className="w-4 h-4" />
          <span className="hidden sm:inline text-foreground text-sm">{t("agents.export")}</span>
        </Button>
        <Button
          variant="outline"
          onClick={() => setAutoRefresh((p) => !p)}
          className={`h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
            autoRefresh
              ? "bg-success border-success text-white hover:bg-success/10"
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
          className="h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem]"
          title={viewMode === "table" ? t("agents.switch_grid") : t("agents.switch_table")}
        >
          {viewMode === "table" ? <Grip className="w-4 h-4" /> : <ListOrdered className="w-4 h-4" />}
        </Button>
        <Button
          variant="outline"
          onClick={() => { setPage(1); loadBeacons(); }}
          className="h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem]"
        >
          <RefreshCw className="w-4 h-4" />
          <span className="hidden sm:inline text-foreground text-sm">{t("agents.refresh")}</span>
        </Button>
        <Button render={<Link href="/generate" />}>
          <Plus className="w-4 h-4" />
          <span className="hidden sm:inline">{t("agents.generate_implant")}</span>
          <span className="sm:hidden">{t("agents.new")}</span>
        </Button>
      </>}
    >
      {error && (
        <DataError
          message={error}
          onRetry={() => { setError(null); loadBeacons(); }}
          onDismiss={() => setError(null)}
          className="mb-4"
        />
      )}

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
      <Card className="overflow-hidden">
        <div
          ref={agentScrollRef}
          onScroll={onAgentScroll}
          className={agentVirtualized ? "overflow-auto max-h-[min(70vh,720px)]" : "overflow-auto"}
        >
        <Table className="text-sm min-w-[850px]">
          <TableHeader className="sticky top-0 z-10">
            <TableRow>
              <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5 w-10">
                <Checkbox aria-label={t("agents.select_all")} name="input-4"
                  checked={allVisibleSelected || someVisibleSelected}
                  indeterminate={someVisibleSelected && !allVisibleSelected}
                  onCheckedChange={(v) => toggleSelectAll(v !== false)}
                />
              </TableHead>
              {visibleCols.hostname && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "hostname" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("hostname")} onKeyDown={handleSortKeyDown("hostname")}>
                {t("agents.col_hostname")} {sortIcon("hostname")}
              </TableHead>
              )}
              {visibleCols.username && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "username" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("username")} onKeyDown={handleSortKeyDown("username")}>
                {t("agents.col_user")} {sortIcon("username")}
              </TableHead>
              )}
              {visibleCols.os && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "os" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("os")} onKeyDown={handleSortKeyDown("os")}>
                {t("agents.col_os")} {sortIcon("os")}
              </TableHead>
              )}
              {visibleCols.ip && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "ip" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("ip")} onKeyDown={handleSortKeyDown("ip")}>
                {t("agents.col_ip")} {sortIcon("ip")}
              </TableHead>
              )}
              {visibleCols.last_seen && (
              <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "last_seen" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("last_seen")} onKeyDown={handleSortKeyDown("last_seen")}>
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
              <TableHead className="text-center py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "status" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("status")} onKeyDown={handleSortKeyDown("status")}>
                {t("agents.col_status")} {sortIcon("status")}
              </TableHead>
              )}
              <TableHead className="text-right py-3 px-4 sm:py-3.5 sm:px-5">{t("agents.col_actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="divide-y divide-border">
            {loading && Array.from({ length: 6 }).map((_, i) => (
              <TableRow key={`skel-${i}`}>
                {Array.from({ length: emptyColSpan }).map((_, j) => (
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
                onInteract={handleSelectAgent}
                onDetails={handleSelectAgent}
                onMenu={setMenuPoint}
                onQuickNav={handleQuickNav}
                onEditNotes={openNotesEdit}
                taskCount={taskCountMap[beacon.id || ""] ?? 0}
                lockUser={agentLocks[beacon.id || ""] || null}
                visibleCols={visibleCols}
              />
            ))}
            {!loading && agentVirtualized && agentTotalHeight - agentOffsetTop - visibleBeacons.length * rowHeight > 0 && (
              <TableRow aria-hidden className="hover:bg-transparent">
                <TableCell colSpan={emptyColSpan} style={{ height: agentTotalHeight - agentOffsetTop - visibleBeacons.length * rowHeight, padding: 0, border: 0 }} />
              </TableRow>
            )}
            {!loading && beacons.length === 0 && (
              <TableRow>
                <TableCell colSpan={emptyColSpan} className="py-10">
                  <EmptyState
                    icon={Radio}
                    title={t("agents.no_beacons")}
                    message={statusFilter || osFilter ? t("agents.no_beacons_filtered") : t("agents.no_beacons_hint")}
                    action={!statusFilter && !osFilter ? (
                      <Button render={<Link href="/generate" />}>
                        <Plus className="w-4 h-4" />
                        <span>{t("agents.generate_implant")}</span>
                      </Button>
                    ) : undefined}
                  />
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        </div>
        <div className="sm:hidden px-4 py-2 text-center text-xs text-muted-foreground border-t border-border bg-muted">
          <ArrowLeftRight className="w-4 h-4" /> {t("agents.swipe_hint")}
        </div>

        <Pagination page={page} pageSize={50} total={total} onPageChange={setPage} />
      </Card>
      )}

      {viewMode === "grid" && loading && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3 p-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Card key={`gskel-${i}`} className="p-4 space-y-3">
              <div className="flex items-center gap-2.5">
                <Skeleton className="w-10 h-10 rounded-xl" />
                <div className="space-y-1.5 flex-1"><Skeleton className="h-4 w-24" /><Skeleton className="h-3 w-16" /></div>
              </div>
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-2/3" />
              <Skeleton className="h-3 w-1/2" />
            </Card>
          ))}
        </div>
      )}

      {viewMode === "grid" && !loading && beacons.length === 0 && (
        <Card className="overflow-hidden">
          <EmptyState
            icon={Radio}
            title={t("agents.no_beacons")}
            message={statusFilter || osFilter ? t("agents.no_beacons_filtered") : t("agents.no_beacons_hint")}
            action={!statusFilter && !osFilter ? (
              <Button render={<Link href="/generate" />}>
                <Plus className="w-4 h-4" />
                <span>{t("agents.generate_implant")}</span>
              </Button>
            ) : undefined}
          />
        </Card>
      )}

      {viewMode === "grid" && !loading && beacons.length > 0 && (
        <>
          <AgentGrid
            beacons={sortedBeacons}
            tagsByAgent={tagsByAgent}
            taskCountMap={taskCountMap}
            onInteract={handleSelectAgent}
            onDetails={handleSelectAgent}
            onMenu={setMenuPoint}
            selected={selected}
            onToggleSelect={toggleSelect}
          />
          <Pagination page={page} pageSize={50} total={total} onPageChange={setPage} />
        </>
      )}

      <ConfirmModal
        open={confirm?.type === "kill"}
        title={t("agents.confirm_kill_title")}
        message={t("agents.confirm_kill_msg").replace("{name}", confirm?.hostname || confirm?.id || "")}
        confirmText={t("agents.confirm_kill_btn")}
        danger
        requireText={confirm?.hostname || ""}
        onConfirm={() => { if (confirm?.id) void killAgent(confirm.id); }}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "delete"}
        title={t("agents.confirm_delete_title")}
        message={t("agents.confirm_delete_msg").replace("{name}", confirm?.hostname || confirm?.id || "")}
        confirmText={t("agents.confirm_delete_btn")}
        danger
        requireText={confirm?.hostname || ""}
        onConfirm={() => { if (confirm?.id) void deleteAgent(confirm.id); }}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "uninstall"}
        title={t("agents.uninstall_agent")}
        message={t("agents.uninstall_msg").replace("{name}", confirm?.hostname || confirm?.id || "")}
        confirmText={t("agents.uninstall")}
        danger
        requireText={confirm?.hostname || ""}
        onConfirm={() => { if (confirm?.id) void uninstallAgent(confirm.id); }}
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
          {selectedAgentId && <AgentDetailPage key={selectedAgentId} agentId={selectedAgentId} onClose={handleCloseDetail} />}
        </SheetContent>
      </Sheet>

      <AgentContextMenu
        point={menuPoint}
        onClose={() => setMenuPoint(null)}
        onAction={(action) => { if (menuPoint) handleMenuAction(action, menuPoint); }}
      />
    </PageContainer>
  );
}

