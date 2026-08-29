"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Check, X, Clock, Bot } from "lucide-react";
import { toast } from "sonner";

interface PendingTask {
  id: number;
  agent_id: string;
  hostname: string;
  command: string;
  type: string;
  created_at: string;
}

interface PendingAITasksProps {
  onTaskFeedback: (content: string) => void;
}

// Poll the task endpoint until it reaches a terminal state (max 30s).
// Lives outside the component so Date.now() usage never taints render purity.
async function pollTaskResult(taskId: number, agentId: string): Promise<{ status: string; result: string; error: string } | null> {
  const deadline = new Promise<never>((_, reject) => setTimeout(() => reject(new Error("timeout")), 30000));
  for (;;) {
    await new Promise((r) => setTimeout(r, 2000));
    try {
      const d = await Promise.race([
        api.get<{ data?: { status?: string; result?: string; error?: string } }>(
          paths.agents.task(agentId, taskId),
        ),
        deadline,
      ]);
      const data = d.data || d;
      const status = (data as { status?: string }).status;
      if (status === "completed" || status === "failed") {
        return {
          status,
          result: (data as { result?: string }).result || "",
          error: (data as { error?: string }).error || "",
        };
      }
    } catch (e) {
      if (e instanceof Error && e.message === "timeout") return null;
      // ignore transient fetch errors and keep polling
    }
  }
}

export function PendingAITasks({ onTaskFeedback }: PendingAITasksProps) {
  const { t } = useI18n();
  const [tasks, setTasks] = useState<PendingTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [actingId, setActingId] = useState<number | null>(null);

  const load = useCallback(async () => {
    try {
      const d = await api.get<{ tasks?: PendingTask[] }>(paths.ai.pendingTasks);
      setTasks(d.tasks || []);
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 10000);
    return () => clearInterval(id);
  }, [load]);

  const handleApprove = async (task: PendingTask) => {
    setActingId(task.id);
    try {
      await api.post(paths.tasksCollab.approve(task.id), {});
      toast.success(t("ai.pending_approved"));
      onTaskFeedback(`[System] Task #${task.id} (${task.command}) approved for agent ${task.hostname || task.agent_id}. Waiting for result...`);
      const res = await pollTaskResult(task.id, task.agent_id);
      if (res) {
        if (res.status === "completed") {
          onTaskFeedback(`[System] Task #${task.id} completed. Result:\n${res.result || "(empty)"}`);
        } else if (res.status === "failed") {
          onTaskFeedback(`[System] Task #${task.id} failed. Error: ${res.error || "unknown"}\nResult: ${res.result || ""}`);
        }
      } else {
        onTaskFeedback(`[System] Task #${task.id} still pending after 30s. Check Tasks page later.`);
      }
      load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.pending_approve_failed"));
    } finally {
      setActingId(null);
    }
  };

  const handleReject = async (task: PendingTask) => {
    setActingId(task.id);
    try {
      await api.post(paths.tasksCollab.reject(task.id), {});
      toast.success(t("ai.pending_rejected"));
      onTaskFeedback(`[System] Task #${task.id} rejected by operator.`);
      load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.pending_reject_failed"));
    } finally {
      setActingId(null);
    }
  };

  if (loading && tasks.length === 0) return null;
  if (tasks.length === 0) return null;

  return (
    <Card className="shrink-0 p-3 border-warning/30 bg-warning/5">
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
                disabled={actingId === task.id}
                onClick={() => handleApprove(task)}
                className="flex-1 gap-1"
              >
                <Check className="size-3" /> {t("ai.approve")}
              </Button>
              <Button
                variant="outline"
                size="xs"
                disabled={actingId === task.id}
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
