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
    migrateSuccess: string;
    migrateFailed: string;
  },
) {
  const [busy, setBusy] = useState<string | null>(null);

  // Every action resolves to whether it actually succeeded. They never reject
  // (the catch below already toasts), so a caller that does `await run(); close()`
  // would close its confirm dialog on failure too — losing whatever the operator
  // typed into it. Returning the outcome lets the caller keep the dialog open.

  const killAgent = useCallback(async (): Promise<boolean> => {
    if (!agentId) return false;
    setBusy("kill");
    try {
      await api.postJson(paths.agents.kill(agentId), {});
      toast.success(messages.killSuccess);
      return true;
    } catch {
      toast.error(messages.killFailed);
      return false;
    } finally {
      setBusy(null);
    }
  }, [agentId, messages.killSuccess, messages.killFailed]);

  const uninstallAgent = useCallback(async (): Promise<boolean> => {
    if (!agentId) return false;
    setBusy("uninstall");
    try {
      await api.postJson(paths.agents.uninstall(agentId), {});
      toast.success(messages.uninstallSuccess);
      return true;
    } catch {
      toast.error(messages.uninstallFailed);
      return false;
    } finally {
      setBusy(null);
    }
  }, [agentId, messages.uninstallSuccess, messages.uninstallFailed]);

  const setKillDate = useCallback(async (killDate: string): Promise<boolean> => {
    if (!agentId) return false;
    setBusy("kill_date");
    try {
      await api.postJson(paths.agents.killDate(agentId), { kill_date: killDate });
      toast.success(messages.killDateSuccess);
      await Promise.resolve(reloadDetail());
      return true;
    } catch {
      toast.error(messages.killDateFailed);
      return false;
    } finally {
      setBusy(null);
    }
  }, [agentId, reloadDetail, messages.killDateSuccess, messages.killDateFailed]);

  const clearKillDate = useCallback(async (): Promise<boolean> => {
    if (!agentId) return false;
    setBusy("clear_kill_date");
    try {
      await api.del(paths.agents.killDate(agentId));
      toast.success(messages.clearKillDateSuccess);
      await Promise.resolve(reloadDetail());
      return true;
    } catch {
      toast.error(messages.clearKillDateFailed);
      return false;
    } finally {
      setBusy(null);
    }
  }, [agentId, reloadDetail, messages.clearKillDateSuccess, messages.clearKillDateFailed]);

  const migrateAgent = useCallback(async (path: string): Promise<boolean> => {
    if (!agentId) return false;
    setBusy("migrate");
    try {
      await api.post(paths.agents.migrate(agentId), path.trim() ? { path: path.trim() } : {});
      toast.success(messages.migrateSuccess);
      return true;
    } catch {
      toast.error(messages.migrateFailed);
      return false;
    } finally {
      setBusy(null);
    }
  }, [agentId, messages.migrateSuccess, messages.migrateFailed]);

  return {
    busy,
    killAgent,
    uninstallAgent,
    migrateAgent,
    setKillDate,
    clearKillDate,
  };
}
