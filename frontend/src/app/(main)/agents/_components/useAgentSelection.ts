"use client";

import { useState, useCallback } from "react";
import type { Beacon } from "./types";

export function useAgentSelection(beacons: Beacon[]) {
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const toggleSelect = useCallback((id: string, checked: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id); else next.delete(id);
      return next;
    });
  }, []);

  const toggleSelectAll = useCallback((checked: boolean) => {
    if (checked) setSelected(new Set(beacons.map((b) => b.id || "").filter(Boolean)));
    else setSelected(new Set());
  }, [beacons]);

  return {
    selected,
    setSelected,
    toggleSelect,
    toggleSelectAll,
  };
}
