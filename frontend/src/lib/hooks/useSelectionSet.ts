"use client";

import { useRef, useState, useCallback } from "react";

interface UseSelectionSetResult {
  toggleSelect: (id: string) => void;
  toggleSelectAll: (ids: string[]) => void;
  clearSelection: () => void;
  isSelected: (id: string) => boolean;
  count: number;
  selectedSet: ReadonlySet<string>;
}

export function useSelectionSet(): UseSelectionSetResult {
  const ref = useRef(new Set<string>());
  const [version, setVersion] = useState(0);

  const toggleSelect = useCallback((id: string) => {
    const s = ref.current;
    if (s.has(id)) { s.delete(id); } else { s.add(id); }
    setVersion(v => v + 1);
  }, []);

  const toggleSelectAll = useCallback((ids: string[]) => {
    const s = ref.current;
    const allSelected = ids.every(id => s.has(id));
    if (allSelected) {
      ref.current = new Set();
    } else {
      ref.current = new Set(ids);
    }
    setVersion(v => v + 1);
  }, []);

  const clearSelection = useCallback(() => {
    ref.current = new Set();
    setVersion(v => v + 1);
  }, []);

  const isSelected = useCallback((id: string) => ref.current.has(id), []);

  return {
    toggleSelect,
    toggleSelectAll,
    clearSelection,
    isSelected,
    count: ref.current.size,
    selectedSet: ref.current,
  };
}
