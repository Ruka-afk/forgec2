import { useCallback } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useErrorToast } from "@/lib/hooks/useErrorToast";
import { useMutation } from "@/lib/hooks/useMutation";
import { useI18n } from "@/lib/i18n";

export function useAgentDangerActions(
  agentId: string,
  reloadDetail: () => Promise<void> | void,
) {
  const { t } = useI18n();
  const { toastError } = useErrorToast();

  // Every action resolves to whether it actually succeeded. The mutations
  // never reject (their onError already toasts), so a caller that does
  // `await run(); close()` would close its confirm dialog on failure too —
  // losing whatever the operator typed into it. Returning the outcome lets
  // the caller keep the dialog open.

  const { mutate: runKill, isPending: killing } = useMutation({
    fn: async () => {
      await api.postJson(paths.agents.kill(agentId), {});
      return true;
    },
    onSuccess: () => toast.success(t("agents.detail_kill_sent")),
    onError: (e) => toastError(e, "agents.detail_kill_failed"),
  });

  const { mutate: runUninstall, isPending: uninstalling } = useMutation({
    fn: async () => {
      await api.postJson(paths.agents.uninstall(agentId), {});
      return true;
    },
    onSuccess: () => toast.success(t("agents.detail_uninstall_sent")),
    onError: (e) => toastError(e, "agents.detail_uninstall_failed"),
  });

  const { mutate: runSetKillDate, isPending: settingKillDate } = useMutation({
    fn: async (killDate: string) => {
      await api.postJson(paths.agents.killDate(agentId), { kill_date: killDate });
      return true;
    },
    onSuccess: () => {
      toast.success(t("agents.detail_kill_date_set"));
      // useAgentDetail never rejects; keep the reload off the promise chain.
      void Promise.resolve(reloadDetail());
    },
    onError: (e) => toastError(e, "agents.detail_kill_date_set_failed"),
  });

  const { mutate: runClearKillDate, isPending: clearingKillDate } = useMutation({
    fn: async () => {
      await api.del(paths.agents.killDate(agentId));
      return true;
    },
    onSuccess: () => {
      toast.success(t("agents.detail_kill_date_cleared"));
      void Promise.resolve(reloadDetail());
    },
    onError: (e) => toastError(e, "agents.detail_kill_date_clear_failed"),
  });

  const { mutate: runMigrate, isPending: migrating } = useMutation({
    fn: async (path: string) => {
      await api.post(paths.agents.migrate(agentId), path.trim() ? { path: path.trim() } : {});
      return true;
    },
    onSuccess: () => toast.success(t("agents.detail_migrate_sent")),
    onError: (e) => toastError(e, "agents.detail_migrate_failed"),
  });

  const killAgent = useCallback(async (): Promise<boolean> => {
    if (!agentId) return false;
    return (await runKill()) === true;
  }, [agentId, runKill]);

  const uninstallAgent = useCallback(async (): Promise<boolean> => {
    if (!agentId) return false;
    return (await runUninstall()) === true;
  }, [agentId, runUninstall]);

  const setKillDate = useCallback(async (killDate: string): Promise<boolean> => {
    if (!agentId) return false;
    return (await runSetKillDate(killDate)) === true;
  }, [agentId, runSetKillDate]);

  const clearKillDate = useCallback(async (): Promise<boolean> => {
    if (!agentId) return false;
    return (await runClearKillDate()) === true;
  }, [agentId, runClearKillDate]);

  const migrateAgent = useCallback(async (path: string): Promise<boolean> => {
    if (!agentId) return false;
    return (await runMigrate(path)) === true;
  }, [agentId, runMigrate]);

  return {
    busy: killing || uninstalling || settingKillDate || clearingKillDate || migrating,
    killAgent,
    uninstallAgent,
    migrateAgent,
    setKillDate,
    clearKillDate,
  };
}
