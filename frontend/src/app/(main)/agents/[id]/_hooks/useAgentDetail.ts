"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

const MIN_RELOAD_INTERVAL_MS = 2000;

export function useAgentDetail<T>(agentId: string) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const lastReloadRef = useRef(0);
  const pendingReloadRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const reload = useCallback(
    async (background = false, signal?: AbortSignal) => {
      if (!agentId) return;
      if (!background) {
        setLoading(true);
        setLoadError(false);
      }
      try {
        const response = await api.get<T>(
          `${paths.agents.one(agentId)}?include_screenshots=false&include_unlinked=false`,
          { signal },
        );
        lastReloadRef.current = Date.now();
        setData(response);
      } catch (error) {
        if ((error as Error).name !== "AbortError") {
          setData(null);
          setLoadError(true);
        }
      } finally {
        setLoading(false);
      }
    },
    [agentId],
  );

  /**
   * Throttled full reload: coalesces bursts of WS events into at most one
   * refetch per MIN_RELOAD_INTERVAL_MS, trailing-edge style — the latest
   * request wins, intermediate ones are dropped.
   */
  const reloadThrottled = useCallback(
    (background = true) => {
      if (!agentId) return;
      if (pendingReloadRef.current) return;
      const elapsed = Date.now() - lastReloadRef.current;
      if (elapsed >= MIN_RELOAD_INTERVAL_MS) {
        reload(background);
        return;
      }
      pendingReloadRef.current = setTimeout(() => {
        pendingReloadRef.current = null;
        reload(background);
      }, MIN_RELOAD_INTERVAL_MS - elapsed);
    },
    [agentId, reload],
  );

  useEffect(() => {
    const controller = new AbortController();
    reload(false, controller.signal);
    return () => {
      controller.abort();
      if (pendingReloadRef.current) {
        clearTimeout(pendingReloadRef.current);
        pendingReloadRef.current = null;
      }
    };
  }, [reload]);

  return { data, setData, loading, loadError, reload, reloadThrottled };
}