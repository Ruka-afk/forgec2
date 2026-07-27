"use client";
import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { EmptyState, PageHeader, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Play, Plus, Workflow, History } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import WorkflowEditorDialog from "./_components/WorkflowEditorDialog";
import ExecutionHistoryDialog from "./_components/ExecutionHistoryDialog";

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

export default function WorkflowsPage() {
  const { t } = useI18n();
  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [loading, setLoading] = useState(true);
  const [showEditor, setShowEditor] = useState(false);
  const [editWf, setEditWf] = useState<Workflow | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [historyWfId, setHistoryWfId] = useState<string | null>(null);

  const fetchWorkflows = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.get("/workflows");
      setWorkflows((data.workflows || []) as Workflow[]);
    } catch { setWorkflows([]); toast.error(t("workflows.toast.load_failed")); }
    setLoading(false);
  }, [t]);

  useEffect(() => { fetchWorkflows(); }, [fetchWorkflows]);

  function openCreate() {
    setEditWf(null);
    setShowEditor(true);
  }

  function openEdit(w: Workflow) {
    setEditWf(w);
    setShowEditor(true);
  }

  async function handleSave(payload: { name: string; description: string; scope_type: string; scope_ids: string[]; steps: WorkflowStep[] }) {
    try {
      const data = editWf
        ? await api.putJson(`/workflows/${editWf.id}`, payload)
        : await api.postJson("/workflows", payload);
      if (data.success) { setShowEditor(false); fetchWorkflows(); toast.success(editWf ? t("workflows.toast.updated") : t("workflows.toast.created")); }
      else { toast.error((data.error as string) || t("workflows.toast.save_failed")); }
    } catch { toast.error(t("workflows.toast.save_failed")); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    const id = deleteId;
    setDeleteId(null);
    try {
      const data = await api.del(`/workflows/${id}`);
      if (data.success) { fetchWorkflows(); toast.success(t("workflows.toast.deleted")); }
    } catch { toast.error(t("workflows.toast.delete_failed")); }
  }

  async function handleToggle(w: Workflow) {
    try {
      await api.post(`/workflows/${w.id}/toggle`);
      fetchWorkflows();
    } catch { toast.error(t("workflows.toast.toggle_failed")); }
  }

  async function handleExecute(w: Workflow) {
    try {
      const data = await api.postJson(`/workflows/${w.id}/execute`, {});
      if (data.success) { toast.success(t("workflows.toast.executed", { task_count: String(data.task_count), agents_count: String(data.agents_count) })); } else { toast.error((data.error as string)); }
    } catch { toast.error(t("workflows.toast.execute_failed")); }
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("workflows.title")} subtitle={t("workflows.subtitle")}>
        <Button onClick={openCreate}><Plus className="w-4 h-4" /> {t("workflows.new")}</Button>
      </PageHeader>
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Spinner />
        </div>
      ) : workflows.length === 0 ? (
        <EmptyState icon={Workflow} title={t("workflows.empty")} message={t("workflows.empty_desc")} action={<Button size="sm" onClick={openCreate}>{t("workflows.new")}</Button>} />
      ) : (
        <div className="flex flex-col gap-3">
          {workflows.map(w => (
            <Card key={w.id} className="p-4">
              <div className="flex justify-between items-center">
                <div className="flex items-center gap-2">
                  <span className={`w-2 h-2 rounded-full ${w.enabled ? "bg-emerald-500" : "bg-muted-foreground"}`}></span>
                  <h3 className="text-sm font-semibold text-foreground m-0">{w.name}</h3>
                  <Badge variant="secondary">{scopeLabel(w.scope_type, t)}</Badge>
                </div>
                <div className="flex gap-1.5">
                  <Tooltip>
                    <TooltipTrigger render={<Button variant="outline" size="xs" onClick={() => setHistoryWfId(w.id)} aria-label="Execution History" />}>
                      <History className="w-4 h-4" />
                    </TooltipTrigger>
                    <TooltipContent>Execution History</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger render={<Button variant="outline" size="xs" onClick={() => handleExecute(w)} aria-label={t("workflows.execute_now")} className="border-emerald-400 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-50 dark:hover:bg-emerald-900/20" />}>
                      <Play className="w-4 h-4" />
                    </TooltipTrigger>
                    <TooltipContent>{t("workflows.execute_now")}</TooltipContent>
                  </Tooltip>
                  <Button variant="outline" size="xs" onClick={() => openEdit(w)}>{t("workflows.edit")}</Button>
                  <Button variant="outline" size="xs" onClick={() => handleToggle(w)}>{w.enabled ? t("workflows.disable") : t("workflows.enable")}</Button>
                  <Button variant="destructive" size="xs" onClick={() => setDeleteId(w.id)}>{t("workflows.delete")}</Button>
                </div>
              </div>
              {w.description && <p className="text-xs text-muted-foreground mt-2 mb-0">{w.description}</p>}
              <div className="flex flex-col gap-1 mt-2">
                {w.steps.map((s, i) => (
                  <div key={i} className="flex items-center gap-2 px-2.5 py-1.5 rounded bg-muted text-xs">
                    <span className="w-5 h-5 rounded-full bg-indigo-500 text-white flex items-center justify-center text-(--font-size-micro-sm) font-semibold">{s.step_order}</span>
                    <span className="font-semibold text-indigo-600 dark:text-indigo-400 min-w-[80px]">{s.task_type}</span>
                    <span className="flex-1 font-mono text-foreground truncate">{s.command?.substring(0, 60)}</span>
                    <span className="text-muted-foreground">{s.shell} &middot; {s.timeout_sec}s{s.stop_on_failure ? ` \u00b7 ${t("workflows.stop_on_fail")}` : ""}{s.condition ? ` \u00b7 if: ${s.condition}` : ""}{(s.repeat_count ?? 0) > 0 ? ` \u00b7 repeat ${s.repeat_count}x` : ""}</span>
                  </div>
                ))}
              </div>
            </Card>
          ))}
        </div>
      )}

      <WorkflowEditorDialog open={showEditor} onOpenChange={setShowEditor} editWf={editWf} onSave={handleSave} />

      <Dialog open={deleteId !== null} onOpenChange={open => { if (!open) setDeleteId(null); }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("workflows.delete_title")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">{t("workflows.delete_msg")}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteId(null)}>{t("workflows.cancel")}</Button>
            <Button variant="destructive" onClick={handleDelete}>{t("workflows.delete")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ExecutionHistoryDialog workflowId={historyWfId} onClose={() => setHistoryWfId(null)} />
    </div>
  );
}
