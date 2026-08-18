"use client";

import { fetchAgentListCached, AGENTS_CACHE_KEY } from "@/lib/agents";
import { useCachedData } from "@/lib/hooks/useCachedData";
import type { Agent } from "@/types/agent";

/**
 * Full agent list for dropdown-style consumers. Served from the shared
 * module-level cache (AGENTS_CACHE_KEY) so switching pages/remounting does not
 * re-fetch a list that just loaded elsewhere. Callers may `refresh()` to
 * force revalidation for chatty views.
 */
export function useAgentList() {
  const { data, loading, error, refresh } = useCachedData<Agent[]>(AGENTS_CACHE_KEY, {
    fetcher: fetchAgentListCached,
    ttlMs: 60_000,
  });

  return {
    agents: data ?? [],
    loading,
    error: error ? "Failed to load agents" : null,
    refresh,
  };
}