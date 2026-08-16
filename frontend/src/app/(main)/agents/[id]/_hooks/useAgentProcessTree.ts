"use client";

import { useCallback, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { describeProcessSnapshot } from "../_components/process-snapshot";
import { usePersistedState } from "@/lib/hooks/usePersistedState";

export function useAgentProcessTree(agentId: string, emptyMessage: string, errorMessage: string, persistKey?: string) {
  const [processList, setProcessList] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = usePersistedState(
    persistKey ?? `agents.detail.${agentId}.process`,
    false,
  );

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!agentId) return;
    if (expanded && processList) {
      setExpanded(false);
      return;
    }
    setExpanded(true);
    if (processList) return;
    setLoading(true);
    try {
      const response = await api.get(paths.agents.processTree(agentId), { signal }) as Record<string, unknown>;
      const snap = describeProcessSnapshot(response);
      setProcessList(snap.text || emptyMessage);
    } catch {
      setProcessList(errorMessage);
    } finally {
      setLoading(false);
    }
  }, [agentId, expanded, processList, emptyMessage, errorMessage, setExpanded]);

  return {
    processList,
    loading,
    expanded,
    setExpanded,
    load,
  };
}