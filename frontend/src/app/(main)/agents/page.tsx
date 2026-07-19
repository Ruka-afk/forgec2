"use client";

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useWS } from "@/lib/wsContext";
import { useRouter } from "next/navigation";
import { downloadText } from "@/lib/download";
import Link from "next/link";
import { api } from "@/lib/api";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
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
import type { Beacon, BulkResult } from "./_components/types";
import { copyToClipboard } from "./_components/types";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { useI18n } from "@/lib/i18n";
import { AlertCircle, ArrowDown, ArrowLeftRight, ArrowUp, ArrowUpDown, Download, Grip, History, ListChecks, ListOrdered, Pause, Play, Plus, Radio, RefreshCw, X } from "lucide-react";
import { toast } from "sonner";

export type { Beacon };

async function fetchBeacons(search = "", status = "", os = "", page = 1, pageSize = 20, tag_id = "") {
  const p = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
  if (search) p.set("search", search);
  if (status) p.set("status", status);
  if (os) p.set("os", os);
  if (tag_id) p.set("tag_id", tag_id);
  return api.get(`/api/v1/agents?${p.toString()}`);
}

export default function BeaconsPage() {
  const { t } = useI18n();
  const router = useRouter();
  const [beacons, setBeacons] = useState<Beacon[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [confirm, setConfirm] = useState<{ type: "kill" | "delete" | "batch-delete" | "bulk-kill" | "bulk-uninstall"; id?: string; hostname?: string } | null>(null);
  const [actionMsg, setActionMsg] = useState<string | null>(null);
  const [allTags, setAllTags] = useState<{ id: string; name: string; color: string }[]>([]);
  const [tagsByAgent, setTagsByAgent] = useState<Record<string, { id: string; name: string; color: string }[]>>({});
  const [taskCountMap, setTaskCountMap] = useState<Record<string, number>>({});
  const [error, setError] = useState<string | null>(null);
  const [agentLocks, setAgentLocks] = useState<Record<string, string>>({});
  const [bulkMode, setBulkMode] = useState(false);
  const [cmdModalOpen, setCmdModalOpen] = useState(false);
  const [cmdType, setCmdType] = useState("shell");
  const [cmdText, setCmdText] = useState("");
  const [bulkResults, setBulkResults] = useState<BulkResult[]>([]);
  const [showResults, setShowResults] = useState(false);
  const [quickSleepAgent, setQuickSleepAgent] = useState<{ id: string; hostname: string; interval: number; jitter: number } | null>(null);
  const [sleepInterval, setSleepInterval] = useState("30");
  const [sleepJitter, setSleepJitter] = useState("20");
  const [editingNotesId, setEditingNotesId] = useState<string | null>(null);
  const [editNotesText, setEditNotesText] = useState("");
  const [lastTaskMap, setLastTaskMap] = useState<Record<string, { type?: string; status?: string; command?: string }>>({});
  const [copiedField, setCopiedField] = useState("");
  const [screenshotConfirmOpen, setScreenshotConfirmOpen] = useState(false);
  const [batchSleepOpen, setBatchSleepOpen] = useState(false);
  const [batchSleepInterval, setBatchSleepInterval] = useState("30");
  const [batchSleepJitter, setBatchSleepJitter] = useState("20");

  const {
    searchInput, setSearchInput, searchQuery,
    statusFilter, setStatusFilter, osFilter, setOsFilter,
    page, setPage, sortKey, sortDir, toggleSort,
    linkedFilter, setLinkedFilter, tagFilter, setTagFilter,
    autoRefresh, setAutoRefresh, viewMode, setViewMode,
    visibleCols, setVisibleCols, colMenuOpen, setColMenuOpen,
    sortedBeacons,
  } = useAgentFilters(beacons);
  const { selected, setSelected, toggleSelect, toggleSelectAll } = useAgentSelection(beacons);

  const { subscribe } = useWS();

  const loadBeacons = useCallback(() => {
    setLoading(true);
    fetchBeacons(searchQuery, statusFilter, osFilter, page, 20, tagFilter)
      .then((data) => {
        const list = (data.agents || []) as Beacon[];
        setBeacons(list);
        setTotal(Number(data.total) || list.length);
      })
      .catch(() => {
        setBeacons([]);
        setTotal(0);
        setError(t("agents.load_failed"));
      })
      .finally(() => setLoading(false));
  }, [searchQuery, statusFilter, osFilter, page, tagFilter]);

  const loadBeaconsRef = useRef(loadBeacons);
  loadBeaconsRef.current = loadBeacons;

  useEffect(() => { loadBeacons(); }, [loadBeacons]);

  useVisibleInterval(() => loadBeacons(), autoRefresh ? 30000 : 0);

  const loadLocks = useCallback(() => {
      api.get<{ agents: Record<string, string>[] }>("/collab/agents")
      .then((data) => {
        const agents = data.agents || [];
        const locks: Record<string, string> = {};
        for (const a of agents) {
          if (a.locked_by) locks[a.id] = a.locked_by;
        }
        setAgentLocks(locks);
      })
      .catch(() => { toast.error(t("agents.locks_failed")); });
  }, []);

  useEffect(() => { loadLocks(); }, [loadLocks]);

  useEffect(() => {
    api.get("/tasks?page=1&page_size=20&format=json")
      .then((d) => {
        const tasks = d.tasks || d.data || [];
        const countMap: Record<string, number> = {};
        const lastMap: Record<string, { type?: string; status?: string; command?: string }> = {};
        (tasks as { agent_id?: string; AgentID?: string; type?: string; Type?: string; status?: string; Status?: string; command?: string; Command?: string }[]).forEach((t) => {
          const aid = t.agent_id || "";
          if (!aid) return;
          countMap[aid] = (countMap[aid] || 0) + 1;
          if (!lastMap[aid]) {
            lastMap[aid] = { type: t.type, status: t.status, command: t.command };
          }
        });
        setTaskCountMap(countMap);
        setLastTaskMap(lastMap);
      })
      .catch(() => { setTaskCountMap({}); setLastTaskMap({}); });
  }, []);

  useEffect(() => {
    api.get("/api/tags")
      .then((d) => setAllTags((d.tags || []) as { id: string; name: string; color: string }[]))
      .catch(() => setAllTags([]));
  }, []);

  useEffect(() => {
    if (beacons.length === 0) { setTagsByAgent({}); return; }
    const ids = beacons.map((b) => b.id || "").filter(Boolean);
    api.postJson<{ tags: Record<string, { id: string; name: string; color: string }[]> }>("/agents/batch/tags", { agent_ids: ids })
      .then((d) => setTagsByAgent(d.tags || {}))
      .catch(() => setTagsByAgent({}));
  }, [beacons]);

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
  }, [showResults]);

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
  }, [subscribe]);

  const sortIcon = (field: typeof sortKey) => {
    if (sortKey !== field) return <ArrowUpDown className="w-3 h-3 text-muted-foreground" />;
    return sortDir === "asc" ? <ArrowUp className="w-3 h-3 text-indigo-500" /> : <ArrowDown className="w-3 h-3 text-indigo-500" />;
  };

  const handleRowConfirm = useCallback(
    (type: "kill" | "delete" | "batch-delete" | "bulk-kill" | "bulk-uninstall", id: string, hostname: string) => {
      setConfirm({ type, id, hostname });
    },
    [],
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
    setScreenshotConfirmOpen(true);
  };

  const doBatchScreenshot = () => {
    setScreenshotConfirmOpen(false);
    runBatch({ agent_ids: Array.from(selected), task_type: "screenshot" });
  };

  const batchSleep = () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    setBatchSleepInterval("30");
    setBatchSleepJitter("20");
    setBatchSleepOpen(true);
  };

  const doBatchSleep = () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    runBatch({ agent_ids: ids, task_type: "sleep", args: `${batchSleepInterval},${batchSleepJitter}` });
    setBatchSleepOpen(false);
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
    setCmdModalOpen(false);
    setCmdText("");
    setCmdType("shell");
  };

  const openCmdModal = () => {
    setCmdType("shell");
    setCmdText("");
    setCmdModalOpen(true);
  };

  const sendScreenshot = async (id: string) => {
    try {
      await api.post(`/agents/${id}/screenshot`);
      setActionMsg(t("agents.screenshot_sent"));
    } catch { setActionMsg(t("agents.screenshot_failed")); }
  };

  const openQuickSleep = (e: React.MouseEvent, agent: Beacon) => {
    e.stopPropagation();
    setQuickSleepAgent({ id: agent.id || "", hostname: agent.hostname || "", interval: agent.current_interval || 30, jitter: agent.current_jitter || 20 });
    setSleepInterval(String(agent.current_interval || 30));
    setSleepJitter(String(agent.current_jitter || 20));
  };

  const submitQuickSleep = async () => {
    if (!quickSleepAgent) return;
    try {
      await api.postJson(`/agents/${quickSleepAgent.id}/set_sleep`, { interval: Number(sleepInterval), jitter: Number(sleepJitter) });
      setActionMsg(t("agents.sleep_updated").replace("{name}", quickSleepAgent.hostname));
      setQuickSleepAgent(null);
      loadBeacons();
    } catch { setActionMsg(t("agents.sleep_failed")); }
  };

  const openNotesEdit = (e: React.MouseEvent, agent: Beacon) => {
    e.stopPropagation();
    setEditingNotesId(agent.id || "");
    setEditNotesText(agent.notes || "");
  };

  const submitNotes = async () => {
    if (!editingNotesId) return;
    try {
      await api.postJson(`/agents/${editingNotesId}/note`, { notes: editNotesText });
      setActionMsg(t("agents.notes_updated"));
      setEditingNotesId(null);
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

  const onlineCount = useMemo(() => beacons.filter((b) => b.status === "online").length, [beacons]);
  const staleCount = useMemo(() => beacons.filter((b) => b.status === "stale").length, [beacons]);
  const offlineCount = useMemo(() => beacons.filter((b) => b.status === "offline").length, [beacons]);
  const windowsCount = useMemo(() => beacons.filter((b) => (b.os || "").toLowerCase().includes("windows")).length, [beacons]);
  const linuxCount = useMemo(() => beacons.filter((b) => (b.os || "").toLowerCase().includes("linux")).length, [beacons]);
  const darwinCount = useMemo(() => beacons.filter((b) => (b.os || "").toLowerCase().includes("darwin")).length, [beacons]);

  const emptyColSpan = Object.values(visibleCols).filter(Boolean).length + 3;

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      {error && (
        <div className="mb-4 px-4 py-3 bg-destructive/10 border border-destructive/20 rounded-xl flex items-center gap-3 text-sm text-destructive">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
          <Button variant="outline" size="xs" className="ml-auto mr-2" onClick={() => { setError(null); loadBeacons(); }}>{t("agents.retry")}</Button>
          <Button variant="ghost" size="icon-xs" onClick={() => setError(null)} className="text-muted-foreground hover:text-destructive" aria-label={t("agents.dismiss")}>
            <X className="w-4 h-4" />
          </Button>
        </div>
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
        <Link
          href="/generate"
          className="inline-flex items-center justify-center gap-x-2 bg-primary text-primary-foreground hover:bg-primary/80 transition-colors px-4 sm:px-5 h-9 sm:h-10 rounded-xl text-sm font-medium min-w-[2.75rem] min-h-[2.75rem]"
        >
          <Plus className="w-4 h-4" />
          <span className="hidden sm:inline">{t("agents.generate_implant")}</span>
          <span className="sm:hidden">{t("agents.new")}</span>
        </Link>
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
        staleCount={staleCount}
        offlineCount={offlineCount}
        windowsCount={windowsCount}
        linuxCount={linuxCount}
        darwinCount={darwinCount}
      />

      {viewMode === "table" && (
      <Card className="sm:rounded-xl overflow-auto">
        <Table className="text-sm min-w-[850px]">
          <TableHeader className="border-b border-border bg-muted">
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
              {visibleCols.tags && (
              <TableHead className="text-center py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_tags")}</TableHead>
              )}
              {visibleCols.tasks && (
              <TableHead className="text-center py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_tasks")}</TableHead>
              )}
              {visibleCols.tasks && <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_last_task")}</TableHead>}
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
                {Array.from({ length: 13 }).map((_, j) => (
                  <TableCell key={j} className="py-3 px-3 sm:py-3.5 sm:px-4">
                    <Skeleton className="h-4 w-3/4" />
                  </TableCell>
                ))}
              </TableRow>
            ))}
            {!loading && sortedBeacons.map((beacon) => (
              <AgentRow
                key={beacon.id || ""}
                beacon={beacon}
                isSelected={selected.has(beacon.id || "")}
                onToggleSelect={toggleSelect}
                onScreenshot={sendScreenshot}
                onQuickSleep={openQuickSleep}
                onNotes={openNotesEdit}
                onConfirm={handleRowConfirm}
                tags={tagsByAgent[beacon.id || ""] || []}
                taskCount={taskCountMap[beacon.id || ""] ?? 0}
                lastTask={lastTaskMap[beacon.id || ""]}
                lockUser={agentLocks[beacon.id || ""] || null}
                visibleCols={visibleCols}
                copiedField={copiedField}
                onCopy={handleRowCopy}
              />
            ))}
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
                      <Link href="/generate" className="inline-flex items-center gap-2 px-4 py-2.5 bg-primary text-primary-foreground hover:bg-primary/80 rounded-xl transition-colors min-w-[2.75rem] min-h-[2.75rem]">
                        <Plus className="w-4 h-4" />
                        <span>{t("agents.generate_implant")}</span>
                      </Link>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
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
            onSelect={(id) => router.push(`/agents/${id}`)}
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
        onClose={() => setCmdModalOpen(false)}
      />

      {quickSleepAgent && (
        <QuickSleepModal
          agent={quickSleepAgent}
          sleepInterval={sleepInterval}
          setSleepInterval={setSleepInterval}
          sleepJitter={sleepJitter}
          setSleepJitter={setSleepJitter}
          onSubmit={submitQuickSleep}
          onClose={() => setQuickSleepAgent(null)}
        />
      )}

      {editingNotesId && (
        <NotesEditModal
          notesText={editNotesText}
          setNotesText={setEditNotesText}
          onSubmit={submitNotes}
          onClose={() => setEditingNotesId(null)}
        />
      )}

      <ConfirmModal
        open={screenshotConfirmOpen}
        title={t("agents.confirm_screenshot_title")}
        message={t("agents.confirm_screenshot_msg").replace("{count}", String(selected.size))}
        confirmText={t("agents.confirm_screenshot_btn")}
        danger={false}
        onConfirm={doBatchScreenshot}
        onCancel={() => setScreenshotConfirmOpen(false)}
      />

      {batchSleepOpen && (
        <BatchSleepModal
          agentCount={selected.size}
          interval={batchSleepInterval}
          setInterval={setBatchSleepInterval}
          jitter={batchSleepJitter}
          setJitter={setBatchSleepJitter}
          onSubmit={doBatchSleep}
          onClose={() => setBatchSleepOpen(false)}
        />
      )}
    </div>
  );
}

