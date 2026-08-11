"use client";

import { memo } from "react";
import { Button } from "@/components/ui/button";
import { AvatarFallback } from "@/components/ui/avatar";
import { StatusBadge } from "@/components/UI";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { timeAgo, formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import type { Beacon } from "./types";
import { avatarColor } from "./types";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { TableCell } from "@/components/ui/table";
import { Apple, Camera, Check, Clock, Copy, Link as LinkIcon, Lock, Maximize2, Monitor, Shield, StickyNote, Terminal, Trash2, Unlock } from "lucide-react";

interface AgentRowProps {
  beacon: Beacon;
  isSelected: boolean;
  onToggleSelect: (id: string, checked: boolean) => void;
  onScreenshot: (id: string) => void;
  onQuickSleep: (e: React.MouseEvent, beacon: Beacon) => void;
  onNotes: (e: React.MouseEvent, beacon: Beacon) => void;
  onConfirm: (type: "kill" | "delete" | "batch-delete" | "bulk-kill" | "bulk-uninstall", id: string, hostname: string) => void;
  onSelect: (id: string) => void;
  taskCount: number;
  lockUser: string | null;
  visibleCols: Record<string, boolean>;
  copiedField: string;
  onCopy: (field: string, value: string) => void;
}

export const AgentRow = memo(function AgentRow({
  beacon,
  isSelected,
  onToggleSelect,
  onScreenshot,
  onQuickSleep,
  onNotes,
  onConfirm,
  onSelect,
  taskCount,
  lockUser,
  visibleCols,
  copiedField,
  onCopy,
}: AgentRowProps) {
  const id = beacon.id || "";
  const { t } = useI18n();
  const hostname = beacon.hostname || "-";
  const username = beacon.username || "-";
  const ip = beacon.ip || "-";
  const os = beacon.os || "";
  const arch = beacon.arch || "";
  const status = beacon.status || "offline";
  const lastSeen = beacon.last_seen || "";
  const integrity = beacon.integrity || "";
  const elevated = beacon.elevated || false;
  const notes = beacon.notes || "";
  const activeWindow = beacon.active_window || "";

  const borderLeft = status === "online" ? "border-l-2 border-l-emerald-500" :
    status === "stale" ? "border-l-2 border-l-amber-500" : "border-l-2 border-l-red-500";
  const OsIcon = os.toLowerCase() === "windows" ? Monitor :
    os.toLowerCase() === "linux" ? Terminal : Apple;

  return (
    <tr
      className={`group hover:bg-secondary/60 transition-all duration-150 ${borderLeft} even:bg-muted/30 hover:shadow-sm`}
    >
      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5" data-label="">
        <Checkbox aria-label={t("common.select_item")} name={`select-${id}`}
          checked={isSelected}
          onCheckedChange={(v) => onToggleSelect(id, v === true)}
          onClick={(e) => e.stopPropagation()}
        />
      </TableCell>
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4" data-label="Hostname">
        <div className="flex items-center gap-2">
          <AvatarFallback name={hostname} size="sm" shape="square" color={avatarColor(hostname)} />
          <div className="min-w-0">
            <Button
              variant="link"
              size="sm"
              onClick={(e) => { e.stopPropagation(); onSelect(id); }}
              className="font-semibold text-primary hover:underline text-sm text-left p-0 h-auto justify-start"
            >
              {hostname}
            </Button>
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={(e) => { e.stopPropagation(); onCopy(`host-${id}`, hostname); }}
              className="ml-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity"
              aria-label={t("agents.copy_hostname")}
            >
              {copiedField === `host-${id}` ? <Check className="w-3 h-3 text-emerald-500" /> : <Copy className="w-3 h-3" />}
            </Button>
            {notes && <span className="ml-1.5" title={notes}><StickyNote className="w-4 h-4" /></span>}
            {beacon.parent_id && (
              <span className="ml-1 text-(--fs-micro-sm) text-purple-600 dark:text-purple-400" title={t("agents.p2p_chained")}>
                <LinkIcon className="w-4 h-4" />
              </span>
            )}
          </div>
        </div>
      </TableCell>
      {visibleCols.username && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-muted-foreground text-xs font-mono font-medium" data-label="User">{username}</TableCell>
      )}
      {visibleCols.os && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4" data-label="OS">
        <div className="flex items-center gap-1.5 flex-wrap">
          <Badge variant="secondary" className="text-(--fs-xs-sm) gap-1.5 whitespace-nowrap bg-secondary/60">
            <OsIcon className="w-3 h-3 text-muted-foreground" />
            {os}{arch ? ` ${arch}` : ""}
          </Badge>
          {integrity && (
            <Badge variant={
              integrity === "System" ? "destructive" :
              integrity === "High" ? "success" :
              integrity === "Medium" ? "warning" :
              "secondary"
            } className="text-(--fs-micro-sm) font-medium">{integrity}</Badge>
          )}
          {elevated && <Badge variant="destructive" className="text-(--fs-micro-sm) font-bold" title={t("agents.elevated")}><Shield className="w-3 h-3" /></Badge>}
        </div>
      </TableCell>
      )}
      {visibleCols.ip && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4" data-label="IP">
        <span className="font-mono text-xs text-muted-foreground">{ip}</span>
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={(e) => { e.stopPropagation(); onCopy(`ip-${id}`, ip); }}
          className="ml-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity"
          aria-label={t("agents.copy_ip")}
        >
          {copiedField === `ip-${id}` ? <Check className="w-3 h-3 text-emerald-500" /> : <Copy className="w-3 h-3" />}
        </Button>
      </TableCell>
      )}
      {visibleCols.last_seen && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-xs text-muted-foreground font-mono whitespace-nowrap" data-label="Last Seen">
        <span title={lastSeen ? formatTime(lastSeen) : ""}>{timeAgo(lastSeen, t)}</span>
      </TableCell>
      )}
      {visibleCols.window && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-xs text-foreground max-w-[140px] max-sm:hidden" data-label="Window">
        {activeWindow ? (
          <span className="inline-flex items-center gap-1 truncate" title={activeWindow}>
            <Maximize2 className="w-4 h-4" />
            <span className="truncate">{activeWindow}</span>
          </span>
        ) : <span className="text-muted-foreground/70 dark:text-muted-foreground"></span>}
      </TableCell>
      )}
      {visibleCols.lock && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-center max-sm:hidden" data-label="Lock">
        {lockUser ? (
          <span className="inline-flex items-center gap-1 text-xs" title={t("agents.locked_by").replace("{user}", lockUser)}>
            <Lock className="w-4 h-4" />
            <span className="text-(--fs-micro-sm) text-amber-600 dark:text-amber-400 font-medium truncate max-w-[80px]">{lockUser}</span>
          </span>
        ) : (
          <span className="text-muted-foreground/70 text-(--fs-xs-sm)">
            <Unlock className="w-4 h-4" />
          </span>
        )}
      </TableCell>
      )}
      {visibleCols.tasks && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-center max-sm:hidden" data-label={t("agents.tasks_label")}>
        <Tooltip>
          <TooltipTrigger render={<span className="text-xs font-mono text-foreground cursor-default">{taskCount}</span>} />
          <TooltipContent side="top">
            {beacon.taskStats ? (
              <div className="space-y-0.5">
                {beacon.taskStats.pending > 0 && <div>{t("agents.task_pending")}: {beacon.taskStats.pending}</div>}
                {beacon.taskStats.running > 0 && <div>{t("agents.task_running")}: {beacon.taskStats.running}</div>}
                <div>{t("agents.task_completed")}: {beacon.taskStats.completed}</div>
                {beacon.taskStats.failed > 0 && <div>{t("agents.task_failed")}: {beacon.taskStats.failed}</div>}
              </div>
            ) : (
              <span>{t("agents.total_tasks").replace("{n}", String(taskCount))}</span>
            )}
          </TooltipContent>
        </Tooltip>
      </TableCell>
      )}
      {visibleCols.version && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-xs text-muted-foreground/70 max-sm:hidden" data-label="Version">
        {beacon.version || <span className="text-muted-foreground/70">-</span>}
      </TableCell>
      )}
      {visibleCols.status && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4" data-label="Status">
        <StatusBadge status={status} pulse={status === "online"} />
      </TableCell>
      )}
      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-right" data-label="Actions">
        <div className="flex items-center justify-end gap-1">
          <Tooltip>
            <TooltipTrigger render={
              <Button
                variant="secondary"
                size="icon"
                className="min-w-[44px] min-h-[44px] hover:bg-primary/10 hover:text-primary"
                aria-label={t("agents.screenshot")}
                onClick={(e) => { e.stopPropagation(); onScreenshot(id); }}
              >
                <Camera className="w-3.5 h-3.5" />
              </Button>
            } />
            <TooltipContent side="top">{t("agents.screenshot")}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger render={
              <Button
                variant="secondary"
                size="icon"
                className="min-w-[44px] min-h-[44px] hover:bg-violet-100 dark:hover:bg-violet-900/40 hover:text-violet-700 dark:hover:text-violet-400"
                aria-label={t("agents.quick_sleep")}
                onClick={(e) => { e.stopPropagation(); onQuickSleep(e, beacon); }}
              >
                <Clock className="w-3.5 h-3.5" />
              </Button>
            } />
            <TooltipContent side="top">{t("agents.quick_sleep")}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger render={
              <Button
                variant="secondary"
                size="icon"
                className="min-w-[44px] min-h-[44px] hover:bg-amber-100 dark:hover:bg-amber-900/40 hover:text-amber-700 dark:hover:text-amber-400"
                aria-label={t("agents.edit_notes")}
                onClick={(e) => { e.stopPropagation(); onNotes(e, beacon); }}
              >
                <StickyNote className="w-3.5 h-3.5" />
              </Button>
            } />
            <TooltipContent side="top">{t("agents.edit_notes")}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger render={
              <Button
                variant="secondary"
                size="icon"
                className="min-w-[44px] min-h-[44px] hover:bg-destructive/10 hover:text-destructive"
                aria-label={t("agents.delete")}
                onClick={(e) => { e.stopPropagation(); onConfirm("delete", id, hostname); }}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </Button>
            } />
            <TooltipContent side="top">{t("agents.delete")}</TooltipContent>
          </Tooltip>
        </div>
      </TableCell>
    </tr>
  );
});
