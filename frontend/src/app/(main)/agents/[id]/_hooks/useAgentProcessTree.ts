"use client";

import { useCallback, useState } from "react";
import { api } from "@/lib/api";

export function useAgentProcessTree(agentId: string, emptyMessage: string, errorMessage: string) {
  const [processList, setProcessList] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState(false);

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
      const response = await api.get(`/api/agents/${agentId}/process-tree`, { signal }) as Record<string, unknown>;
      setProcessList((response.processes as string) || emptyMessage);
    } catch {
      setProcessList(errorMessage);
    } finally {
      setLoading(false);
    }
  }, [agentId, expanded, processList, emptyMessage, errorMessage]);

  return {
    processList,
    loading,
    expanded,
    setExpanded,
    load,
  };
}
