"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

const MIN_RELOAD_INTERVAL_MS = 2000;

interface MergableTask {
  id?: number | string;
  ID?: number | string;
  status?: string;
  result?: string;
  error?: string;
}

/**
 * Merge a freshly-fetched full snapshot with WS-incremental progress already
 * present in `prev`. The fetch may have started BEFORE one or more task_update
 * frames arrived, so a naive replace would clobber those live updates (e.g. a
 * task that flipped to completed right as the fetch was resolving). This keeps
 * any status/result/error the WS already advanced that the snapshot hasn't
 * caught up to yet.
 */
export function mergeSnapshotWithPrev<T>(snapshot: T, prev: T | null): T {
  if (!prev || !snapshot) return snapshot;
  const snap = snapshot as Record<string, unknown>;
  const old = prev as Record<string, unknown>;
  if (typeof snap !== "object" || typeof old !== "object") return snapshot;

  const snapTasks = Array.isArray(snap.tasks) ? (snap.tasks as MergableTask[]) : null;
  const prevTasks = Array.isArray(old.tasks) ? (old.tasks as MergableTask[]) : null;
  if (!snapTasks || !prevTasks) return snapshot;

  const merged = snapTasks.map((st) => st);
  const prevById = new Map<number, MergableTask>();
  for (const pt of prevTasks) {
    const id = Number(pt.id ?? pt.ID);
    if (Number.isFinite(id) && id > 0) prevById.set(id, pt);
  }
  let changed = false;
  for (let i = 0; i < merged.length; i++) {
    const snapTask = merged[i];
    const id = Number(snapTask.id ?? snapTask.ID);
    if (!Number.isFinite(id)) continue;
    const prior = prevById.get(id);
    if (!prior) continue;
    // Only adopt WS-advanced fields; never overwrite server truth with a
    // fallback. A WS frame reflects a more-recent transition than a snapshot
    // taken earlier, so prefer prior.status/result/error when they differ.
    const out = { ...snapTask };
    let taskChanged = false;
    if (prior.status && snapTask.status !== prior.status) { out.status = prior.status; taskChanged = true; }
    if (prior.result && snapTask.result !== prior.result) { out.result = prior.result; taskChanged = true; }
    if (prior.error && snapTask.error !== prior.error) { out.error = prior.error; taskChanged = true; }
    if (taskChanged) { merged[i] = out; changed = true; }
  }
  if (!changed) return snapshot;
  return { ...snapshot, tasks: merged };
}

export function useAgentDetail<T>(agentId: string) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const lastReloadRef = useRef(0);
  const pendingReloadRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hasDataRef = useRef(false);
  const lastSnapshotRef = useRef<string | null>(null);

  const reload = useCallback(
    async (background = false, signal?: AbortSignal) => {
      if (!agentId) return;
      if (!background) {
        setLoadError(false);
        if (!hasDataRef.current) setLoading(true);
      }
      try {
        const response = await api.get<T>(
          `${paths.agents.one(agentId)}?include_screenshots=false&include_unlinked=false`,
          { signal },
        );
        lastReloadRef.current = Date.now();
        setLoadError(false);
        hasDataRef.current = true;
        setData((prev) => {
          const merged = mergeSnapshotWithPrev<T>(response, prev);
          const snap = JSON.stringify(merged);
          if (lastSnapshotRef.current === snap) return prev;
          lastSnapshotRef.current = snap;
          return merged;
        });
      } catch (error) {
        if ((error as Error).name !== "AbortError") {
          if (!hasDataRef.current) setLoadError(true);
        }
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    },
    [agentId],
  );

  /**
   * Throttled full reload: coalesces bursts of WS events. Queues trailing
   * call with latest background flag instead of dropping it.
   */
  const pendingBackgroundRef = useRef(true);
  const reloadThrottled = useCallback(
    (background = true) => {
      if (!agentId) return;
      if (pendingReloadRef.current) {
        pendingBackgroundRef.current = background;
        clearTimeout(pendingReloadRef.current);
        const elapsed = Date.now() - lastReloadRef.current;
        const delay = Math.max(0, MIN_RELOAD_INTERVAL_MS - elapsed);
        pendingReloadRef.current = setTimeout(() => {
          pendingReloadRef.current = null;
          reload(pendingBackgroundRef.current);
        }, delay);
        return;
      }
      const elapsed = Date.now() - lastReloadRef.current;
      if (elapsed >= MIN_RELOAD_INTERVAL_MS) {
        reload(background);
        return;
      }
      pendingBackgroundRef.current = background;
      pendingReloadRef.current = setTimeout(() => {
        pendingReloadRef.current = null;
        reload(pendingBackgroundRef.current);
      }, MIN_RELOAD_INTERVAL_MS - elapsed);
    },
    [agentId, reload],
  );

  useEffect(() => {
    hasDataRef.current = false;
    lastSnapshotRef.current = null;
    setData(null);
    setLoading(true);
    setLoadError(false);
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