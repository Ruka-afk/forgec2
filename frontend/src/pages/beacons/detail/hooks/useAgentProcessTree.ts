import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, api, runTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import {
  describeProcessSnapshot,
  type ProcessSnapshotMeta,
} from "@/lib/process-snapshot";
import { usePersistedState } from "@/lib/hooks/usePersistedState";

/**
 * Load the last completed process snapshot and collect fresh ones on demand.
 *
 * A 404 from /process-tree means "nothing collected yet", not a broken panel:
 * it clears the snapshot without turning the card into an error state.
 */
export function useAgentProcessTree(agentId: string, persistKey?: string) {
  const [snapshot, setSnapshot] = useState<ProcessSnapshotMeta | null>(null);
  const [loading, setLoading] = useState(false);
  const [collecting, setCollecting] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [expanded, setExpanded] = usePersistedState(
    persistKey ?? `agents.detail.${agentId}.process`,
    false,
  );

  const loadedRef = useRef(false);
  const generationRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);

  const cancelInFlight = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    generationRef.current += 1;
  }, []);

  useEffect(() => {
    generationRef.current += 1;
    abortRef.current?.abort();
    abortRef.current = null;
    loadedRef.current = false;
    setSnapshot(null);
    setLoading(false);
    setCollecting(false);
    setLoadFailed(false);
    setLoadError(null);
    return () => {
      generationRef.current += 1;
      abortRef.current?.abort();
      abortRef.current = null;
    };
  }, [agentId]);

  const applySnapshot = useCallback((next: ProcessSnapshotMeta | null) => {
    loadedRef.current = true;
    setSnapshot(next);
    setLoadFailed(false);
    setLoadError(null);
  }, []);

  const load = useCallback(async () => {
    if (!agentId) return;
    if (loadedRef.current && !loadFailed) return;

    const gen = ++generationRef.current;
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);
    setLoadFailed(false);
    setLoadError(null);
    try {
      const response = await api.get(paths.agents.processTree(agentId), {
        signal: controller.signal,
      }) as Record<string, unknown>;
      if (gen !== generationRef.current) return;
      applySnapshot(describeProcessSnapshot(response));
    } catch (error) {
      if (controller.signal.aborted || gen !== generationRef.current) return;
      if (error instanceof ApiError && error.status === 404) {
        // No snapshot yet: show the empty state with a Collect action instead
        // of a misleading "failed to load" message.
        applySnapshot(null);
      } else {
        setLoadFailed(true);
        setLoadError(error);
        loadedRef.current = true;
      }
    } finally {
      if (gen === generationRef.current) {
        setLoading(false);
        if (abortRef.current === controller) abortRef.current = null;
      }
    }
  }, [agentId, applySnapshot, loadFailed]);

  const refresh = useCallback(async () => {
    if (!agentId) return;

    const gen = ++generationRef.current;
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setCollecting(true);
    setLoadFailed(false);
    setLoadError(null);
    try {
      const status = await runTask(agentId, paths.agents.collectProcesses(agentId), {
        signal: controller.signal,
      });
      if (gen !== generationRef.current) return;
      if (status.status !== "completed") {
        throw new Error(status.error || "process collection failed");
      }

      let next: ProcessSnapshotMeta | null = null;
      try {
        const response = await api.get(paths.agents.processTree(agentId), {
          signal: controller.signal,
        }) as Record<string, unknown>;
        next = describeProcessSnapshot(response);
      } catch (error) {
        // The completed task already carries the snapshot. Keep the panel
        // useful even if the follow-up envelope read races or fails.
        if (controller.signal.aborted || gen !== generationRef.current) return;
        if (error instanceof ApiError && error.status === 404) {
          if (status.result) {
            next = {
              text: status.result,
              source: "process_tree",
              live: false,
              kind: "process_tree",
              taskId: status.id,
              collectedAt: new Date().toISOString(),
            };
          }
        } else {
          throw error;
        }
      }
      if (gen !== generationRef.current) return;
      applySnapshot(next);
    } catch (error) {
      if (controller.signal.aborted || gen !== generationRef.current) return;
      setLoadFailed(true);
      setLoadError(error);
      throw error;
    } finally {
      if (gen === generationRef.current) {
        setCollecting(false);
        if (abortRef.current === controller) abortRef.current = null;
      }
    }
  }, [agentId, applySnapshot]);

  useEffect(() => cancelInFlight, [cancelInFlight]);

  return {
    snapshot,
    loading,
    collecting,
    loadFailed,
    loadError,
    expanded,
    setExpanded,
    load,
    refresh,
  };
}
