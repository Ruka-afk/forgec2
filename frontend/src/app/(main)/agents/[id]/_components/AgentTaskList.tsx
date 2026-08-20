"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Card } from "@/components/ui/card";
import Link from "next/link";
import { StatusBadge } from "@/components/ui/status-indicator";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { timeAgo, formatTime } from "@/lib/utils";
import type { AgentTaskRecord } from "@/types/agent";
import { ArrowDown, ArrowUp, Camera, CheckCircle, ChevronDown, Clipboard, Clock, Copy, Database, Download, Folder, Keyboard, ListChecks, Search, Shield, Skull, Terminal, Upload, XCircle } from "lucide-react";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";
import { useI18n } from "@/lib/i18n";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { Spinner } from "@/components/ui/spinner";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { downloadText } from "@/lib/download";
import { toast } from "sonner";

import { hueStyles, hueForTaskType } from "@/lib/ui/statusStyles";

const MAX_VISIBLE_TASKS = 8;
const EXPANDED_VISIBLE_TASKS = 20;
const MAX_FILTER_TYPES = 6;
const MAX_RESULT_CHARS = 5000;

function getTaskTypeIcon(type: string): React.ReactNode {
  const s = "w-2.5 h-2.5";
  switch (type) {
    case "shell": return <Terminal className={s} />;
    case "screenshot": return <Camera className={s} />;
    case "ps": return <ListChecks className={s} />;
    case "kill": return <Skull className={s} />;
    case "ls": return <Folder className={s} />;
    case "download": return <Download className={s} />;
    case "upload": return <Upload className={s} />;
    case "keylogger_start": case "keylogger_dump": case "keylogger_stop": return <Keyboard className={s} />;
    case "clipboard_get": return <Clipboard className={s} />;
    case "creds_dump": return <Database className={s} />;
    case "privesc_check": return <Shield className={s} />;
    case "sleep": return <Clock className={s} />;
    case "hashdump": return <Database className={s} />;
    default: return <Terminal className={s} />;
  }
}

function getTaskTypeColor(type: string): string {
  const hue = hueStyles[hueForTaskType(type)];
  return `${hue.bg} ${hue.text}`;
}

/**
 * Lazy full-result loader: WS previews are truncated to ~200 chars; the real
 * payload lives at /api/v1/tasks/:id. Fetched once per task id and cached
 * for the lifetime of the list section.
 */
function useFullTaskResults() {
  const [results, setResults] = useState<Record<number, { result?: string; error?: string }>>({});
  const inflightRef = useRef<Set<number>>(new Set());

  const load = useCallback(async (taskId: number) => {
    if (inflightRef.current.has(taskId)) return;
    inflightRef.current.add(taskId);
    try {
      const full = await api.get<{ result?: string; error?: string }>(paths.tasks.one(taskId));
      setResults((prev) => ({ ...prev, [taskId]: { result: full?.result, error: full?.error } }));
    } catch {
      // Keep the preview; the next expand retries.
    } finally {
      inflightRef.current.delete(taskId);
    }
  }, []);

  const invalidate = useCallback((taskId: number) => {
    setResults((prev) => {
      if (!(taskId in prev)) return prev;
      const next = { ...prev };
      delete next[taskId];
      return next;
    });
  }, []);

  return { results, load, invalidate };
}

interface AgentTaskListProps {
  tasks: AgentTaskRecord[];
  agentId: string;
  expandedTaskId: number | null;
  onToggleExpand: (id: number) => void;
  totalTasks?: number;
  completedTasks?: number;
  pendingTasks?: number;
  failedTasks?: number;
}

function StatCell({ icon, label, value, tone }: { icon: React.ReactNode; label: string; value: number; tone: string }) {
  return (
    <div className="flex items-center gap-1.5">
      {icon}
      <span className="text-(--fs-micro-sm) font-medium text-muted-foreground/70">{label}</span>
      <span className={`text-xs font-bold ${tone}`}>{value}</span>
    </div>
  );
}

export default memo(function AgentTaskList({
  tasks, agentId, expandedTaskId, onToggleExpand,
  totalTasks, completedTasks, pendingTasks, failedTasks,
}: AgentTaskListProps) {
  const { t } = useI18n();
  const [filter, setFilter] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [oldestFirst, setOldestFirst] = useState(false);
  const [showAll, setShowAll] = useState(false);
  const { results, load, invalidate } = useFullTaskResults();

  // WS chunk updates evolve task.result over time; drop the stale full-result
  // cache when the preview changes so the expanded view never shows an old
  // snapshot, and re-fetch if that task is currently expanded.
  const lastResultRef = useRef<Map<number, string>>(new Map());
  useEffect(() => {
    for (const task of tasks) {
      const id = typeof task.id === "number" ? task.id : Number(task.id);
      if (!Number.isFinite(id) || id <= 0) continue;
      const cur = task.result ?? "";
      const prev = lastResultRef.current.get(id);
      if (prev !== undefined && prev !== cur) {
        invalidate(id);
        if (expandedTaskId === id) load(id);
      }
      lastResultRef.current.set(id, cur);
    }
  }, [tasks, expandedTaskId, invalidate, load]);

  const filterTypes = useMemo(() => {
    const counts = new Map<string, number>();
    for (const task of tasks) {
      const type = task.type || "";
      if (type) counts.set(type, (counts.get(type) || 0) + 1);
    }
    return [...counts.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, MAX_FILTER_TYPES)
      .map(([type]) => type);
  }, [tasks]);

  const total = totalTasks ?? tasks.length;
  const completed = completedTasks ?? tasks.filter((task) => task.status === "completed").length;
  const pending = pendingTasks ?? tasks.filter((task) => task.status === "pending").length;
  const failed = failedTasks ?? tasks.filter((task) => task.status === "failed").length;

  const visibleTasks = useMemo(() => {
    let list = tasks;
    if (filter) list = list.filter((task) => (task.type || "") === filter);
    if (query.trim()) {
      const q = query.trim().toLowerCase();
      list = list.filter((task) => (task.command || "").toLowerCase().includes(q) || (task.type || "").toLowerCase().includes(q));
    }
    if (oldestFirst) list = [...list].reverse();
    return showAll ? list : list.slice(0, MAX_VISIBLE_TASKS);
  }, [tasks, filter, query, oldestFirst, showAll]);

  if (tasks.length === 0) {
    return (
      <Card className="mb-4 overflow-hidden border-border/70 bg-card/90 shadow-sm">
        <div className="h-1 w-full bg-gradient-to-r from-primary via-chart-2 to-chart-1" />
        <div className="px-4 py-3 flex flex-wrap items-center justify-between gap-2 border-b border-border/70">
          <h3 className="text-sm font-semibold text-foreground flex items-center gap-2"><ListChecks className="w-3.5 h-3.5 text-primary" />{t("agents.tasklist_recent")}</h3>
          <Link href={`/timeline?tab=tasks&agent_id=${agentId}`} className="text-xs text-primary hover:underline">{t("agents.tasklist_view_all")} &rarr;</Link>
        </div>
        <EmptyState icon={ListChecks} title={t("agents.tasklist_empty")} message={t("agents.tasklist_empty_hint")} className="py-10" />
      </Card>
    );
  }

  return (
    <Card className="mb-4 overflow-hidden border-border/70 bg-card/90 shadow-sm">
      <div className="h-1 w-full bg-gradient-to-r from-primary via-chart-2 to-chart-1" />
      <div className="px-4 py-3 flex flex-wrap items-center justify-between gap-2 border-b border-border/70">
        <h3 className="text-sm font-semibold text-foreground flex items-center gap-2"><ListChecks className="w-3.5 h-3.5 text-primary" />{t("agents.tasklist_recent")}</h3>
        <Link href={`/timeline?tab=tasks&agent_id=${agentId}`} className="text-xs text-primary hover:underline">{t("agents.tasklist_view_all")} &rarr;</Link>
      </div>

      <div className="px-4 py-2.5 flex items-center gap-2 sm:gap-4 flex-wrap border-b border-border/70">
        <div className="flex items-center gap-2.5 sm:gap-4 flex-wrap">
          <StatCell icon={<ListChecks className="w-3 h-3 text-primary" />} label={t("agents.detail_total_tasks")} value={total} tone="text-foreground" />
          <div className="w-px h-3.5 bg-border" />
          <StatCell icon={<CheckCircle className="w-3 h-3 text-success" />} label={t("agents.detail_completed")} value={completed} tone="text-success" />
          <div className="w-px h-3.5 bg-border" />
          <StatCell icon={<Clock className="w-3 h-3 text-warning" />} label={t("agents.detail_pending")} value={pending} tone="text-warning" />
          <div className="w-px h-3.5 bg-border" />
          <StatCell icon={<XCircle className="w-3 h-3 text-destructive" />} label={t("agents.detail_failed")} value={failed} tone="text-destructive" />
        </div>
        <div className="ml-auto flex items-center gap-1.5">
          <div className="relative">
            <Search className="w-3 h-3 absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground/60" aria-hidden="true" />
            <Input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("agents.tasklist_search_placeholder")}
              aria-label={t("agents.tasklist_search_placeholder")}
              className="h-7 w-40 sm:w-52 pl-8 pr-2 text-xs font-mono"
            />
          </div>
          <Tooltip>
            <TooltipTrigger>
              <Button
                variant="outline"
                size="icon-xs"
                onClick={() => setOldestFirst((v) => !v)}
                aria-label={oldestFirst ? t("agents.tasklist_sort_newest") : t("agents.tasklist_sort_oldest")}
              >
                {oldestFirst ? <ArrowUp className="w-3.5 h-3.5" /> : <ArrowDown className="w-3.5 h-3.5" />}
              </Button>
            </TooltipTrigger>
            <TooltipContent>{oldestFirst ? t("agents.tasklist_sort_newest") : t("agents.tasklist_sort_oldest")}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {filterTypes.length > 1 && (
        <div className="px-4 py-2 flex flex-wrap items-center gap-1.5 border-b border-border/70">
          {[null, ...filterTypes].map((type) => (
            <button
              key={type || "all"}
              type="button"
              onClick={() => setFilter(type)}
              className={`px-2.5 py-1 rounded-full text-(--fs-micro-sm) font-medium transition-colors border ${
                filter === type
                  ? "bg-primary/15 border-primary/30 text-primary"
                  : "border-border/70 bg-muted/40 text-muted-foreground hover:bg-muted hover:text-foreground"
              }`}
            >
              {type ? `${type} (${tasks.filter((task) => (task.type || "") === type).length})` : t("agents.tasklist_filter_all")}
            </button>
          ))}
        </div>
      )}

      <div className="divide-y divide-border/70">
        {visibleTasks.map((task, i) => {
          const taskId = task.id ?? i;
          const isExpanded = expandedTaskId === taskId;
          const tType = task.type || "";
          const command = (task.command || "").substring(0, 60);
          const full = results[taskId];
          const hasFullResult = Boolean(full);
          const isFailed = task.status === "failed";
          const resultText = full?.result ?? task.result ?? "";

          const copyResult = async () => {
            try {
              await navigator.clipboard.writeText(resultText);
              toast.success(t("agents.detail_copied"));
            } catch {
              toast.error(t("agents.detail_copy_failed"));
            }
          };

          const downloadResult = () => {
            downloadText(resultText, `task-${taskId}.txt`, "text/plain");
          };

          return (
            <Collapsible key={taskId} open={isExpanded} onOpenChange={(open) => { if (open) { if (!hasFullResult) load(taskId); onToggleExpand(taskId); } else if (expandedTaskId === taskId) onToggleExpand(taskId); }}>
              <div className="group relative">
                {isFailed && <div className="absolute left-0 top-2 bottom-2 w-0.5 rounded-r bg-destructive/70" aria-hidden="true" />}
                <CollapsibleTrigger className="w-full" aria-label={t("agents.tasklist_toggle", { id: String(taskId) })}>
                  <div
                    className="px-4 py-3 flex items-start justify-between gap-3 cursor-pointer transition-colors hover:bg-muted/50"
                  >
                    <div className="flex items-start gap-3 min-w-0 flex-1">
                      <span className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 border border-border/60 shadow-sm ${getTaskTypeColor(tType)}`}>
                        {getTaskTypeIcon(tType)}
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="text-xs font-semibold text-foreground truncate">{tType}</span>
                          {(task.command) && (
                            <span className="text-(--fs-micro-sm) text-muted-foreground/70 font-mono truncate max-w-[220px]">
                              {command}
                            </span>
                          )}
                        </div>
                        <div className="mt-1 flex items-center gap-2 text-(--fs-micro-sm) text-muted-foreground/70">
                          <span className="rounded-full border border-border/70 bg-muted/40 px-2 py-0.5 font-mono">#{taskId}</span>
                          {task.created_by && <span className="truncate">{task.created_by}</span>}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <StatusBadge status={task.status || "pending"} />
                      {task.result && !hasFullResult && (
                        <Badge variant="outline" className="text-(--fs-micro-sm)">{t("agents.tasklist_preview")}</Badge>
                      )}
                      <Tooltip>
                        <TooltipTrigger>
                          <span className="rounded-full border border-border/70 bg-muted/30 px-2 py-1 text-(--fs-micro-sm) text-muted-foreground/70 whitespace-nowrap">
                            {(task.created_at) ? timeAgo(String(task.created_at), t) : ""}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent>{String(task.created_at || "")}</TooltipContent>
                      </Tooltip>
                      <ChevronDown className="w-2.5 h-2.5 text-muted-foreground/70" />
                    </div>
                  </div>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className="border-t border-border/70 bg-muted/20 px-4 pb-4">
                    <div className="grid gap-2 pt-3 text-xs md:grid-cols-2">
                      <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_id")}</span> <span className="font-mono text-foreground">{taskId}</span></div>
                      <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_type")}</span> <span className="text-foreground">{tType}</span></div>
                      <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_created")}</span> <span className="text-foreground">{task.created_at ? formatTime(String(task.created_at)) : "\u2014"}</span></div>
                      {(task.created_by) && <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_by")}</span> <span className="text-foreground">{task.created_by}</span></div>}
                      {(task.command) && <div className="md:col-span-2"><span className="text-muted-foreground/70">{t("agents.tasklist_label_command")}</span> <span className="font-mono text-foreground break-all">{task.command}</span></div>}
                      {resultText ? (
                        <div className="md:col-span-2">
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-muted-foreground/70">{t("agents.tasklist_label_result")}</span>
                            <div className="flex items-center gap-1">
                              {resultText.length > MAX_RESULT_CHARS && (
                                <span className="text-(--fs-micro-sm) text-muted-foreground/70">{t("agents.tasklist_result_truncated")}</span>
                              )}
                              <Button variant="ghost" size="xs" onClick={copyResult} className="text-(--fs-micro-sm) text-muted-foreground hover:text-foreground gap-1">
                                <Copy className="w-3 h-3" /> {t("agents.tasklist_copy_result")}
                              </Button>
                              <Button variant="ghost" size="xs" onClick={downloadResult} className="text-(--fs-micro-sm) text-muted-foreground hover:text-foreground gap-1">
                                <Download className="w-3 h-3" /> {t("agents.tasklist_download_result")}
                              </Button>
                            </div>
                          </div>
                          <pre className="mt-1 max-h-60 overflow-y-auto rounded-lg border border-border/70 bg-card p-3 font-mono text-xs text-foreground whitespace-pre-wrap break-all">{resultText.substring(0, MAX_RESULT_CHARS)}</pre>
                        </div>
                      ) : (hasFullResult ? null : (
                        isExpanded ? (
                          <div className="md:col-span-2"><span className="text-muted-foreground/70">{t("agents.tasklist_label_result")}</span><div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground/70"><Spinner size="xs" />{t("agents.tasklist_loading_result")}</div></div>
                        ) : null
                      ))}
                      {((full?.error ?? task.error)) && <div className="md:col-span-2"><span className="text-muted-foreground/70">{t("agents.tasklist_label_error")}</span><pre className="mt-1 max-h-32 overflow-y-auto rounded-lg border border-destructive/20 bg-destructive/10 p-3 font-mono text-xs text-destructive whitespace-pre-wrap break-all">{full?.error ?? task.error}</pre></div>}
                    </div>
                  </div>
                </CollapsibleContent>
              </div>
            </Collapsible>
          );
        })}
        {visibleTasks.length === 0 && (
          <div className="px-4 py-8 text-center text-xs text-muted-foreground/70">{t("agents.tasklist_no_match")}</div>
        )}
      </div>
      {!showAll && tasks.length > MAX_VISIBLE_TASKS && (
        <div className="px-4 py-2 border-t border-border/70">
          <button
            type="button"
            onClick={() => setShowAll(true)}
            className="w-full py-1.5 rounded-lg text-xs font-medium text-primary hover:bg-muted/60 transition-colors"
          >
            {t("agents.tasklist_load_more", { count: String(Math.min(tasks.length, EXPANDED_VISIBLE_TASKS) - MAX_VISIBLE_TASKS) })}
          </button>
        </div>
      )}
    </Card>
  );
})