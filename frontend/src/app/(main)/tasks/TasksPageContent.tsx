"use client";
import { PageContainer } from "@/components/ui/page-container";
import { ErrorState } from "@/components/ui/error-state";

import { Suspense, useState, useCallback, useEffect, memo, useMemo, useRef } from "react";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { downloadText } from "@/lib/download";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Pagination } from "@/components/ui/pagination";
import { Spinner } from "@/components/ui/spinner";
import { StatusBadge } from "@/components/ui/status-indicator";
import { Badge } from "@/components/ui/badge";
import { DataState } from "@/components/ui/data-state";
import OperatorBadge from "@/components/OperatorBadge";
import { useAppStore } from "@/lib/store";
import type { Agent } from "@/types/agent";
import { normalizeAgentList } from "@/lib/agents";
import { paths } from "@/lib/api-paths";
import { firstArray, firstNumber } from "@/lib/envelope";
import { formatTime } from "@/lib/utils";
import { useVirtualWindow } from "@/lib/hooks/useVirtualWindow";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { useCachedData } from "@/lib/hooks/useCachedData";
import { useWS } from "@/lib/wsContext";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Ban, Check, ChevronDown, ChevronUp, FileSpreadsheet, Hand, Inbox, Maximize2, RotateCw, X, ArrowUpDown, ArrowUp, ArrowDown } from "lucide-react";
import type { Task } from "@/types/task";

type SortKey = "created_at" | "type" | "command" | "status";

function TasksPage({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  const searchParams = useSearchParams();
  const [tasks, setTasks] = useState<Task[]>([]);
  const { data: agents } = useCachedData<Agent[]>("agents:names", {
    fetcher: async () => {
      const data = await api.get(paths.agents.list("page=1&pageSize=200"));
      return normalizeAgentList(data);
    },
    ttlMs: 120_000,
    onError: () => toast.error(t("tasks.toast_load_agents_failed")),
  });
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState("");
  const [agentFilter, setAgentFilter] = useState(searchParams.get("agent_id") || "");
  const [typeFilter, setTypeFilter] = useState("");
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());
  const [detailTask, setDetailTask] = useState<Task | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const toggleSort = useCallback((key: SortKey) => {
    if (sortKey === key) setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    else { setSortKey(key); setSortDir(key === "created_at" ? "desc" : "asc"); }
  }, [sortKey]);

  const sortIcon = (col: SortKey) => {
    if (sortKey !== col) return <ArrowUpDown className="w-3 h-3 inline-block ml-1 text-muted-foreground/50" />;
    return sortDir === "asc"
      ? <ArrowUp className="w-3 h-3 inline-block ml-1 text-primary" />
      : <ArrowDown className="w-3 h-3 inline-block ml-1 text-primary" />;
  };

  const handleSortKeyDown = useCallback(
    (field: SortKey) => (e: React.KeyboardEvent) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        toggleSort(field);
      }
    },
    [toggleSort],
  );

  const getAgentName = useCallback((agentId: string) => {
    return (agents || []).find((a) => a.id === agentId)?.hostname;
  }, [agents]);

  const loadTasksAbortRef = useRef<AbortController | null>(null);

  const loadTasks = useCallback((background = false) => {
    loadTasksAbortRef.current?.abort();
    const ac = new AbortController();
    loadTasksAbortRef.current = ac;
    if (!background) {
      setLoading(true);
      setError(null);
    }
    const params = new URLSearchParams({ page: String(page), pageSize: "50" });
    if (statusFilter) params.set("status", statusFilter);
    if (agentFilter) params.set("agent", agentFilter);
    if (typeFilter) params.set("type", typeFilter);
    api.get(paths.tasks.list(params.toString()), { signal: ac.signal })
      .then((data: Record<string, unknown>) => {
        if (ac.signal.aborted) return;
        setTasks(firstArray(data, ["tasks", "data", "Tasks"]) as Task[]);
        setTotal(firstNumber(data, ["total", "Total"], 0));
      })
      .catch((e) => {
        if (ac.signal.aborted) return;
        setTasks([]);
        setTotal(0);
        const msg = e instanceof Error ? e.message : t("tasks.toast_load_failed");
        setError(msg);
        toast.error(msg);
      })
      .finally(() => {
        if (!ac.signal.aborted) setLoading(false);
      });
  }, [page, statusFilter, agentFilter, typeFilter, t]);

  useEffect(() => { loadTasks(); }, [loadTasks]);

  const loadTasksRef = useRef(loadTasks);
  loadTasksRef.current = loadTasks;
  const pollTasks = useCallback(() => { loadTasksRef.current(true); }, []);

  const { subscribe } = useWS();
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    const unsub = subscribe((msg) => {
      if (msg.type === "task_update" || msg.type === "task_created" || msg.type === "task_deleted") {
        if (timer) clearTimeout(timer);
        timer = setTimeout(() => { loadTasksRef.current(true); }, 500);
      }
    });
    return () => {
      if (timer) clearTimeout(timer);
      unsub();
    };
  }, [subscribe]);

  useVisibleInterval(pollTasks, 30000);

  const handleExportCSV = () => {
    const headers = ["Time", "Agent", "Type", "Command", "Status", "Result", "Duration"];
    const rows = tasks.map((t) => [
      t.created_at || "",
      getAgentName(t.agent_id) || t.agent_id || String(t.id).substring(0, 8) || "",
      t.type || "",
      `"${(t.command || "").replace(/"/g, '""')}"`,
      t.status || "",
      `"${(t.result || "").replace(/"/g, '""').replace(/\n/g, " ")}"`,
      calcDuration(t.created_at, t.updated_at),
    ]);
    const csv = [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");
    downloadText(csv, `tasks_export_${Date.now()}.csv`, "text/csv");
  };

  const toggleRow = useCallback((id: number) => {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const handleCancel = useCallback(async (task: Task) => {
    try {
      await api.post(paths.agents.cancelTask(task.agent_id, task.id));
      loadTasks();
    } catch { toast.error(t("tasks.toast_cancel_failed")); }
  }, [loadTasks, t]);

  const handleRerun = useCallback(async (task: Task) => {
    try {
      await api.post(paths.agents.rerunTask(task.agent_id, task.id));
      loadTasks();
    } catch { toast.error(t("tasks.toast_rerun_failed")); }
  }, [loadTasks, t]);

  const handleApprove = useCallback(async (task: Task) => {
    try {
      await api.post(paths.tasksCollab.approve(task.id));
      loadTasks();
    } catch { toast.error(t("tasks.toast_approve_failed")); }
  }, [loadTasks, t]);

  const handleReject = useCallback(async (task: Task) => {
    try {
      await api.post(paths.tasksCollab.reject(task.id));
      loadTasks();
    } catch { toast.error(t("tasks.toast_reject_failed")); }
  }, [loadTasks, t]);

  const handleClaim = useCallback(async (taskId: number) => {
    try {
      await api.post(paths.tasksCollab.claim(taskId));
      loadTasks();
    } catch { toast.error(t("tasks.toast_claim_failed")); }
  }, [loadTasks, t]);

  const handleRelease = useCallback(async (taskId: number) => {
    try {
      await api.post(paths.tasksCollab.release(taskId));
      loadTasks();
    } catch { toast.error(t("tasks.toast_release_failed")); }
  }, [loadTasks, t]);

  const TASK_ROW_H = 52;
  const {
    scrollRef,
    onScroll,
    virtualized,
    start: virtStart,
    end: virtEnd,
    offsetTop,
    totalHeight,
  } = useVirtualWindow({ count: tasks.length, rowHeight: TASK_ROW_H, threshold: 25 });

  const sortedTasks = useMemo(() => {
    const list = [...tasks];
    const dir = sortDir === "asc" ? 1 : -1;
    list.sort((a, b) => {
      if (sortKey === "created_at") {
        const at = a.created_at ? new Date(a.created_at).getTime() : 0;
        const bt = b.created_at ? new Date(b.created_at).getTime() : 0;
        return (at - bt) * dir;
      }
      const av = String(a[sortKey as keyof Task] || "");
      const bv = String(b[sortKey as keyof Task] || "");
      return av.localeCompare(bv) * dir;
    });
    return list;
  }, [tasks, sortKey, sortDir]);

  const visibleTasks = useMemo(
    () => (virtualized ? sortedTasks.slice(virtStart, virtEnd) : sortedTasks),
    [sortedTasks, virtualized, virtStart, virtEnd],
  );

  const getStatusBadge = useCallback((status: string): React.ReactNode => {
    return <StatusBadge status={status} />;
  }, []);

  const getTypeBadge = useCallback((type: string): React.ReactNode => {
    const variantMap: Record<string, "warning" | "info" | "destructive" | "secondary"> = {
      shell: "warning",
      screenshot: "info",
      kill: "destructive",
    };
    return <Badge variant={variantMap[type] || "secondary"}>{type}</Badge>;
  }, []);

  return (
    <PageContainer embedded={embedded} title={!embedded ? t("tasks.title") : undefined} subtitle={!embedded ? `${t("tasks.subtitle_prefix")} \u00b7 ${total} total` : undefined} actions={<>
        <Button onClick={handleExportCSV} variant="secondary">
          <FileSpreadsheet className="w-4 h-4" /> {t("tasks.export_csv")}
        </Button>
      </>}>
      {embedded && (
        <div className="mb-4 flex justify-end">
          <Button onClick={handleExportCSV} variant="secondary" size="sm">
            <FileSpreadsheet className="w-4 h-4" /> {t("tasks.export_csv")}
          </Button>
        </div>
      )}

      <Card className="p-4 sm:p-5 mb-4">
        <div className="flex flex-col sm:flex-row gap-3">
          <Select value={statusFilter || "all"} onValueChange={(val) => { setStatusFilter(val === "all" ? "" : val ?? ""); setPage(1); }}>
            <SelectTrigger aria-label={t("tasks.status_filter")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tasks.all_status")}</SelectItem>
              <SelectItem value="pending_approval">{t("tasks.approval_pending")}</SelectItem>
              <SelectItem value="completed">{t("tasks.completed")}</SelectItem>
              <SelectItem value="pending">{t("tasks.pending")}</SelectItem>
              <SelectItem value="failed">{t("tasks.failed")}</SelectItem>
              <SelectItem value="cancelled">{t("tasks.cancelled")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={agentFilter || "all"} onValueChange={(val) => { setAgentFilter(val === "all" ? "" : val ?? ""); setPage(1); }}>
            <SelectTrigger aria-label={t("tasks.agent_filter")} className="min-w-[150px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tasks.all_agents")}</SelectItem>
              {(agents || []).filter((a) => a.id).map((a) => (
                <SelectItem key={a.id} value={a.id!}>{a.hostname || a.id!.substring(0, 8)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={typeFilter || "all"} onValueChange={(val) => { setTypeFilter(val === "all" ? "" : val ?? ""); setPage(1); }}>
            <SelectTrigger aria-label={t("tasks.type_filter")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tasks.all_types")}</SelectItem>
              <SelectItem value="shell">shell</SelectItem>
              <SelectItem value="screenshot">screenshot</SelectItem>
              <SelectItem value="file">file</SelectItem>
              <SelectItem value="download">download</SelectItem>
              <SelectItem value="upload">upload</SelectItem>
              <SelectItem value="ps">ps</SelectItem>
              <SelectItem value="creds">creds</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </Card>

      <Card>
        <DataState
          loading={loading}
          error={error}
          empty={tasks.length === 0}
          emptyIcon={Inbox}
          emptyTitle={t("tasks.empty_title")}
          emptyMessage={t("tasks.empty_message")}
          onRetry={loadTasks}
          loadingSkeleton={
            <div className="overflow-x-auto p-4 space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-8 w-full" />
              ))}
            </div>
          }
        >
          <div
            ref={scrollRef}
            onScroll={onScroll}
            className={virtualized ? "overflow-auto max-h-[min(70vh,720px)]" : "overflow-x-auto"}
          >
          <Table>
            <TableHeader className="bg-muted/50 sticky top-0 z-10">
              <TableRow>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[140px] cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "created_at" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("created_at")} onKeyDown={handleSortKeyDown("created_at")}>
                  {t("tasks.col_time")} {sortIcon("created_at")}
                </TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[120px]">{t("tasks.col_agent")}</TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[100px] cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "type" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("type")} onKeyDown={handleSortKeyDown("type")}>
                  {t("tasks.col_type")} {sortIcon("type")}
                </TableHead>
                <TableHead className="text-left py-3 px-4 font-normal min-w-[180px] cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "command" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("command")} onKeyDown={handleSortKeyDown("command")}>
                  {t("tasks.col_command")} {sortIcon("command")}
                </TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[200px]">{t("tasks.col_result")}</TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[80px]">{t("tasks.col_claimed_by")}</TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[90px]">{t("tasks.col_duration")}</TableHead>
                <TableHead className="text-center py-3 px-4 font-normal min-w-[90px] cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === "status" ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort("status")} onKeyDown={handleSortKeyDown("status")}>
                  {t("tasks.col_status")} {sortIcon("status")}
                </TableHead>
                <TableHead className="text-center py-3 px-4 font-normal min-w-[160px]">{t("tasks.col_actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-border">
              {virtualized && offsetTop > 0 && (
                <TableRow aria-hidden className="hover:bg-transparent">
                  <TableCell colSpan={9} style={{ height: offsetTop, padding: 0, border: 0 }} />
                </TableRow>
              )}
              {visibleTasks.map((task) => (
                <TaskRow key={task.id} task={task} expanded={expandedRows.has(task.id)} onToggle={toggleRow} onDetail={setDetailTask} onCancel={handleCancel} onRerun={handleRerun} onApprove={handleApprove} onReject={handleReject} onClaim={handleClaim} onRelease={handleRelease} getAgentName={getAgentName} getTypeBadge={getTypeBadge} getStatusBadge={getStatusBadge} />
              ))}
              {virtualized && totalHeight - offsetTop - visibleTasks.length * TASK_ROW_H > 0 && (
                <TableRow aria-hidden className="hover:bg-transparent">
                  <TableCell colSpan={9} style={{ height: totalHeight - offsetTop - visibleTasks.length * TASK_ROW_H, padding: 0, border: 0 }} />
                </TableRow>
              )}
            </TableBody>
          </Table>
          </div>
        </DataState>
      </Card>

      <Pagination page={page} pageSize={50} total={total} onPageChange={setPage} />

      {detailTask && <TaskDetailModal task={detailTask} onClose={() => setDetailTask(null)} getAgentName={getAgentName} getStatusBadge={getStatusBadge} getTypeBadge={getTypeBadge} />}
    </PageContainer>
  );
}

export default function TasksPageWrapper({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  return (
    <Suspense fallback={<div className="flex items-center justify-center py-20 text-muted-foreground"><Spinner size="sm" /> {t("tasks.loading")}</div>}>
      <TasksPage embedded={embedded} />
    </Suspense>
  );
}

const TaskRow = memo(function TaskRow({ task, expanded, onToggle, onDetail, onCancel, onRerun, onApprove, onReject, onClaim, onRelease, getAgentName, getTypeBadge, getStatusBadge }: {
  task: Task;
  expanded: boolean;
  onToggle: (id: number) => void;
  onDetail: (task: Task) => void;
  onCancel: (task: Task) => void;
  onRerun: (task: Task) => void;
  onApprove: (task: Task) => void;
  onReject: (task: Task) => void;
  onClaim: (taskId: number) => void;
  onRelease: (taskId: number) => void;
  getAgentName: (id: string) => string | undefined;
  getTypeBadge: (t: string) => React.ReactNode;
  getStatusBadge: (s: string) => React.ReactNode;
}) {
  const currentUsername = useAppStore((s) => s.currentUsername);
  const { t } = useI18n();
  return (
    <>
      <TableRow className="hover:bg-muted/50 transition-colors cursor-pointer" onClick={() => onDetail(task)}>
        <TableCell className="max-sm:hidden py-3 px-4 font-mono text-xs text-muted-foreground whitespace-nowrap">{task.created_at ? formatTime(task.created_at) : "-"}</TableCell>
        <TableCell className="max-sm:hidden py-3 px-4"><div className="font-medium text-foreground text-sm">{getAgentName(task.agent_id) || task.agent_id?.substring(0, 8)}</div></TableCell>
        <TableCell className="max-sm:hidden py-3 px-4">{getTypeBadge(task.type)}</TableCell>
        <TableCell className="py-3 px-4 font-mono text-xs text-muted-foreground max-w-xs truncate">
          <Button
            variant="link"
            size="sm"
            onClick={(e) => { e.stopPropagation(); onDetail(task); }}
            className="font-mono text-xs text-muted-foreground hover:text-foreground p-0 h-auto justify-start max-w-full truncate"
          >
            {task.command || "-"}
          </Button>
        </TableCell>
        <TableCell className="max-sm:hidden py-3 px-4 max-w-sm">
          {task.result ? (
            <Button
              variant="link"
              size="sm"
              onClick={(e) => { e.stopPropagation(); onToggle(task.id); }}
              aria-expanded={expanded}
              aria-label={expanded ? t("tasks.collapse") : t("tasks.expand")}
              className="text-xs text-primary hover:underline flex items-center gap-1 p-0 h-auto justify-start"
            >
              {expanded ? <ChevronUp className="w-2.5 h-2.5" /> : <ChevronDown className="w-2.5 h-2.5" />} {t("tasks.expand")}
            </Button>
          ) : "-"}
        </TableCell>
        <TableCell className="max-sm:hidden py-3 px-4">
          {task.claimed_by ? (
            <OperatorBadge username={task.claimed_by} isCurrentUser={task.claimed_by === currentUsername} size="sm" />
          ) : (
            <span className="text-muted-foreground text-xs">-</span>
          )}
        </TableCell>
        <TableCell className="max-sm:hidden py-3 px-4 text-xs text-muted-foreground font-mono">{calcDuration(task.created_at, task.updated_at)}</TableCell>
        <TableCell className="py-3 px-4 text-center">{getStatusBadge(task.status)}</TableCell>
        <TableCell className="py-3 px-4 text-center" onClick={(e) => e.stopPropagation()}>
          <div className="flex items-center justify-center gap-1">
            {(!task.claimed_by || task.claimed_by === currentUsername) && (task.status === "pending" || task.status === "running") && (
              <>
                {task.claimed_by !== currentUsername ? (
                   <Button variant="ghost" size="icon-xs" onClick={() => onClaim(task.id)} className="text-muted-foreground hover:text-primary hover:bg-primary/10 dark:hover:bg-chart-3/20" title={t("tasks.claim")} aria-label={t("tasks.claim")}>
                     <Hand className="w-4 h-4" />
                   </Button>
                 ) : (
                   <Button variant="ghost" size="icon-xs" onClick={() => onRelease(task.id)} className="text-muted-foreground hover:text-warning hover:bg-warning/15" title={t("tasks.release")} aria-label={t("tasks.release")}>
                    <Hand className="w-4 h-4" />
                  </Button>
                )}
              </>
            )}
            {(task.status === "pending" || task.status === "running") && !task.claimed_by && (
               <Button variant="ghost" size="icon-xs" onClick={() => onCancel(task)} className="text-muted-foreground hover:text-destructive hover:bg-destructive/10" title={t("tasks.cancel")} aria-label={t("tasks.cancel")}>
                <Ban className="w-3 h-3" />
              </Button>
            )}
            {(task.status === "completed" || task.status === "failed" || task.status === "cancelled") && (
               <Button variant="ghost" size="icon-xs" onClick={() => onRerun(task)} className="text-muted-foreground hover:text-primary hover:bg-primary/10 dark:hover:bg-chart-3/20" title={t("tasks.rerun")} aria-label={t("tasks.rerun")}>
                <RotateCw className="w-4 h-4" />
              </Button>
            )}
            {task.status === "pending_approval" && (
              <>
                 <Button variant="ghost" size="icon-xs" onClick={() => onApprove(task)} className="text-muted-foreground hover:text-success hover:bg-success/15" title={t("tasks.approve")} aria-label={t("tasks.approve")}>
                  <Check className="w-4 h-4" />
                </Button>
                 <Button variant="ghost" size="icon-xs" onClick={() => onReject(task)} className="text-muted-foreground hover:text-destructive hover:bg-destructive/10" title={t("tasks.reject")} aria-label={t("tasks.reject")}>
                  <X className="w-4 h-4" />
                </Button>
              </>
            )}
          </div>
        </TableCell>
      </TableRow>
      {expanded && task.result && (
        <TableRow>
          <TableCell colSpan={9} className="px-4 py-3 bg-card">
            <div className="relative">
               <Button variant="ghost" size="xs" onClick={() => onDetail(task)} className="absolute top-2 right-2 text-xs text-muted-foreground hover:text-foreground bg-secondary px-2 py-1 rounded">
                <Maximize2 className="w-4 h-4" />{t("tasks.full_view")}
              </Button>
              <pre className="text-xs text-success font-mono overflow-x-auto max-h-60 p-2 whitespace-pre-wrap break-all">{task.result}</pre>
            </div>
          </TableCell>
        </TableRow>
      )}
    </>
  );
});

function TaskDetailModal({ task, onClose, getAgentName, getStatusBadge, getTypeBadge }: {
  task: Task;
  onClose: () => void;
  getAgentName: (id: string) => string | undefined;
  getStatusBadge: (s: string) => React.ReactNode;
  getTypeBadge: (t: string) => React.ReactNode;
}) {
  const { t } = useI18n();
  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="sm:max-w-3xl max-h-[85vh] flex flex-col">
        <DialogHeader>
          <div className="flex items-center gap-3">
            {getTypeBadge(task.type)}
            {getStatusBadge(task.status)}
            <span className="text-xs font-mono text-muted-foreground">{String(task.id).substring(0, 12)}</span>
          </div>
        </DialogHeader>
        <div className="border-b border-border grid grid-cols-2 sm:grid-cols-4 gap-4 text-xs">
          <div><span className="text-muted-foreground">{t("tasks.detail_agent")}</span><p className="font-medium text-foreground mt-0.5">{getAgentName(task.agent_id) || task.agent_id?.substring(0, 8)}</p></div>
          <div><span className="text-muted-foreground">{t("tasks.detail_created")}</span><p className="font-medium text-foreground mt-0.5">{task.created_at ? formatTime(task.created_at) : "-"}</p></div>
          <div><span className="text-muted-foreground">{t("tasks.detail_duration")}</span><p className="font-medium text-foreground mt-0.5">{calcDuration(task.created_at, task.updated_at)}</p></div>
          <div><span className="text-muted-foreground">{t("tasks.detail_by")}</span><p className="font-medium text-foreground mt-0.5">{task.created_by || t("tasks.detail_system")}</p></div>
          {task.approved_by && (
            <div><span className="text-muted-foreground">{t("tasks.detail_approved_by")}</span><p className="font-medium text-success mt-0.5">{task.approved_by}</p></div>
          )}
        </div>
        <div className="overflow-y-auto flex-1">
          <div className="mb-4">
            <h3 className="text-xs font-semibold text-muted-foreground mb-2 uppercase tracking-wider">{t("tasks.detail_command")}</h3>
            <code className="block bg-muted border border-border rounded-lg p-3 text-sm font-mono text-foreground">{task.command || "-"}</code>
          </div>
          {task.result && (
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground mb-2 uppercase tracking-wider">{t("tasks.detail_output")}</h3>
              <pre className="bg-card text-success font-mono text-xs rounded-lg p-4 overflow-x-auto whitespace-pre-wrap break-all max-h-96">{task.result}</pre>
            </div>
          )}
          {task.error && (
            <div className="mt-4">
              <ErrorState title={t("tasks.detail_error")} message={<pre className="text-xs font-mono whitespace-pre-wrap">{task.error}</pre>} />
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function calcDuration(created?: string, updated?: string): string {
  if (!created) return "-";
  const start = new Date(created).getTime();
  if (isNaN(start)) return "-";
  const end = updated ? new Date(updated).getTime() : Date.now();
  if (isNaN(end)) return "-";
  const ms = end - start;
  if (ms < 0) return "-";
  if (ms < 1000) return `${ms}ms`;
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  const remSec = sec % 60;
  if (min < 60) return `${min}m ${remSec}s`;
  const hr = Math.floor(min / 60);
  return `${hr}h ${min % 60}m`;
}

