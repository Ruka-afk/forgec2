"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useWS } from "@/lib/wsContext";

interface AgentScreenshotsResponse {
  screenshots?: string[];
}

const SNAPSHOT_DEBOUNCE_MS = 500;

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
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { subscribe } = useWS();

  const reload = useCallback(async (signal?: AbortSignal) => {
    if (!agentId) return;
    try {
      const response = await api.get<AgentScreenshotsResponse>(
        paths.agents.screenshots(agentId, "page=1&page_size=50"),
        { signal },
      );
      const list = response.screenshots || [];
      const fresh = list.filter((fn) => !knownRef.current.has(fn));
      if (fresh.length > 0) {
        fresh.forEach((fn) => knownRef.current.add(fn));
        setNewScreenshots(fresh);
      }
      setScreenshots(list);
    } catch (error) {
      if ((error as Error).name !== "AbortError") {
        setScreenshots([]);
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
        if (frame.task_type === "screenshot" && frame.status === "completed") {
          reloadDebounced();
        }
      }
    });
  }, [agentId, online, subscribe, reloadDebounced]);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  return { screenshots, newScreenshots, reload };
}