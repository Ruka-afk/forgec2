"use client";

import { useCallback, useEffect, useState, useMemo, useRef, memo } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api, ApiError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadText } from "@/lib/download";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { PageContainer } from "@/components/ui/page-container";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { logger } from "@/lib/logger";
import { useInteractStore } from "@/lib/interact-store";
import { Collapsible, CollapsibleContent } from "@/components/ui/collapsible";
import { Bug } from "lucide-react";
import { Banner } from "@/components/ui/banner";
import { AgentStatus, AgentDetail as AgentDetailExt, AgentTaskRecord } from "@/types/agent";
import AgentHeader from "./_components/AgentHeader";
import AgentStatsGrid from "./_components/AgentStatsGrid";
import AgentTaskList from "./_components/AgentTaskList";
import AgentScreenshots from "./_components/AgentScreenshots";
import AgentChildList from "./_components/AgentChildList";
import AgentStatusBar from "./_components/AgentStatusBar";
import QuickShellSection from "./_components/QuickShellSection";
import { AISuggestCard } from "./_components/AISuggestCard";
import NotesTagsSection from "./_components/NotesTagsSection";
import ProcessSection from "./_components/ProcessSection";
import ConnectionLogSection from "./_components/ConnectionLogSection";
import EvasionSection from "./_components/EvasionSection";
import InjectSection from "./_components/InjectSection";
import TimelineSection from "./_components/TimelineSection";
import { HostInfoCard } from "./_components/HostInfoCard";
import {
  buildAgentCopyText,
  buildAgentMarkdown,
  toLocalDateInput,
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
import { useAgentTaskSync } from "./_hooks/useAgentTaskSync";
import { usePersistedState } from "@/lib/hooks/usePersistedState";
import { credActionBlockReason, credActionEndpoint, hasMimikatzModule, parseModuleNames } from "../../credentials/_components/cred-quality";
import { sessionActionQuality } from "./_components/session-quality";
import { implantBlocksDest } from "../_components/implant-version";

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
  const [expandedTask, setExpandedTask] = useState<string | number | null>(null);
  const [lbOpen, setLbOpen] = useState(false);
  const [lbIndex, setLbIndex] = useState(0);
  const lbOpenRef = useRef(false);
  lbOpenRef.current = lbOpen;
  const confirmOpen = confirmUninstall || confirmKill || confirmKillDate || confirmClearKillDate || confirmMigrate;
  const confirmOpenRef = useRef(confirmOpen);
  confirmOpenRef.current = confirmOpen;
  const [childrenExpanded, setChildrenExpanded] = usePersistedState(`agents.detail.${id}.children`, false);

  const { data, setData, loading, loadError, reload: loadDetail, reloadThrottled } = useAgentDetail<AgentDetailResponse>(id);
  const status = (data?.agent?.status || "offline") as AgentStatus;
  const { screenshots, newScreenshots } = useAgentScreenshots(id, status === "online");
  useAgentTaskSync(id, status === "online", setData, reloadThrottled);

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
  } = useAgentQuickShell(id, data?.agent?.os, t("agents.detail_command_sent"), t("agents.detail_command_send_failed"), `agents.detail.${id}.quick_shell`);

  const [sleepValue, setSleepValue] = useState(0);
  const [jitterValue, setJitterValue] = useState(0);
  const [sleepSaving, setSleepSaving] = useState(false);
  const sleepDirtyRef = useRef(false);

  // Route reuse: /agents/[id] keeps the same component instance when
  // navigating parent → child, so per-agent editor state (dirty flag,
  // expanded task, etc.) must reset on id change — otherwise agent A's
  // unsent sleep edits get POSTed to agent B.
  useEffect(() => {
    sleepDirtyRef.current = false;
    setExpandedTask(null);
  }, [id]);

  // Lightbox index clamp: screenshots refresh (WS push / deletion) can
  // shrink the array while the viewer is open — without this, lbIndex
  // pointed past the end and rendered a blank stage with a wrong counter.
  useEffect(() => {
    if (!lbOpen || lbIndex === null) return;
    if (!lbOpen) return;
    const count = screenshots.length;
    if (count === 0) {
      setLbOpen(false);
    } else if (lbIndex > count - 1) {
      setLbIndex(count - 1);
    }
  }, [lbOpen, lbIndex, screenshots.length]);

  const [credCount, setCredCount] = useState<number | null>(null);
  const [mimikatzReady, setMimikatzReady] = useState(false);

  const { processList, loading: processLoading, loadFailed, expanded: processExpanded, setExpanded: setProcessExpanded, load: loadProcessList, refresh: refreshProcessList } = useAgentProcessTree(
    id,
    t("agents.detail_no_data"),
    t("agents.detail_process_load_failed"),
    `agents.detail.${id}.process`,
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

  useEffect(() => {
    if (!lbOpen) return;
    const h = (e: KeyboardEvent) => {
      if (e.key === "Escape") setLbOpen(false);
      else if (e.key === "ArrowLeft") setLbIndex((i) => Math.max(0, i - 1));
      else if (e.key === "ArrowRight") setLbIndex((i) => Math.min(screenshots.length - 1, i + 1));
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [lbOpen, screenshots.length]);

  useEffect(() => {
    const handleKeydown = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
      if (lbOpenRef.current) return;
      if (confirmOpenRef.current) return;
      if (onCloseRef.current && e.key === "Escape") { onCloseRef.current(); return; }
      if (e.key === "s") router.push(`/agents/${id}/shell`);
      else if (e.key === "f") router.push(`/agents/${id}/files`);
      else if (e.key === "d") router.push(`/agents/${id}/screen`);
      else if (e.key === "Escape") router.push("/agents");
    };
    window.addEventListener("keydown", handleKeydown);
    return () => window.removeEventListener("keydown", handleKeydown);
  }, [router, id]);

  useEffect(() => {
    if (!id) return;
    const controller = new AbortController();
    api.get(paths.credentials.byAgent(id, 1), { signal: controller.signal })
      .then((r: { total?: number }) => { if (r && typeof r.total === "number") setCredCount(r.total); })
      .catch((e) => { if (e.name !== 'AbortError') toast.error(t("agents.detail_creds_load_failed")); });
    return () => controller.abort();
  }, [id, t]);

  useEffect(() => {
    const controller = new AbortController();
    api.get(paths.modules.list, { signal: controller.signal })
      .then((r) => setMimikatzReady(hasMimikatzModule(parseModuleNames(r))))
      .catch(() => setMimikatzReady(false));
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (!data?.agent || sleepDirtyRef.current) return;
    setSleepValue(data.agent.current_interval ?? 0);
    setJitterValue(data.agent.current_jitter ?? 0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data?.agent?.current_interval, data?.agent?.current_jitter]);

  const agent = data?.agent || ({} as AgentDetailModel);
  const tasks: TaskEntry[] = useMemo(() => data?.tasks || [], [data?.tasks]);
  const rawTags = agent.tags || "";
  const tagsList = useMemo(() => (rawTags ? rawTags.split(",").map((tag) => tag.trim()).filter(Boolean) : []), [rawTags]);
  const note = agent.note || "";
  const handleStartEditNotes = useCallback(() => startEditNotes(rawTags, note), [startEditNotes, rawTags, note]);
  const childAgents: AgentDetailModel[] = data?.children || [];
  const logs: LogEntry[] = data?.logs || [];
  const totalTasks = data?.total_tasks ?? tasks.length;
  const completedTasks = data?.completed_tasks ?? 0;
  const pendingTasks = data?.pending_tasks ?? 0;
  const failedTasks = data?.failed_tasks ?? 0;

  const quickAction = useCallback(
    async (action: string, label: string) => {
      if (credActionBlockReason(action, mimikatzReady) === "missing_module") {
        toast.error(t("cred.missing_module"));
        return;
      }
      if (implantBlocksDest(agent.version, sessionActionQuality(action))) {
        toast.error(t("agents.version_unknown_dest"));
        return;
      }
      setActionLoading(action);
      try {
        const suffix = credActionEndpoint(action);
        const data = suffix
          ? await api.post(paths.agents.cmd(id, suffix), {})
          : await api.postJson(paths.agents.command(id), { type: action, command: "" });
        const taskId = Number((data as { task_id?: number }).task_id);
        const queued = Number.isFinite(taskId) && taskId > 0;
        const kind = queued ? useInteractStore.getState().revealTask(id, taskId) : "offer";
        toast.success(t("agents.detail_action_queued", { label }), {
          action: kind === "offer" && queued
            ? {
                label: t("agents.dock_view_result"),
                onClick: () => useInteractStore.getState().open(id, { tab: "tasks", expandedTaskId: taskId }),
              }
            : undefined,
        });
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          toast.warning(err.message);
        } else {
          logger.error("quickAction failed", err);
          toast.error(t("agents.detail_action_failed").replace("{label}", label));
        }
      } finally {
        setActionLoading(null);
      }
    },
    [id, t, mimikatzReady, agent.version],
  );

  const handleApplySleep = useCallback(async () => {
    // Same bounds as the agents-list quick-sleep path: HTML min/max don't
    // stop typing, and Number("") === 0 would silently flip a beacon toward
    // real-time cadence.
    const interval = Number(sleepValue);
    const jitter = Number(jitterValue);
    if (!Number.isFinite(interval) || interval < 1 || interval > 86400 ||
        !Number.isFinite(jitter) || jitter < 0 || jitter > 100) {
      toast.error(t("agents.sleep_invalid"));
      return;
    }
    setSleepSaving(true);
    try {
      await api.postJson(paths.agents.setSleep(id), { interval, jitter });
      toast.success(t("agents.sleep_updated").replace("{name}", agent?.hostname || ""));
      sleepDirtyRef.current = false;
      loadDetail();
    } catch {
      toast.error(t("agents.sleep_failed"));
    }
    setSleepSaving(false);
  }, [id, t, sleepValue, jitterValue, loadDetail, agent?.hostname]);

  const exportJSON = useCallback(() => {
    if (!data) return;
    downloadText(JSON.stringify(data, null, 2), `agent-${id}.json`, "application/json");
  }, [data, id]);

  const exportMarkdown = useCallback(() => {
    if (!data) return;
    downloadText(buildAgentMarkdown(data), `agent-${id}.md`, "text/markdown");
  }, [data, id]);

  const copyAllInfo = useCallback(async () => {
    if (!data) return;
    try {
      await navigator.clipboard.writeText(buildAgentCopyText(data));
      toast.success(t("agents.detail_copied"));
    } catch {
      toast.error(t("agents.detail_copy_failed"));
    }
  }, [data, t]);

  const onToggleExpand = useCallback((taskId: string | number) => {
    setExpandedTask((cur) => (String(cur ?? "") === String(taskId) ? null : taskId));
  }, []);

  const onOpenLightbox = useCallback((idx: number) => {
    setLbIndex(idx);
    setLbOpen(true);
  }, []);

  const onCloseLightbox = useCallback(() => setLbOpen(false), []);
  const onPrevLightbox = useCallback(() => setLbIndex((i) => Math.max(0, i - 1)), []);
  const onNextLightbox = useCallback(
    () => setLbIndex((i) => Math.min(screenshots.length - 1, i + 1)),
    [screenshots.length],
  );

  const handleToggleChildren = useCallback(() => setChildrenExpanded((v) => !v), [setChildrenExpanded]);

  const handleSetKillDate = useCallback(() => {
    const kd = agent?.kill_date;
    if (kd) {
      const d = new Date(kd);
      setKillDateValue(Number.isNaN(d.getTime()) ? kd.substring(0, 10) : toLocalDateInput(d));
    } else {
      const tomorrow = new Date();
      tomorrow.setDate(tomorrow.getDate() + 1);
      setKillDateValue(toLocalDateInput(tomorrow));
    }
    setConfirmKillDate(true);
  }, [agent?.kill_date]);

  const handleClearKillDate = useCallback(() => setConfirmClearKillDate(true), []);

  const handleToggleProcess = useCallback(
    (open: boolean) => {
      setProcessExpanded(open);
      if (open && !processLoading) loadProcessList();
    },
    [processLoading, loadProcessList, setProcessExpanded],
  );

  const handleKill = useCallback(() => setConfirmKill(true), []);
  const handleUninstall = useCallback(() => setConfirmUninstall(true), []);
  const handleMigrate = useCallback(() => {
    setMigratePath("");
    setConfirmMigrate(true);
  }, []);
  const handlePopOut = useCallback(() => useInteractStore.getState().open(id, { tab: "shell" }), [id]);
  const handleSleepChange = useCallback((v: number) => { sleepDirtyRef.current = true; setSleepValue(v); }, []);
  const handleJitterChange = useCallback((v: number) => { sleepDirtyRef.current = true; setJitterValue(v); }, []);

  if (loading) {
    return (
      <PageContainer>
        <div className="space-y-4">
          <Skeleton className="h-4 w-24" />
          <Card className="p-(--card-spacing)"><div className="flex items-center gap-4">
            <Skeleton className="size-14 rounded-lg" />
            <div className="space-y-2"><Skeleton className="h-5 w-40" /><Skeleton className="h-3 w-60" /></div>
          </div></Card>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">{[1,2,3,4].map((n) => (<Card key={n} className="p-4"><Skeleton className="h-3 w-16 mb-2" /><Skeleton className="h-4 w-24" /></Card>))}</div>
        </div>
      </PageContainer>
    );
  }

  if (!data) {
    return (
      <PageContainer>
        <div className="text-center py-20">
          <Bug className="size-4" />
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
      </PageContainer>
    );
  }

  return (
    <PageContainer className="relative">
      {loadError && (
        <Banner tone="destructive" className="mb-3" action={<Button variant="ghost" size="sm" onClick={() => loadDetail()}>{t("agents.detail_retry")}</Button>}>
          {t("agents.detail_load_error_msg")}
        </Banner>
      )}
      <AgentStatusBar agent={agent as Partial<AgentDetailExt>} agentId={id} status={status} />
      <AgentHeader
        agent={agent as Partial<AgentDetailExt>}
        agentId={id}
        status={status}
        actionLoading={actionLoading}
        onQuickAction={quickAction}
        credCount={credCount}
        mimikatzReady={mimikatzReady}
        onKill={handleKill}
        onUninstall={handleUninstall}
        onMigrate={handleMigrate}
        onPopOut={handlePopOut}
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        {/* ── Main column: live sections ── */}
        <div className="min-w-0">
          <AgentTaskList
            tasks={tasks as AgentTaskRecord[]}
            agentId={id}
            expandedTaskId={expandedTask}
            onToggleExpand={onToggleExpand}
            totalTasks={totalTasks}
            completedTasks={completedTasks}
            pendingTasks={pendingTasks}
            failedTasks={failedTasks}
          />

          <AgentScreenshots
            screenshots={screenshots}
            newScreenshots={newScreenshots}
            agentId={id}
            lightboxIdx={lbOpen ? lbIndex : null}
            onOpenLightbox={onOpenLightbox}
            onCloseLightbox={onCloseLightbox}
            onPrevLightbox={onPrevLightbox}
            onNextLightbox={onNextLightbox}
          />

          <ProcessSection
            processList={processList}
            loading={processLoading}
            loadFailed={loadFailed}
            expanded={processExpanded}
            onToggle={handleToggleProcess}
            onRefresh={refreshProcessList}
          />

          <EvasionSection agentId={id} online={status === "online"} />

          <InjectSection agentId={id} online={status === "online"} osType={agent.os} />

          <TimelineSection agentId={id} online={status === "online"} />

          <HostInfoCard agentId={id} online={status === "online"} />
        </div>

        {/* ── Right rail: reference + quick controls ── */}
        <div className="min-w-0 lg:sticky lg:top-12 lg:max-h-[calc(100vh-8rem)] lg:overflow-y-auto lg:pr-0.5">
          <AISuggestCard agentId={id} online={status === "online"} />
          <AgentStatsGrid
            agent={agent as Partial<AgentDetailExt>}
            uptime={data?.uptime}
            timeSinceLastSeen={data?.time_since_last_seen}
            sleepValue={sleepValue}
            jitterValue={jitterValue}
            onSleepChange={handleSleepChange}
            onJitterChange={handleJitterChange}
            onApplySleep={handleApplySleep}
            sleepSaving={sleepSaving}
            status={status}
            childAgents={childAgents}
            childrenExpanded={childrenExpanded}
            onToggleChildren={handleToggleChildren}
            onExportJSON={exportJSON}
            onExportMarkdown={exportMarkdown}
            onCopyAllInfo={copyAllInfo}
            killDate={agent?.kill_date}
            onSetKillDate={handleSetKillDate}
            onClearKillDate={handleClearKillDate}
            rail
          />

          <Collapsible open={childrenExpanded} onOpenChange={setChildrenExpanded}>
            <CollapsibleContent>
              <AgentChildList childAgents={childAgents} />
            </CollapsibleContent>
          </Collapsible>

          {status === "online" && (
            <QuickShellSection
              expanded={shellExpanded}
              onExpandedChange={setShellExpanded}
              shellInterpreter={shellInterpreter}
              onShellChange={setShellInterpreter}
              command={shellCommand}
              onCommandChange={setShellCommand}
              history={shellHistory}
              sending={shellSending}
              onSend={sendShellCommand}
              os={agent.os}
            />
          )}

          <NotesTagsSection
            editing={editingNote}
            tags={editTags}
            onTagsChange={setEditTags}
            notes={editNotes}
            onNotesChange={setEditNotes}
            saving={savingNote}
            onStartEdit={handleStartEditNotes}
            onCancelEdit={cancelEditNotes}
            onSave={handleSaveNote}
            displayTags={tagsList}
            note={note}
          />

          <ConnectionLogSection logs={logs} />
        </div>
      </div>

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
    </PageContainer>
  );
})