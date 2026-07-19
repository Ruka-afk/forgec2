"use client";

import { useState, useCallback } from "react";
import { api } from "@/lib/api";
import { Agent } from "@/types/agent";

export function useAgentList() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.get("/agents");
      const list = (data as Record<string, unknown>)?.agents || (Array.isArray(data) ? data : []);
      setAgents(list as Agent[]);
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  }, []);

  return { agents, loading, refresh };
}
