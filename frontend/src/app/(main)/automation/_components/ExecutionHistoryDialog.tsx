"use client";

import { useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { History, ChevronRight, X } from "lucide-react";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { formatTime, enumLabel } from "@/lib/utils";

interface WorkflowExecution {
  execution_id: string;
  workflow_id: string;
  workflow_name: string;
  status: string;
  tasks_created: number;
  agents_count: number;
  started_at: string;
  completed_at?: string;
  error_msg?: string;
}

interface StepLog {
  id: number;
  execution_id: string;
  step_order: number;
  task_type: string;
  command: string;
  task_id: number;
  agent_id: string;
  agent_host?: string;
  status: string;
  result: string;
  branch_action: string;
  branch_target: string;
  error_msg?: string;
  started_at: string;
  completed_at?: string;
}

const statusBadge = (status: string) => {
  const cls = status === "completed" ? "bg-success/15 text-success"
    : status === "failed" ? "bg-destructive/15 text-destructive"
    : status === "aborted" ? "bg-warning/15 text-warning"
    : "bg-info/15 text-info";
  return <Badge className={cls}>{status}</Badge>;
};

interface ExecutionHistoryDialogProps {
  workflowId: string | null;
  onClose: () => void;
}

export default function ExecutionHistoryDialog({ workflowId, onClose }: ExecutionHistoryDialogProps) {
  const { t } = useI18n();
  const [executions, setExecutions] = useState<WorkflowExecution[]>([]);
  const [selectedExec, setSelectedExec] = useState<string | null>(null);
  const [execLogs, setExecLogs] = useState<StepLog[]>([]);
  const [loading, setLoading] = useState(false);

  const openHistory = useCallback(async (wfId: string) => {
    setSelectedExec(null);
    setExecLogs([]);
    setLoading(true);
    try {
      const data = await api.get(`${paths.workflows.one(wfId)}/executions`);
      setExecutions((data.executions || []) as WorkflowExecution[]);
    } catch {
      setExecutions([]);
      toast.error(t("workflows.toast.load_exec_failed"));
    }
    setLoading(false);
  }, [t]);

  const loadExecutions = useCallback(async () => {
    if (!workflowId) return;
    await openHistory(workflowId);
  }, [workflowId, openHistory]);

  const viewExecution = useCallback(async (execId: string) => {
    setSelectedExec(execId);
    try {
      const data = await api.get(`${paths.workflows.one(String(workflowId))}/executions/${execId}`);
      setExecLogs((data.logs || []) as StepLog[]);
    } catch {
      setExecLogs([]);
      toast.error(t("workflows.toast.load_exec_failed"));
    }
  }, [workflowId, t]);

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setSelectedExec(null);
      setExecLogs([]);
      setExecutions([]);
      onClose();
    }
  };

  if (workflowId) {
    if (executions.length === 0 && !loading) {
      loadExecutions();
    }
  }

  return (
    <Dialog open={workflowId !== null} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-4xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <History className="size-4" /> Execution History
            {selectedExec !== null && (
              <Button variant="ghost" size="icon-xs" onClick={() => { setSelectedExec(null); setExecLogs([]); }} aria-label={t("workflows.back_to_list")}>
                <X className="size-3" />
              </Button>
            )}
          </DialogTitle>
        </DialogHeader>
        {loading ? (
          <div className="flex justify-center py-8"><Spinner /></div>
        ) : selectedExec === null ? (
          executions.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">{t("workflows.no_executions")}</p>
          ) : (
            <div className="flex flex-col gap-2">
              {executions.map(ex => (
                <button key={ex.execution_id} type="button" className="flex w-full items-center gap-3 p-3 rounded-lg bg-muted hover:bg-muted/80 cursor-pointer transition-colors text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={() => viewExecution(ex.execution_id)}>
                  {statusBadge(ex.status)}
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium truncate">{ex.workflow_name}</div>
                    <div className="text-xs text-muted-foreground">{ex.agents_count} agent(s), {ex.tasks_created} task(s) &middot; {formatTime(ex.started_at)}</div>
                    {ex.error_msg && <div className="text-xs text-destructive truncate">{ex.error_msg}</div>}
                  </div>
                  <ChevronRight className="size-4 text-muted-foreground shrink-0" />
                </button>
              ))}
            </div>
          )
        ) : (
          <div className="space-y-3">
            {execLogs.map(log => (
              <div key={log.id} className="p-3 rounded-lg bg-muted border-l-2 border-l-primary">
                <div className="flex items-center gap-2 mb-1">
                  <span className="size-5 rounded-full bg-primary text-primary-foreground flex items-center justify-center text-(--fs-micro-sm) font-semibold">{log.step_order}</span>
                  <span className="font-semibold text-xs text-primary">{log.task_type}</span>
                  <Badge variant={log.status === "completed" ? "default" : log.status === "failed" ? "destructive" : "secondary"} className="text-(--fs-micro-sm)">{enumLabel(t, "tasks", log.status)}</Badge>
                  {log.branch_action && log.branch_action !== "continue" && (
                    <Badge variant="outline" className="text-(--fs-micro-sm)">
                      {log.branch_action}{log.branch_target ? ` → ${log.branch_target}` : ""}
                    </Badge>
                  )}
                  <span className="text-xs text-muted-foreground ml-auto">{log.agent_host || log.agent_id?.substring(0, 12)}</span>
                </div>
                {log.command && <div className="text-xs font-mono text-foreground truncate mb-1">{log.command.substring(0, 120)}</div>}
                {log.result && <div className="text-xs text-muted-foreground whitespace-pre-wrap max-h-24 overflow-y-auto">{log.result.substring(0, 500)}</div>}
                {log.error_msg && <div className="text-xs text-destructive mt-1">{log.error_msg}</div>}
                {log.completed_at && <div className="text-(--fs-micro-sm) text-muted-foreground mt-1">{formatTime(log.completed_at)}</div>}
              </div>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
