"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentListCached } from "@/lib/agents";
import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { CalendarClock, Plus, Trash2, Zap } from "lucide-react";

interface OneShotTask {
  id: number;
  agent_id: string;
  type: string;
  command: string;
  run_at: string;
  status: "pending" | "done" | "cancelled" | "error";
  task_id?: number;
  created_by?: string;
}

const STATUS_VARIANT: Record<string, "success" | "destructive" | "warning" | "secondary"> = {
  done: "success",
  error: "destructive",
  pending: "warning",
  cancelled: "secondary",
};

function toLocalRFC3339(localInput: string): string {
  const d = new Date(localInput);
  if (Number.isNaN(d.getTime())) return "";
  return d.toISOString();
}

export function OneShotTasksCard() {
  const { t } = useI18n();
  const { confirm, modal } = useConfirm();
  const [tasks, setTasks] = useState<OneShotTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);

  // form state
  const [agentId, setAgentId] = useState("");
  const [command, setCommand] = useState("");
  const [runAtLocal, setRunAtLocal] = useState("");
  const [agents, setAgents] = useState<Array<{ id: string; hostname?: string; status?: string }>>([]);
  const loadAgents = useCallback(async () => {
    try {
      const list = await fetchAgentListCached();
      setAgents(list.map((a) => ({ id: String(a.id || ""), hostname: a.hostname, status: a.status })));
    } catch {
      setAgents([]);
    }
  }, []);
  useEffect(() => { void loadAgents(); }, [loadAgents]);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      const d = await api.get<{ tasks?: OneShotTask[] }>(paths.scheduler.oneshotList);
      setTasks(d.tasks || []);
    } catch {
      setTasks([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    const id = setInterval(load, 30000);
    return () => clearInterval(id);
  }, [load]);

  const handleCreate = async () => {
    if (!agentId || !command.trim()) {
      toast.error(t("oneshot.toast_missing"));
      return;
    }
    setSaving(true);
    try {
      await api.post(paths.scheduler.oneshotList, {
        agent_id: agentId,
        command: command.trim(),
        run_at: runAtLocal ? toLocalRFC3339(runAtLocal) : "",
      });
      toast.success(t("oneshot.toast_created"));
      setShowForm(false);
      setCommand("");
      setRunAtLocal("");
      load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("oneshot.toast_create_failed"));
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = async (task: OneShotTask) => {
    if (!(await confirm({ message: t("oneshot.confirm_cancel", { id: String(task.id) }) }))) return;
    try {
      await api.del(paths.scheduler.oneshot(task.id));
      toast.success(t("oneshot.toast_cancelled"));
      load();
    } catch {
      toast.error(t("oneshot.toast_cancel_failed"));
    }
  };

  return (
    <Card className="p-(--card-spacing) mt-4">
      {modal}
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-semibold text-foreground flex items-center gap-2">
          <Zap className="size-4" />{t("oneshot.title")}
        </span>
        <Button size="sm" onClick={() => setShowForm(!showForm)} className="gap-1.5">
          <Plus className="size-4" />{t("oneshot.new")}
        </Button>
      </div>

      {showForm && (
        <div className="rounded-lg border border-border bg-muted/40 p-3 space-y-3 mb-3">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <Label className="text-xs text-muted-foreground">{t("oneshot.agent")}</Label>
              <select
                value={agentId}
                onChange={(e) => setAgentId(e.target.value)}
                className="mt-1 w-full h-9 rounded-lg border border-border bg-transparent px-2 text-sm"
              >
                <option value="">{t("oneshot.select_agent")}</option>
                {agents.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.hostname || a.id.slice(0, 8)} ({a.status})
                  </option>
                ))}
              </select>
            </div>
            <div>
              <Label className="text-xs text-muted-foreground">{t("oneshot.run_at")}</Label>
              <Input
                type="datetime-local"
                value={runAtLocal}
                onChange={(e) => setRunAtLocal(e.target.value)}
                className="mt-1"
              />
              <span className="block text-(--fs-micro-sm) text-muted-foreground mt-0.5">
                {t("oneshot.run_at_hint")}
              </span>
            </div>
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">{t("auto.command")}</Label>
            <Input value={command} onChange={(e) => setCommand(e.target.value)} placeholder="mimikatz # sekurlsa::logonpasswords" className="mt-1 font-mono" />
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setShowForm(false)}>{t("common.cancel")}</Button>
            <Button size="sm" onClick={handleCreate} disabled={saving} className="gap-1.5">
              {saving ? <Spinner size="xs" /> : <CalendarClock className="size-4" />}{t("oneshot.schedule_btn")}
            </Button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="py-6 text-center"><Spinner /></div>
      ) : tasks.length === 0 ? (
        <EmptyState icon={CalendarClock} title={t("oneshot.empty_title")} message={t("oneshot.empty_desc")} />
      ) : (
        <div className="divide-y divide-border max-h-80 overflow-y-auto">
          {tasks.map((st) => (
            <div key={st.id} className="py-2.5 flex items-center gap-3 flex-wrap text-sm">
              <Badge variant={STATUS_VARIANT[st.status] || "secondary"} className="shrink-0">{st.status}</Badge>
              <span className="font-medium text-foreground truncate max-w-[200px]">{st.agent_id.slice(0, 12)}</span>
              <code className="text-xs font-mono text-muted-foreground truncate flex-1 min-w-[120px]">{st.command}</code>
              <span className="text-(--fs-micro-sm) text-muted-foreground shrink-0" title={formatTime(st.run_at)}>
                {formatTime(st.run_at)}
              </span>
              {st.status === "pending" && (
                <Button variant="ghost" size="icon-xs" onClick={() => handleCancel(st)} aria-label={t("common.cancel")} className="text-destructive shrink-0">
                  <Trash2 className="size-3.5" />
                </Button>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}
