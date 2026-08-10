"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

export function useAgentDetail<T>(agentId: string) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);

  const reload = useCallback(async (background = false, signal?: AbortSignal) => {
    if (!agentId) return;
    if (!background) {
      setLoading(true);
      setLoadError(false);
    }
    try {
      const response = await api.get<T>(`${paths.agents.one(agentId)}?include_screenshots=false`, { signal });
      setData(response);
    } catch (error) {
      if ((error as Error).name !== "AbortError") {
        setData(null);
        setLoadError(true);
      }
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  useEffect(() => {
    const controller = new AbortController();
    reload(false, controller.signal);
    return () => controller.abort();
  }, [reload]);

  return { data, setData, loading, loadError, reload };
}
