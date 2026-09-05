"use client";

import { memo } from "react";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { SafeImg } from "@/components/ui/safe-img";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Clock, Monitor, TriangleAlert, Zap } from "lucide-react";
import { ScreenVideoPlayer } from "@/components/ScreenVideoPlayer";
import { safeImageSrc } from "@/lib/safeUrl";
import type { BusyAction, CaptureStatus, MonitorStatus } from "./useScreenMonitor";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface ScreenViewerProps {
  t: TKey;
  screenshot: string | null;
  videoMode: boolean;
  wsLive: boolean;
  interval: number;
  status: CaptureStatus;
  monitoring: boolean;
  monitoringStatus: MonitorStatus;
  busyAction: BusyAction;
  lastUpdate: string;
  resolution: { width: number; height: number } | null;
  onOpenModal: (image: string) => void;
  onActivatePreview: (event: React.KeyboardEvent<HTMLDivElement>, image: string) => void;
}

/** Live view card: status strip + video/still frame + capture overlay. */
export default memo(function ScreenViewer({
  t, screenshot, videoMode, wsLive, interval, status,
  monitoring, monitoringStatus, busyAction, lastUpdate, resolution,
  onOpenModal, onActivatePreview,
}: ScreenViewerProps) {
  const indicator = (() => {
    if (status === "capturing") return { color: "bg-warning", text: t("screen.capturing"), icon: <Spinner size="xs" /> };
    if (status === "error") return { color: "bg-destructive", text: t("screen.error"), icon: <TriangleAlert className="size-3" /> };
    if (monitoring && monitoringStatus === "connected") {
      return { color: "bg-success", text: t("agents.rdp_connected"), icon: <span className="size-1.5 rounded-full bg-current" /> };
    }
    return { color: "bg-muted-foreground", text: t("agents.rdp_standby"), icon: <span className="size-1.5 rounded-full bg-current" /> };
  })();
  const capturing = busyAction === "capture" || busyAction === "window";

  return (
    <Card className="flex min-h-[28rem] min-w-0 flex-col overflow-hidden lg:min-h-0">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-border bg-muted/70 px-4 py-2.5">
        <div className="flex items-center gap-2">
          <Monitor className="size-4" aria-hidden="true" />
          <span className="text-sm font-medium text-foreground">{t("agents.screen_live_view")}</span>
        </div>
        <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
          <span className={`flex items-center gap-1.5 ${status === "error" ? "text-destructive" : ""}`}>
            <span className={`size-2 rounded-full ${indicator.color}`} aria-hidden="true" />
            {indicator.icon}
            {indicator.text}
          </span>
          <span className="hidden items-center gap-1 sm:flex">
            <Clock className="size-3.5" aria-hidden="true" />
            {lastUpdate}
          </span>
          {wsLive && (
            <Tooltip>
              <TooltipTrigger render={<span className="flex items-center gap-1 text-chart-1" />}>
                <Zap className="size-3.5" aria-hidden="true" /> WS
              </TooltipTrigger>
              <TooltipContent>{t("agents.screen_ws_live")}</TooltipContent>
            </Tooltip>
          )}
        </div>
      </div>

      <div
        className="relative flex flex-1 items-center justify-center overflow-hidden bg-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
        role={screenshot && !videoMode ? "button" : undefined}
        tabIndex={screenshot && !videoMode ? 0 : undefined}
        aria-label={screenshot && !videoMode ? t("agents.screen_alt_full") : undefined}
        onClick={() => screenshot && !videoMode && onOpenModal(screenshot)}
        onKeyDown={(event) => screenshot && !videoMode && onActivatePreview(event, screenshot)}
      >
        {videoMode ? (
          <ScreenVideoPlayer src={screenshot} wsLive={wsLive} fps={Math.round(1000 / Math.max(1000, interval * 1000)) || 1} className="h-full w-full" onFullscreen={() => screenshot && onOpenModal(screenshot)} />
        ) : screenshot ? (
          <SafeImg
            src={safeImageSrc(screenshot)}
            alt={t("agents.screenshot")}
            width={resolution?.width || undefined}
            height={resolution?.height || undefined}
            style={{ aspectRatio: resolution ? `${resolution.width} / ${resolution.height}` : undefined }}
            className="max-h-full max-w-full object-contain"
            loading="eager"
            decoding="async"
          />
        ) : (
          <EmptyState icon={Monitor} title={t("agents.screen_no_screenshots")} message={`${t("agents.screen_start")} / ${t("agents.screen_capture")}`} />
        )}
        {capturing && (
          <div className="absolute inset-0 flex items-center justify-center bg-background/75 backdrop-blur-sm">
            <div className="flex flex-col items-center gap-3 rounded-xl border border-border bg-card px-6 py-5 shadow-lg">
              <Spinner size="xl" />
              <span className="text-sm font-medium text-foreground">{t("screen.waiting_for_agent")}</span>
            </div>
          </div>
        )}
      </div>
    </Card>
  );
});
