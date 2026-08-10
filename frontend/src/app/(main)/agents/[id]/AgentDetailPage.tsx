"use client";

import { useEffect, useState, useMemo, useRef, memo } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadText } from "@/lib/download";
import { ConfirmModal, Spinner } from "@/components/UI";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useWS } from "@/lib/wsContext";
import { timeAgo } from "@/lib/utils";
import { AgentStatus, AgentDetail as AgentDetailExt, AgentDetailData, AgentTaskRecord } from "@/types/agent";
import AgentHeader from "./_components/AgentHeader";
import AgentStatsGrid from "./_components/AgentStatsGrid";
import AgentTaskList from "./_components/AgentTaskList";
import AgentScreenshots from "./_components/AgentScreenshots";
import AgentChildList from "./_components/AgentChildList";
import AgentTaskOverview from "./_components/AgentTaskOverview";
import EvasionSection from "./_components/EvasionSection";
import {
  buildAgentCopyText,
  buildAgentMarkdown,
  computeActivityBuckets,
  computeHealthScore,
  computeSparklinePoints,
  type AgentDetailModel,
  type AgentDetailResponse,
  type LogEntry,
  type TaskEntry,
} from "./_components/agent-detail-utils";
import { useAgentDetail } from "./_hooks/useAgentDetail";
import { useAgentScreenshots } from "./_hooks/useAgentScreenshots";
import { useAgentQuickShell } from "./_hooks/useAgentQuickShell";
import { useAgentProcessTree } from "./_hooks/useAgentProcessTree";
import { useAgentNotes } from "./_hooks/useAgentNotes";
import { useAgentDangerActions } from "./_hooks/useAgentDangerActions";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";
import { Bug, Check, ChevronDown, Circle, Eye, History, Pencil, Send, Tag, Terminal, X } from "lucide-react";

interface AgentDetailPageProps {
  agentId?: string;
  onClose?: () => void;
}

export default memo(function AgentDetailPage({ agentId: agentIdProp, onClose }: AgentDetailPageProps = {}) {
  const { t } = useI18n();
  const params = useParams();
  const router = useRouter();
  const id = agentIdProp || (params?.id as string);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const [confirmUninstall, setConfirmUninstall] = useState(false);
  const [confirmKill, setConfirmKill] = useState(false);
  const [confirmKillDate, setConfirmKillDate] = useState(false);
  const [killDateValue, setKillDateValue] = useState("");
  const [confirmClearKillDate, setConfirmClearKillDate] = useState(false);
  const [confirmMigrate, setConfirmMigrate] = useState(false);
  const [migratePath, setMigratePath] = useState("");
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [expandedTask, setExpandedTask] = useState<number | null>(null);
  const [lbOpen, setLbOpen] = useState(false);
  const [lbIndex, setLbIndex] = useState(0);
  const [childrenExpanded, setChildrenExpanded] = useState(false);
  const [agentAge, setAgentAge] = useState("");
  const {
    command: shellCommand,
    setCommand: setShellCommand,
    shell: shellInterpreter,
    setShell: setShellInterpreter,
    history: shellHistory,
    sending: shellSending,
    expanded: shellExpanded,
    setExpanded: setShellExpanded,
    sendCommand: sendShellCommand,
  } = useAgentQuickShell(id, t("agents.detail_command_sent"), t("agents.detail_command_send_failed"));

  const [sleepValue, setSleepValue] = useState(0);
  const [jitterValue, setJitterValue] = useState(0);
  const [sleepSaving, setSleepSaving] = useState(false);

  const [credCount, setCredCount] = useState<number | null>(null);

  // healthScore is computed via useMemo below

  const [lastResultExpanded, setLastResultExpanded] = useState(false);
  const { data, setData, loading, loadError, reload: loadDetail } = useAgentDetail<AgentDetailResponse>(id);
  const { screenshots, reload: loadScreenshots } = useAgentScreenshots(id);
  const { processList, loading: processLoading, expanded: processExpanded, load: loadProcessList } = useAgentProcessTree(
    id,
    t("agents.detail_no_data"),
    t("agents.detail_process_load_failed"),
  );
  const {
    editing: editingNote,
    tags: editTags,
    setTags: setEditTags,
    notes: editNotes,
    setNotes: setEditNotes,
    saving: savingNote,
    startEditing: startEditNotes,
    cancelEditing: cancelEditNotes,
    save: handleSaveNote,
  } = useAgentNotes(id, loadDetail, t("agents.detail_notes_save_failed"));
  const { busy: dangerBusy, killAgent: runKillAgent, uninstallAgent: runUninstallAgent, migrateAgent: runMigrateAgent, setKillDate: runSetKillDate, clearKillDate: runClearKillDate } = useAgentDangerActions(id, loadDetail, {
    killSuccess: t("agents.detail_kill_sent"),
    killFailed: t("agents.detail_kill_failed"),
    uninstallSuccess: t("agents.detail_uninstall_sent"),
    uninstallFailed: t("agents.detail_uninstall_failed"),
    killDateSuccess: t("agents.detail_kill_date_set"),
    killDateFailed: t("agents.detail_kill_date_set_failed"),
    clearKillDateSuccess: t("agents.detail_kill_date_cleared"),
    clearKillDateFailed: t("agents.detail_kill_date_clear_failed"),
    migrateSuccess: t("agents.detail_migrate_sent"),
    migrateFailed: t("agents.detail_migrate_failed"),
  });

  const { subscribe } = useWS();
  useEffect(() => {
    if (!id) return;
    const unsub = subscribe((msg) => {
      if (msg.type === "agent_online" || msg.type === "agent_offline") {
        if (String(msg.agent_id) === id) loadDetail();
      } else if (msg.type === "agent_data_update" && String(msg.agent_id) === id) {
        setData((prev) => prev ? { ...prev, agent: { ...(prev.agent || {}), ...((msg.data || {}) as Partial<AgentDetailModel>) } } : prev);
      } else if (msg.type === "task_update" && String(msg.agent_id) === id) {
        loadDetail();
        loadScreenshots();
      }
    });
    return () => unsub();
  }, [subscribe, id, loadDetail, loadScreenshots, setData]);

  useEffect(() => {
    if (!lbOpen) return;
    const h = (e: KeyboardEvent) => {
      if (e.key === "Escape") setLbOpen(false);
      else if (e.key === "ArrowLeft") setLbIndex((i) => Math.max(0, i - 1));
      else if (e.key === "ArrowRight") setLbIndex((i) => Math.min(screenshots.length - 1, i + 1));
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [lbOpen, screenshots]);

  useEffect(() => {
    if (!data?.agent) return;
    const created = data.agent.created_at;
    if (!created) return;
    const tick = () => {
      const ms = Date.now() - new Date(created).getTime();
      const d = Math.floor(ms / 86400000);
      const h = Math.floor((ms % 86400000) / 3600000);
      const m = Math.floor((ms % 3600000) / 60000);
      setAgentAge(d > 0 ? `${d}d ${h}h` : h > 0 ? `${h}h ${m}m` : `${m}m`);
    };
    tick();
    const iv = setInterval(tick, 60000);
    return () => clearInterval(iv);
  }, [data?.agent?.created_at, data?.agent]);

  useEffect(() => {
    const handleKeydown = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
      if (lbOpen) return;
      if (onCloseRef.current && e.key === "Escape") { onCloseRef.current(); return; }
      if (e.key === "s") router.push(`/agents/${id}/shell`);
      else if (e.key === "f") router.push(`/agents/${id}/files`);
      else if (e.key === "d") router.push(`/agents/${id}/screen`);
      else if (e.key === "Escape") router.push("/agents");
    };
    window.addEventListener("keydown", handleKeydown);
    return () => window.removeEventListener("keydown", handleKeydown);
  }, [router, id, lbOpen]);

  useEffect(() => {
    if (!id) return;
    const controller = new AbortController();
    api.get(paths.credentials.byAgent(id, 1), { signal: controller.signal })
      .then((r: { total?: number }) => { if (r && typeof r.total === "number") setCredCount(r.total); })
      .catch((e) => { if (e.name !== 'AbortError') toast.error(t("agents.detail_creds_load_failed")); });
    return () => controller.abort();
  }, [id, t]);

  const healthScore = useMemo(() => {
    if (!data) return 0;
    const agent = data.agent || {};
    const tasks = data.tasks || [];
    const s = agent.status || "offline";
    const sr = data.success_rate ?? 0;
    const ls = agent.last_seen || "";
    return computeHealthScore(s, sr, ls, tasks);
  }, [data]);

  useEffect(() => {
    if (!data?.agent) return;
    const interval = data.agent.current_interval ?? 0;
    const jitter = data.agent.current_jitter ?? 0;
    setSleepValue(interval);
    setJitterValue(jitter);
  }, [data?.agent]);

  const agent = data?.agent || ({} as AgentDetailModel);
  const tasks: TaskEntry[] = useMemo(() => data?.tasks || [], [data?.tasks]);
  const status = (agent.status || "offline") as AgentStatus;
  const rawTags = agent.tags || "";
  const tagsList = rawTags ? rawTags.split(",").map((tag) => tag.trim()).filter(Boolean) : [];
  const note = agent.note || "";
  const childAgents: AgentDetailModel[] = data?.children || [];
  const logs: LogEntry[] = data?.logs || [];
  const totalTasks = data?.total_tasks ?? tasks.length;
  const completedTasks = data?.completed_tasks ?? 0;
  const pendingTasks = data?.pending_tasks ?? 0;
  const failedTasks = data?.failed_tasks ?? 0;
  const successRate = data?.success_rate ?? 0;
  const avgResponseTime = data?.avg_response_time || "N/A";

  const typeBreakdown = useMemo(() => {
    if (!data) return [];
    return [
      { label: "Shell", count: data.shell_tasks ?? 0, color: "bg-primary/100" },
      { label: "Screenshot", count: data.screenshot_tasks ?? 0, color: "bg-cyan-500" },
      { label: "Process", count: data.ps_tasks ?? 0, color: "bg-muted-foreground" },
      { label: "Kill", count: data.kill_tasks ?? 0, color: "bg-red-500" },
    ].filter((item) => item.count > 0);
  }, [data]);

  const otherTasks = totalTasks - (data?.shell_tasks ?? 0) - (data?.screenshot_tasks ?? 0) - (data?.ps_tasks ?? 0) - (data?.kill_tasks ?? 0);

  const { activityBuckets, maxActivity } = useMemo(
    () => (data ? computeActivityBuckets(tasks, Date.now()) : { activityBuckets: Array.from({ length: 24 }, () => 0), maxActivity: 1 }),
    [tasks, data],
  );

  const sparklinePoints = useMemo(
    () => (data ? computeSparklinePoints(tasks) : []),
    [tasks, data],
  );

  const lastResultSnippet = useMemo(() => {
    if (!data) return "";
    const lastCompletedTask = tasks.find((task) => (task.status) === "completed" && (task.result));
    return lastCompletedTask ? (lastCompletedTask.result || "").substring(0, 500) : "";
  }, [tasks, data]);

   const quickAction = async (action: string, label: string) => {
     setActionLoading(action);
      try {       await api.postJson(paths.agents.command(id), { type: action, command: "" }); toast.success(t("agents.detail_action_sent").replace("{label}", label)); }
     catch (err) { console.error("quickAction failed:", err); toast.error(t("agents.detail_action_failed").replace("{label}", label)); } finally { setActionLoading(null); }
   };

  const handleApplySleep = async () => {
    setSleepSaving(true);
    try {
      await api.postJson(paths.agents.setSleep(id), { interval: Number(sleepValue), jitter: Number(jitterValue) });
      toast.success(t("agents.sleep_updated").replace("{name}", agent?.hostname || ""));
      loadDetail();
    } catch {
      toast.error(t("agents.sleep_failed"));
    }
    setSleepSaving(false);
  };

  const exportJSON = () => {
    if (!data) return;
    downloadText(JSON.stringify(data, null, 2), `agent-${id}.json`, "application/json");
  };

  const exportMarkdown = () => {
    if (!data) return;
    downloadText(buildAgentMarkdown(data, healthScore), `agent-${id}.md`, "text/markdown");
  };

  const copyAllInfo = async () => {
    if (!data) return;
    try {
      await navigator.clipboard.writeText(buildAgentCopyText(data));
      toast.success(t("agents.detail_copied"));
    } catch {
      toast.error(t("agents.detail_copy_failed"));
    }
  };

  if (loading) {
    return (
      <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">        <div className="space-y-4">
          <Skeleton className="h-4 w-24" />
          <Card className="p-4 sm:p-5"><div className="flex items-center gap-4">
            <Skeleton className="w-14 h-14 rounded-xl" />
            <div className="space-y-2"><Skeleton className="h-5 w-40" /><Skeleton className="h-3 w-60" /></div>
          </div></Card>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">{[1,2,3,4].map((n) => (<Card key={n} className="p-4"><Skeleton className="h-3 w-16 mb-2" /><Skeleton className="h-4 w-24" /></Card>))}</div>
        </div>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
        <div className="text-center py-20">
          <Bug className="w-4 h-4" />
          <h2 className="text-xl font-semibold tracking-tight text-foreground leading-tight mb-2">{loadError ? t("agents.detail_load_failed") : t("agents.detail_not_found")}</h2>
          <p className="text-sm text-muted-foreground mb-6">{loadError ? t("agents.detail_load_error_msg") : t("agents.detail_not_found_msg")}</p>
          <div className="flex items-center justify-center gap-3">
            {loadError && <Button variant="default" onClick={() => loadDetail()}>{t("agents.detail_retry")}</Button>}
            {onClose ? (
              <Button variant="default" onClick={onClose}>{t("agents.detail_back_to_agents")}</Button>
            ) : (
              <Button render={<Link href="/agents" />}>{t("agents.detail_back_to_agents")}</Button>
            )}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <AgentHeader
        agent={agent as Partial<AgentDetailExt>}
        agentId={id}
        agentAge={agentAge}
        status={status}
        actionLoading={actionLoading}
        onQuickAction={quickAction}
        credCount={credCount}
        onKill={() => setConfirmKill(true)}
        onUninstall={() => setConfirmUninstall(true)}
        onMigrate={() => { setMigratePath(""); setConfirmMigrate(true); }}
        onClose={onClose}
      />

      <Collapsible open={childrenExpanded} onOpenChange={setChildrenExpanded}>
      <AgentStatsGrid
        agent={agent as Partial<AgentDetailExt>}
        data={data as AgentDetailData}
        healthScore={healthScore}
        activityBuckets={activityBuckets}
        maxActivity={maxActivity}
        sleepValue={sleepValue}
        jitterValue={jitterValue}
        onSleepChange={setSleepValue}
        onJitterChange={setJitterValue}
        onApplySleep={handleApplySleep}
        sleepSaving={sleepSaving}
        status={status}
        childAgents={childAgents}
        childrenExpanded={childrenExpanded}
        onToggleChildren={() => setChildrenExpanded(!childrenExpanded)}
        onExportJSON={exportJSON}
        onExportMarkdown={exportMarkdown}
        onCopyAllInfo={copyAllInfo}
        killDate={agent?.kill_date}
        onSetKillDate={() => {
          const kd = agent?.kill_date;
          if (kd) setKillDateValue(kd.substring(0, 10));
          else {
            const tomorrow = new Date();
            tomorrow.setDate(tomorrow.getDate() + 1);
            setKillDateValue(tomorrow.toISOString().substring(0, 10));
          }
          setConfirmKillDate(true);
        }}
        onClearKillDate={() => setConfirmClearKillDate(true)}
      />

      <CollapsibleContent>
        <AgentChildList childAgents={childAgents} />
      </CollapsibleContent>
      </Collapsible>

      <AgentTaskOverview
        totalTasks={totalTasks}
        completedTasks={completedTasks}
        pendingTasks={pendingTasks}
        failedTasks={failedTasks}
        successRate={successRate}
        avgResponseTime={avgResponseTime}
        typeBreakdown={typeBreakdown}
        otherTasks={otherTasks}
        sparklinePoints={sparklinePoints}
      />

      {lastResultSnippet && (
        <Collapsible open={lastResultExpanded} onOpenChange={setLastResultExpanded}>
          <Card className="mb-5">
            <div className="px-5 py-3.5 border-b border-border flex items-center justify-between">
              <h3 className="text-sm font-semibold text-foreground"><Eye className="w-3.5 h-3.5" />{t("agents.detail_last_result")}</h3>
              <CollapsibleTrigger>
                <Button variant="ghost" size="sm"
                  className="text-xs h-auto p-0 text-primary hover:bg-transparent hover:underline">
                  {lastResultExpanded ? t("agents.detail_collapse") : t("agents.detail_expand")}
                </Button>
              </CollapsibleTrigger>
            </div>
            <CollapsibleContent>
              <div className="px-5 py-3">
                <pre className="font-mono text-xs text-foreground whitespace-pre-wrap break-all">{lastResultSnippet}</pre>
              </div>
            </CollapsibleContent>
          </Card>
        </Collapsible>
      )}

      <AgentTaskList
        tasks={tasks as AgentTaskRecord[]}
        agentId={id}
        expandedTaskId={expandedTask}
        onToggleExpand={(taskId) => setExpandedTask(expandedTask === taskId ? null : taskId)}
      />

      <AgentScreenshots
        screenshots={screenshots}
        agentId={id}
        lightboxIdx={lbOpen ? lbIndex : null}
        onOpenLightbox={(idx) => { setLbIndex(idx); setLbOpen(true); }}
        onCloseLightbox={() => setLbOpen(false)}
        onPrevLightbox={() => setLbIndex((i) => Math.max(0, i - 1))}
        onNextLightbox={() => setLbIndex((i) => Math.min(screenshots.length - 1, i + 1))}
      />

      <EvasionSection agentId={id} online={status === "online"} />

      <Collapsible open={processExpanded} onOpenChange={(v) => { if (v) loadProcessList(); }}>
        <Card className="mb-4">
          <div className="px-4 py-3 border-b border-border flex items-center justify-between">
            <h3 className="text-sm font-semibold text-foreground"><Terminal className="w-3.5 h-3.5" />{t("agents.detail_process_list")}</h3>
            <CollapsibleTrigger>
              <Button variant="ghost" size="sm" className="text-xs h-auto p-0 text-primary hover:bg-transparent hover:underline">{processExpanded ? t("agents.detail_hide") : t("agents.detail_load")}</Button>
            </CollapsibleTrigger>
          </div>
          <CollapsibleContent>
            <div className="p-3">{processLoading ? (<div className="flex items-center justify-center py-6"><Spinner size="md" /></div>) : (<pre className="p-3 bg-muted rounded-xl font-mono text-xs text-foreground whitespace-pre-wrap break-all max-h-64 overflow-y-auto border border-border">{processList || t("agents.detail_no_data")}</pre>)}</div>
          </CollapsibleContent>
        </Card>
      </Collapsible>

      <Card className="mb-4">
        <div className="px-4 py-3 border-b border-border flex items-center justify-between">
          <h3 className="text-sm font-semibold text-foreground"><Tag className="w-3.5 h-3.5" />{t("agents.detail_notes_tags")}</h3>
          {!editingNote ? (<Button variant="ghost" size="sm" onClick={() => startEditNotes(rawTags, note)} className="text-xs h-auto p-0 text-primary hover:bg-transparent gap-1.5"><Pencil className="w-4 h-4" /> {t("agents.detail_edit")}</Button>) : (
            <div className="flex items-center gap-2"><Button size="sm" onClick={handleSaveNote} disabled={savingNote} className="text-xs h-7 gap-1.5">{savingNote ? <Spinner size="xs" color="white" /> : <Check className="w-4 h-4" />} {t("agents.detail_save")}</Button><Button variant="ghost" size="sm" onClick={cancelEditNotes} className="text-xs h-7 text-muted-foreground gap-1.5"><X className="w-4 h-4" /> {t("agents.detail_cancel")}</Button></div>)}
        </div>
        <div className="p-4">{editingNote ? (<div className="space-y-3">
          <div><span className="block text-xs font-medium text-muted-foreground mb-1.5">{t("agents.detail_tags_hint")}</span><Input aria-label={t("agents.detail_tags")} name="agent-tags" type="text" value={editTags} onChange={(e) => setEditTags(e.target.value)} placeholder={t("agents.detail_tags_placeholder")} /></div>
          <div><span className="block text-xs font-medium text-muted-foreground mb-1.5">{t("agents.detail_notes")}</span><Textarea aria-label={t("agents.detail_notes")} name="agent-notes" value={editNotes} onChange={(e) => setEditNotes(e.target.value)} rows={3} placeholder={t("agents.detail_notes_placeholder")} /></div>
        </div>) : (<div className="space-y-3">
          <div><div className="text-xs font-medium text-muted-foreground mb-2">{t("agents.detail_tags")}</div>{tagsList.length > 0 ? (<div className="flex flex-wrap gap-1.5">{tagsList.map((tag) => { return <Link key={tag} href={`/agents?tag=${encodeURIComponent(tag)}`}><Badge variant="outline" className="cursor-pointer hover:opacity-80 transition-opacity">{tag}</Badge></Link>; })}</div>) : <span className="text-xs text-muted-foreground/70">{t("agents.detail_no_tags")}</span>}</div>
          <div><div className="text-xs font-medium text-muted-foreground mb-1">{t("agents.detail_notes")}</div>{note ? <p className="text-sm text-foreground whitespace-pre-wrap leading-relaxed">{note}</p> : <span className="text-xs text-muted-foreground/70">{t("agents.detail_no_notes")}</span>}</div>
        </div>)}</div>
      </Card>

      {logs.length > 0 && (
        <Card className="mb-4">
          <div className="px-4 py-3 border-b border-border"><h3 className="text-sm font-semibold text-foreground"><History className="w-3.5 h-3.5" />{t("agents.detail_connection_log")}</h3></div>
          <div className="max-h-64 overflow-y-auto"><div className="divide-y divide-border">{logs.map((log, i) => (
            <div key={log.id || i} className="px-4 py-2 flex items-center justify-between">
              <div className="flex items-center gap-2.5"><Circle className={`w-1.5 h-1.5 fill-current ${log.type === "online" ? "text-emerald-500" : log.type === "offline" ? "text-red-500" : "text-primary"}`} /><span className="text-xs text-foreground">{log.user || t("agents.detail_log_system")}</span>{log.message && <span className="text-xs text-muted-foreground/70">{log.message}</span>}</div>
              <span className="text-(--fs-micro-sm) text-muted-foreground/70 whitespace-nowrap">{(log.created_at) ? timeAgo(String(log.created_at), t) : ""}</span>
            </div>))}</div></div>
        </Card>
      )}

      {status === "online" && (
        <Collapsible open={shellExpanded} onOpenChange={setShellExpanded}>
          <Card className="mb-4">
            <CollapsibleTrigger className="w-full">
              <div className="px-4 py-3 border-b border-border flex items-center justify-between cursor-pointer">
                <h3 className="text-sm font-semibold text-foreground"><Terminal className="w-3.5 h-3.5" />{t("agents.detail_quick_shell")}</h3>
                <ChevronDown className="w-2.5 h-2.5 text-muted-foreground/70" />
              </div>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <div className="p-4">
                <div className="flex items-center gap-2 mb-3">
                  <Select value={shellInterpreter} onValueChange={(v) => { if (v !== null) setShellInterpreter(v); }}>
                    <SelectTrigger className="h-8 w-[160px] text-xs" aria-label={t("agents.detail_quick_shell")}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="cmd.exe">cmd.exe</SelectItem>
                      <SelectItem value="powershell.exe">powershell.exe</SelectItem>
                    </SelectContent>
                  </Select>
                  <Input type="text" value={shellCommand} onChange={(e) => setShellCommand(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") sendShellCommand(); }}
                    placeholder={t("agents.detail_enter_command")} aria-label={t("agents.detail_enter_command")}
                    className="flex-1 h-8 font-mono text-xs" />
                  <Button size="sm" onClick={sendShellCommand} disabled={shellSending || !shellCommand.trim()}
                    className="h-8 px-4 text-xs gap-1.5 shrink-0">
                    {shellSending ? <Spinner size="xs" color="white" /> : <Send className="w-4 h-4" />} {t("agents.detail_send")}
                  </Button>
                </div>
                {shellHistory.length > 0 && (
                  <div className="space-y-2 max-h-60 overflow-y-auto">
                    {shellHistory.map((entry, idx) => (
                      <div key={entry.timestamp + idx} className="p-2.5 rounded-lg bg-muted/50 border border-border">
                        <div className="flex items-center gap-2 mb-1">
                          <Badge variant="secondary" className="text-(--fs-micro-sm) font-mono">{entry.shell}</Badge>
                          <span className="text-xs font-mono text-foreground">{entry.command}</span>
                          <span className="text-(--fs-micro-sm) text-muted-foreground/70 ml-auto">{timeAgo(entry.timestamp, t)}</span>
                        </div>
                        <pre className="font-mono text-(--fs-micro-sm) text-muted-foreground whitespace-pre-wrap break-all max-h-20 overflow-y-auto">{entry.result}</pre>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </CollapsibleContent>
          </Card>
        </Collapsible>
      )}

      <ConfirmModal open={confirmKill} title={t("agents.kill_agent")} message={t("agents.kill_msg")} confirmText={t("agents.kill")} danger onConfirm={async () => { await runKillAgent(); setConfirmKill(false); }} onCancel={() => setConfirmKill(false)} />
      <ConfirmModal open={confirmUninstall} title={t("agents.uninstall_agent")} message={t("agents.uninstall_msg")} confirmText={t("agents.uninstall")} danger onConfirm={async () => { await runUninstallAgent(); setConfirmUninstall(false); }} onCancel={() => setConfirmUninstall(false)} />
      {confirmKillDate && (
        <Dialog open={confirmKillDate} onOpenChange={(v) => !v && setConfirmKillDate(false)}>
          <DialogContent className="w-80 gap-0">
            <DialogHeader>
              <DialogTitle>{t("agents.detail_set_kill_date")}</DialogTitle>
            </DialogHeader>
            <p className="text-xs text-muted-foreground mb-3">{t("agents.detail_kill_date_msg")}</p>
            <Input type="date" value={killDateValue} onChange={(e) => setKillDateValue(e.target.value)} className="mb-3 text-sm" />
            <DialogFooter>
              <Button variant="ghost" size="sm" onClick={() => setConfirmKillDate(false)} disabled={!!dangerBusy}>{t("agents.detail_cancel")}</Button>
              <Button variant="destructive" size="sm" onClick={async () => { await runSetKillDate(killDateValue); setConfirmKillDate(false); }} disabled={!!dangerBusy}>{t("agents.detail_set_kill_date")}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
      <ConfirmModal open={confirmClearKillDate} title={t("agents.detail_clear_kill_date")} message={t("agents.detail_clear_kill_date_msg")} confirmText={t("agents.detail_clear")} danger onConfirm={async () => { await runClearKillDate(); setConfirmClearKillDate(false); }} onCancel={() => setConfirmClearKillDate(false)} />
      {confirmMigrate && (
        <Dialog open={confirmMigrate} onOpenChange={(v) => !v && setConfirmMigrate(false)}>
          <DialogContent className="w-96 gap-0">
            <DialogHeader>
              <DialogTitle>{t("agents.migrate_agent")}</DialogTitle>
            </DialogHeader>
            <p className="text-xs text-muted-foreground mb-3">{t("agents.migrate_msg")}</p>
            <span className="block text-xs font-medium text-muted-foreground mb-1.5">{t("agents.migrate_path_label")}</span>
            <Input
              type="text"
              value={migratePath}
              onChange={(e) => setMigratePath(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") { runMigrateAgent(migratePath); setConfirmMigrate(false); } }}
              placeholder={t("agents.migrate_path_placeholder")}
              className="mb-3 text-sm font-mono"
            />
            <DialogFooter>
              <Button variant="ghost" size="sm" onClick={() => setConfirmMigrate(false)} disabled={!!dangerBusy}>{t("agents.detail_cancel")}</Button>
              <Button variant="destructive" size="sm" onClick={async () => { await runMigrateAgent(migratePath); setConfirmMigrate(false); }} disabled={!!dangerBusy}>{t("agents.migrate")}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
})
