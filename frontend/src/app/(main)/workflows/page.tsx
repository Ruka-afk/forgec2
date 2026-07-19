"use client";
import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { EmptyState, PageHeader, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Play, Plus, Workflow, ChevronUp, ChevronDown, History, ChevronRight, X } from "lucide-react";

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

interface WorkflowExecution {
  id: number;
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
  execution_id: number;
  step_order: number;
  task_type: string;
  command: string;
  task_id: number;
  agent_id: string;
  status: string;
  result: string;
  branch_action: string;
  branch_target: string;
  started_at: string;
  completed_at?: string;
}

const scopeLabel = (s: string, t: (k: string) => string) => s === "all" ? t("workflows.scope_all") : s === "tags" ? t("workflows.scope_tags") : s === "groups" ? t("workflows.scope_groups") : t("workflows.scope_selected");

const statusBadge = (status: string) => {
  const cls = status === "completed" ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
    : status === "failed" ? "bg-red-500/15 text-red-600 dark:text-red-400"
    : status === "aborted" ? "bg-amber-500/15 text-amber-600 dark:text-amber-400"
    : "bg-blue-500/15 text-blue-600 dark:text-blue-400";
  return <Badge className={cls}>{status}</Badge>;
};

export default function WorkflowsPage() {
  const { t } = useI18n();
  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editWf, setEditWf] = useState<Workflow | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [formName, setFormName] = useState("");
  const [formDesc, setFormDesc] = useState("");
  const [formScope, setFormScope] = useState("all");
  const [formSteps, setFormSteps] = useState<WorkflowStep[]>([{ step_order: 1, task_type: "shell", command: "", shell: "cmd", timeout_sec: 60, stop_on_failure: true, condition_expr: "", condition: "", on_success: "continue", on_failure: "continue", repeat_count: 0, repeat_delay: 0 }]);

  const defaultStep: WorkflowStep = { step_order: 1, task_type: "shell", command: "", shell: "cmd", timeout_sec: 60, stop_on_failure: true, condition_expr: "", condition: "", on_success: "continue", on_failure: "continue", repeat_count: 0, repeat_delay: 0 };

  const [executions, setExecutions] = useState<WorkflowExecution[]>([]);
  const [selectedExec, setSelectedExec] = useState<number | null>(null);
  const [execLogs, setExecLogs] = useState<StepLog[]>([]);
  const [historyWfId, setHistoryWfId] = useState<string | null>(null);
  const [historyLoading, setHistoryLoading] = useState(false);

  const fetchWorkflows = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.json("/workflows");
      setWorkflows((data.workflows || []) as Workflow[]);
    } catch { setWorkflows([]); toast.error(t("workflows.toast.load_failed")); }
    setLoading(false);
  }, [t]);

  useEffect(() => { fetchWorkflows(); }, [fetchWorkflows]);

  function openCreate() {
    setEditWf(null); setFormName(""); setFormDesc(""); setFormScope("all");
    setFormSteps([{ ...defaultStep }]);
    setShowModal(true);
  }

  function openEdit(w: Workflow) {
    setEditWf(w); setFormName(w.name); setFormDesc(w.description); setFormScope(w.scope_type);
    setFormSteps(w.steps.length > 0 ? w.steps.map((s, i) => ({ ...defaultStep, ...s, step_order: i + 1 })) : [{ ...defaultStep }]);
    setShowModal(true);
  }

  function addStep() {
    setFormSteps([...formSteps, { ...defaultStep, step_order: formSteps.length + 1 }]);
  }

  function removeStep(idx: number) {
    setFormSteps(formSteps.filter((_, i) => i !== idx).map((s, i) => ({ ...s, step_order: i + 1 })));
  }

  function moveStep(idx: number, direction: "up" | "down") {
    const newIdx = direction === "up" ? idx - 1 : idx + 1;
    if (newIdx < 0 || newIdx >= formSteps.length) return;
    const updated = [...formSteps];
    [updated[idx], updated[newIdx]] = [updated[newIdx], updated[idx]];
    setFormSteps(updated.map((s, i) => ({ ...s, step_order: i + 1 })));
  }

  function updateStep(idx: number, field: string, value: unknown) {
    const updated = [...formSteps];
    updated[idx] = { ...updated[idx], [field]: value };
    setFormSteps(updated);
  }

  async function handleSave() {
    if (!formName.trim()) return;
    const payload = { name: formName.trim(), description: formDesc, scope_type: formScope, scope_ids: [], steps: formSteps };
    try {
      const data = editWf
        ? await api.putJson(`/workflows/${editWf.id}`, payload)
        : await api.postJson("/workflows", payload);
      if (data.success) { setShowModal(false); fetchWorkflows(); toast.success(editWf ? t("workflows.toast.updated") : t("workflows.toast.created")); }
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
      if (data.success) { toast.success(`Executed: ${data.task_count} tasks to ${data.agents_count} agents`); } else { toast.error((data.error as string)); }
    } catch { toast.error(t("workflows.toast.execute_failed")); }
  }

  async function openHistory(wfId: string) {
    setHistoryWfId(wfId);
    setSelectedExec(null);
    setExecLogs([]);
    setHistoryLoading(true);
    try {
      const data = await api.json(`/workflows/${wfId}/executions`);
      setExecutions((data.executions || []) as WorkflowExecution[]);
    } catch { setExecutions([]); }
    setHistoryLoading(false);
  }

  async function viewExecution(execId: number) {
    setSelectedExec(execId);
    try {
      const wfId = historyWfId;
      const data = await api.json(`/workflows/${wfId}/executions/${execId}`);
      setExecLogs((data.logs || []) as StepLog[]);
    } catch { setExecLogs([]); }
  }

  function closeHistory() {
    setHistoryWfId(null);
    setSelectedExec(null);
    setExecLogs([]);
  }

  const inputCls = "w-full";

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("workflows.title")} subtitle={t("workflows.subtitle")}>
        <Button onClick={openCreate}><Plus className="w-4 h-4" /> {t("workflows.new")}</Button>
      </PageHeader>
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Spinner />
        </div>
      ) : workflows.length === 0 ? (
        <EmptyState icon={Workflow} title={t("workflows.empty")} message={t("workflows.empty_desc")} action={{ label: t("workflows.new"), onClick: openCreate }} />
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
                  <Button variant="outline" size="xs" onClick={() => openHistory(w.id)} title="Execution History" aria-label="Execution History"><History className="w-4 h-4" /></Button>
                  <Button variant="outline" size="xs" onClick={() => handleExecute(w)} title={t("workflows.execute_now")} aria-label={t("workflows.execute_now")} className="border-emerald-400 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-50 dark:hover:bg-emerald-900/20"><Play className="w-4 h-4" /></Button>
                  <Button variant="outline" size="xs" onClick={() => openEdit(w)}>{t("workflows.edit")}</Button>
                  <Button variant="outline" size="xs" onClick={() => handleToggle(w)}>{w.enabled ? t("workflows.disable") : t("workflows.enable")}</Button>
                  <Button variant="destructive" size="xs" onClick={() => setDeleteId(w.id)}>{t("workflows.delete")}</Button>
                </div>
              </div>
              {w.description && <p className="text-xs text-muted-foreground mt-2 mb-0">{w.description}</p>}
              <div className="flex flex-col gap-1 mt-2">
                {w.steps.map((s, i) => (
                  <div key={i} className="flex items-center gap-2 px-2.5 py-1.5 rounded bg-muted text-xs">
                    <span className="w-5 h-5 rounded-full bg-indigo-500 text-white flex items-center justify-center text-[10px] font-semibold">{s.step_order}</span>
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

      <Dialog open={showModal} onOpenChange={setShowModal}>
        <DialogContent className="sm:max-w-3xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editWf ? t("workflows.dialog_edit") : t("workflows.dialog_create")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <Label className="mb-1">{t("workflows.field_name")} *</Label>
                <Input aria-label="Workflow name" name="input-0" value={formName} onChange={e => setFormName(e.target.value)} placeholder={t("workflows.field_name")} className={inputCls} />
              </div>
              <div>
                <Label className="mb-1">{t("workflows.field_scope")}</Label>
                <Select value={formScope} onValueChange={v => { if (v !== null) setFormScope(v); }}>
                  <SelectTrigger className="w-full"><SelectValue placeholder={t("workflows.field_scope")} /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("workflows.scope_all")}</SelectItem>
                    <SelectItem value="tags">{t("workflows.scope_tags")}</SelectItem>
                    <SelectItem value="groups">{t("workflows.scope_groups")}</SelectItem>
                    <SelectItem value="agents">{t("workflows.scope_selected")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div>
              <Label className="mb-1">{t("workflows.field_desc")}</Label>
              <Input aria-label="Optional description" name="input-2" value={formDesc} onChange={e => setFormDesc(e.target.value)} placeholder={t("workflows.field_desc")} className={inputCls} />
            </div>
            <div>
              <div className="flex items-center justify-between mb-2">
                <h4 className="text-sm font-semibold text-foreground m-0">{t("workflows.steps")}</h4>
                <Button variant="outline" size="xs" onClick={addStep}><Plus className="w-4 h-4" />{t("workflows.add_step")}</Button>
              </div>
              <div className="space-y-2">
                {formSteps.map((step, idx) => (
                  <div key={idx} className="p-2 rounded-lg bg-muted space-y-2">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-xs text-muted-foreground min-w-[20px]">#{step.step_order}</span>
                      <Button variant="ghost" size="icon-xs" onClick={() => moveStep(idx, "up")} disabled={idx === 0} aria-label="Move up"><ChevronUp className="w-3 h-3" /></Button>
                      <Button variant="ghost" size="icon-xs" onClick={() => moveStep(idx, "down")} disabled={idx === formSteps.length - 1} aria-label="Move down"><ChevronDown className="w-3 h-3" /></Button>
                      <Select value={step.task_type} onValueChange={v => { if (v !== null) updateStep(idx, "task_type", v); }}>
                        <SelectTrigger className="w-32"><SelectValue placeholder={t("common.type")} /></SelectTrigger>
                        <SelectContent>
                          <SelectItem value="shell">shell</SelectItem>
                          <SelectItem value="powershell">powershell</SelectItem>
                          <SelectItem value="mimikatz">mimikatz</SelectItem>
                          <SelectItem value="bof">bof</SelectItem>
                          <SelectItem value="screenshot">screenshot</SelectItem>
                          <SelectItem value="keylogger_start">keylogger_start</SelectItem>
                          <SelectItem value="download_url">download_url</SelectItem>
                          <SelectItem value="upload">upload</SelectItem>
                        </SelectContent>
                      </Select>
                      <Input aria-label="Command" name={`cmd-${idx}`} value={step.command} onChange={e => updateStep(idx, "command", e.target.value)} placeholder={t("common.command")} className="flex-1 h-8 px-2 text-xs font-mono" />
                      <Select value={step.shell} onValueChange={v => { if (v !== null) updateStep(idx, "shell", v); }}>
                        <SelectTrigger className="w-28"><SelectValue placeholder="Shell" /></SelectTrigger>
                        <SelectContent>
                          <SelectItem value="cmd">cmd</SelectItem>
                          <SelectItem value="powershell">powershell</SelectItem>
                        </SelectContent>
                      </Select>
                      <Input aria-label="Step timeout in seconds" name={`timeout-${idx}`} type="number" value={step.timeout_sec} onChange={e => updateStep(idx, "timeout_sec", parseInt(e.target.value) || 60)} className="w-16 h-8 px-2 text-xs" />
                      <Label className="text-xs text-muted-foreground whitespace-nowrap">Repeats</Label>
                      <Input type="number" min={0} value={step.repeat_count || 0} onChange={e => updateStep(idx, "repeat_count", Number(e.target.value))} className="h-8 text-xs w-16" />
                      <Label className="text-xs text-muted-foreground whitespace-nowrap">Delay(s)</Label>
                      <Input type="number" min={0} value={step.repeat_delay || 0} onChange={e => updateStep(idx, "repeat_delay", Number(e.target.value))} className="h-8 text-xs w-16" />
                      <Label className="flex items-center gap-1 text-xs text-muted-foreground whitespace-nowrap"><Checkbox checked={step.stop_on_failure} onCheckedChange={v => updateStep(idx, "stop_on_failure", v)} /> {t("workflows.stop")}</Label>
                      <Button variant="destructive" size="icon-xs" onClick={() => removeStep(idx)} aria-label="Remove step">&times;</Button>
                    </div>
                    <div className="flex items-center gap-2 ml-7">
                      <Select value={step.condition || "none"} onValueChange={v => { if (v !== null) updateStep(idx, "condition", v === "none" ? "" : v); }}>
                        <SelectTrigger className="w-40"><SelectValue placeholder="Condition" /></SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">No condition</SelectItem>
                          <SelectItem value="not_empty">Result not empty</SelectItem>
                          <SelectItem value="empty">Result empty</SelectItem>
                          <SelectItem value="contains('success')">contains('success')</SelectItem>
                          <SelectItem value="contains('error')">contains('error')</SelectItem>
                          <SelectItem value="equals('0')">equals('0')</SelectItem>
                        </SelectContent>
                      </Select>
                      <span className="text-xs text-muted-foreground">On success:</span>
                      <Select value={step.on_success || "continue"} onValueChange={v => { if (v !== null) updateStep(idx, "on_success", v); }}>
                        <SelectTrigger className="h-8 text-xs w-32"><SelectValue /></SelectTrigger>
                        <SelectContent>
                          <SelectItem value="continue">Continue</SelectItem>
                          <SelectItem value="abort">Abort</SelectItem>
                          {formSteps.filter((_, j) => j !== idx).map((s) => (
                            <SelectItem key={s.step_order} value={String(s.step_order)}>
                              Go to step {s.step_order}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <span className="text-xs text-muted-foreground">On failure:</span>
                      <Select value={step.on_failure || "continue"} onValueChange={v => { if (v !== null) updateStep(idx, "on_failure", v); }}>
                        <SelectTrigger className="h-8 text-xs w-32"><SelectValue /></SelectTrigger>
                        <SelectContent>
                          <SelectItem value="continue">Continue</SelectItem>
                          <SelectItem value="abort">Abort</SelectItem>
                          {formSteps.filter((_, j) => j !== idx).map((s) => (
                            <SelectItem key={s.step_order} value={String(s.step_order)}>
                              Go to step {s.step_order}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowModal(false)}>{t("workflows.cancel")}</Button>
            <Button onClick={handleSave}>{editWf ? t("workflows.save") : t("workflows.create")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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

      <Dialog open={historyWfId !== null} onOpenChange={open => { if (!open) closeHistory(); }}>
        <DialogContent className="sm:max-w-4xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <History className="w-4 h-4" /> Execution History
              {selectedExec !== null && (
                <Button variant="ghost" size="icon-xs" onClick={() => { setSelectedExec(null); setExecLogs([]); }} aria-label="Back to list">
                  <X className="w-3 h-3" />
                </Button>
              )}
            </DialogTitle>
          </DialogHeader>
          {historyLoading ? (
            <div className="flex justify-center py-8"><Spinner /></div>
          ) : selectedExec === null ? (
            executions.length === 0 ? (
              <p className="text-sm text-muted-foreground py-4 text-center">No executions yet.</p>
            ) : (
              <div className="flex flex-col gap-2">
                {executions.map(ex => (
                  <div key={ex.id} className="flex items-center gap-3 p-3 rounded-lg bg-muted hover:bg-muted/80 cursor-pointer transition-colors" onClick={() => viewExecution(ex.id)}>
                    {statusBadge(ex.status)}
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium truncate">{ex.workflow_name}</div>
                      <div className="text-xs text-muted-foreground">{ex.agents_count} agent(s), {ex.tasks_created} task(s) &middot; {new Date(ex.started_at).toLocaleString()}</div>
                    </div>
                    <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
                  </div>
                ))}
              </div>
            )
          ) : (
            <div className="space-y-3">
              {execLogs.map(log => (
                <div key={log.id} className="p-3 rounded-lg bg-muted border-l-2 border-l-indigo-500">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="w-5 h-5 rounded-full bg-indigo-500 text-white flex items-center justify-center text-[10px] font-semibold">{log.step_order}</span>
                    <span className="font-semibold text-xs text-indigo-600 dark:text-indigo-400">{log.task_type}</span>
                    <Badge variant={log.status === "completed" ? "default" : log.status === "failed" ? "destructive" : "secondary"} className="text-[10px]">{log.status}</Badge>
                    {log.branch_action && log.branch_action !== "continue" && (
                      <Badge variant="outline" className="text-[10px]">
                        {log.branch_action}{log.branch_target ? ` \u2192 ${log.branch_target}` : ""}
                      </Badge>
                    )}
                    <span className="text-xs text-muted-foreground ml-auto">{log.agent_id?.substring(0, 12)}</span>
                  </div>
                  {log.command && <div className="text-xs font-mono text-foreground truncate mb-1">{log.command.substring(0, 120)}</div>}
                  {log.result && <div className="text-xs text-muted-foreground whitespace-pre-wrap max-h-24 overflow-y-auto">{log.result.substring(0, 500)}</div>}
                  {log.completed_at && <div className="text-[10px] text-muted-foreground mt-1">{new Date(log.completed_at).toLocaleString()}</div>}
                </div>
              ))}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
