"use client";

import { memo } from "react";
import { Card } from "@/components/ui/card";
import { AvatarFallback } from "@/components/ui/avatar";
import { timeAgo } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import type { Beacon, Tag } from "./types";
import { avatarColor, formatUptime } from "./types";
import { Apple, Clock, Globe, Monitor, Terminal } from "lucide-react";

function getOsIcon(os: string) {
  switch (os.toLowerCase()) {
    case "windows": return Monitor;
    case "linux": return Terminal;
    default: return Apple;
  }
}

interface AgentGridProps {
  beacons: Beacon[];
  tagsByAgent: Record<string, Tag[]>;
  taskCountMap: Record<string, number>;
  onSelect: (id: string) => void;
}

export const AgentGrid = memo(function AgentGrid({ beacons, tagsByAgent, taskCountMap, onSelect }: AgentGridProps) {
  const { t } = useI18n();
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3 p-4">
      {beacons.map((beacon) => {
        const id = beacon.id || "";
        const hostname = beacon.hostname || "-";
        const ip = beacon.ip || "-";
        const os = beacon.os || "";
        const status = beacon.status || "offline";
        const OsIcon = getOsIcon(os);
        const borderColor = status === "online" ? "border-emerald-500" :
          status === "stale" ? "border-amber-500" : "border-red-500";
        return (
          <Card
            key={id}
            onClick={() => onSelect(id)}
            className={`p-4 cursor-pointer hover:ring-2 hover:ring-indigo-500/50 hover:shadow-md transition-all duration-200 border-l-4 ${borderColor} group ring-0 shadow-sm`}
          >
            <div className="flex items-start justify-between mb-2">
              <div className="flex items-center gap-2.5 min-w-0">
                <AvatarFallback name={hostname} size="md" shape="xl" color={avatarColor(hostname)} />
                <div className="min-w-0">
                  <span className="font-semibold text-sm text-indigo-600 dark:text-indigo-400 truncate block group-hover:underline">{hostname}</span>
                  <span className="text-(--font-size-xs-sm) text-muted-foreground/70 truncate block">{beacon.username || ""}</span>
                </div>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <span className={`w-2 h-2 rounded-full ${status === "online" ? "bg-emerald-500 animate-pulse" : status === "stale" ? "bg-amber-500" : "bg-red-500"}`} />
                <span className="text-(--font-size-micro-sm) text-muted-foreground/70 capitalize">{status}</span>
              </div>
            </div>
            <div className="space-y-1.5 text-xs text-muted-foreground">
              <div className="flex items-center gap-1.5">
                <OsIcon className="w-3 h-3 text-muted-foreground/50" />
                <span>{os}{beacon.arch ? ` ${beacon.arch}` : ""}</span>
                {beacon.integrity && <span className={`px-1 py-0.5 rounded text-(--font-size-micro) font-semibold ${
                  beacon.integrity === "System" ? "bg-destructive/10 text-destructive" :
                  beacon.integrity === "High" ? "bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-400" :
                  "bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-400"
                }`}>{beacon.integrity}</span>}
              </div>
              <div className="flex items-center gap-1.5">
                <Globe className="w-3 h-3 text-muted-foreground/50" />
                <span className="font-mono">{ip}</span>
              </div>
              <div className="flex items-center gap-1.5">
                <Clock className="w-3 h-3 text-muted-foreground/50" />
                <span title={beacon.last_seen ? new Date(beacon.last_seen).toLocaleString() : ""}>{timeAgo(beacon.last_seen || "")}</span>
                {beacon.created_at && <span className="text-(--font-size-micro) text-muted-foreground/70 ml-1">up {formatUptime(beacon.last_seen || beacon.created_at)}</span>}
              </div>
            </div>
            <div className="flex items-center gap-1.5 mt-3 pt-2 border-t border-border/40">
              {(tagsByAgent[id] || []).slice(0, 5).map((t) => (
                <span key={t.id} className="w-2.5 h-2.5 rounded-full ring-1.5 ring-white dark:ring-border shadow-sm" style={{ backgroundColor: t.color }} title={t.name} />
              ))}
              <span className="ml-auto text-(--font-size-micro-sm) text-muted-foreground/70">{t("agents.n_tasks").replace("{n}", String(taskCountMap[id] ?? 0))}</span>
            </div>
          </Card>
        );
      })}
    </div>
  );
})
