"use client";

import { memo } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Compass, Expand, Maximize2, Play, Square, Users } from "lucide-react";
import { isExperimentalDesktop } from "../../_components/session-quality";
import type { ResolutionOption } from "./useRemoteDesktop";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface RdpToolbarProps {
  t: TKey;
  agentId: string;
  nativeWidth: number;
  nativeHeight: number;
  resolutions: ResolutionOption[];
  resolution: string;
  setResolution: (v: string) => void;
  monitoring: boolean;
  isFullscreen: boolean;
  onToggleFullscreen: () => void;
  versionBlocked: boolean;
  onStart: () => void;
  onStop: () => void;
}

/** Remote-desktop header: title, resolution, fullscreen, start/stop. */
export default memo(function RdpToolbar({
  t, agentId, nativeWidth, nativeHeight, resolutions, resolution, setResolution,
  monitoring, isFullscreen, onToggleFullscreen, versionBlocked, onStart, onStop,
}: RdpToolbarProps) {
  return (
    <div className="flex items-center justify-between mb-3 gap-3 flex-wrap shrink-0">
      <div className="flex items-center gap-3">
        <h1 className="text-lg font-bold text-foreground flex items-center gap-2">
          <Users className="size-4" />
          {t("agents.rdp_title")}
          {isExperimentalDesktop("remote-desktop") ? (
            <span className="text-(--fs-micro) font-normal text-warning">{t("generate.quality_experimental")}</span>
          ) : null}
        </h1>
        <Badge variant="secondary" className="text-xs text-muted-foreground font-mono bg-muted/50 px-2 py-0.5 rounded-lg">
          {agentId}
        </Badge>
        {nativeWidth > 0 && nativeHeight > 0 && (
          <Badge variant="secondary" className="text-xs bg-muted/50 text-muted-foreground px-2.5 py-1 rounded-lg flex items-center gap-1">
            <Maximize2 className="size-4" />
            {nativeWidth} x {nativeHeight}
          </Badge>
        )}
      </div>
      <div className="flex items-center gap-2 flex-wrap">
        <Select value={resolution} onValueChange={(v) => { if (v !== null) setResolution(v); }} disabled={monitoring}>
          <SelectTrigger className="w-[180px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {resolutions.map((r) => (
              <SelectItem key={r.value} value={r.value}>
                {r.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Tooltip>
          <TooltipTrigger render={<Button
              variant="outline"
              size="icon-lg"
              onClick={onToggleFullscreen}
              aria-label={isFullscreen ? t("agents.rdp_exit_fullscreen") : t("agents.rdp_enter_fullscreen")}
            />}>
            {isFullscreen ? <Compass className="size-4" /> : <Expand className="size-4" />}
          </TooltipTrigger>
          <TooltipContent>{isFullscreen ? t("agents.rdp_exit_fullscreen") : t("agents.rdp_fullscreen")}</TooltipContent>
        </Tooltip>
        {!monitoring ? (
          <Button
            onClick={onStart}
            disabled={versionBlocked}
            title={versionBlocked ? t("agents.version_unknown_dest") : undefined}
          >
            <Play className="size-4" />
            {t("agents.rdp_start")}
          </Button>
        ) : (
          <Button
            variant="destructive"
            onClick={onStop}
          >
            <Square className="size-4" />
            {t("agents.rdp_stop")}
          </Button>
        )}
      </div>
    </div>
  );
});
