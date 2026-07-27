"use client";

import { Suspense, useState, useCallback, useEffect, memo, useMemo } from "react";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { downloadText } from "@/lib/download";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { PageHeader, Pagination, Spinner, StatusBadge } from "@/components/UI";
import { DataState } from "@/components/ui/data-state";
import OperatorBadge from "@/components/OperatorBadge";
import { useAppStore } from "@/lib/store";
import type { Agent } from "@/types/agent";
import { normalizeAgentList } from "@/lib/agents";
import { formatTime } from "@/lib/utils";
import { useVirtualWindow } from "@/lib/hooks/useVirtualWindow";
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
import { Ban, ChevronDown, ChevronUp, FileSpreadsheet, Hand, Inbox, Maximize2, RotateCw } from "lucide-react";
import type { Task } from "@/types/task";

function TasksPage() {
  const { t } = useI18n();
  const searchParams = useSearchParams();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState("");
  const [agentFilter, setAgentFilter] = useState(searchParams.get("agent_id") || "");
  const [typeFilter, setTypeFilter] = useState("");
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());
  const [detailTask, setDetailTask] = useState<Task | null>(null);

  const getAgentName = useCallback((agentId: string) => {
    return agents.find((a) => a.id === agentId)?.hostname;
  }, [agents]);

  const loadAgents = useCallback(async () => {
    try {
      const data = await api.get("/agents?page=1&pageSize=200");
      setAgents(normalizeAgentList(data));
    } catch { toast.error(t("tasks.toast_load_agents_failed")); }
  }, [t]);

  const loadTasks = useCallback(() => {
    setLoading(true);
    setError(null);
    const params = new URLSearchParams({ page: String(page), pageSize: "50" });
    if (statusFilter) params.set("status", statusFilter);
    if (agentFilter) params.set("agentId", agentFilter);
    if (typeFilter) params.set("type", typeFilter);
    api.get(`/tasks?${params}`)
      .then((data: Record<string, unknown>) => {
        setTasks((data.tasks || data || []) as Task[]);
        setTotal(Number(data.Total) || Number(data.total) || 0);
      })
      .catch((e) => {
        setTasks([]);
        setTotal(0);
        const msg = e instanceof Error ? e.message : t("tasks.toast_load_failed");
        setError(msg);
        toast.error(msg);
      })
      .finally(() => setLoading(false));
  }, [page, statusFilter, agentFilter, typeFilter, t]);

  useEffect(() => { loadAgents(); }, [loadAgents]);
  useEffect(() => { loadTasks(); }, [loadTasks]);

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

  const toggleRow = (id: number) => {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleCancel = async (task: Task) => {
    try {
      await api.post(`/agents/${task.agent_id}/tasks/${task.id}/cancel`);
      loadTasks();
    } catch { toast.error(t("tasks.toast_cancel_failed")); }
  };

  const handleRerun = async (task: Task) => {
    try {
      await api.post(`/agents/${task.agent_id}/task/${task.id}/rerun`);
      loadTasks();
    } catch { toast.error(t("tasks.toast_rerun_failed")); }
  };

  const handleClaim = async (taskId: number) => {
    try {
      await api.post(`/collab/tasks/${taskId}/claim`);
      loadTasks();
    } catch { toast.error(t("tasks.toast_claim_failed")); }
  };

  const handleRelease = async (taskId: number) => {
    try {
      await api.post(`/collab/tasks/${taskId}/release`);
      loadTasks();
    } catch { toast.error(t("tasks.toast_release_failed")); }
  };

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

  const visibleTasks = useMemo(
    () => (virtualized ? tasks.slice(virtStart, virtEnd) : tasks),
    [tasks, virtualized, virtStart, virtEnd],
  );

  const getStatusBadge = (status: string): React.ReactNode => {
    return <StatusBadge status={status} />;
  };

  const getTypeBadge = (type: string): React.ReactNode => {
    const colorMap: Record<string, string> = {
      shell: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
      screenshot: "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400",
      kill: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
    };
    const color = colorMap[type] || "bg-secondary text-muted-foreground";
    return <span className={`inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-lg ${color}`}>{type}</span>;
  };

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("tasks.title")} subtitle={`${t("tasks.subtitle_prefix")} \u00b7 ${total} total`}>
        <Button onClick={handleExportCSV} variant="secondary">
          <FileSpreadsheet className="w-4 h-4" /> {t("tasks.export_csv")}
        </Button>
      </PageHeader>

      <Card className="p-4 sm:p-5 mb-4">
        <div className="flex flex-col sm:flex-row gap-3">
          <Select value={statusFilter || "all"} onValueChange={(val) => { setStatusFilter(val === "all" ? "" : val ?? ""); setPage(1); }}>
            <SelectTrigger aria-label="Status filter">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tasks.all_status")}</SelectItem>
              <SelectItem value="completed">{t("tasks.completed")}</SelectItem>
              <SelectItem value="pending">{t("tasks.pending")}</SelectItem>
              <SelectItem value="failed">{t("tasks.failed")}</SelectItem>
              <SelectItem value="cancelled">{t("tasks.cancelled")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={agentFilter || "all"} onValueChange={(val) => { setAgentFilter(val === "all" ? "" : val ?? ""); setPage(1); }}>
            <SelectTrigger aria-label="Agent filter" className="min-w-[150px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tasks.all_agents")}</SelectItem>
              {agents.filter((a) => a.id).map((a) => (
                <SelectItem key={a.id} value={a.id!}>{a.hostname || a.id!.substring(0, 8)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={typeFilter || "all"} onValueChange={(val) => { setTypeFilter(val === "all" ? "" : val ?? ""); setPage(1); }}>
            <SelectTrigger aria-label="Type filter">
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

      <Card className="sm:rounded-2xl">
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
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[140px]">{t("tasks.col_time")}</TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[120px]">{t("tasks.col_agent")}</TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[100px]">{t("tasks.col_type")}</TableHead>
                <TableHead className="text-left py-3 px-4 font-normal min-w-[180px]">{t("tasks.col_command")}</TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[200px]">{t("tasks.col_result")}</TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[80px]">{t("tasks.col_claimed_by")}</TableHead>
                <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal min-w-[90px]">{t("tasks.col_duration")}</TableHead>
                <TableHead className="text-center py-3 px-4 font-normal min-w-[90px]">{t("tasks.col_status")}</TableHead>
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
                <TaskRow key={task.id} task={task} expanded={expandedRows.has(task.id)} onToggle={() => toggleRow(task.id)} onDetail={() => setDetailTask(task)} onCancel={() => handleCancel(task)} onRerun={() => handleRerun(task)} onClaim={() => handleClaim(task.id)} onRelease={() => handleRelease(task.id)} getAgentName={getAgentName} getTypeBadge={getTypeBadge} getStatusBadge={getStatusBadge} />
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
    </div>
  );
}

export default function TasksPageWrapper() {
  const { t } = useI18n();
  return (
    <Suspense fallback={<div className="flex items-center justify-center py-20 text-muted-foreground"><Spinner size="sm" /> {t("tasks.loading")}</div>}>
      <TasksPage />
    </Suspense>
  );
}

const TaskRow = memo(function TaskRow({ task, expanded, onToggle, onDetail, onCancel, onRerun, onClaim, onRelease, getAgentName, getTypeBadge, getStatusBadge }: {
  task: Task;
  expanded: boolean;
  onToggle: () => void;
  onDetail: () => void;
  onCancel: () => void;
  onRerun: () => void;
  onClaim: () => void;
  onRelease: () => void;
  getAgentName: (id: string) => string | undefined;
  getTypeBadge: (t: string) => React.ReactNode;
  getStatusBadge: (s: string) => React.ReactNode;
}) {
  const { currentUsername } = useAppStore();
  const { t } = useI18n();
  return (
    <>
      <TableRow className="hover:bg-muted/50 transition-colors cursor-pointer" onClick={onDetail}
        tabIndex={0} role="button"
        onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onDetail(); } }}>
        <TableCell className="max-sm:hidden py-3 px-4 font-mono text-xs text-muted-foreground whitespace-nowrap">{task.created_at ? formatTime(task.created_at) : "-"}</TableCell>
        <TableCell className="max-sm:hidden py-3 px-4"><div className="font-medium text-foreground text-sm">{getAgentName(task.agent_id) || task.agent_id?.substring(0, 8)}</div></TableCell>
        <TableCell className="max-sm:hidden py-3 px-4">{getTypeBadge(task.type)}</TableCell>
        <TableCell className="py-3 px-4 font-mono text-xs text-muted-foreground max-w-xs truncate">{task.command || "-"}</TableCell>
        <TableCell className="max-sm:hidden py-3 px-4 max-w-sm" onClick={(e) => { e.stopPropagation(); onToggle(); }}>
          {task.result ? (
            <div>
              <span className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline flex items-center gap-1 cursor-pointer">
                {expanded ? <ChevronUp className="w-2.5 h-2.5" /> : <ChevronDown className="w-2.5 h-2.5" />} {t("tasks.expand")}
              </span>
            </div>
          ) : "-"}
        </TableCell>
        <TableCell className="max-sm:hidden py-3 px-4" data-label="Claimed By">
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
                  <Button variant="ghost" size="icon-xs" onClick={onClaim} className="text-muted-foreground hover:text-indigo-500 hover:bg-indigo-50 dark:hover:bg-indigo-900/20" title={t("tasks.claim")} aria-label={t("tasks.claim")}>
                    <Hand className="w-4 h-4" />
                  </Button>
                ) : (
                  <Button variant="ghost" size="icon-xs" onClick={onRelease} className="text-muted-foreground hover:text-amber-500 hover:bg-amber-50 dark:hover:bg-amber-900/20" title={t("tasks.release")} aria-label={t("tasks.release")}>
                    <Hand className="w-4 h-4" />
                  </Button>
                )}
              </>
            )}
            {(task.status === "pending" || task.status === "running") && !task.claimed_by && (
              <Button variant="ghost" size="icon-xs" onClick={onCancel} className="text-muted-foreground hover:text-destructive hover:bg-destructive/10" title={t("tasks.cancel")} aria-label={t("tasks.cancel")}>
                <Ban className="w-3 h-3" />
              </Button>
            )}
            {(task.status === "completed" || task.status === "failed" || task.status === "cancelled") && (
              <Button variant="ghost" size="icon-xs" onClick={onRerun} className="text-muted-foreground hover:text-indigo-500 hover:bg-indigo-50 dark:hover:bg-indigo-900/20" title={t("tasks.rerun")} aria-label={t("tasks.rerun")}>
                <RotateCw className="w-4 h-4" />
              </Button>
            )}
          </div>
        </TableCell>
      </TableRow>
      {expanded && task.result && (
        <TableRow>
          <TableCell colSpan={9} className="px-4 py-3 bg-card">
            <div className="relative">
              <Button variant="ghost" size="xs" onClick={onDetail} className="absolute top-2 right-2 text-xs text-muted-foreground hover:text-foreground bg-secondary px-2 py-1 rounded">
                <Maximize2 className="w-4 h-4" />{t("tasks.full_view")}
              </Button>
              <pre className="text-xs text-emerald-700 dark:text-emerald-300 font-mono overflow-x-auto max-h-60 p-2 whitespace-pre-wrap break-all">{task.result}</pre>
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
        </div>
        <div className="overflow-y-auto flex-1">
          <div className="mb-4">
            <h3 className="text-xs font-semibold text-muted-foreground mb-2 uppercase tracking-wider">{t("tasks.detail_command")}</h3>
            <code className="block bg-muted border border-border rounded-xl p-3 text-sm font-mono text-foreground">{task.command || "-"}</code>
          </div>
          {task.result && (
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground mb-2 uppercase tracking-wider">{t("tasks.detail_output")}</h3>
              <pre className="bg-card text-emerald-700 dark:text-emerald-300 font-mono text-xs rounded-xl p-4 overflow-x-auto whitespace-pre-wrap break-all max-h-96">{task.result}</pre>
            </div>
          )}
          {task.error && (
            <div className="mt-4">
              <h3 className="text-xs font-semibold text-muted-foreground mb-2 uppercase tracking-wider">{t("tasks.detail_error")}</h3>
              <pre className="bg-red-900/20 text-red-400 font-mono text-xs rounded-xl p-4 overflow-x-auto whitespace-pre-wrap">{task.error}</pre>
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

