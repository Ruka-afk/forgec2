import { Fragment, memo, useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { SearchInput } from "@/components/SearchInput";
import { Spinner } from "@/components/ui/spinner";
import { SectionCard } from "@/components/ui/section-card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { useErrorToast } from "@/lib/hooks/useErrorToast";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { formatBytes, timeAgo } from "@/lib/utils";
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ChevronDown,
  ChevronRight,
  Info,
  Pause,
  Play,
  RefreshCw,
  Skull,
  Terminal,
} from "lucide-react";
import {
  filterProcessRows,
  filterProcessTree,
  flattenProcessTree,
  isProtectedPid,
  parseProcessSnapshot,
  processProtection,
  sortProcessRows,
  sortProcessTree,
  type ProcessRow,
  type ProcessSnapshotMeta,
  type ProcessSortDir,
  type ProcessSortKey,
} from "@/lib/process-snapshot";

type ProcessAction = "suspend" | "resume" | "killproc";

interface ProcessSectionProps {
  agentId: string;
  online: boolean;
  snapshot: ProcessSnapshotMeta | null;
  loading: boolean;
  collecting: boolean;
  loadFailed: boolean;
  expanded: boolean;
  /** Collapse/expand toggle from the card header. */
  onToggle: (open: boolean) => void;
  /** Re-read the last snapshot (does not queue a task). */
  onRetry?: () => void;
  /** Queue a fresh process snapshot and wait for the agent. */
  onRefresh?: () => Promise<void> | void;
}

const EMPTY_COLLAPSED: ReadonlySet<string> = new Set();

export default memo(function ProcessSection({
  agentId,
  online,
  snapshot,
  loading,
  collecting,
  loadFailed,
  expanded,
  onToggle,
  onRetry,
  onRefresh,
}: ProcessSectionProps) {
  const { t } = useI18n();
  const { confirm, modal } = useConfirm();
  const { toastError } = useErrorToast();
  const [manualPid, setManualPid] = useState("");
  const [busyByPid, setBusyByPid] = useState<Map<string, ProcessAction>>(new Map());
  const [batchBusy, setBatchBusy] = useState(false);
  const [query, setQuery] = useState("");
  const [view, setView] = useState<"tree" | "flat">("tree");
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [detailsOpen, setDetailsOpen] = useState<Set<string>>(new Set());
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [sortKey, setSortKey] = useState<ProcessSortKey>("pid");
  const [sortDir, setSortDir] = useState<ProcessSortDir>("asc");

  const parsed = useMemo(() => parseProcessSnapshot(snapshot?.text ?? ""), [snapshot?.text]);
  const rows = parsed.rows;
  const queryActive = query.trim().length > 0;

  const visibleTree = useMemo(
    () => flattenProcessTree(
      sortProcessTree(filterProcessTree(parsed.roots, query), sortKey, sortDir),
      queryActive ? EMPTY_COLLAPSED : collapsed,
    ),
    [collapsed, parsed.roots, query, queryActive, sortDir, sortKey],
  );
  const visibleFlat = useMemo(
    () => sortProcessRows(filterProcessRows(rows, query), sortKey, sortDir),
    [query, rows, sortDir, sortKey],
  );
  const visibleRows: ProcessRow[] = view === "tree" ? visibleTree : visibleFlat;

  const protectionByPid = useMemo(() => {
    const map = new Map<string, "core" | "critical">();
    for (const row of rows) {
      const reason = processProtection(row);
      if (reason && !map.has(row.pid)) map.set(row.pid, reason);
    }
    return map;
  }, [rows]);

  const selectablePids = useMemo(
    () => [...new Set(visibleRows.filter((row) => !processProtection(row)).map((row) => row.pid))],
    [visibleRows],
  );
  const allVisibleSelected = selectablePids.length > 0 && selectablePids.every((pid) => selected.has(pid));
  const someVisibleSelected = selectablePids.some((pid) => selected.has(pid));
  const selectedPids = useMemo(
    () => [...selected].filter((pid) => !isProtectedPid(pid, rows)).sort((a, b) => Number(a) - Number(b)),
    [rows, selected],
  );

  // A new agent must never carry another agent's filter, selection, expanded
  // branches, or PID draft into its process table.
  useEffect(() => {
    setManualPid("");
    setBusyByPid(new Map());
    setBatchBusy(false);
    setQuery("");
    setView("tree");
    setCollapsed(new Set());
    setDetailsOpen(new Set());
    setSelected(new Set());
    setSortKey("pid");
    setSortDir("asc");
  }, [agentId]);

  // A fresh collection can reuse PIDs for different processes. Drop selection
  // and tree state when the snapshot changes so an old checked row cannot kill
  // an unrelated process.
  useEffect(() => {
    setSelected(new Set());
  }, [snapshot?.taskId, snapshot?.collectedAt, snapshot?.text]);

  const runRefresh = async () => {
    if (!onRefresh || collecting) return;
    try {
      await onRefresh();
    } catch (error) {
      toastError(error, "agents.proc_collect_failed");
    }
  };

  const setBusy = (pid: string, action: ProcessAction | null) => {
    setBusyByPid((prev) => {
      const next = new Map(prev);
      if (action) next.set(pid, action);
      else next.delete(pid);
      return next;
    });
  };

  const queueAction = async (
    action: ProcessAction,
    pid: string,
    opts: { needConfirm: boolean; silent?: boolean },
  ): Promise<boolean> => {
    const target = pid.trim();
    if (!/^\d+$/.test(target)) {
      toast.error(t("agents.proc_invalid_pid"));
      return false;
    }
    if (action === "killproc" && isProtectedPid(target, rows)) {
      toast.error(t("agents.proc_protected_kill"));
      return false;
    }
    if (opts.needConfirm && !(await confirm({ message: t("agents.proc_kill_confirm").replace("{pid}", target) }))) {
      return false;
    }

    setBusy(target, action);
    try {
      await api.post(paths.agents.cmd(agentId, action), { target });
      if (!opts.silent) {
        toast.success(
          t("agents.proc_action_queued")
            .replace("{action}", action)
            .replace("{pid}", target),
        );
      }
      return true;
    } catch (error) {
      if (!opts.silent) toastError(error, "agents.proc_action_failed");
      return false;
    } finally {
      setBusy(target, null);
    }
  };

  const runManualAction = (action: ProcessAction) => {
    void queueAction(action, manualPid, { needConfirm: action === "killproc" });
  };

  const toggleSelect = (pid: string) => {
    if (isProtectedPid(pid, rows)) return;
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(pid)) next.delete(pid);
      else next.add(pid);
      return next;
    });
  };

  const toggleSelectAll = () => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (allVisibleSelected) {
        for (const pid of selectablePids) next.delete(pid);
      } else {
        for (const pid of selectablePids) next.add(pid);
      }
      return next;
    });
  };

  const toggleDetails = (key: string) => {
    setDetailsOpen((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const toggleBranch = (key: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const toggleSort = (key: ProcessSortKey) => {
    if (key === sortKey) {
      setSortDir((dir) => (dir === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const killSelected = async () => {
    if (selectedPids.length === 0) return;
    const pidsLabel = selectedPids.slice(0, 8).join(", ") + (selectedPids.length > 8 ? ", …" : "");
    const message = t("agents.proc_batch_kill_confirm")
      .replace("{count}", String(selectedPids.length))
      .replace("{pids}", pidsLabel);
    if (!(await confirm({ message }))) return;

    setBatchBusy(true);
    let queued = 0;
    let failed = 0;
    for (const pid of selectedPids) {
      const ok = await queueAction("killproc", pid, { needConfirm: false, silent: true });
      if (ok) queued += 1;
      else failed += 1;
    }
    setBatchBusy(false);
    setSelected(new Set());
    if (failed === 0) {
      toast.success(t("agents.proc_batch_kill_queued").replace("{count}", String(queued)));
    } else {
      toast.error(
        t("agents.proc_batch_kill_partial")
          .replace("{queued}", String(queued))
          .replace("{failed}", String(failed)),
      );
    }
  };

  const sortIcon = (key: ProcessSortKey) => {
    if (sortKey !== key) return <ArrowUpDown className="size-3 text-muted-foreground" />;
    return sortDir === "asc"
      ? <ArrowUp className="size-3" />
      : <ArrowDown className="size-3" />;
  };

  const sortableHeader = (key: ProcessSortKey, label: string, className?: string) => (
    <TableHead className={className} aria-sort={sortKey === key ? (sortDir === "asc" ? "ascending" : "descending") : "none"}>
      <button
        type="button"
        className="inline-flex items-center gap-1 text-left font-normal hover:text-foreground"
        onClick={() => toggleSort(key)}
      >
        {label}
        {sortIcon(key)}
      </button>
    </TableHead>
  );

  const sourceLabel = snapshot?.live
    ? t("agents.proc_source_live")
    : snapshot?.source === "process_tree"
      ? t("agents.proc_source_tree")
      : snapshot?.source === "ps"
        ? t("agents.proc_source_ps")
        : snapshot?.source || t("agents.proc_source_unknown");

  const manualKillProtected = !!manualPid && isProtectedPid(manualPid, rows);
  const manualDisabled = !online || batchBusy || busyByPid.size > 0;

  return (
    <SectionCard
      className="mb-(--page-gap)"
      title={t("agents.detail_process_list")}
      icon={Terminal}
      tone="muted"
      count={rows.length || undefined}
      description={t("agents.detail_process_snapshot_hint")}
      collapsible
      // Controlled: the "Load"/"Retry" action button drives onToggle(true),
      // which an uncontrolled defaultOpen panel would ignore (data loaded
      // invisibly behind a closed card).
      open={expanded}
      onOpenChange={onToggle}
      action={
        expanded ? (
          <span className="flex items-center gap-1">
            {onRefresh && (
              <Button
                variant="ghost"
                size="sm"
                className="text-xs h-auto p-0 text-primary hover:bg-transparent hover:underline"
                onClick={() => void runRefresh()}
                disabled={!online || collecting}
              >
                {collecting ? <Spinner size="xs" /> : <RefreshCw className="size-3" />}
                {collecting ? t("agents.proc_collecting") : t("agents.proc_collect")}
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              className="text-xs h-auto p-0 text-muted-foreground hover:bg-transparent hover:underline"
              onClick={() => onToggle(false)}
            >
              {t("agents.detail_hide")}
            </Button>
          </span>
        ) : (
          <Button
            variant="ghost"
            size="sm"
            className="text-xs h-auto p-0 text-primary hover:bg-transparent hover:underline"
            onClick={() => onToggle(!expanded)}
          >
            {loadFailed ? t("agents.detail_retry") : (expanded ? t("agents.detail_hide") : t("agents.detail_load"))}
          </Button>
        )
      }
    >
      <div className="space-y-2 p-3">
        {loading && !snapshot ? (
          <div className="flex items-center justify-center py-6"><Spinner size="md" /></div>
        ) : loadFailed && !snapshot ? (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-xs" role="alert">
            <p className="text-destructive-ink">{t("agents.proc_load_failed_msg")}</p>
            {onRetry && (
              <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
                {t("agents.detail_retry")}
              </Button>
            )}
          </div>
        ) : rows.length > 0 ? (
          <>
            {loadFailed && (
              <div className="flex items-center justify-between gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-2.5 py-1.5 text-xs" role="alert">
                <span className="text-destructive-ink">{t("agents.proc_collect_failed")}</span>
                {onRetry && (
                  <Button variant="ghost" size="xs" onClick={onRetry}>{t("agents.detail_retry")}</Button>
                )}
              </div>
            )}
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <Badge variant="outline">{sourceLabel}</Badge>
              <span>{t("agents.proc_count").replace("{count}", String(rows.length))}</span>
              {snapshot?.collectedAt && (
                <span title={snapshot.collectedAt}>
                  {t("agents.proc_collected").replace("{time}", timeAgo(snapshot.collectedAt, t))}
                </span>
              )}
              {collecting && (
                <span className="inline-flex items-center gap-1 text-primary">
                  <Spinner size="xs" />
                  {t("agents.proc_collecting")}
                </span>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <SearchInput
                value={query}
                onChange={setQuery}
                onClear={() => setQuery("")}
                placeholder={t("agents.proc_search_ph")}
                label={t("agents.proc_search")}
                className="min-w-[180px] flex-1"
                inputClassName="h-8 text-xs"
              />
              <div className="inline-flex rounded-lg border border-border bg-background p-0.5" role="group" aria-label={t("agents.proc_view_label")}>
                <Button
                  variant={view === "tree" ? "secondary" : "ghost"}
                  size="xs"
                  aria-pressed={view === "tree"}
                  onClick={() => setView("tree")}
                >
                  {t("agents.proc_view_tree")}
                </Button>
                <Button
                  variant={view === "flat" ? "secondary" : "ghost"}
                  size="xs"
                  aria-pressed={view === "flat"}
                  onClick={() => setView("flat")}
                >
                  {t("agents.proc_view_flat")}
                </Button>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <Input
                value={manualPid}
                onChange={(e) => setManualPid(e.target.value.replace(/[^0-9]/g, "").slice(0, 10))}
                placeholder={t("agents.proc_pid_ph")}
                aria-label={t("agents.proc_pid_ph")}
                className="h-8 w-36 font-mono text-xs"
                inputMode="numeric"
              />
              {(["suspend", "resume", "killproc"] as const).map((action) => (
                <Button
                  key={action}
                  size="sm"
                  variant={action === "killproc" ? "destructive" : "outline"}
                  disabled={manualDisabled || !manualPid || (action === "killproc" && manualKillProtected)}
                  onClick={() => runManualAction(action)}
                  title={action === "killproc" && manualKillProtected ? t("agents.proc_protected_kill") : undefined}
                >
                  {t(`agents.proc_${action}`)}
                </Button>
              ))}
              <span className="text-xs text-muted-foreground">{t("agents.proc_manual_hint")}</span>
            </div>

            {selectedPids.length > 0 && (
              <div className="flex flex-wrap items-center gap-2 rounded-lg border border-destructive/25 bg-destructive/5 px-2.5 py-1.5">
                <span className="text-xs font-medium text-destructive-ink">
                  {t("agents.proc_selected").replace("{count}", String(selectedPids.length))}
                </span>
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={!online || batchBusy}
                  onClick={() => void killSelected()}
                >
                  {batchBusy ? <Spinner size="xs" /> : <Skull className="size-3.5" />}
                  {t("agents.proc_kill_selected").replace("{count}", String(selectedPids.length))}
                </Button>
                <Button size="sm" variant="ghost" disabled={batchBusy} onClick={() => setSelected(new Set())}>
                  {t("agents.proc_clear_selection")}
                </Button>
              </div>
            )}

            <div className="max-h-80 overflow-auto rounded-lg border border-border">
              <Table>
                <TableHeader className="sticky top-0 z-10 bg-card/95 backdrop-blur supports-[backdrop-filter]:bg-card/90">
                  <TableRow className="bg-muted/50">
                    <TableHead className="w-9">
                      <Checkbox
                        checked={allVisibleSelected}
                        indeterminate={someVisibleSelected && !allVisibleSelected}
                        disabled={selectablePids.length === 0}
                        onCheckedChange={toggleSelectAll}
                        aria-label={t("agents.proc_select_all")}
                      />
                    </TableHead>
                    {sortableHeader("pid", "PID", "w-20")}
                    {sortableHeader("name", t("agents.proc_col_process"))}
                    {sortableHeader("user", t("agents.proc_col_user"), "hidden w-28 md:table-cell")}
                    <TableHead className="w-36 text-right">{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {visibleRows.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5}>
                        <EmptyState compact icon={Terminal} title={t("agents.proc_no_matches")} />
                      </TableCell>
                    </TableRow>
                  ) : visibleRows.map((row) => {
                    const rowBusy = busyByPid.get(row.pid);
                    const protection = protectionByPid.get(row.pid);
                    const node = view === "tree" ? row as ProcessRow & { children?: ProcessRow[] } : null;
                    const hasChildren = !!node?.children?.length;
                    const branchCollapsed = collapsed.has(row.key);
                    const expandedDetails = detailsOpen.has(row.key);
                    const actionDisabled = !online || batchBusy || !!rowBusy;
                    return (
                      <Fragment key={row.key}>
                        <TableRow className="hover:bg-muted/40">
                          <TableCell>
                            <Checkbox
                              checked={selected.has(row.pid)}
                              disabled={!!protection}
                              onCheckedChange={() => toggleSelect(row.pid)}
                              aria-label={t("agents.proc_select_row").replace("{pid}", row.pid)}
                              title={protection ? t("agents.proc_protected") : undefined}
                            />
                          </TableCell>
                          <TableCell className="font-mono text-xs">{row.pid}</TableCell>
                          <TableCell className="max-w-0">
                            <div className="flex min-w-0 items-center gap-0.5" style={{ paddingLeft: view === "tree" ? `${Math.min(row.depth, 16) * 12}px` : undefined }}>
                              {view === "tree" ? (
                                hasChildren ? (
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    aria-expanded={!branchCollapsed}
                                    aria-label={branchCollapsed ? t("agents.proc_expand_branch") : t("agents.proc_collapse_branch")}
                                    onClick={() => toggleBranch(row.key)}
                                  >
                                    {branchCollapsed ? <ChevronRight className="size-3" /> : <ChevronDown className="size-3" />}
                                  </Button>
                                ) : (
                                  <span className="inline-block size-6 shrink-0" aria-hidden="true" />
                                )
                              ) : null}
                              <span className="truncate font-mono text-xs" title={row.raw}>
                                {row.name}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell className="hidden max-w-28 truncate text-xs text-muted-foreground md:table-cell" title={row.user}>
                            {row.user || "—"}
                          </TableCell>
                          <TableCell className="text-right">
                            <span className="inline-flex items-center gap-1">
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <Button variant="ghost" size="icon-xs" disabled={actionDisabled} onClick={() => void queueAction("suspend", row.pid, { needConfirm: false })} aria-label={t("agents.proc_suspend")}>
                                      {rowBusy === "suspend" ? <Spinner size="xs" /> : <Pause className="size-3.5" />}
                                    </Button>
                                  }
                                />
                                <TooltipContent>{t("agents.proc_suspend")}</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <Button variant="ghost" size="icon-xs" disabled={actionDisabled} onClick={() => void queueAction("resume", row.pid, { needConfirm: false })} aria-label={t("agents.proc_resume")}>
                                      {rowBusy === "resume" ? <Spinner size="xs" /> : <Play className="size-3.5" />}
                                    </Button>
                                  }
                                />
                                <TooltipContent>{t("agents.proc_resume")}</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <Button variant="ghost" size="icon-xs" className="text-destructive-ink" disabled={actionDisabled || !!protection} onClick={() => void queueAction("killproc", row.pid, { needConfirm: true })} aria-label={t("agents.proc_kill")}>
                                      {rowBusy === "killproc" ? <Spinner size="xs" /> : <Skull className="size-3.5" />}
                                    </Button>
                                  }
                                />
                                <TooltipContent>{protection ? t("agents.proc_protected") : t("agents.proc_kill")}</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <Button variant="ghost" size="icon-xs" aria-expanded={expandedDetails} aria-label={t("agents.proc_details_for").replace("{pid}", row.pid)} onClick={() => toggleDetails(row.key)}>
                                      <Info className="size-3.5" />
                                    </Button>
                                  }
                                />
                                <TooltipContent>{t("agents.proc_details")}</TooltipContent>
                              </Tooltip>
                            </span>
                          </TableCell>
                        </TableRow>
                        {expandedDetails && (
                          <TableRow className="bg-muted/20 hover:bg-muted/20">
                            <TableCell colSpan={5} className="whitespace-normal">
                              <div className="grid gap-x-6 gap-y-2 py-1 text-xs sm:grid-cols-2 lg:grid-cols-3">
                                <DetailItem label={t("agents.proc_col_ppid")} value={row.ppid || "—"} mono />
                                <DetailItem label={t("agents.proc_col_user")} value={row.user || "—"} />
                                <DetailItem label={t("agents.proc_threads")} value={row.threads !== undefined ? String(row.threads) : "—"} mono />
                                <DetailItem label={t("agents.proc_memory")} value={row.memoryKb !== undefined ? formatBytes(row.memoryKb * 1024) : "—"} mono />
                                <div className="sm:col-span-2 lg:col-span-3">
                                  <span className="block text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                                    {row.command ? t("agents.proc_command") : t("agents.proc_raw")}
                                  </span>
                                  <code className="mt-0.5 block break-all font-mono text-xs text-foreground">
                                    {row.command || row.raw}
                                  </code>
                                </div>
                              </div>
                            </TableCell>
                          </TableRow>
                        )}
                      </Fragment>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          </>
        ) : (
          <EmptyState
            compact
            icon={Terminal}
            title={snapshot ? t("agents.proc_unparsed_title") : t("agents.proc_empty_title")}
            message={snapshot ? t("agents.proc_unparsed_msg") : t("agents.proc_empty_msg")}
            action={
              <Button
                size="sm"
                variant="outline"
                disabled={!online || collecting || !onRefresh}
                onClick={() => void runRefresh()}
              >
                {collecting ? <Spinner size="xs" /> : <RefreshCw className="size-3.5" />}
                {collecting ? t("agents.proc_collecting") : t("agents.proc_collect")}
              </Button>
            }
          />
        )}
      </div>
      {modal}
    </SectionCard>
  );
});

function DetailItem({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <span className="block text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{label}</span>
      <span className={mono ? "font-mono text-xs text-foreground" : "text-xs text-foreground"}>{value}</span>
    </div>
  );
}
