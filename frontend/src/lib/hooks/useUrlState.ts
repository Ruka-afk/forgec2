"use client";

import { useCallback, useEffect, useState } from "react";

// readUrlState extracts a validated filter value from the query string.
// Unknown or missing values fall back to `initial`.
export function readUrlState<T extends string>(key: string, initial: T, allowed: readonly T[]): T {
  let raw: string | null = null;
  try {
    raw = new URLSearchParams(window.location.search).get(key);
  } catch {
    // ignore
  }
  return raw !== null && (allowed as readonly string[]).includes(raw) ? (raw as T) : initial;
}

// applyUrlState persists a filter value into the query string without adding a
// history entry (replaceState). Values equal to `initial` are removed so the
// URL stays clean for the default state.
export function applyUrlState<T extends string>(key: string, value: T, initial: T): void {
  try {
    const url = new URL(window.location.href);
    if (value === initial) url.searchParams.delete(key);
    else url.searchParams.set(key, value);
    window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);
  } catch {
    // ignore
  }
}

// useUrlState persists a UI filter/value in the query string via
// history.replaceState. The value is read back on mount (deep links) and on
// back/forward navigation (popstate).
export function useUrlState<T extends string>(
  key: string,
  initial: T,
  allowed: readonly T[],
): [T, (v: T) => void] {
  const [value, setValue] = useState<T>(initial);

  useEffect(() => {
    const sync = () => setValue(readUrlState(key, initial, allowed));
    sync();
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, [key, initial, allowed]);

  const set = useCallback(
    (v: T) => {
      setValue(v);
      applyUrlState(key, v, initial);
    },
    [key, initial],
  );

  return [value, set];
}