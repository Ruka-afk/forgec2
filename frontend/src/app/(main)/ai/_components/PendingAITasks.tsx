"use client";

import { useCallback, useState } from "react";
import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Check, X, Clock, Bot } from "lucide-react";
import { toast } from "sonner";
import { useApiResource } from "@/lib/hooks/useApiResource";

interface PendingTask {
  id: number;
  agent_id: string;
  hostname: string;
  command: string;
  type: string;
  created_at: string;
}

interface PendingAITasksProps {
  activeSessionId: number | null;
  onTaskFeedback: (content: string, sessionId: number | null) => Promise<void>;
}

// Poll the task endpoint until it reaches a terminal state (max 30s).
// Lives outside the component so Date.now() usage never taints render purity.
async function pollTaskResult(taskId: number, agentId: string): Promise<{ status: string; result: string; error: string } | null> {
  try {
    const data = await pollTask(agentId, taskId, { intervalMs: 2000, timeoutMs: 30000 });
    return { status: data.status, result: data.result || "", error: data.error || "" };
  } catch {
    return null;
  }
}

export function PendingAITasks({ activeSessionId, onTaskFeedback }: PendingAITasksProps) {
  const { t } = useI18n();
  const [actingIds, setActingIds] = useState<Set<number>>(() => new Set());

  const setActing = (id: number, active: boolean) => {
    setActingIds((current) => {
      const next = new Set(current);
      if (active) next.add(id);
      else next.delete(id);
      return next;
    });
  };

  const fetchTasks = useCallback(async (signal?: AbortSignal) => {
    const data = await api.get<{ tasks?: PendingTask[] }>(paths.ai.pendingTasks, { signal });
    return data.tasks || [];
  }, []);
  const { data, loading, refresh } = useApiResource<PendingTask[]>({
    fetcher: fetchTasks,
    pollMs: 10000,
  });
  const tasks = data || [];

  const handleApprove = async (task: PendingTask) => {
    // Keep every asynchronous status update attached to the conversation from
    // which the operator approved it, even if they switch sessions while the
    // task is polling in the background.
    const feedbackSessionId = activeSessionId;
    setActing(task.id, true);
    try {
      await api.post(paths.tasksCollab.approve(task.id), {});
      toast.success(t("ai.pending_approved"));
      await onTaskFeedback(`[System] Task #${task.id} (${task.command}) approved for agent ${task.hostname || task.agent_id}. Waiting for result...`, feedbackSessionId);
      const res = await pollTaskResult(task.id, task.agent_id);
      if (res) {
        if (res.status === "completed") {
          await onTaskFeedback(`[System] Task #${task.id} completed. Result:\n${res.result || "(empty)"}`, feedbackSessionId);
        } else if (res.status === "failed" || res.status === "cancelled") {
          await onTaskFeedback(`[System] Task #${task.id} failed. Error: ${res.error || "unknown"}\nResult: ${res.result || ""}`, feedbackSessionId);
        }
      } else {
        await onTaskFeedback(`[System] Task #${task.id} still pending after 30s. Check Tasks page later.`, feedbackSessionId);
      }
      await refresh();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.pending_approve_failed"));
    } finally {
      setActing(task.id, false);
    }
  };

  const handleReject = async (task: PendingTask) => {
    const feedbackSessionId = activeSessionId;
    setActing(task.id, true);
    try {
      await api.post(paths.tasksCollab.reject(task.id), {});
      toast.success(t("ai.pending_rejected"));
      await onTaskFeedback(`[System] Task #${task.id} rejected by operator.`, feedbackSessionId);
      await refresh();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.pending_reject_failed"));
    } finally {
      setActing(task.id, false);
    }
  };

  if (loading && tasks.length === 0) return null;
  if (tasks.length === 0) return null;

  return (
    <Card className="shrink-0 p-3 border-warning/30 bg-warning/5" aria-live="polite">
      <div className="flex items-center gap-2 mb-2">
        <Bot className="size-4 text-warning" />
        <span className="text-sm font-medium">{t("ai.pending_title")}</span>
        <Badge variant="warning" className="ml-auto">{tasks.length}</Badge>
      </div>
      <div className="space-y-2 max-h-64 overflow-y-auto">
        {tasks.map((task) => (
          <div key={task.id} className="rounded-lg border border-border bg-card p-2 flex flex-col gap-1.5">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Clock className="size-3" />
              <span className="font-mono">{task.hostname || task.agent_id.slice(0, 8)}</span>
              <span>·</span>
              <span>{formatTime(task.created_at)}</span>
            </div>
            <pre className="text-xs font-mono bg-muted rounded p-1.5 whitespace-pre-wrap break-words max-h-20 overflow-auto">
              {task.command}
            </pre>
            <div className="flex gap-2">
              <Button
                size="xs"
                disabled={actingIds.has(task.id)}
                onClick={() => handleApprove(task)}
                className="flex-1 gap-1"
              >
                <Check className="size-3" /> {t("ai.approve")}
              </Button>
              <Button
                variant="outline"
                size="xs"
                disabled={actingIds.has(task.id)}
                onClick={() => handleReject(task)}
                className="flex-1 gap-1"
              >
                <X className="size-3" /> {t("ai.reject")}
              </Button>
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}
