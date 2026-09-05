"use client";

import { useEffect, useMemo, useState } from "react";
import type { FileEntry } from "./types";

export type FileSortKey = "name" | "size" | "mod_time";

/**
 * Directory-view state for the file browser: row selection (scoped to the
 * current directory, so it resets on navigate) + dirs-first sorting.
 */
export function useFilesView(entries: FileEntry[], currentPath: string) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [sortKey, setSortKey] = useState<FileSortKey>("name");
  const [sortDir, setSortDir] = useState<1 | -1>(1);

  // Selection refers to names in the current directory; drop it on navigate.
  useEffect(() => {
    setSelected(new Set());
  }, [currentPath]);

  const toggleSelect = (name: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  const toggleSort = (key: FileSortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 1 ? -1 : 1));
    } else {
      setSortKey(key);
      setSortDir(1);
    }
  };

  const sortedEntries = useMemo(() => {
    const list = [...entries];
    const dir = sortDir;
    list.sort((a, b) => {
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
      switch (sortKey) {
        case "size":
          return (a.size - b.size) * dir;
        case "mod_time":
          return (a.mod_time < b.mod_time ? -1 : a.mod_time > b.mod_time ? 1 : 0) * dir;
        default:
          return a.name.localeCompare(b.name) * dir;
      }
    });
    return list;
  }, [entries, sortKey, sortDir]);

  const allSelected = entries.length > 0 && entries.every((e) => selected.has(e.name));

  return { selected, setSelected, sortKey, sortDir, toggleSelect, toggleSort, sortedEntries, allSelected };
}
