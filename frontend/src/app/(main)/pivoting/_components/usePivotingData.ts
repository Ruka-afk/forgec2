"use client";

import { useCallback } from "react";
import { toast } from "sonner";
import { api, formatThrownError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { normalizeListEnvelope } from "@/lib/envelope";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { POLL } from "@/lib/polling";
import type { PivotAgent, RelaySession, RPortForwardStatus } from "./types";

export function usePivotingData() {
  const { t } = useI18n();

  const { data, loading, refresh: loadData } = useApiResource<{
    sessions: RelaySession[];
    agents: PivotAgent[];
    rportForwards: RPortForwardStatus[];
  }>({
    fetcher: async () => {
      const [sessData, agentsData, rportData] = await Promise.all([
        api.get(paths.socks.sessions).catch(() => null),
        api.get(paths.agents.list("status=online")).catch(() => null),
        api.get(paths.rportfwd.status).catch(() => null),
      ]);
      return {
        sessions: sessData ? normalizeListEnvelope(sessData, ["sessions", "data"]) as RelaySession[] : [],
        agents: agentsData ? normalizeListEnvelope(agentsData, ["agents", "data"]) as PivotAgent[] : [],
        rportForwards: rportData ? normalizeListEnvelope(rportData, ["forwards", "data"]) as RPortForwardStatus[] : [],
      };
    },
    pollMs: POLL.pivoting,
    toastThrottleMs: POLL.toastThrottleShort,
    errorMessage: t("pivoting.toast.load_failed"),
  });
  const sessions = data?.sessions ?? [];
  const agents = data?.agents ?? [];
  const rportForwards = data?.rportForwards ?? [];

  const startRelay = useCallback(
    async (selectedAgent: string, relayPort: number, relayHost: string, relayProtocol: string) => {
      if (!selectedAgent) return false;
      try {
        await api.post(paths.agents.socksRelayStart(selectedAgent), {
          agent_id: selectedAgent,
          port: relayPort.toString(),
          host: relayHost,
          protocol: relayProtocol,
        });
        toast.success(t("pivoting.toast.socks_relay_started"));
        await loadData();
        return true;
      } catch {
        toast.error(t("pivoting.toast.socks_relay_start_failed"));
        return false;
      }
    },
    [loadData, t],
  );

  const stopRelay = useCallback(
    async (agentId: string) => {
      try {
        await api.post(paths.agents.socksRelayStop(agentId), { agent_id: agentId });
        toast.success(t("pivoting.toast.socks_relay_stopped"));
      } catch {
        toast.error(t("pivoting.toast.socks_relay_stop_failed"));
      }
      await loadData();
    },
    [loadData, t],
  );

  const startLocalProxy = useCallback(
    async (
      throughAgent: string,
      localPort: number,
      auth?: { username: string; password: string },
    ) => {
      try {
        const body: Record<string, string> = {
          port: localPort.toString(),
          through_agent: throughAgent,
        };
        if (auth) {
          body.auth_enabled = "true";
          body.username = auth.username;
          body.password = auth.password;
        }
        await api.post(paths.agents.socks(throughAgent), body);
        toast.success(t("pivoting.toast.local_proxy_started", { localPort: String(localPort) }));
        return true;
      } catch {
        toast.error(t("pivoting.toast.local_proxy_start_failed"));
        return false;
      }
    },
    [t],
  );

  const startRPort = useCallback(
    async (opts: {
      rportAgent: string;
      remoteHost: string;
      remotePort: number;
      localPort: number;
      protocol: string;
    }) => {
      if (!opts.rportAgent) return false;
      try {
        await api.post(paths.agents.rportfwdStart(opts.rportAgent), {
          lport: opts.localPort.toString(),
          target: `${opts.remoteHost}:${opts.remotePort}`,
        });
        toast.success(t("pivoting.toast.rport_started"));
        await loadData();
        return true;
      } catch (err) {
        toast.error(formatThrownError(err));
        return false;
      }
    },
    [loadData, t],
  );

  const stopRPort = useCallback(
    async (agentId: string, lport: number) => {
      try {
        await api.post(paths.agents.rportfwdStop(agentId), { lport: lport.toString() });
        toast.success(t("pivoting.toast.rport_stopped"));
      } catch {
        toast.error(t("pivoting.toast.rport_stop_failed"));
      }
      await loadData();
    },
    [loadData, t],
  );

  const checkRPortStatus = useCallback(
    async (agentId: string) => {
      try {
        const data = await api.get(paths.agents.rportfwdStatus(agentId));
        toast.info(
          t("pivoting.toast.rport_status", {
            id: agentId,
            status: data.active ? "Active" : "Inactive",
          }),
        );
      } catch {
        toast.error(t("pivoting.toast.rport_status_check_failed"));
      }
    },
    [t],
  );

  return {
    sessions,
    agents,
    loading,
    rportForwards,
    loadData,
    startRelay,
    stopRelay,
    startLocalProxy,
    startRPort,
    stopRPort,
    checkRPortStatus,
  };
}
