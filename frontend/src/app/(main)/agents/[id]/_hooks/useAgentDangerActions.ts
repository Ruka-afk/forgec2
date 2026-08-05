"use client";

import { useCallback, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";

export function useAgentDangerActions(
  agentId: string,
  reloadDetail: () => Promise<void> | void,
  messages: {
    killSuccess: string;
    killFailed: string;
    uninstallSuccess: string;
    uninstallFailed: string;
    killDateSuccess: string;
    killDateFailed: string;
    clearKillDateSuccess: string;
    clearKillDateFailed: string;
  },
) {
  const [busy, setBusy] = useState<string | null>(null);

  const killAgent = useCallback(async () => {
    if (!agentId) return;
    setBusy("kill");
    try {
      await api.postJson(paths.agents.kill(agentId), {});
      toast.success(messages.killSuccess);
    } catch {
      toast.error(messages.killFailed);
    } finally {
      setBusy(null);
    }
  }, [agentId, messages.killSuccess, messages.killFailed]);

  const uninstallAgent = useCallback(async () => {
    if (!agentId) return;
    setBusy("uninstall");
    try {
      await api.postJson(paths.agents.uninstall(agentId), {});
      toast.success(messages.uninstallSuccess);
    } catch {
      toast.error(messages.uninstallFailed);
    } finally {
      setBusy(null);
    }
  }, [agentId, messages.uninstallSuccess, messages.uninstallFailed]);

  const setKillDate = useCallback(async (killDate: string) => {
    if (!agentId) return;
    setBusy("kill_date");
    try {
      await api.postJson(paths.agents.killDate(agentId), { kill_date: killDate });
      toast.success(messages.killDateSuccess);
      await Promise.resolve(reloadDetail());
    } catch {
      toast.error(messages.killDateFailed);
    } finally {
      setBusy(null);
    }
  }, [agentId, reloadDetail, messages.killDateSuccess, messages.killDateFailed]);

  const clearKillDate = useCallback(async () => {
    if (!agentId) return;
    setBusy("clear_kill_date");
    try {
      await api.del(paths.agents.killDate(agentId));
      toast.success(messages.clearKillDateSuccess);
      await Promise.resolve(reloadDetail());
    } catch {
      toast.error(messages.clearKillDateFailed);
    } finally {
      setBusy(null);
    }
  }, [agentId, reloadDetail, messages.clearKillDateSuccess, messages.clearKillDateFailed]);

  return {
    busy,
    killAgent,
    uninstallAgent,
    setKillDate,
    clearKillDate,
  };
}
