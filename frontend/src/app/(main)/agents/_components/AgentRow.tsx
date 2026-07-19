"use client";

import { memo } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/UI";
import { timeAgo } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import type { Beacon, Tag } from "./types";
import { avatarInitial, avatarColor } from "./types";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { TableCell } from "@/components/ui/table";
import { Apple, Camera, Check, Clock, Copy, Folder, Link as LinkIcon, List, Lock, Maximize2, Monitor, Power, Shield, StickyNote, Terminal, Trash2, Unlock } from "lucide-react";

interface AgentRowProps {
  beacon: Beacon;
  isSelected: boolean;
  onToggleSelect: (id: string, checked: boolean) => void;
  onScreenshot: (id: string) => void;
  onQuickSleep: (e: React.MouseEvent, beacon: Beacon) => void;
  onNotes: (e: React.MouseEvent, beacon: Beacon) => void;
  onConfirm: (type: "kill" | "delete" | "batch-delete" | "bulk-kill" | "bulk-uninstall", id: string, hostname: string) => void;
  tags: Tag[];
  taskCount: number;
  lastTask: { type?: string; status?: string; command?: string } | undefined;
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
  tags,
  taskCount,
  lastTask,
  lockUser,
  visibleCols,
  copiedField,
  onCopy,
}: AgentRowProps) {
  const id = beacon.id || "";
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
  const { t } = useI18n();

  const borderLeft = status === "online" ? "border-l-2 border-l-emerald-500" :
    status === "stale" ? "border-l-2 border-l-amber-500" : "border-l-2 border-l-red-500";
  const OsIcon = os.toLowerCase() === "windows" ? Monitor :
    os.toLowerCase() === "linux" ? Terminal : Apple;

  return (
    <tr
      className={`group hover:bg-indigo-50/50 dark:hover:bg-indigo-900/20 transition-all duration-150 cursor-pointer ${borderLeft} even:bg-muted/30 hover:shadow-sm`}
    >
      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5" data-label="">
        <Checkbox aria-label="Select item" name={`select-${id}`}
          checked={isSelected}
          onCheckedChange={(v) => onToggleSelect(id, v === true)}
          onClick={(e) => e.stopPropagation()}
        />
      </TableCell>
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4" data-label="Hostname">
        <div className="flex items-center gap-2">
          <span className={`w-7 h-7 rounded-lg ${avatarColor(hostname)} flex items-center justify-center text-white text-xs font-bold shrink-0`}>
            {avatarInitial(hostname)}
          </span>
          <div className="min-w-0">
            <Link
              href={`/agents/${id}`}
              onClick={(e) => e.stopPropagation()}
              className="font-semibold text-indigo-600 dark:text-indigo-400 hover:underline text-sm"
            >
              {hostname}
            </Link>
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={(e) => { e.stopPropagation(); onCopy(`host-${id}`, hostname); }}
              className="ml-1 opacity-0 group-hover:opacity-100 transition-opacity"
              title={t("agents.copy_hostname")}
              aria-label={t("agents.copy_hostname")}
            >
              {copiedField === `host-${id}` ? <Check className="w-3 h-3 text-emerald-500" /> : <Copy className="w-3 h-3" />}
            </Button>
            {notes && <span className="ml-1.5" title={notes}><StickyNote className="w-4 h-4" /></span>}
            {beacon.parent_id && (
              <span className="ml-1 text-[10px] text-purple-600" title={t("agents.p2p_chained")}>
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
          <Badge variant="secondary" className="text-[11px] gap-1.5 whitespace-nowrap bg-secondary/60">
            <OsIcon className="w-3 h-3 text-muted-foreground" />
            {os}{arch ? ` ${arch}` : ""}
          </Badge>
          {integrity && (
            <Badge variant={
              integrity === "System" ? "destructive" :
              integrity === "High" ? "success" :
              integrity === "Medium" ? "warning" :
              "secondary"
            } className="text-[10px] font-medium">{integrity}</Badge>
          )}
          {elevated && <Badge variant="destructive" className="text-[10px] font-bold" title={t("agents.elevated")}><Shield className="w-3 h-3" /></Badge>}
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
          className="ml-1 opacity-0 group-hover:opacity-100 transition-opacity"
          title="Copy IP"
          aria-label={t("agents.copy_ip")}
        >
          {copiedField === `ip-${id}` ? <Check className="w-3 h-3 text-emerald-500" /> : <Copy className="w-3 h-3" />}
        </Button>
      </TableCell>
      )}
      {visibleCols.last_seen && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-xs text-muted-foreground font-mono whitespace-nowrap" data-label="Last Seen">
        <span title={lastSeen ? new Date(lastSeen).toLocaleString() : ""}>{timeAgo(lastSeen)}</span>
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
            <span className="text-[10px] text-amber-600 dark:text-amber-400 font-medium truncate max-w-[80px]">{lockUser}</span>
          </span>
        ) : (
          <span className="text-muted-foreground/70 text-[11px]">
            <Unlock className="w-4 h-4" />
          </span>
        )}
      </TableCell>
      )}
      {visibleCols.tags && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-center max-sm:hidden" data-label="Tags">
        <span className="inline-flex flex-wrap gap-1 justify-center">
          {tags.slice(0, 4).map((t) => (
            <span key={t.id} className="w-2.5 h-2.5 rounded-full ring-1.5 ring-white dark:ring-border shadow-sm" style={{ backgroundColor: t.color }} title={t.name} />
          ))}
          {tags.length > 4 && (
            <Badge variant="secondary" className="text-[9px] font-mono h-4 px-1">+{tags.length - 4}</Badge>
          )}
        </span>
      </TableCell>
      )}
      {visibleCols.tasks && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-center max-sm:hidden" data-label={t("agents.tasks_label")}>
        <span className="text-xs font-mono text-foreground">{taskCount}</span>
      </TableCell>
      )}
      {visibleCols.tasks && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden" data-label="Last Task">
        {lastTask ? (
          <span className={`inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-md font-medium ${
            lastTask.status === "completed" ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400" :
            lastTask.status === "failed" ? "bg-destructive/10 text-destructive" :
            lastTask.status === "pending" ? "bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-400" :
            "bg-secondary text-muted-foreground"
          }`} title={lastTask.command || ""}>
            {lastTask.type === "screenshot" ? <Camera className="w-2 h-2" /> :
              lastTask.type === "ps" ? <List className="w-2 h-2" /> :
              lastTask.type === "kill" ? <Power className="w-2 h-2" /> :
              lastTask.type === "ls" ? <Folder className="w-2 h-2" /> :
              <Terminal className="w-2 h-2" />}
            <span className="truncate max-w-[60px]">{lastTask.type || "-"}</span>
          </span>
        ) : <span className="text-[10px] text-muted-foreground/70">-</span>}
      </TableCell>
      )}
      {visibleCols.version && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-xs text-muted-foreground/70 max-sm:hidden" data-label="Version">
        {beacon.version || <span className="text-muted-foreground/70">-</span>}
      </TableCell>
      )}
      {visibleCols.status && (
      <TableCell className="py-3 px-3 sm:py-3.5 sm:px-4 text-center" data-label="Status">
        <StatusBadge status={status} pulse={status === "online"} />
      </TableCell>
      )}
      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-right" data-label="Actions">
        <div className="flex items-center justify-end gap-1">
          <Button
            variant="secondary"
            size="icon"
            className="min-w-[36px] min-h-[36px] hover:bg-primary/10 hover:text-primary"
            title={t("agents.screenshot")}
            aria-label={t("agents.screenshot")}
            onClick={(e) => { e.stopPropagation(); onScreenshot(id); }}
          >
            <Camera className="w-3.5 h-3.5" />
          </Button>
          <Button
            variant="secondary"
            size="icon"
            className="min-w-[36px] min-h-[36px] hover:bg-violet-100 dark:hover:bg-violet-900/40 hover:text-violet-700 dark:hover:text-violet-400"
            title={t("agents.quick_sleep")}
            aria-label={t("agents.quick_sleep")}
            onClick={(e) => { e.stopPropagation(); onQuickSleep(e, beacon); }}
          >
            <Clock className="w-3.5 h-3.5" />
          </Button>
          <Button
            variant="secondary"
            size="icon"
            className="min-w-[36px] min-h-[36px] hover:bg-amber-100 dark:hover:bg-amber-900/40 hover:text-amber-700 dark:hover:text-amber-400"
            title={t("agents.edit_notes")}
            aria-label={t("agents.edit_notes")}
            onClick={(e) => { e.stopPropagation(); onNotes(e, beacon); }}
          >
            <StickyNote className="w-3.5 h-3.5" />
          </Button>
          <Button
            variant="secondary"
            size="icon"
            className="min-w-[36px] min-h-[36px] hover:bg-destructive/10 hover:text-destructive"
            title={t("agents.delete")}
            aria-label={t("agents.delete")}
            onClick={(e) => { e.stopPropagation(); onConfirm("delete", id, hostname); }}
          >
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        </div>
      </TableCell>
    </tr>
  );
});
