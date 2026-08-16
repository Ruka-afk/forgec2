"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Camera, ListChecks, Network, Timer } from "lucide-react";
import {
  parseSleepArgs,
  parseSocksPort,
  queueDockCommand,
  socksRelayStatus,
  startDockSocks,
  stopDockSocks,
  type DockCommandKind,
} from "./dock-commands";

interface AgentDockCommandsProps {
  agentId: string;
  intervalHint?: number;
  jitterHint?: number;
  onQueued: (task: { task_id: number; type: string; command: string }) => void;
}

export function AgentDockCommands({ agentId, intervalHint, jitterHint, onQueued }: AgentDockCommandsProps) {
  const { t } = useI18n();
  const [sleepRaw, setSleepRaw] = useState(
    `${intervalHint && intervalHint > 0 ? intervalHint : 60},${jitterHint && jitterHint >= 0 ? jitterHint : 10}`,
  );
  const [busy, setBusy] = useState<DockCommandKind | "socks" | null>(null);
  const [socksPort, setSocksPort] = useState("1080");
  const [socksOn, setSocksOn] = useState(false);

  useEffect(() => {
    let cancelled = false;
    socksRelayStatus(agentId)
      .then((st) => {
        if (cancelled) return;
        setSocksOn(st.active);
        if (st.port) setSocksPort(String(st.port));
      })
      .catch(() => {
        if (!cancelled) setSocksOn(false);
      });
    return () => { cancelled = true; };
  }, [agentId]);

  // Reset per-agent params when switching sessions (hints arrive together
  // with the new beacon on the same render as the id change).
  useEffect(() => {
    setSleepRaw(`${intervalHint && intervalHint > 0 ? intervalHint : 60},${jitterHint && jitterHint >= 0 ? jitterHint : 10}`);
    setSocksPort("1080");
    setBusy(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agentId]);

  const run = async (kind: DockCommandKind) => {
    if (!agentId || busy) return;
    let sleep: { interval: number; jitter: number } | undefined;
    if (kind === "sleep") {
      const parsed = parseSleepArgs(sleepRaw);
      if (!parsed) {
        toast.error(t("agents.dock_cmd_sleep_invalid"));
        return;
      }
      sleep = parsed;
    }
    setBusy(kind);
    try {
      const queued = await queueDockCommand(agentId, kind, sleep);
      toast.success(t("agents.dock_cmd_queued", { type: queued.type }));
      onQueued(queued);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("agents.detail_action_failed", { label: kind }));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="flex items-center gap-1 border-b border-border px-2 py-1">
      <Button type="button" variant="ghost" size="xs" disabled={!!busy} onClick={() => void run("ps")}>
        <ListChecks className="size-3.5" />
        {t("agents.dock_cmd_ps")}
      </Button>
      <Button type="button" variant="ghost" size="xs" disabled={!!busy} onClick={() => void run("screenshot")}>
        <Camera className="size-3.5" />
        {t("agents.dock_cmd_screenshot")}
      </Button>
      <Input
        value={sleepRaw}
        onChange={(e) => setSleepRaw(e.target.value)}
        aria-label={t("agents.dock_cmd_sleep")}
        placeholder="60,10"
        className="h-7 w-24 font-mono text-xs"
      />
      <Button type="button" variant="ghost" size="xs" disabled={!!busy} onClick={() => void run("sleep")}>
        <Timer className="size-3.5" />
        {t("agents.dock_cmd_sleep")}
      </Button>
      <Input
        value={socksPort}
        onChange={(e) => setSocksPort(e.target.value)}
        aria-label={t("agents.dock_cmd_socks")}
        className="h-7 w-16 font-mono text-xs"
      />
      <Button
        type="button"
        variant="ghost"
        size="xs"
        disabled={!!busy}
        onClick={() => {
          if (socksOn) {
            setBusy("socks");
            void stopDockSocks(agentId)
              .then(() => {
                setSocksOn(false);
                toast.success(t("agents.dock_socks_stopped"));
              })
              .catch((e) => toast.error(e instanceof Error ? e.message : t("agents.dock_socks_failed")))
              .finally(() => setBusy(null));
            return;
          }
          const port = parseSocksPort(socksPort);
          if (!port) {
            toast.error(t("agents.dock_socks_port_invalid"));
            return;
          }
          setBusy("socks");
          void startDockSocks(agentId, port)
            .then((res) => {
              setSocksOn(true);
              setSocksPort(String(res.port));
              toast.success(res.message || t("agents.dock_socks_started", { port: String(res.port) }));
            })
            .catch((e) => toast.error(e instanceof Error ? e.message : t("agents.dock_socks_failed")))
            .finally(() => setBusy(null));
        }}
      >
        <Network className="size-3.5" />
        {socksOn ? t("agents.dock_socks_stop") : t("agents.dock_cmd_socks")}
      </Button>
    </div>
  );
}
