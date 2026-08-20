"use client";

import { useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { firstArray, normalizeListEnvelope } from "@/lib/envelope";
import { normalizeAgentList } from "@/lib/agents";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { isAgentStatus } from "@/lib/status";
import { POLL } from "@/lib/polling";
import { useI18n } from "@/lib/i18n";
import type { NormalizedAgent } from "@/types/agent";
import type { Task } from "@/types/task";
import { emptyLootData, normalizeLootData } from "../../loot/_components/types";
import type { LootData } from "@/types/loot";
import { indexListenerHealth, type ListenerHealth } from "../../listeners/_components/listener-health";

function asAgent(row: Record<string, unknown>): NormalizedAgent {
  const statusRaw = String(row.status ?? "");
  const status: NormalizedAgent["status"] = isAgentStatus(statusRaw) ? statusRaw : "offline";
  return {
    id: String(row.id ?? ""),
    hostname: String(row.hostname ?? ""),
    username: String(row.username ?? ""),
    ip: String(row.ip ?? row.internal_ip ?? ""),
    os: String(row.os ?? ""),
    status,
    last_seen: String(row.last_seen ?? ""),
    listener_id: String(row.listener_id ?? ""),
    tags: String(row.tags ?? ""),
  };
}

interface OpsHomeData {
  agents: NormalizedAgent[];
  healthByTarget: Record<string, ListenerHealth>;
  failedTasks: Task[];
  pendingTasks: Task[];
  approvalTasks: Task[];
  loot: LootData;
  loading: boolean;
  error: string | null;
  refresh: () => void;
}

export function useOpsHomeData(): OpsHomeData {
  const { t } = useI18n();

  const {
    data,
    loading,
    error,
    refresh: refreshData,
  } = useApiResource<Pick<OpsHomeData, "agents" | "healthByTarget" | "failedTasks" | "pendingTasks" | "approvalTasks" | "loot">>({
    fetcher: useCallback(async (signal) => {
      const settled = await Promise.allSettled([
        api.get(paths.agents.list("page=1&pageSize=50"), { signal }),
        api.get(paths.circuitBreaker.detail, { signal }),
        api.get(paths.tasks.list("status=failed&page=1&pageSize=8"), { signal }),
        api.get(paths.tasks.list("status=pending&page=1&pageSize=8"), { signal }),
        api.get(paths.tasks.list("status=pending_approval&page=1&pageSize=8"), { signal }),
        api.get(paths.loot.page, { signal }),
      ]);

      const [agentsRes, healthRes, failedRes, pendingRes, approvalRes, lootRes] = settled;
      let failedLoads = 0;
      let agents: NormalizedAgent[] = [];
      let healthByTarget: Record<string, ListenerHealth> = {};
      let failedTasks: Task[] = [];
      let pendingTasks: Task[] = [];
      let approvalTasks: Task[] = [];
      let loot = emptyLootData();

      if (agentsRes.status === "fulfilled") {
        agents = normalizeAgentList(agentsRes.value)
          .map((a) => asAgent(a as Record<string, unknown>))
          .filter((a) => a.id);
      } else {
        failedLoads += 1;
      }

      if (healthRes.status === "fulfilled") {
        const list = firstArray(healthRes.value, ["listeners", "data"]) as ListenerHealth[];
        healthByTarget = indexListenerHealth(list);
      } else {
        failedLoads += 1;
      }

      if (failedRes.status === "fulfilled") {
        failedTasks = normalizeListEnvelope(failedRes.value, ["tasks", "data", "Tasks"]) as Task[];
      } else {
        failedLoads += 1;
      }

      if (pendingRes.status === "fulfilled") {
        pendingTasks = normalizeListEnvelope(pendingRes.value, ["tasks", "data", "Tasks"]) as Task[];
      } else {
        failedLoads += 1;
      }

      if (approvalRes.status === "fulfilled") {
        approvalTasks = normalizeListEnvelope(approvalRes.value, ["tasks", "data", "Tasks"]) as Task[];
      } else {
        failedLoads += 1;
      }

      if (lootRes.status === "fulfilled") {
        loot = normalizeLootData(lootRes.value as Record<string, unknown>);
      } else {
        failedLoads += 1;
      }

      if (failedLoads === settled.length) throw new Error("all-ops-endpoints-failed");
      return { agents, healthByTarget, failedTasks, pendingTasks, approvalTasks, loot };
    }, []),
    pollMs: POLL.opsHome,
    errorMessage: t("dashboard.ops_load_failed"),
  });

  return {
    agents: data?.agents ?? [],
    healthByTarget: data?.healthByTarget ?? {},
    failedTasks: data?.failedTasks ?? [],
    pendingTasks: data?.pendingTasks ?? [],
    approvalTasks: data?.approvalTasks ?? [],
    loot: data?.loot ?? emptyLootData(),
    loading,
    error,
    refresh: () => {
      void refreshData();
    },
  };
}
