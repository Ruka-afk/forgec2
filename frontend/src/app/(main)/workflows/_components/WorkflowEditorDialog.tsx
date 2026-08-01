"use client";

import { useState, useRef } from "react";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Plus, ChevronUp, ChevronDown } from "lucide-react";

interface WorkflowStep {
  _key?: number;
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

const DEFAULT_STEP: WorkflowStep = { step_order: 1, task_type: "shell", command: "", shell: "cmd", timeout_sec: 60, stop_on_failure: true, condition_expr: "", condition: "", on_success: "continue", on_failure: "continue", repeat_count: 0, repeat_delay: 0 };

interface WorkflowEditorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editWf: Workflow | null;
  onSave: (payload: { name: string; description: string; scope_type: string; scope_ids: string[]; steps: WorkflowStep[] }) => void;
}

export default function WorkflowEditorDialog({ open, onOpenChange, editWf, onSave }: WorkflowEditorDialogProps) {
  const { t } = useI18n();
  const stepKeyRef = useRef(0);
  const nextKey = () => ++stepKeyRef.current;
  const [formName, setFormName] = useState(editWf?.name || "");
  const [formDesc, setFormDesc] = useState(editWf?.description || "");
  const [formScope, setFormScope] = useState(editWf?.scope_type || "all");
  const [formSteps, setFormSteps] = useState<WorkflowStep[]>(() =>
    editWf?.steps?.length
      ? editWf.steps.map((s, i) => ({ ...DEFAULT_STEP, ...s, step_order: i + 1 }))
      : [{ ...DEFAULT_STEP, _key: nextKey() }]
  );

  const syncFromEdit = () => {
    if (editWf) {
      setFormName(editWf.name);
      setFormDesc(editWf.description);
      setFormScope(editWf.scope_type);
      setFormSteps(editWf.steps.length > 0 ? editWf.steps.map((s, i) => ({ ...DEFAULT_STEP, ...s, step_order: i + 1 })) : [{ ...DEFAULT_STEP, _key: nextKey() }]);
    } else {
      setFormName("");
      setFormDesc("");
      setFormScope("all");
      setFormSteps([{ ...DEFAULT_STEP, _key: nextKey() }]);
    }
  };

  const handleOpen = (isOpen: boolean) => {
    if (isOpen) syncFromEdit();
    onOpenChange(isOpen);
  };

  function addStep() {
    setFormSteps([...formSteps, { ...DEFAULT_STEP, _key: nextKey(), step_order: formSteps.length + 1 }]);
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

  function handleSave() {
    if (!formName.trim()) return;
    onSave({ name: formName.trim(), description: formDesc, scope_type: formScope, scope_ids: [], steps: formSteps });
  }

  return (
    <Dialog open={open} onOpenChange={handleOpen}>
      <DialogContent className="sm:max-w-3xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{editWf ? t("workflows.dialog_edit") : t("workflows.dialog_create")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <Label className="mb-1">{t("workflows.field_name")} *</Label>
              <Input aria-label="Workflow name" name="wf-name" value={formName} onChange={e => setFormName(e.target.value)} placeholder={t("workflows.field_name")} />
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
            <Input aria-label="Optional description" name="wf-desc" value={formDesc} onChange={e => setFormDesc(e.target.value)} placeholder={t("workflows.field_desc")} />
          </div>
          <div>
            <div className="flex items-center justify-between mb-2">
              <h4 className="text-sm font-semibold text-foreground m-0">{t("workflows.steps")}</h4>
              <Button variant="outline" size="xs" onClick={addStep}><Plus className="w-4 h-4" />{t("workflows.add_step")}</Button>
            </div>
            <div className="space-y-2">
              {formSteps.map((step, idx) => (
                <div key={step.id ?? step._key ?? idx} className="p-2 rounded-lg bg-muted space-y-2">
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
                        <SelectItem value="contains('success')">contains(&apos;success&apos;)</SelectItem>
                        <SelectItem value="contains('error')">contains(&apos;error&apos;)</SelectItem>
                        <SelectItem value="equals('0')">equals(&apos;0&apos;)</SelectItem>
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
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t("workflows.cancel")}</Button>
          <Button onClick={handleSave}>{editWf ? t("workflows.save") : t("workflows.create")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
