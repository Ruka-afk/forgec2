"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { useDebounce } from "@/lib/hooks/useDebounce";
import type { Beacon } from "./types";

export type SortKey = "hostname" | "username" | "os" | "ip" | "last_seen" | "status";
export type LinkedFilter = "" | "direct" | "chained";

export function useAgentFilters(beacons: Beacon[]) {
  const [searchInput, setSearchInput] = useState("");
  const searchQuery = useDebounce(searchInput, 300);
  const [statusFilter, setStatusFilter] = useState("");
  const [osFilter, setOsFilter] = useState("");
  const [page, setPage] = useState(1);
  const [sortKey, setSortKey] = useState<SortKey>("last_seen");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [linkedFilter, setLinkedFilter] = useState<LinkedFilter>("");
  const [tagFilter, setTagFilter] = useState("");
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [viewMode, setViewMode] = useState<"table" | "grid">("table");
  const [visibleCols, setVisibleCols] = useState<Record<string, boolean>>({
    hostname: true, username: true, os: true, ip: true, last_seen: true,
    window: true, lock: true, tasks: true, status: true, version: true,
  });
  const [colMenuOpen, setColMenuOpen] = useState(false);

  useEffect(() => { if (searchInput) setPage(1); }, [searchInput]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "/" && !["INPUT", "TEXTAREA", "SELECT"].includes((e.target as HTMLElement)?.tagName)) {
        e.preventDefault();
        document.getElementById("agent-search")?.focus();
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => { if (tagFilter) setPage(1); }, [tagFilter]);

  useEffect(() => { setPage(1); }, [sortKey, sortDir]);

  const toggleSort = useCallback((key: SortKey) => {
    if (sortKey === key) setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    else { setSortKey(key); setSortDir("asc"); }
  }, [sortKey]);

  const filteredBeacons = useMemo(() => {
    if (!linkedFilter) return beacons;
    return beacons.filter((b) => {
      const pid = b.parent_id || "";
      if (linkedFilter === "direct") return !pid;
      if (linkedFilter === "chained") return !!pid;
      return true;
    });
  }, [beacons, linkedFilter]);

  // Sorting is performed server-side across all pages; the current page's
  // rows arrive already ordered, so no client-side re-sort is applied.
  const sortedBeacons = filteredBeacons;

  return {
    searchInput, setSearchInput,
    searchQuery,
    statusFilter, setStatusFilter,
    osFilter, setOsFilter,
    page, setPage,
    sortKey, sortDir, toggleSort,
    linkedFilter, setLinkedFilter,
    tagFilter, setTagFilter,
    autoRefresh, setAutoRefresh,
    viewMode, setViewMode,
    visibleCols, setVisibleCols,
    colMenuOpen, setColMenuOpen,
    filteredBeacons,
    sortedBeacons,
  };
}
