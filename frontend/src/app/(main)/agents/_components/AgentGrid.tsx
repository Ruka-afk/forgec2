"use client";

import { memo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Card } from "@/components/ui/card";
import { AvatarFallback } from "@/components/ui/avatar";
import { timeAgo, formatTime, enumLabel } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import type { Beacon, Tag } from "./types";
import { avatarColor, formatUptime, osIcon, agentStatusBorderClass, integrityTone } from "./types";
import type { AgentMenuPoint } from "./agent-menu-actions";
import { groupBeaconsByHost } from "./groupBeaconsByHost";
import { Clock, Globe } from "lucide-react";
import { StatusDot } from "@/components/ui/status-dot";

interface AgentGridProps {
  beacons: Beacon[];
  tagsByAgent: Record<string, Tag[]>;
  taskCountMap: Record<string, number>;
  activeId?: string | null;
  onInteract: (id: string) => void;
  onDetails: (id: string) => void;
  onMenu: (point: AgentMenuPoint) => void;
  selected?: Set<string>;
  onToggleSelect?: (id: string, checked: boolean) => void;
}

// Cap initial card render: thousands of host groups would mount thousands
// of cards at once on the (unvirtualized) mobile grid path.
const GRID_PAGE = 120;

export const AgentGrid = memo(function AgentGrid({ beacons, tagsByAgent, taskCountMap, activeId, onInteract, onDetails, onMenu, selected, onToggleSelect }: AgentGridProps) {
  const { t } = useI18n();
  const [visible, setVisible] = useState(GRID_PAGE);
  const groups = groupBeaconsByHost(beacons);
  const shown = groups.slice(0, visible);
  return (
    <div className="p-4">
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
      {shown.map((group) => {
        const beacon = group.sessions[0];
        const id = beacon.id || "";
        const hostname = group.hostname || "-";
        const ip = group.ip || "-";
        const os = group.os || "";
        const status = group.status || "offline";
        const sessionN = group.sessions.length;
        const isSelected = selected?.has(id) ?? false;
        const OsIcon = osIcon(os);
        const itone = integrityTone(beacon.integrity);
        const integrityClass = itone === "destructive" ? "bg-destructive/10 text-destructive"
          : itone === "success" ? "bg-success/15 text-success"
          : "bg-warning/15 text-warning";
        return (
          <Card
            key={id}
            role="button"
            tabIndex={0}
            aria-label={`${hostname} ${os} ${status} ${ip}`}
            onClick={() => onInteract(id)}
            onDoubleClick={() => onDetails(id)}
            onContextMenu={(e) => {
              e.preventDefault();
              onMenu({ x: e.clientX, y: e.clientY, beacon });
            }}
            onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onInteract(id); } }}
            className={`p-4 cursor-pointer hover:ring-2 hover:ring-primary/50 hover:shadow-md transition-all duration-200 ${agentStatusBorderClass(status)} group ring-0 shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/70 ${activeId === id ? "ring-2 ring-primary/50" : ""} ${isSelected ? "ring-2 ring-primary/60" : ""}`}
          >
            <div className="flex items-start justify-between mb-2">
              <div className="flex items-center gap-2.5 min-w-0">
                {onToggleSelect && (
                  <Checkbox
                    checked={isSelected}
                    onCheckedChange={(checked) => onToggleSelect(id, checked === true)}
                    onClick={(e) => e.stopPropagation()}
                    onKeyDown={(e) => e.stopPropagation()}
                    aria-label={t("agents.select_agent")}
                    className="shrink-0"
                  />
                )}
                <AvatarFallback name={hostname} size="md" shape="xl" color={avatarColor(hostname)} />
                <div className="min-w-0">
                  <span className="font-semibold text-sm text-primary truncate block group-hover:underline">{hostname}</span>
                  <span className="text-(--fs-xs-sm) text-muted-foreground/100 truncate block">
                    {sessionN > 1 ? t("agents.host_sessions", { n: sessionN }) : (beacon.username || "")}
                  </span>
                </div>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <StatusDot status={status} size="sm" pulse={status === "online"} />
                <span className="text-(--fs-micro-sm) text-muted-foreground/100 capitalize">{enumLabel(t, "agents", `${status}_label`)}</span>
              </div>
            </div>
            <div className="space-y-1.5 text-xs text-muted-foreground">
              <div className="flex items-center gap-1.5">
                <OsIcon className="size-3 text-muted-foreground/100" aria-hidden="true" />
                <span>{os}{beacon.arch ? ` ${beacon.arch}` : ""}</span>
                {beacon.integrity && <span className={`px-1 py-0.5 rounded text-(--fs-micro) font-semibold ${integrityClass}`}>{beacon.integrity}</span>}
              </div>
              <div className="flex items-center gap-1.5">
                <Globe className="size-3 text-muted-foreground/100" aria-hidden="true" />
                <span className="font-mono">{ip}</span>
              </div>
              <div className="flex items-center gap-1.5">
                <Clock className="size-3 text-muted-foreground/100" aria-hidden="true" />
                <span title={beacon.last_seen ? formatTime(beacon.last_seen) : ""}>{timeAgo(beacon.last_seen || "", t)}</span>
                {beacon.created_at && <span className="text-(--fs-micro) text-muted-foreground/100 ml-1">{t("format.uptime.up")} {formatUptime(beacon.last_seen || beacon.created_at, t)}</span>}
              </div>
            </div>
            <div className="flex items-center gap-1.5 mt-3 pt-2 border-t border-border/40">
              {(tagsByAgent[id] || []).slice(0, 5).map((tag) => (
                <span key={tag.id} className="size-2.5 rounded-full ring-1.5 ring-white dark:ring-border shadow-sm" style={{ backgroundColor: tag.color }} title={tag.name} />
              ))}
              <span className="ml-auto text-(--fs-micro-sm) text-muted-foreground/100">{t("agents.n_tasks").replace("{n}", String(taskCountMap[id] ?? 0))}</span>
            </div>
          </Card>
        );
      })}
    </div>
    {visible < groups.length && (
      <div className="mt-3 flex justify-center">
        <Button variant="outline" size="sm" onClick={() => setVisible((v) => v + GRID_PAGE)} className="rounded-full">
          {t("agents.grid_show_more").replace("{shown}", String(shown.length)).replace("{total}", String(groups.length))}
        </Button>
      </div>
    )}
    </div>
  );
})
