"use client";

import { useEffect, useState } from "react";

/**
 * useState persisted to localStorage. Reads once on mount (client-side),
 * writes on every change. Safe when storage is unavailable or the tab
 * restored the value as invalid JSON — falls back to `initial`.
 */
export function usePersistedState<T>(key: string, initial: T) {
  const [value, setValue] = useState<T>(() => {
    if (typeof window === "undefined") return initial;
    try {
      const raw = window.localStorage.getItem(key);
      return raw === null ? initial : (JSON.parse(raw) as T);
    } catch {
      return initial;
    }
  });

  useEffect(() => {
    try {
      window.localStorage.setItem(key, JSON.stringify(value));
    } catch {
      // Storage full / disabled — state still works in-memory.
    }
  }, [key, value]);

  return [value, setValue] as const;
}