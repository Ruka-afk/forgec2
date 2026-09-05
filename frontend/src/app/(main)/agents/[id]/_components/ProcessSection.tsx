"use client";

import { memo, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { SectionCard } from "@/components/ui/section-card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { Pause, Play, RefreshCw, Skull, Terminal } from "lucide-react";
import { parsePsRows } from "./process-snapshot";

interface ProcessSectionProps {
  agentId: string;
  online: boolean;
  processList: string | null;
  loading: boolean;
  loadFailed: boolean;
  expanded: boolean;
  /** Collapse/expand toggle from the card header. */
  onToggle: (open: boolean) => void;
  /** Force a fresh process snapshot without toggling the card. */
  onRefresh?: () => void;
}

export default memo(function ProcessSection({ agentId, online, processList, loading, loadFailed, expanded, onToggle, onRefresh }: ProcessSectionProps) {
  const { t } = useI18n();
  const { confirm, modal } = useConfirm();
  const [acting, setActing] = useState<string | null>(null);
  const [manualPid, setManualPid] = useState("");
  const rows = useMemo(() => parsePsRows(processList || ""), [processList]);

  const runAction = async (action: "suspend" | "resume" | "killproc", pid: string, needConfirm: boolean) => {
    const target = pid.trim();
    if (!/^\d+$/.test(target)) {
      toast.error(t("agents.proc_invalid_pid"));
      return;
    }
    if (needConfirm && !(await confirm({ message: t("agents.proc_kill_confirm").replace("{pid}", target) }))) return;
    setActing(`${action}:${target}`);
    try {
      await api.post(paths.agents.cmd(agentId, action), { target });
      toast.success(t("agents.proc_action_queued").replace("{action}", action).replace("{pid}", target));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("agents.proc_action_failed"));
    } finally {
      setActing(null);
    }
  };

  return (
    <SectionCard
      className="mb-4"
      title={t("agents.detail_process_list")}
      icon={<Terminal className="size-3.5" />}
      description={t("agents.detail_process_snapshot_hint")}
      collapsible
      // Controlled: the "Load"/"Retry" action button drives onToggle(true),
      // which an uncontrolled defaultOpen panel would ignore (data loaded
      // invisibly behind a closed card).
      open={expanded}
      onOpenChange={onToggle}
      action={
        expanded && processList && !loading ? (
          <Button variant="ghost" size="sm" className="text-xs h-auto p-0 text-primary hover:bg-transparent hover:underline" onClick={onRefresh}>
            <RefreshCw className="size-3" />{t("agents.detail_refresh")}
          </Button>
        ) : (
          <Button variant="ghost" size="sm" className="text-xs h-auto p-0 text-primary hover:bg-transparent hover:underline" onClick={() => onToggle(!expanded)}>
            {loadFailed ? t("agents.detail_retry") : (expanded ? t("agents.detail_hide") : t("agents.detail_load"))}
          </Button>
        )
      }
    >
      <div className="space-y-2 p-3">
        {loading ? (
          <div className="flex items-center justify-center py-6"><Spinner size="md" /></div>
        ) : rows.length > 0 ? (
          <>
            <div className="flex gap-2">
              <Input
                value={manualPid}
                onChange={(e) => setManualPid(e.target.value.replace(/[^0-9]/g, "").slice(0, 10))}
                placeholder={t("agents.proc_pid_ph")}
                aria-label={t("agents.proc_pid_ph")}
                className="h-8 flex-1 font-mono text-xs"
                inputMode="numeric"
              />
              {(["suspend", "resume", "killproc"] as const).map((action) => (
                <Button
                  key={action}
                  size="sm"
                  variant={action === "killproc" ? "destructive" : "outline"}
                  disabled={!online || acting !== null || !manualPid}
                  onClick={() => void runAction(action, manualPid, action === "killproc")}
                >
                  {t(`agents.proc_${action}`)}
                </Button>
              ))}
            </div>
            <div className="max-h-72 overflow-auto rounded-lg border border-border">
              <Table>
                <TableHeader>
                  <TableRow className="bg-muted/50">
                    <TableHead className="w-20">PID</TableHead>
                    <TableHead>{t("agents.proc_col_process")}</TableHead>
                    <TableHead className="w-40 text-right">{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((row) => {
                    const rowBusy = acting !== null && acting === `suspend:${row.pid}`;
                    return (
                      <TableRow key={row.pid}>
                        <TableCell className="font-mono text-xs">{row.pid}</TableCell>
                        <TableCell className="max-w-0 truncate font-mono text-xs" title={row.raw.trim()}>
                          {row.name}
                        </TableCell>
                        <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                          <span className="inline-flex items-center gap-1">
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button variant="ghost" size="icon-xs" disabled={!online || acting !== null} onClick={() => void runAction("suspend", row.pid, false)} aria-label={t("agents.proc_suspend")}>
                                    {rowBusy ? <Spinner size="xs" /> : <Pause className="size-3.5" />}
                                  </Button>
                                }
                              />
                              <TooltipContent>{t("agents.proc_suspend")}</TooltipContent>
                            </Tooltip>
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button variant="ghost" size="icon-xs" disabled={!online || acting !== null} onClick={() => void runAction("resume", row.pid, false)} aria-label={t("agents.proc_resume")}>
                                    <Play className="size-3.5" />
                                  </Button>
                                }
                              />
                              <TooltipContent>{t("agents.proc_resume")}</TooltipContent>
                            </Tooltip>
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button variant="ghost" size="icon-xs" className="text-destructive" disabled={!online || acting !== null} onClick={() => void runAction("killproc", row.pid, true)} aria-label={t("agents.proc_kill")}>
                                    <Skull className="size-3.5" />
                                  </Button>
                                }
                              />
                              <TooltipContent>{t("agents.proc_kill")}</TooltipContent>
                            </Tooltip>
                          </span>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          </>
        ) : (
          <pre className="p-3 bg-muted rounded-lg font-mono text-xs text-foreground whitespace-pre-wrap break-all max-h-64 overflow-y-auto border border-border">
            {processList || t("agents.detail_no_data")}
          </pre>
        )}
      </div>
      {modal}
    </SectionCard>
  );
});
