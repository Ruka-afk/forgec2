"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useWS } from "@/lib/wsContext";

interface AgentScreenshotsResponse {
  screenshots?: string[];
}

const SNAPSHOT_DEBOUNCE_MS = 500;
const NEW_BADGE_TTL_MS = 30000;

/**
 * Screenshot list for the agent detail page. Refetches only when something
 * relevant actually happened (agent came online, a screenshot task
 * completed) instead of piggybacking on every task_update, and stops
 * polling entirely while the agent is offline. Tracks which filenames are
 * new since the last fetch so the UI can badge them.
 */
export function useAgentScreenshots(agentId: string, online: boolean) {
  const [screenshots, setScreenshots] = useState<string[]>([]);
  const [newScreenshots, setNewScreenshots] = useState<string[]>([]);
  const knownRef = useRef<Set<string>>(new Set());
  const initializedRef = useRef(false);
  const lastListRef = useRef("");
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const badgeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { subscribe } = useWS();

  // Reset per-agent gallery state when navigating between agents so the
  // previous implant's screenshots and NEW badges never leak over.
  useEffect(() => {
    knownRef.current = new Set();
    initializedRef.current = false;
    lastListRef.current = "";
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    if (badgeTimerRef.current) clearTimeout(badgeTimerRef.current);
    badgeTimerRef.current = null;
    setScreenshots([]);
    setNewScreenshots([]);
  }, [agentId]);

  const reload = useCallback(async (signal?: AbortSignal) => {
    if (!agentId) return;
    try {
      const response = await api.get<AgentScreenshotsResponse>(
        paths.agents.screenshots(agentId, "page=1&page_size=50"),
        { signal },
      );
      const list = response.screenshots || [];
      if (!initializedRef.current) {
        // First load: treat everything as known so the UI doesn't badge
        // the whole gallery as "new" on the initial visit.
        initializedRef.current = true;
        list.forEach((fn) => knownRef.current.add(fn));
        setNewScreenshots([]);
      } else {
        const fresh = list.filter((fn) => !knownRef.current.has(fn));
        if (fresh.length > 0) {
          fresh.forEach((fn) => knownRef.current.add(fn));
          setNewScreenshots(fresh);
          if (badgeTimerRef.current) clearTimeout(badgeTimerRef.current);
          badgeTimerRef.current = setTimeout(() => setNewScreenshots([]), NEW_BADGE_TTL_MS);
        }
      }
      const joined = list.join("\n");
      if (joined !== lastListRef.current) {
        lastListRef.current = joined;
        setScreenshots(list);
      }
    } catch (error) {
      // Keep the previous list on transient errors; the next successful
      // reload (WS event, retry) replaces it.
      if ((error as Error).name !== "AbortError") {
        setNewScreenshots([]);
      }
    }
  }, [agentId]);

  const reloadDebounced = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      timerRef.current = null;
      reload();
    }, SNAPSHOT_DEBOUNCE_MS);
  }, [reload]);

  useEffect(() => {
    const controller = new AbortController();
    reload(controller.signal);
    return () => controller.abort();
  }, [reload]);

  useEffect(() => {
    if (!agentId || !online) return;
    return subscribe((msg) => {
      if (String((msg as { agent_id?: unknown }).agent_id ?? "") !== agentId) return;
      if (msg.type === "agent_online") {
        reloadDebounced();
      } else if (msg.type === "task_update") {
        const frame = msg as { task_type?: string; status?: string };
        // Window captures and trigger matches land in the same gallery as
        // plain screenshots (server saves every frame to disk).
        if (
          frame.status === "completed" &&
          (frame.task_type === "screenshot" ||
            frame.task_type === "screenshot_window" ||
            frame.task_type === "screen_trigger_start")
        ) {
          reloadDebounced();
        }
      }
    });
  }, [agentId, online, subscribe, reloadDebounced]);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
      if (badgeTimerRef.current) clearTimeout(badgeTimerRef.current);
    };
  }, []);

  return { screenshots, newScreenshots, reload };
}