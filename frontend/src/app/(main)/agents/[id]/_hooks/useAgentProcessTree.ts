"use client";

import { useCallback, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { describeProcessSnapshot } from "../_components/process-snapshot";
import { usePersistedState } from "@/lib/hooks/usePersistedState";

export function useAgentProcessTree(agentId: string, emptyMessage: string, errorMessage: string, persistKey?: string) {
  const [processList, setProcessList] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [expanded, setExpanded] = usePersistedState(
    persistKey ?? `agents.detail.${agentId}.process`,
    false,
  );

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!agentId) return;
    if (expanded && processList && !loadFailed) {
      setExpanded(false);
      return;
    }
    setExpanded(true);
    if (processList && !loadFailed) return;
    setLoading(true);
    try {
      const response = await api.get(paths.agents.processTree(agentId), { signal }) as Record<string, unknown>;
      const snap = describeProcessSnapshot(response);
      setProcessList(snap.text || emptyMessage);
      setLoadFailed(false);
    } catch {
      setProcessList(errorMessage);
      setLoadFailed(true);
    } finally {
      setLoading(false);
    }
  }, [agentId, expanded, processList, loadFailed, emptyMessage, errorMessage, setExpanded]);

  return {
    processList,
    loading,
    loadFailed,
    expanded,
    setExpanded,
    load,
  };
}