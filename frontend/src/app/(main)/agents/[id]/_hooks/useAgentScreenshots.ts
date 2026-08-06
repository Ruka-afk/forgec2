"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

interface AgentScreenshotsResponse {
  screenshots?: string[];
}

export function useAgentScreenshots(agentId: string) {
  const [screenshots, setScreenshots] = useState<string[]>([]);

  const reload = useCallback(async (signal?: AbortSignal) => {
    if (!agentId) return;
    try {
      const response = await api.get<AgentScreenshotsResponse>(paths.agents.screenshots(agentId, "page=1&page_size=50"), { signal });
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
