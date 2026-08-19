"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { firstArray } from "@/lib/envelope";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { useWS } from "@/lib/wsContext";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { StatusBadge } from "@/components/ui/status-indicator";
import { Spinner } from "@/components/ui/spinner";
import { GitCompare, Maximize2, Pin, PinOff, X } from "lucide-react";
import type { Beacon } from "./types";
import type { AgentTaskRecord } from "@/types/agent";
import { AgentDockFiles } from "./AgentDockFiles";
import { AgentDockCommands } from "./AgentDockCommands";
import { AgentDockShot } from "./AgentDockShot";
import { shouldRefreshDockShot } from "./dock-shot";
import { applyTaskEvent, canApproveOwnTask, canCancelTask, canReviewTask, isDockTaskEvent, shouldRevealTaskResult, taskEventId } from "./dock-tasks";
import { diffChangeLines, diffResults, previousComparableTask, resultLooksComparable } from "./result-diff";
import { useAppStore } from "@/lib/store";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { POLL } from "@/lib/polling";
import { clampDockHeight, type InteractTab } from "@/lib/interact-storage";
import { useInteractStore } from "@/lib/interact-store";

const ShellTerminal = dynamic(() => import("@/components/ShellTerminal"), {
  ssr: false,
  loading: () => (
    <div className="flex h-full items-center justify-center">
      <Spinner />
    </div>
  ),
});

interface AgentInteractDockProps {
  beacon: Beacon;
  height: number;
  pinned: boolean;
  tab: InteractTab;
  onTabChange: (tab: InteractTab) => void;
  onHeightChange: (height: number) => void;
  onTogglePin: () => void;
  onClose: () => void;
  onOpenDetails: () => void;
}

export function AgentInteractDock({
  beacon,
  height,
  pinned,
  tab,
  onTabChange,
  onHeightChange,
  onTogglePin,
  onClose,
  onOpenDetails,
}: AgentInteractDockProps) {
  const { t } = useI18n();
  const id = beacon.id || "";
  const os = (beacon.os || "").toLowerCase();
  const osType = os.includes("linux") || os.includes("darwin") ? "linux" : "windows";
  const [tasks, setTasks] = useState<AgentTaskRecord[]>([]);
  const [tasksLoading, setTasksLoading] = useState(false);
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [shotKey, setShotKey] = useState(0);
  const [diffOpen, setDiffOpen] = useState(false);
  const revealId = useInteractStore((s) => s.expandedTaskId);
  const { subscribe, connected } = useWS();
  const currentUsername = useAppStore((s) => s.currentUsername);

  const loadTasks = useCallback(async (silent = false) => {
    if (!id) return;
    if (!silent) setTasksLoading(true);
    try {
      const data = await api.get(paths.agents.tasks(id));
      const raw = firstArray(data, ["tasks", "data", "Tasks"]);
      setTasks(raw as AgentTaskRecord[]);
    } catch {
      if (!silent) setTasks([]);
    } finally {
      if (!silent) setTasksLoading(false);
    }
  }, [id]);

  // Load the task list as soon as the dock mounts (or the agent changes);
  // WS only pushes events after that, so without this an idle agent would
  // show an empty list until something new happens.
  useEffect(() => {
    void loadTasks(true);
  }, [loadTasks]);

  useEffect(() => {
    if (!revealId) return;
    // Reveal is handled incrementally by the WS applyTaskEvent handler below;
    // a full reload here was redundant (and re-rendered the whole dock).
    setExpandedId(revealId);
    setDiffOpen(false);
    onTabChange("tasks");
  }, [revealId, onTabChange]);

  useEffect(() => {
    if (!id) return;
    return subscribe((msg) => {
      if (!isDockTaskEvent(msg, id)) return;
      setTasks((prev) => applyTaskEvent(prev, msg));
      if (shouldRefreshDockShot(msg)) setShotKey((n) => n + 1);
      if (shouldRevealTaskResult(msg)) {
        // Completion reveals the result in place — highlight the task but
        // don't steal focus from the tab the operator is actually using
        // (tab switching is reserved for freshly-queued pending tasks).
        const tid = taskEventId(msg);
        if (tid) setExpandedId(tid);
      }
    });
  }, [id, subscribe]);

  // While the socket is down, poll the task list in the background (pauses
  // when the tab is hidden; standalone fetches are gated by the guard).
  useVisibleInterval(() => {
    if (id && !connected) void loadTasks(true);
  }, POLL.wsDownPoll);

  const reviewTask = useCallback(async (taskId: number, action: "approve" | "reject") => {
    if (!id) return;
    try {
      await api.post(action === "approve" ? paths.tasksCollab.approve(taskId) : paths.tasksCollab.reject(taskId));
      setTasks((prev) => prev.map((t) => (Number(t.id) !== taskId ? t : {
        ...t,
        status: action === "approve" ? "pending" : "cancelled",
        error: action === "reject" ? (t.error || "rejected") : t.error,
      })));
      if (action === "approve") toast.success(t("agents.dock_task_approved"));
      else toast.success(t("agents.dock_task_rejected"));
    } catch {
      if (action === "approve") toast.error(t("agents.dock_task_approve_failed"));
      else toast.error(t("agents.dock_task_reject_failed"));
    }
  }, [id, t]);

  const cancelTask = useCallback(async (taskId: number) => {
    if (!id) return;
    try {
      await api.post(paths.agents.cancelTask(id, taskId));
      setTasks((prev) => prev.map((t) => (Number(t.id) !== taskId ? t : { ...t, status: "cancelled", error: t.error || "cancelled" })));
      toast.success(t("agents.dock_task_cancelled"));
    } catch {
      toast.error(t("agents.dock_task_cancel_failed"));
    }
  }, [id, t]);

  const openTask = useCallback(async (taskId: number) => {
    if (expandedId === taskId) {
      setExpandedId(null);
      setDiffOpen(false);
      return;
    }
    setExpandedId(taskId);
    setDiffOpen(false);
    try {
      const data = await api.get(paths.agents.task(id, taskId)) as AgentTaskRecord;
      if (!data || typeof data !== "object") return;
      setTasks((prev) => prev.map((t) => (Number(t.id) !== taskId ? t : {
        ...t,
        type: data.type || t.type,
        command: data.command || t.command,
        status: data.status || t.status,
        result: data.result ?? t.result,
        error: data.error ?? t.error,
        created_by: data.created_by || t.created_by,
      })));
    } catch {
      /* keep preview from the list / WS */
    }
  }, [expandedId, id]);

  const expandedTask = useMemo(
    () => (expandedId == null ? undefined : tasks.find((t) => Number(t.id) === expandedId)),
    [expandedId, tasks],
  );
  const previousResult = useMemo(
    () => (expandedTask ? previousComparableTask(tasks, expandedTask) : null),
    [expandedTask, tasks],
  );
  const resultDiff = useMemo(() => {
    if (!diffOpen || !expandedTask || !previousResult) return null;
    if (!resultLooksComparable(expandedTask.result) || !resultLooksComparable(previousResult.result)) return null;
    return diffResults(previousResult.result || "", expandedTask.result || "");
  }, [diffOpen, expandedTask, previousResult]);

  const onResizePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const startY = e.clientY;
    const startH = height;
    const handle = e.currentTarget;
    handle.setPointerCapture(e.pointerId);
    const onMove = (ev: PointerEvent) => {
      onHeightChange(clampDockHeight(startH + (startY - ev.clientY)));
    };
    const onUp = () => {
      handle.releasePointerCapture(e.pointerId);
      handle.removeEventListener("pointermove", onMove);
      handle.removeEventListener("pointerup", onUp);
    };
    handle.addEventListener("pointermove", onMove);
    handle.addEventListener("pointerup", onUp);
  };

  return (
    <div
      className="relative z-20 flex shrink-0 flex-col border-t border-border bg-card shadow-(--shadow-dock-up)"
      style={{ height }}
      role="region"
      aria-label={t("agents.interact")}
    >
      <div
        role="separator"
        aria-orientation="horizontal"
        aria-label={t("agents.dock_resize")}
        tabIndex={0}
        onPointerDown={onResizePointerDown}
        onKeyDown={(e) => {
          if (e.key === "ArrowUp") { e.preventDefault(); onHeightChange(clampDockHeight(height + 24)); }
          if (e.key === "ArrowDown") { e.preventDefault(); onHeightChange(clampDockHeight(height - 24)); }
        }}
        className="absolute inset-x-0 top-0 z-10 h-2 cursor-ns-resize hover:bg-primary/20"
      />
      <div className="flex h-full flex-col pt-1">
        <div className="flex items-center gap-2 border-b border-border px-3 py-1.5">
          <StatusBadge status={beacon.status || "offline"} pulse={beacon.status === "online"} />
          <span className="truncate text-sm font-semibold">{beacon.hostname || id}</span>
          {beacon.username && (
            <span className="hidden truncate font-mono text-xs text-muted-foreground sm:inline">
              {beacon.username}
            </span>
          )}
          {beacon.ip && (
            <span className="hidden font-mono text-xs text-muted-foreground/70 md:inline">{beacon.ip}</span>
          )}
          <div className="ml-auto flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={onTogglePin}
              aria-label={pinned ? t("agents.dock_unpin") : t("agents.dock_pin")}
              aria-pressed={pinned}
            >
              {pinned ? <PinOff className="size-3.5" /> : <Pin className="size-3.5" />}
            </Button>
            <Button variant="ghost" size="xs" onClick={onOpenDetails} className="gap-1">
              <Maximize2 className="size-3.5" />
              {t("agents.dock_open_details")}
            </Button>
            <Button variant="ghost" size="icon-xs" onClick={onClose} aria-label={t("agents.dock_close")}>
              <X className="size-4" />
            </Button>
          </div>
        </div>
        {id && (
          <AgentDockCommands
            agentId={id}
            intervalHint={beacon.current_interval}
            jitterHint={beacon.current_jitter}
            onQueued={(queued) => {
              useInteractStore.getState().revealTask(id, queued.task_id);
              setTasks((prev) => applyTaskEvent(prev, {
                type: "task_created",
                task_id: queued.task_id,
                task_type: queued.type,
                command: queued.command,
                status: "pending",
                agent_id: id,
              }));
            }}
          />
        )}
        {id && <AgentDockShot agentId={id} refreshKey={shotKey} />}
        <Tabs value={tab} onValueChange={(v) => { if (v === "shell" || v === "files" || v === "tasks") onTabChange(v); }} className="flex min-h-0 flex-1 flex-col">
          <TabsList className="h-8 w-full justify-start rounded-none border-b bg-transparent px-2">
            <TabsTrigger value="shell" className="h-7 text-xs">{t("agents.dock_tab_shell")} <span className="ml-1 text-muted-foreground/60">1</span></TabsTrigger>
            <TabsTrigger value="files" className="h-7 text-xs">{t("agents.dock_tab_files")} <span className="ml-1 text-muted-foreground/60">2</span></TabsTrigger>
            <TabsTrigger value="tasks" className="h-7 text-xs">{t("agents.dock_tab_tasks")} <span className="ml-1 text-muted-foreground/60">3</span></TabsTrigger>
          </TabsList>
          <TabsContent value="shell" className="mt-0 min-h-0 flex-1 p-0">
            {id && (
              <ShellTerminal
                agentId={id}
                osType={osType}
                showHeader={false}
                className="flex h-full flex-col overflow-hidden bg-background text-foreground"
              />
            )}
          </TabsContent>
          <TabsContent value="files" className="mt-0 min-h-0 flex-1 p-0">
            {id && <AgentDockFiles agentId={id} osType={osType} />}
          </TabsContent>
          <TabsContent value="tasks" className="mt-0 min-h-0 flex-1 p-0">
            {tasksLoading ? (
              <div className="flex h-full items-center justify-center"><Spinner size="sm" /></div>
            ) : tasks.length === 0 ? (
              <p className="px-4 py-8 text-center text-sm text-muted-foreground">{t("agents.dock_no_tasks")}</p>
            ) : (
              <ScrollArea className="h-full">
                <ul className="divide-y divide-border">
                  {tasks.slice(0, 40).map((task) => {
                    const open = expandedId === Number(task.id);
                    return (
                      <li key={task.id}>
                        <button
                          type="button"
                          onClick={() => { void openTask(Number(task.id)); }}
                          className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs hover:bg-secondary/50"
                        >
                          <Badge variant="secondary" className="font-mono">{task.type}</Badge>
                          <StatusBadge status={task.status} />
                          <span className="min-w-0 flex-1 truncate font-mono text-muted-foreground">{task.command}</span>
                          {task.created_by && (
                            <span className="hidden shrink-0 font-mono text-muted-foreground/70 sm:inline">{task.created_by}</span>
                          )}
                        </button>
                        {open && (
                          <div className="space-y-2 border-t border-border bg-muted/20 px-3 py-2">
                            {task.created_by && (
                              <p className="text-(--fs-micro-sm) text-muted-foreground">{t("agents.dock_task_by", { user: task.created_by })}</p>
                            )}
                            {task.result ? (
                              <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all font-mono text-(--fs-xs-sm) text-foreground">{task.result}</pre>
                            ) : task.error ? (
                              <pre className="max-h-32 overflow-auto whitespace-pre-wrap break-all font-mono text-(--fs-xs-sm) text-destructive">{task.error}</pre>
                            ) : (
                              <p className="text-xs text-muted-foreground">{t("agents.dock_waiting_result")}</p>
                            )}
                            {resultLooksComparable(task.result) && previousResult && Number(previousResult.id) !== Number(task.id) && (
                              <div className="space-y-1">
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="xs"
                                  className="gap-1"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    setDiffOpen((v) => !v);
                                  }}
                                >
                                  <GitCompare className="size-3.5" />
                                  {t("agents.dock_diff_prev")}
                                </Button>
                                {diffOpen && resultDiff && (
                                  <div className="space-y-1">
                                    <p className="text-(--fs-micro-sm) text-muted-foreground">
                                      {t("agents.dock_diff_summary", {
                                        added: resultDiff.added,
                                        removed: resultDiff.removed,
                                        id: previousResult.id,
                                      })}
                                    </p>
                                    {resultDiff.mode === "unique" && (
                                      <p className="text-(--fs-micro-sm) text-muted-foreground">{t("agents.dock_diff_unique")}</p>
                                    )}
                                    {resultDiff.added === 0 && resultDiff.removed === 0 ? (
                                      <p className="text-xs text-muted-foreground">{t("agents.dock_diff_same")}</p>
                                    ) : (
                                      <pre className="max-h-40 overflow-auto font-mono text-(--fs-xs-sm)">
                                        {diffChangeLines(resultDiff).map((line, i) => (
                                          <span
                                            key={`${line.kind}-${i}`}
                                            className={`block whitespace-pre-wrap break-all ${line.kind === "add" ? "text-success" : "text-destructive"}`}
                                          >
                                            {line.kind === "add" ? "+" : "-"}{line.text}
                                          </span>
                                        ))}
                                      </pre>
                                    )}
                                  </div>
                                )}
                              </div>
                            )}
                            {canReviewTask(task.status) && (
                              <div className="flex flex-wrap items-center gap-2">
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="xs"
                                  disabled={!canApproveOwnTask(task.created_by, currentUsername)}
                                  title={!canApproveOwnTask(task.created_by, currentUsername) ? t("agents.dock_task_approve_own") : undefined}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    void reviewTask(Number(task.id), "approve");
                                  }}
                                >
                                  {t("agents.dock_task_approve")}
                                </Button>
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="xs"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    void reviewTask(Number(task.id), "reject");
                                  }}
                                >
                                  {t("agents.dock_task_reject")}
                                </Button>
                                {!canApproveOwnTask(task.created_by, currentUsername) && (
                                  <span className="text-(--fs-micro-sm) text-muted-foreground">{t("agents.dock_task_approve_own")}</span>
                                )}
                              </div>
                            )}
                            {canCancelTask(task.status) && (
                              <Button
                                type="button"
                                variant="outline"
                                size="xs"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  void cancelTask(Number(task.id));
                                }}
                              >
                                {t("agents.dock_task_cancel")}
                              </Button>
                            )}
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
                <div className="px-3 py-2">
                  <Link href={`/timeline?tab=tasks&agent_id=${id}`} className="text-xs text-primary hover:underline">
                    {t("agents.tasklist_view_all")}
                  </Link>
                </div>
              </ScrollArea>
            )}
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
