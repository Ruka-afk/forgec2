"use client";

import { memo } from "react";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/status-indicator";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { TableCell } from "@/components/ui/table";
import { timeAgo, formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { Apple, ChevronDown, ChevronRight, Monitor, Terminal } from "lucide-react";
import type { HostGroup } from "./groupBeaconsByHost";
import { hostImplantVersions } from "./implant-version";

interface AgentHostRowProps {
  group: HostGroup;
  expanded: boolean;
  selectedCount: number;
  onToggleExpand: () => void;
  onToggleSelect: (checked: boolean) => void;
  onInteract: (id: string) => void;
  isFocused?: boolean;
  visibleCols: Record<string, boolean>;
}

export const AgentHostRow = memo(function AgentHostRow({
  group,
  expanded,
  selectedCount,
  onToggleExpand,
  onToggleSelect,
  onInteract,
  isFocused,
  visibleCols,
}: AgentHostRowProps) {
  const { t } = useI18n();
  const n = group.sessions.length;
  const allSelected = selectedCount === n && n > 0;
  const users = [...new Set(group.sessions.map((s) => s.username).filter(Boolean))];
  const os = group.os || "";
  const OsIcon = os.toLowerCase() === "windows" ? Monitor : os.toLowerCase() === "linux" ? Terminal : Apple;
  const versions = hostImplantVersions(group.sessions);
  const borderLeft = group.status === "online" ? "border-l-2 border-l-success" :
    group.status === "stale" ? "border-l-2 border-l-warning" : "border-l-2 border-l-destructive";

  return (
    <tr
      tabIndex={0}
      data-agent-id={group.sessions[0]?.id || ""}
      className={`group cursor-default hover:bg-secondary/50 ${borderLeft} bg-muted/20 ${isFocused ? "bg-secondary/70" : ""}`}
      onDoubleClick={onToggleExpand}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          onInteract(group.sessions[0]?.id || "");
        }
      }}
    >
      <TableCell className="py-1 px-2">
        <Checkbox
          aria-label={t("common.select_item")}
          checked={allSelected}
          onCheckedChange={(v) => onToggleSelect(v === true)}
          onClick={(e) => e.stopPropagation()}
        />
      </TableCell>
      <TableCell className="py-1 px-2">
        <div className="flex items-center gap-1 min-w-0">
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            onClick={(e) => { e.stopPropagation(); onToggleExpand(); }}
            aria-expanded={expanded}
            aria-label={t("agents.host_sessions", { n })}
          >
            {expanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
          </Button>
          <Button
            variant="link"
            size="sm"
            onClick={(e) => { e.stopPropagation(); onInteract(group.sessions[0]?.id || ""); }}
            className="font-mono text-xs text-primary hover:underline p-0 h-auto"
          >
            {group.hostname}
          </Button>
          <Badge variant="secondary" className="h-5 px-1.5 text-(--fs-micro-sm) font-mono">
            {t("agents.host_sessions", { n })}
          </Badge>
        </div>
      </TableCell>
      {visibleCols.username && (
        <TableCell className="py-1 px-2 text-xs font-mono text-muted-foreground truncate max-w-[160px]" title={users.join(", ")}>
          {users.join(", ") || "—"}
        </TableCell>
      )}
      {visibleCols.os && (
        <TableCell className="py-1 px-2">
          <Badge variant="secondary" className="text-(--fs-xs-sm) gap-1.5 bg-secondary/60">
            <OsIcon className="w-3 h-3 text-muted-foreground" />
            {os || "—"}
          </Badge>
        </TableCell>
      )}
      {visibleCols.ip && (
        <TableCell className="py-1 px-2 font-mono text-xs text-muted-foreground">{group.ip || "—"}</TableCell>
      )}
      {visibleCols.last_seen && (
        <TableCell className="py-1 px-2 text-xs font-mono text-muted-foreground whitespace-nowrap">
          <span title={group.last_seen ? formatTime(group.last_seen) : ""}>{timeAgo(group.last_seen, t)}</span>
        </TableCell>
      )}
      {visibleCols.window && <TableCell className="py-1 px-2 max-sm:hidden" />}
      {visibleCols.lock && <TableCell className="py-1 px-2 max-sm:hidden" />}
      {visibleCols.tasks && <TableCell className="py-1 px-2 max-sm:hidden" />}
      {visibleCols.version && (
        <TableCell className="py-1 px-2 text-xs font-mono text-muted-foreground/70 max-sm:hidden">
          {versions || <span className="text-warning" title={t("agents.version_unknown_hint")}>{t("agents.version_unknown")}</span>}
        </TableCell>
      )}
      {visibleCols.status && (
        <TableCell className="py-1 px-2">
          <StatusBadge status={group.status} variant="dot" size="sm" pulse={group.status === "online"} />
        </TableCell>
      )}
      <TableCell className="py-1 px-2" />
    </tr>
  );
});
