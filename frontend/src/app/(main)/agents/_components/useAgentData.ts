"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import type { Beacon } from "./types";

export type AgentTag = { id: string; name: string; color: string };

export interface AgentDataState {
  beacons: Beacon[];
  loading: boolean;
  total: number;
  error: string | null;
  allTags: AgentTag[];
  tagsByAgent: Record<string, AgentTag[]>;
  taskCountMap: Record<string, number>;
  agentLocks: Record<string, string>;
}

export function useAgentData(t: (key: string) => string) {
  const [beacons, setBeacons] = useState<Beacon[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [allTags, setAllTags] = useState<AgentTag[]>([]);
  const [tagsByAgent, setTagsByAgent] = useState<Record<string, AgentTag[]>>({});
  const [taskCountMap, setTaskCountMap] = useState<Record<string, number>>({});
  const [agentLocks, setAgentLocks] = useState<Record<string, string>>({});
  const loadAbortRef = useRef<AbortController | null>(null);

  const loadBeacons = useCallback(
    (search = "", status = "", os = "", page = 1, pageSize = 20, tag_id = "", opts?: { background?: boolean }) => {
      loadAbortRef.current?.abort();
      const ac = new AbortController();
      loadAbortRef.current = ac;
      if (!opts?.background) setLoading(true);
      const p = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
      if (search) p.set("search", search);
      if (status) p.set("status", status);
      if (os) p.set("os", os);
      if (tag_id) p.set("tag_id", tag_id);
      api
        .get(`/api/agents?${p.toString()}`, { signal: ac.signal })
        .then((data) => {
          if (ac.signal.aborted) return;
          const list = (data.agents || []) as Beacon[];
          setBeacons(list);
          setTotal(Number(data.total) || list.length);

          const countMap: Record<string, number> = {};
          for (const b of list) {
            const aid = b.id || "";
            if (!aid) continue;
            const ts = b.taskStats;
            countMap[aid] = ts
              ? (ts.pending || 0) + (ts.running || 0) + (ts.completed || 0) + (ts.failed || 0)
              : 0;
          }
          setTaskCountMap(countMap);
        })
        .catch(() => {
          if (ac.signal.aborted) return;
          setBeacons([]);
          setTotal(0);
          setError(t("agents.load_failed"));
        })
        .finally(() => {
          if (!ac.signal.aborted) setLoading(false);
        });
    },
    [t],
  );

  const loadLocks = useCallback(() => {
    api
      .get<{ agents: Record<string, string>[] }>("/collab/agents")
      .then((data) => {
        const agents = data.agents || [];
        const locks: Record<string, string> = {};
        for (const a of agents) {
          if (a.locked_by) locks[a.id] = a.locked_by;
        }
        setAgentLocks(locks);
      })
      .catch(() => {
        toast.error(t("agents.locks_failed"));
      });
  }, [t]);

  useEffect(() => {
    const ac = new AbortController();
    api.get(paths.tags.list, { signal: ac.signal })
      .then((d) => { if (!ac.signal.aborted) setAllTags((d.tags || []) as AgentTag[]); })
      .catch(() => {
        if (!ac.signal.aborted) {
          setAllTags([]);
          toast.error(t("agents.tags_load_failed"));
        }
      });
    return () => ac.abort();
  }, [t]);

  useEffect(() => {
    if (beacons.length === 0) {
      setTagsByAgent({});
      return;
    }
    const ids = beacons.map((b) => b.id || "").filter(Boolean);
    const ac = new AbortController();
    api
      .postJson<{ tags: Record<string, AgentTag[]> }>(paths.agents.batchTags, { agent_ids: ids })
      .then((d) => { if (!ac.signal.aborted) setTagsByAgent(d.tags || {}); })
      .catch(() => {
        if (!ac.signal.aborted) {
          setTagsByAgent({});
        }
      });
    return () => ac.abort();
  }, [beacons]);

  return {
    beacons,
    setBeacons,
    loading,
    total,
    error,
    setError,
    allTags,
    tagsByAgent,
    taskCountMap,
    agentLocks,
    setAgentLocks,
    loadBeacons,
    loadLocks,
  };
}
