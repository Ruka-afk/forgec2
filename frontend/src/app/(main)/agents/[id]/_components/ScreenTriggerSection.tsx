"use client";

import { memo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "sonner";
import { Camera, Crosshair, Square } from "lucide-react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import CollectCard from "./CollectCard";
import { useCollectTask } from "./useCollectTask";

interface ScreenTriggerSectionProps {
  agentId: string;
  online: boolean;
}

export default memo(function ScreenTriggerSection({ agentId, online }: ScreenTriggerSectionProps) {
  const { t } = useI18n();
  const { busy, fire } = useCollectTask(agentId);
  const [match, setMatch] = useState("");
  const [interval, setInterval] = useState("5");
  const [watching, setWatching] = useState<string | null>(null);

  const run = async (kind: "start" | "stop" | "window") => {
    if (kind === "start" && !match.trim()) {
      toast.error(t("agents.trigger_match_required"));
      return;
    }
    if (kind === "start") {
      const secs = Math.max(2, Math.min(120, parseInt(interval, 10) || 5));
      const ok = await fire(
        kind,
        paths.agents.screenTriggerStart(agentId),
        { match: match.trim(), interval: String(secs) },
        t("agents.trigger_started"),
      );
      if (ok) setWatching(match.trim());
    } else if (kind === "stop") {
      const ok = await fire(kind, paths.agents.screenTriggerStop(agentId), {}, t("agents.trigger_stopped"));
      if (ok) setWatching(null);
    } else {
      await fire(kind, paths.agents.screenshotWindow(agentId), {}, t("agents.trigger_window_queued"));
    }
  };

  return (
    <CollectCard
      title={t("agents.trigger_title")}
      icon={<Crosshair className="size-3.5" />}
      headerRight={
        <>
          {watching !== null && (
            <Badge variant="success" className="gap-1 text-xs">
              <span className="size-1.5 animate-pulse rounded-full bg-success" />
              {t("agents.trigger_watching").replace("{match}", watching)}
            </Badge>
          )}
          <Button size="sm" variant="outline" disabled={!online || busy !== null} onClick={() => void run("window")}>
            {busy === "window" ? <Spinner size="xs" /> : <Camera className="size-4" />}
            {t("agents.trigger_window")}
          </Button>
        </>
      }
      emptyIcon={Crosshair}
      emptyTitle={t("agents.trigger_empty_title")}
      emptyHint={t("agents.trigger_empty_hint")}
      result={null}
      resultOverride={watching !== null ? <></> : undefined}
    >
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_6rem_auto]">
        <div>
          <Label className="mb-1 block text-xs text-muted-foreground">{t("agents.trigger_match")}</Label>
          <Input
            value={match}
            onChange={(e) => setMatch(e.target.value)}
            placeholder={t("agents.trigger_match_ph")}
            className="h-8 font-mono text-xs"
          />
        </div>
        <div>
          <Label className="mb-1 block text-xs text-muted-foreground">{t("agents.trigger_interval")}</Label>
          <Input
            value={interval}
            onChange={(e) => setInterval(e.target.value.replace(/[^0-9]/g, "").slice(0, 3))}
            className="h-8 font-mono text-xs"
            inputMode="numeric"
          />
        </div>
        <div className="flex items-end gap-2">
          <Button size="sm" disabled={!online || busy !== null || !match.trim()} onClick={() => void run("start")}>
            {busy === "start" && <Spinner size="xs" />}
            {t("agents.trigger_start")}
          </Button>
          <Button size="sm" variant="outline" disabled={!online || busy !== null} onClick={() => void run("stop")}>
            {busy === "stop" ? <Spinner size="xs" /> : <Square className="size-4" />}
            {t("agents.trigger_stop")}
          </Button>
        </div>
      </div>
    </CollectCard>
  );
});
