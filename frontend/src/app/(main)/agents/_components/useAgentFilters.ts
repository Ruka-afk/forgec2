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

  const sortedBeacons = useMemo(() => {
    const list = [...filteredBeacons];
    const dir = sortDir === "asc" ? 1 : -1;
    list.sort((a, b) => {
      const av = String(a[sortKey] || "");
      const bv = String(b[sortKey] || "");
      if (sortKey === "last_seen") {
        const at = av ? new Date(av).getTime() : 0;
        const bt = bv ? new Date(bv).getTime() : 0;
        return (at - bt) * dir;
      }
      return av.localeCompare(bv) * dir;
    });
    return list;
  }, [filteredBeacons, sortKey, sortDir]);

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
