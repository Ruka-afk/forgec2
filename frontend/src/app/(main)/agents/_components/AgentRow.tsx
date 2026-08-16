"use client";

import { memo, useState } from "react";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/status-indicator";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { timeAgo, formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import type { Beacon } from "./types";
import { agentStatusBorderClass, osIcon, integrityTone } from "./types";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { TableCell } from "@/components/ui/table";
import { Check, Copy, FolderOpen, Link as LinkIcon, Lock, Maximize2, Monitor, MoreHorizontal, Shield, StickyNote, Terminal, Unlock } from "lucide-react";
import type { AgentMenuPoint } from "./agent-menu-actions";
import { knownImplantVersion } from "./implant-version";
import { copyToClipboard } from "./types";

interface AgentRowProps {
  beacon: Beacon;
  isSelected: boolean;
  onToggleSelect: (id: string, checked: boolean) => void;
  onInteract: (id: string) => void;
  onDetails: (id: string) => void;
  onMenu: (point: AgentMenuPoint) => void;
  onEditNotes?: (beacon: Beacon) => void;
  onQuickNav?: (beacon: Beacon, view: "shell" | "files" | "screen") => void;
  taskCount: number;
  lockUser: string | null;
  visibleCols: Record<string, boolean>;
}

export const AgentRow = memo(function AgentRow({
  beacon,
  isSelected,
  onToggleSelect,
  onInteract,
  onDetails,
  onMenu,
  onEditNotes,
  onQuickNav,
  taskCount,
  lockUser,
  visibleCols,
}: AgentRowProps) {
  const id = beacon.id || "";
  const { t } = useI18n();
  const [copiedField, setCopiedField] = useState("");
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
  const version = knownImplantVersion(beacon.version);

  const borderLeft = agentStatusBorderClass(status);
  const OsIcon = osIcon(os);

  return (
    <tr
      tabIndex={0}
      data-agent-id={id}
      onDoubleClick={() => onDetails(id)}
      onContextMenu={(e) => {
        e.preventDefault();
        onMenu({ x: e.clientX, y: e.clientY, beacon });
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          onInteract(id);
        }
      }}
      className={`group cursor-default hover:bg-secondary/50 ${borderLeft}`}
    >
      <TableCell className="py-1 px-2">
        <Checkbox aria-label={t("common.select_item")} name={`select-${id}`}
          checked={isSelected}
          onCheckedChange={(v) => onToggleSelect(id, v === true)}
          onClick={(e) => e.stopPropagation()}
        />
      </TableCell>
      <TableCell className="py-1 px-2">
        <div className="flex items-center gap-1 min-w-0">
          <div className="min-w-0">
            <Button
              variant="link"
              size="sm"
              onClick={(e) => { e.stopPropagation(); onInteract(id); }}
              className="font-mono text-xs text-primary hover:underline text-left p-0 h-auto justify-start"
            >
              {hostname}
            </Button>
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={(e) => { e.stopPropagation(); copyToClipboard(hostname, `host-${id}`, setCopiedField); }}
              className="ml-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity"
              aria-label={t("agents.copy_hostname")}
            >
              {copiedField === `host-${id}` ? <Check className="w-3 h-3 text-success" /> : <Copy className="w-3 h-3" />}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              className={`ml-1 ${notes ? "" : "opacity-0 group-hover:opacity-100"}`}
              title={notes || t("agents.edit_notes")}
              aria-label={t("agents.edit_notes")}
              onClick={(e) => {
                e.stopPropagation();
                onEditNotes?.(beacon);
              }}
            >
              <StickyNote className={`w-3.5 h-3.5 ${notes ? "text-primary" : "text-muted-foreground"}`} />
            </Button>
            {beacon.parent_id && (
              <span className="ml-1 text-(--fs-micro-sm) text-chart-6" title={t("agents.p2p_chained")}>
                <LinkIcon className="w-4 h-4" />
              </span>
            )}
          </div>
        </div>
      </TableCell>
      {visibleCols.username && (
      <TableCell className="py-1 px-2 text-muted-foreground text-xs font-mono">{username}</TableCell>
      )}
      {visibleCols.os && (
      <TableCell className="py-1 px-2">
        <div className="flex items-center gap-1.5 flex-wrap">
          <Badge variant="secondary" className="text-(--fs-xs-sm) gap-1.5 whitespace-nowrap bg-secondary/60">
            <OsIcon className="w-3 h-3 text-muted-foreground" />
            {os}{arch ? ` ${arch}` : ""}
          </Badge>
          {integrity && (
            <Badge variant={integrityTone(integrity)} className="text-(--fs-micro-sm) font-medium">{integrity}</Badge>
          )}
          {elevated && <Badge variant="destructive" className="text-(--fs-micro-sm) font-bold" title={t("agents.elevated")}><Shield className="w-3 h-3" /></Badge>}
        </div>
      </TableCell>
      )}
      {visibleCols.ip && (
      <TableCell className="py-1 px-2">
        <span className="font-mono text-xs text-muted-foreground">{ip}</span>
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={(e) => { e.stopPropagation(); copyToClipboard(ip, `ip-${id}`, setCopiedField); }}
          className="ml-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity"
          aria-label={t("agents.copy_ip")}
        >
          {copiedField === `ip-${id}` ? <Check className="w-3 h-3 text-success" /> : <Copy className="w-3 h-3" />}
        </Button>
      </TableCell>
      )}
      {visibleCols.last_seen && (
      <TableCell className="py-1 px-2 text-xs text-muted-foreground font-mono whitespace-nowrap">
        <span title={lastSeen ? formatTime(lastSeen) : ""}>{timeAgo(lastSeen, t)}</span>
      </TableCell>
      )}
      {visibleCols.window && (
      <TableCell className="py-1 px-2 text-xs text-foreground max-w-[140px] max-sm:hidden">
        {activeWindow ? (
          <span className="inline-flex items-center gap-1 truncate" title={activeWindow}>
            <Maximize2 className="w-4 h-4" />
            <span className="truncate">{activeWindow}</span>
          </span>
        ) : <span className="text-muted-foreground/70 dark:text-muted-foreground"></span>}
      </TableCell>
      )}
      {visibleCols.lock && (
      <TableCell className="py-1 px-2 text-center max-sm:hidden">
        {lockUser ? (
          <span className="inline-flex items-center gap-1 text-xs" title={t("agents.locked_by").replace("{user}", lockUser)}>
            <Lock className="w-4 h-4" />
            <span className="text-(--fs-micro-sm) text-warning font-medium truncate max-w-[80px]">{lockUser}</span>
          </span>
        ) : (
          <span className="text-muted-foreground/70 text-(--fs-xs-sm)">
            <Unlock className="w-4 h-4" />
          </span>
        )}
      </TableCell>
      )}
      {visibleCols.tasks && (
      <TableCell className="py-1 px-2 text-center max-sm:hidden">
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
      <TableCell className="py-1 px-2 text-xs font-mono text-muted-foreground/70 max-sm:hidden">
        {version ? (
          version
        ) : (
          <span className="text-warning" title={t("agents.version_unknown_hint")}>{t("agents.version_unknown")}</span>
        )}
      </TableCell>
      )}
      {visibleCols.status && (
      <TableCell className="py-1 px-2">
        <StatusBadge status={status} variant="dot" size="sm" pulse={status === "online"} />
      </TableCell>
      )}
      <TableCell className="py-1 px-2 text-right">
        <div className="flex items-center justify-end gap-0.5">
          <Button
            variant="ghost"
            size="icon-xs"
            className="opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity"
            aria-label={t("agents.shell_title")}
            onClick={(e) => { e.stopPropagation(); onQuickNav?.(beacon, "shell"); }}
          >
            <Terminal className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon-xs"
            className="opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity"
            aria-label={t("agents.files_title")}
            onClick={(e) => { e.stopPropagation(); onQuickNav?.(beacon, "files"); }}
          >
            <FolderOpen className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon-xs"
            className="opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity"
            aria-label={t("agents.screen_title")}
            onClick={(e) => { e.stopPropagation(); onQuickNav?.(beacon, "screen"); }}
          >
            <Monitor className="w-4 h-4" />
          </Button>
          <Tooltip>
          <TooltipTrigger render={
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t("agents.context_menu")}
              onClick={(e) => {
                e.stopPropagation();
                const rect = e.currentTarget.getBoundingClientRect();
                onMenu({ x: rect.right, y: rect.bottom, beacon });
              }}
            >
              <MoreHorizontal className="w-4 h-4" />
            </Button>
          } />
          <TooltipContent side="top">{t("agents.context_menu")}</TooltipContent>
        </Tooltip>
        </div>
      </TableCell>
    </tr>
  );
});
