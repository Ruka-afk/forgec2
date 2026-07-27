"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { ConfirmModal, EmptyState, PageHeader } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { NormalizedAgent as Agent } from "@/types/agent";
import { formatTime } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Alert } from "@/components/ui/alert";
import { Switch } from "@/components/ui/switch";
import { Bug, Calendar, Clock, Pencil, Plus, Trash2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { fetchTaskTypes, type TaskTypeInfo } from "@/lib/taskTypes";

interface ScheduledTask {
  id: string;
  name: string;
  enabled: boolean;
  agent_id: string;
  task_type: string;
  command: string;
  params: string;
  schedule: string;
  last_run: string;
  next_run: string;
  run_count: number;
  created_by: string;
  created_at: string;
}

export default function SchedulerPage() {
  const { t } = useI18n();
  const [tasks, setTasks] = useState<ScheduledTask[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [agentId, setAgentId] = useState("");
  const [taskType, setTaskType] = useState("shell");
  const [command, setCommand] = useState("");
  const [params, setParams] = useState("");
  const [schedule, setSchedule] = useState("");
  const [taskTypes, setTaskTypes] = useState<TaskTypeInfo[]>([]);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [message, setMessage] = useState("");

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [t, a] = await Promise.all([
        api.get<{ tasks: ScheduledTask[] }>("/scheduler/tasks"),
        api.get<{ agents?: Agent[]; data?: Agent[] }>("/agents"),
      ]);
      setTasks(t.tasks || []);
      setAgents((a.agents || a.data || []) as Agent[]);
    } catch { setMessage(t("scheduler.load_failed")); }
    finally { setLoading(false); }
  }, [t]);

  useEffect(() => { Promise.resolve().then(() => fetchData()); }, [fetchData]);

  useEffect(() => { fetchTaskTypes().then(setTaskTypes); }, []);

  function resetForm() {
    setName(""); setAgentId(""); setTaskType("shell"); setCommand(""); setParams(""); setSchedule("");
    setEditingId(null);
  }

  async function handleSave() {
    if (!name || !agentId || !schedule) {
      setMessage(t("scheduler.fields_required")); return;
    }
    const body = { name, agent_id: agentId, task_type: taskType, command, params, schedule };
    try {
      if (editingId) {
        await api.putJson(`/scheduler/tasks/${editingId}`, body);
      } else {
        await api.postJson("/scheduler/tasks", body);
      }
      resetForm(); setShowForm(false); setMessage(t("scheduler.saved")); fetchData();
    } catch { setMessage(t("scheduler.save_failed")); }
  }

  async function handleToggle(id: string) {
      try { await api.postJson(`/scheduler/tasks/${id}/toggle`, {}); fetchData(); }
    catch { setMessage(t("scheduler.toggle_failed")); }
  }

  function handleDelete(id: string) {
    setCfm({msg: t("scheduler.delete_confirm"), cb: async () => {
      try { await api.del(`/scheduler/tasks/${id}`); fetchData(); }
      catch { setMessage(t("scheduler.delete_failed")); }
    }});
  }

  function editTask(t: ScheduledTask) {
    setEditingId(t.id); setName(t.name); setAgentId(t.agent_id); setTaskType(t.task_type);
    setCommand(t.command); setParams(t.params); setSchedule(t.schedule); setShowForm(true);
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      {message && (
        <Alert className="mb-4 flex items-center justify-between">
          <span>{message}</span>
          <Button variant="ghost" size="icon-sm" onClick={() => setMessage("")} className="text-muted-foreground hover:text-foreground" aria-label="Dismiss message">
            <X className="w-4 h-4" />
          </Button>
        </Alert>
      )}

      <PageHeader title={t("scheduler.title")} subtitle={t("scheduler.subtitle")}>
        <Button onClick={() => { resetForm(); setShowForm(true); }} className="gap-2">
          <Plus className="w-4 h-4" /> {t("scheduler.new_schedule")}
        </Button>
      </PageHeader>

      {showForm && (
        <Card className="mb-6">
          <CardContent className="p-4 sm:p-5">
            <h2 className="text-base font-semibold mb-4">
              {editingId ? t("scheduler.edit_schedule") : t("scheduler.new_schedule")}
            </h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
              <div>
                <Label className="text-xs mb-1">{t("scheduler.name")}</Label>
                <Input value={name} onChange={e => setName(e.target.value)} aria-label="Schedule name" />
              </div>
              <div>
                <Label className="text-xs mb-1">{t("scheduler.agent")}</Label>
                <Select value={agentId} onValueChange={(v) => setAgentId(v ?? "")}>
                  <SelectTrigger aria-label="Select agent">
                    <SelectValue placeholder={t("scheduler.select_agent")} />
                  </SelectTrigger>
                  <SelectContent>
                    {agents.map(a => (
                      <SelectItem key={a.id} value={a.id}>{a.hostname} ({a.ip})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs mb-1">{t("scheduler.task_type")}</Label>
                <Select value={taskType} onValueChange={(v) => setTaskType(v ?? "shell")}>
                  <SelectTrigger aria-label="Select task type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {taskTypes.map(tt => <SelectItem key={tt.type} value={tt.type}>{tt.name || tt.type}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs mb-1">{t("scheduler.schedule")}</Label>
                <Input value={schedule} onChange={e => setSchedule(e.target.value)} placeholder={t("scheduler.schedule_ph")} aria-label="Schedule expression" />
                <p className="text-(--font-size-xs-sm) text-muted-foreground mt-1 space-y-0.5">
                  <code className="px-1 bg-muted rounded text-(--font-size-micro-sm)">every N minutes</code> · <code className="px-1 bg-muted rounded text-(--font-size-micro-sm)">daily HH:MM</code> · <code className="px-1 bg-muted rounded text-(--font-size-micro-sm)">hourly</code>
                </p>
              </div>
            </div>
            <div className="mb-4">
              <Label className="text-xs mb-1">{t("scheduler.command")}</Label>
              <Input value={command} onChange={e => setCommand(e.target.value)} placeholder="whoami" aria-label="Command" />
            </div>
            <div className="mb-4">
              <Label className="text-xs mb-1">{t("scheduler.params")}</Label>
              <Textarea value={params} onChange={e => setParams(e.target.value)} rows={2} aria-label="Params JSON" />
            </div>
            <div className="flex gap-2">
              <Button onClick={handleSave}>{editingId ? t("autotag.update") : t("autotag.create")}</Button>
              <Button variant="outline" onClick={() => { setShowForm(false); resetForm(); }}>{t("common.cancel")}</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="space-y-2">
          {[1, 2, 3].map(i => (
            <Card key={i} className="p-4"><Skeleton className="h-12 w-full" /></Card>
          ))}
        </div>
      ) : tasks.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">
          <EmptyState icon={Clock} title={t("scheduler.empty_title")} />
        </div>
      ) : (
        <div className="space-y-2">
          {tasks.map(task => (
            <Card key={task.id} className="p-4 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30 transition-all cursor-pointer">
              <div className="flex items-center gap-4">
                <Switch checked={task.enabled} onCheckedChange={() => handleToggle(task.id)} aria-label={task.enabled ? "Disable schedule" : "Enable schedule"} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-medium text-sm">{task.name}</span>
                    <Badge variant="secondary">{task.task_type}</Badge>
                    <span className="text-(--font-size-xs-sm) text-muted-foreground">{t("scheduler.run_count")} {task.run_count}x</span>
                  </div>
                  <div className="flex flex-wrap gap-x-4 gap-y-0.5 text-[12px] text-muted-foreground mt-0.5">
                    <span><Bug className="w-4 h-4" />{agents.find(a => a.id === task.agent_id)?.hostname || task.agent_id.slice(0, 8)}</span>
                    <span><Calendar className="w-4 h-4" />{task.schedule}</span>
                    {task.next_run && <span><Clock className="w-4 h-4" />{t("scheduler.next")}: {formatTime(task.next_run)}</span>}
                    {task.last_run && <span className="text-muted-foreground/70">{t("scheduler.last")}: {formatTime(task.last_run)}</span>}
                  </div>
                </div>
                <div className="flex gap-1 shrink-0">
                  <Button variant="outline" size="sm" onClick={() => editTask(task)} className="w-8 h-8 p-0" aria-label="Edit schedule">
                    <Pencil className="w-4 h-4" />
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => handleDelete(task.id)} className="w-8 h-8 p-0 border-destructive/20 text-destructive hover:bg-destructive/10" aria-label="Delete schedule">
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.confirm")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
