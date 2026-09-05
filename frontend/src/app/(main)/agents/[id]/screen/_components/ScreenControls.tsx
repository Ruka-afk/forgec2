"use client";

import { memo } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { StatusDot } from "@/components/ui/status-dot";
import { Switch } from "@/components/ui/switch";
import { Clock, Maximize2, Monitor, RotateCw } from "lucide-react";
import type { BusyAction, ScreenQuality } from "./useScreenMonitor";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface ScreenControlsProps {
  t: TKey;
  monitoring: boolean;
  busyAction: BusyAction;
  interval: number;
  setInterval: (v: number) => void;
  quality: ScreenQuality;
  setQuality: (v: ScreenQuality) => void;
  autoRefresh: boolean;
  setAutoRefresh: (v: boolean) => void;
  videoMode: boolean;
  setVideoMode: (v: boolean) => void;
  triggerMatch: string;
  setTriggerMatch: (v: string) => void;
  triggerOn: boolean;
  onTriggerStart: () => void;
  onTriggerStop: () => void;
  resolution: { width: number; height: number } | null;
}

/** Right-rail settings: interval/quality, toggles, title trigger, resolution. */
export default memo(function ScreenControls({
  t, monitoring, busyAction, interval, setInterval, quality, setQuality,
  autoRefresh, setAutoRefresh, videoMode, setVideoMode,
  triggerMatch, setTriggerMatch, triggerOn, onTriggerStart, onTriggerStop, resolution,
}: ScreenControlsProps) {
  return (
    <Card className="shrink-0 p-4">
      <div className="mb-3 flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        <span>{t("agents.screen_controls")}</span>
        <span className={`flex items-center gap-1.5 font-normal normal-case ${monitoring ? "text-success" : "text-muted-foreground"}`}>
          <StatusDot tone={monitoring ? "success" : "muted"} size="xs" pulse={monitoring} />
          {monitoring ? t("screen.live") : t("screen.off")}
        </span>
      </div>

      <div className="space-y-3">
        <label className="block">
          <span className="mb-1.5 flex items-center gap-1 text-xs text-muted-foreground">
            <Clock className="size-3.5" aria-hidden="true" />
            {t("agents.screen_interval")}
          </span>
          <Select value={String(interval)} onValueChange={(value) => value !== null && setInterval(Number(value))} disabled={monitoring || busyAction === "start"}>
            <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="1">1s · {t("screen.high_cpu")}</SelectItem>
              <SelectItem value="3">3s</SelectItem>
              <SelectItem value="5">5s</SelectItem>
              <SelectItem value="10">10s</SelectItem>
            </SelectContent>
          </Select>
        </label>

        <label className="block">
          <span className="mb-1.5 block text-xs text-muted-foreground">{t("screen.quality")}</span>
          <Select value={quality} onValueChange={(value) => value !== null && setQuality(value as ScreenQuality)} disabled={monitoring || busyAction === "start"}>
            <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="low">{t("screen.quality_low")}</SelectItem>
              <SelectItem value="medium">{t("screen.quality_medium")}</SelectItem>
              <SelectItem value="high">{t("screen.quality_high")}</SelectItem>
            </SelectContent>
          </Select>
        </label>

        <div className="flex items-center justify-between gap-3 rounded-lg bg-muted/60 px-3 py-2.5">
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <RotateCw className="size-3.5" aria-hidden="true" />
            {t("agents.screen_auto_refresh")}
          </span>
          <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
        </div>

        <div className="flex items-center justify-between gap-3 rounded-lg bg-primary/5 px-3 py-2.5 ring-1 ring-primary/10">
          <span className="flex items-center gap-1.5 text-xs font-medium text-primary">
            <Monitor className="size-3.5" aria-hidden="true" />
            视频模式
          </span>
          <Switch checked={videoMode} onCheckedChange={setVideoMode} />
        </div>

        <div className="border-t border-border pt-3">
          <span className="mb-1.5 block text-xs text-muted-foreground">{t("agents.screen_trigger")}</span>
          <Input value={triggerMatch} onChange={(event) => setTriggerMatch(event.target.value)} placeholder={t("agents.screen_trigger_placeholder")} className="mb-2" />
          {!triggerOn ? (
            <Button size="sm" variant="outline" className="w-full" onClick={onTriggerStart}>{t("agents.screen_trigger_start")}</Button>
          ) : (
            <Button size="sm" variant="destructive" className="w-full" onClick={onTriggerStop}>{t("agents.screen_trigger_stop")}</Button>
          )}
        </div>

        {resolution && (
          <div className="border-t border-border pt-3">
            <span className="mb-1.5 flex items-center gap-1 text-xs text-muted-foreground">
              <Maximize2 className="size-3.5" aria-hidden="true" />
              {t("screen.resolution")}
            </span>
            <div className="rounded-lg bg-muted px-3 py-2 font-mono text-sm text-foreground">{resolution.width} × {resolution.height}</div>
          </div>
        )}
      </div>
    </Card>
  );
});
