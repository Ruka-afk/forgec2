"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";

interface AgentScreenshotsResponse {
  screenshots?: string[];
}

export function useAgentScreenshots(agentId: string) {
  const [screenshots, setScreenshots] = useState<string[]>([]);

  const reload = useCallback(async (signal?: AbortSignal) => {
    if (!agentId) return;
    try {
      const response = await api.get<AgentScreenshotsResponse>(`/api/agents/${agentId}/screenshots?page=1&page_size=50`, { signal });
      setScreenshots(response.screenshots || []);
    } catch (error) {
      if ((error as Error).name !== "AbortError") {
        setScreenshots([]);
      }
    }
  }, [agentId]);

  useEffect(() => {
    const controller = new AbortController();
    reload(controller.signal);
    return () => controller.abort();
  }, [reload]);

  return { screenshots, reload };
}
