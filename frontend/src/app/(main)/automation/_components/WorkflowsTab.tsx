"use client";
import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { normalizeListEnvelope } from "@/lib/envelope";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { DataState } from "@/components/ui/data-state";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { StatusDot } from "@/components/ui/status-dot";
import { Skeleton } from "@/components/ui/skeleton";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { Play, Plus, Workflow, History } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import WorkflowEditorDialog from "./WorkflowEditorDialog";
import ExecutionHistoryDialog from "./ExecutionHistoryDialog";

interface WorkflowStep {
  id?: number;
  step_order: number;
  task_type: string;
  command: string;
  shell: string;
  timeout_sec: number;
  stop_on_failure: boolean;
  condition_expr: string;
  condition: string;
  on_success: string;
  on_failure: string;
  repeat_count: number;
  repeat_delay: number;
}

interface Workflow {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  scope_type: string;
  scope_ids: string;
  steps: WorkflowStep[];
  created_by: string;
  created_at: string;
}

const scopeLabel = (s: string, t: (k: string) => string) => s === "all" ? t("workflows.scope_all") : s === "tags" ? t("workflows.scope_tags") : s === "groups" ? t("workflows.scope_groups") : t("workflows.scope_selected");

export function WorkflowsTab() {
  const { t } = useI18n();
  const { data, loading, error, refresh: fetchWorkflows } = useApiResource<{ workflows: Workflow[] }>({
    fetcher: async () => {
      const data = await api.get(paths.workflows.list);
      return { workflows: normalizeListEnvelope(data, ["workflows", "data"]) as Workflow[] };
    },
    toastThrottleMs: 0,
    errorMessage: t("workflows.toast.load_failed"),
  });
  const workflows = data?.workflows ?? [];
  const [showEditor, setShowEditor] = useState(false);
  const [editWf, setEditWf] = useState<Workflow | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [historyWfId, setHistoryWfId] = useState<string | null>(null);

  function openCreate() {
    setEditWf(null);
    setShowEditor(true);
  }

  function openEdit(w: Workflow) {
    setEditWf(w);
    setShowEditor(true);
  }

  async function handleSave(payload: { name: string; description: string; scope_type: string; scope_ids: string; steps: WorkflowStep[] }) {
    try {
      const data = editWf
        ? await api.putJson(paths.workflows.one(editWf.id), payload)
        : await api.postJson(paths.workflows.list, payload);
      if (data.success) { setShowEditor(false); void fetchWorkflows(); toast.success(editWf ? t("workflows.toast.updated") : t("workflows.toast.created")); }
      else { toast.error((data.error as string) || t("workflows.toast.save_failed")); }
    } catch { toast.error(t("workflows.toast.save_failed")); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    const id = deleteId;
    setDeleteId(null);
    try {
      const data = await api.del(paths.workflows.one(id));
      if (data.success) { void fetchWorkflows(); toast.success(t("workflows.toast.deleted")); }
    } catch { toast.error(t("workflows.toast.delete_failed")); }
  }

  // In-flight guards: double-clicking Execute queued every command twice on
  // every scoped agent; double-toggling flipped enabled twice (net no-op)
  // while writing two audit rows.
  const [execBusy, setExecBusy] = useState<string>("");
  const [toggleBusy, setToggleBusy] = useState<string>("");

  async function handleToggle(w: Workflow) {
    if (!w.id || toggleBusy === w.id) return;
    setToggleBusy(w.id);
    try {
      await api.post(`${paths.workflows.one(w.id)}/toggle`);
      void fetchWorkflows();
    } catch { toast.error(t("workflows.toast.toggle_failed")); }
    finally { setToggleBusy(""); }
  }

  async function handleExecute(w: Workflow) {
    if (!w.id || execBusy === w.id) return;
    setExecBusy(w.id);
    try {
      const data = await api.postJson(`${paths.workflows.one(w.id)}/execute`, {});
      if (data.success) { toast.success(t("workflows.toast.executed", { task_count: String(data.task_count), agents_count: String(data.agents_count) })); } else { toast.error((data.error as string)); }
    } catch { toast.error(t("workflows.toast.execute_failed")); }
    finally { setExecBusy(""); }
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-3">
          <div className="size-8 bg-primary/10 rounded-lg flex items-center justify-center text-primary"><Workflow className="size-4" /></div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">{t("workflows.title")}</h2>
            <p className="text-xs text-muted-foreground">{t("workflows.subtitle")}</p>
          </div>
        </div>
        <Button onClick={openCreate} size="sm"><Plus className="size-4" /> {t("workflows.new")}</Button>
      </div>

      <DataState
        loading={loading}
        error={error}
        onRetry={() => void fetchWorkflows()}
        empty={!loading && !error && workflows.length === 0}
        emptyIcon={Workflow}
        emptyTitle={t("workflows.empty")}
        emptyMessage={t("workflows.empty_desc")}
        emptyAction={<Button size="sm" onClick={openCreate}>{t("workflows.new")}</Button>}
        loadingSkeleton={
          <div className="flex flex-col gap-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-24 w-full rounded-lg" />
            ))}
          </div>
        }
      >
        <div className="flex flex-col gap-3">
          {workflows.map(w => (
            <Card key={w.id} className="p-4">
              <div className="flex justify-between items-center">
                <div className="flex items-center gap-2">
                   <StatusDot tone={w.enabled ? "success" : "muted"} size="sm" />
                  <h3 className="text-sm font-semibold text-foreground m-0">{w.name}</h3>
                  <Badge variant="secondary">{scopeLabel(w.scope_type, t)}</Badge>
                </div>
                <div className="flex gap-1.5">
                  <Tooltip>
                    <TooltipTrigger render={<Button variant="outline" size="xs" onClick={() => setHistoryWfId(w.id)} aria-label={t("workflows.exec_history")} />}>
                      <History className="size-4" />
                    </TooltipTrigger>
                    <TooltipContent>{t("workflows.exec_history")}</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger render={<Button variant="outline" size="xs" disabled={execBusy === w.id} onClick={() => handleExecute(w)} aria-label={t("workflows.execute_now")} className="border-success text-success hover:bg-success/15" />}>
                      <Play className="size-4" />
                    </TooltipTrigger>
                    <TooltipContent>{t("workflows.execute_now")}</TooltipContent>
                  </Tooltip>
                  <Button variant="outline" size="xs" onClick={() => openEdit(w)}>{t("workflows.edit")}</Button>
                  <Button variant="outline" size="xs" disabled={toggleBusy === w.id} onClick={() => handleToggle(w)}>{w.enabled ? t("workflows.disable") : t("workflows.enable")}</Button>
                  <Button variant="destructive" size="xs" onClick={() => setDeleteId(w.id)}>{t("workflows.delete")}</Button>
                </div>
              </div>
              {w.description && <p className="text-xs text-muted-foreground mt-2 mb-0">{w.description}</p>}
              <div className="flex flex-col gap-1 mt-2">
                {w.steps.map((s) => (
                  <div key={s.step_order} className="flex items-center gap-2 px-2.5 py-1.5 rounded bg-muted text-xs">
                    <span className="size-5 rounded-full bg-primary text-primary-foreground flex items-center justify-center text-(--fs-micro-sm) font-semibold">{s.step_order}</span>
                    <span className="font-semibold text-primary min-w-[80px]">{s.task_type}</span>
                    <span className="flex-1 font-mono text-foreground truncate">{s.command?.substring(0, 60)}</span>
                    <span className="text-muted-foreground">{s.shell} &middot; {s.timeout_sec}s{s.stop_on_failure ? ` \u00b7 ${t("workflows.stop_on_fail")}` : ""}{s.condition ? ` \u00b7 if: ${s.condition}` : ""}{(s.repeat_count ?? 0) > 0 ? ` \u00b7 repeat ${s.repeat_count}x` : ""}</span>
                  </div>
                ))}
              </div>
            </Card>
          ))}
        </div>
      </DataState>

      <WorkflowEditorDialog open={showEditor} onOpenChange={setShowEditor} editWf={editWf} onSave={handleSave} />

      <ConfirmModal open={deleteId !== null} title={t("workflows.delete_title")} message={t("workflows.delete_msg")} danger onConfirm={handleDelete} onCancel={() => setDeleteId(null)} />

      <ExecutionHistoryDialog workflowId={historyWfId} onClose={() => setHistoryWfId(null)} />
    </div>
  );
}
