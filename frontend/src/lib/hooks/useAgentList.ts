"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { api } from "@/lib/api";
import { normalizeAgentList } from "@/lib/agents";
import { paths } from "@/lib/api-paths";
import { Agent } from "@/types/agent";

export function useAgentList() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const refresh = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      setLoading(true);
      setError(null);
      const data = await api.get(paths.agents.list(), { signal: controller.signal });
      if (controller.signal.aborted) return;
      setAgents(normalizeAgentList(data));
    } catch (e) {
      if (!controller.signal.aborted) {
        setError(e instanceof Error ? e.message : "Failed to load agents");
        setAgents([]);
      }
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    return () => { abortRef.current?.abort(); };
  }, [refresh]);

  return { agents, loading, error, refresh };
}
