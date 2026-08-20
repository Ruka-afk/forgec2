"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useCachedData } from "@/lib/hooks/useCachedData";
import { useApiResource } from "@/lib/hooks/useApiResource";
import type { Beacon } from "./types";

type AgentTag = { id: string; name: string; color: string };

interface BeaconQueryParams {
  search: string;
  status: string;
  os: string;
  page: number;
  pageSize: number;
  tag_id: string;
  linked: string;
  sort_key: string;
  sort_dir: string;
}

interface BeaconPage {
  list: Beacon[];
  total: number;
  countMap: Record<string, number>;
}

const DEFAULT_QUERY: BeaconQueryParams = {
  search: "",
  status: "",
  os: "",
  page: 1,
  pageSize: 20,
  tag_id: "",
  linked: "",
  sort_key: "last_seen",
  sort_dir: "desc",
};

export function useAgentData(t: (key: string) => string) {
  const queryRef = useRef<BeaconQueryParams>(DEFAULT_QUERY);
  const [tagsByAgent, setTagsByAgent] = useState<Record<string, AgentTag[]>>({});
  const [agentLocks, setAgentLocks] = useState<Record<string, string>>({});
  const [operatorPresence, setOperatorPresence] = useState<Record<string, string[]>>({});

  const { data, loading, error, setError, refresh: refreshList, setData } = useApiResource<BeaconPage>({
    fetcher: async (signal) => {
      const q = queryRef.current;
      const p = new URLSearchParams({
        page: String(q.page),
        page_size: String(q.pageSize),
        group: "host",
      });
      if (q.search) p.set("search", q.search);
      if (q.status) p.set("status", q.status);
      if (q.os) p.set("os", q.os);
      if (q.tag_id) p.set("tag_id", q.tag_id);
      if (q.linked) p.set("linked", q.linked);
      p.set("sort_key", q.sort_key);
      p.set("sort_dir", q.sort_dir);
      const d = await api.get<{ agents?: Beacon[]; total?: number | string }>(
        paths.agents.list(p.toString()),
        { signal, unwrap: false },
      );
      const list = (d.agents || []) as Beacon[];
      const countMap: Record<string, number> = {};
      for (const b of list) {
        const aid = b.id || "";
        if (!aid) continue;
        const ts = b.taskStats;
        countMap[aid] = ts
          ? (ts.pending || 0) + (ts.running || 0) + (ts.completed || 0) + (ts.failed || 0)
          : 0;
      }
      return { list, total: Number(d.total) || list.length, countMap };
    },
    errorMessage: t("agents.load_failed"),
  });

  const beacons = useMemo(() => data?.list ?? [], [data]);
  const total = data?.total ?? 0;
  const taskCountMap = useMemo(() => data?.countMap ?? {}, [data]);

  const loadBeacons = useCallback(
    (search = "", status = "", os = "", page = 1, pageSize = 20, tag_id = "", linked = "", sort_key = "last_seen", sort_dir = "desc") => {
      queryRef.current = { search, status, os, page, pageSize, tag_id, linked, sort_key, sort_dir };
      void refreshList();
    },
    [refreshList],
  );

  const setBeacons = useCallback<React.Dispatch<React.SetStateAction<Beacon[]>>>(
    (action) => {
      setData((prev) => {
        const base = prev?.list ?? [];
        const next = typeof action === "function" ? (action as (b: Beacon[]) => Beacon[])(base) : action;
        return { list: next, total: prev?.total ?? next.length, countMap: prev?.countMap ?? {} };
      });
    },
    [setData],
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

  const { data: cachedAllTags } = useCachedData<AgentTag[]>("tags:list", {
    fetcher: async () => {
      const d = await api.get(paths.tags.list);
      return (Array.isArray(d) ? d : d.tags || []) as AgentTag[];
    },
    ttlMs: 60_000,
    onError: () => toast.error(t("agents.tags_load_failed")),
  });

  const allTags = cachedAllTags ?? [];

  const idsKey = useMemo(() => beacons.map((b) => b.id || "").filter(Boolean).sort().join(","), [beacons]);

  useEffect(() => {
    if (beacons.length === 0) {
      setTagsByAgent({});
      return;
    }
    const ac = new AbortController();
    const ids = beacons.map((b) => b.id || "").filter(Boolean);
    // Debounce so rapid list refreshes (sorting, pagination, polling) coalesce
    // into a single batch request instead of a burst.
    const timer = setTimeout(() => {
      if (ac.signal.aborted) return;
      api
        .postJson<{ tags: Record<string, AgentTag[]> }>(paths.agents.batchTags, { agent_ids: ids })
        .then((d) => { if (!ac.signal.aborted) setTagsByAgent(d.tags || {}); })
        .catch(() => {
          if (!ac.signal.aborted) setTagsByAgent({});
        });
    }, 200);
    return () => {
      clearTimeout(timer);
      ac.abort();
    };
    // Only refetch when the set of agent ids actually changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [idsKey]);

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
    operatorPresence,
    setOperatorPresence,
    loadBeacons,
    loadLocks,
  };
}
