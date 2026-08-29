"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface CacheEntry<T> {
  value: T;
  expiresAt: number;
}

const cache = new Map<string, CacheEntry<unknown>>();
const inflight = new Map<string, Promise<unknown>>();

/** Clear the shared module-level cache (used by tests and logout). */
export function clearDataCache(): void {
  cache.clear();
  inflight.clear();
}

/**
 * Deduped, TTL'd fetch with a shared module-level cache.
 * Concurrent callers for the same key share a single in-flight request.
 */
export async function fetchCached<T>(key: string, fetcher: () => Promise<T>, ttlMs: number): Promise<T> {
  const hit = cache.get(key) as CacheEntry<T> | undefined;
  if (hit && hit.expiresAt >= Date.now()) return hit.value;
  const existing = inflight.get(key);
  if (existing) return existing as Promise<T>;
  const run = fetcher()
    .then((value) => {
      cache.set(key, { value, expiresAt: Date.now() + ttlMs });
      return value;
    })
    .finally(() => {
      inflight.delete(key);
    });
  inflight.set(key, run);
  return run;
}

interface UseCachedDataOptions<T> {
  fetcher: () => Promise<T>;
  /** Freshness window in ms. Default 60_000 (1 min). */
  ttlMs?: number;
  /**
   * When the cached entry is stale, keep rendering its previous value while
   * revalidating in the background instead of flipping to a loading state.
   * Default true.
   */
  keepStaleWhileRevalidate?: boolean;
  /** Called when a revalidate fails. */
  onError?: (err: unknown) => void;
}

interface UseCachedDataResult<T> {
  data: T | null;
  loading: boolean;
  error: boolean;
  refresh: () => Promise<T>;
}

export function useCachedData<T>(
  key: string,
  { fetcher, ttlMs = 60_000, keepStaleWhileRevalidate = true, onError }: UseCachedDataOptions<T>,
): UseCachedDataResult<T> {
  const [data, setData] = useState<T | null>(() => {
    const entry = cache.get(key);
    return entry ? (entry.value as T) : null;
  });
  const [loading, setLoading] = useState<boolean>(() => {
    const entry = cache.get(key);
    return entry && entry.expiresAt >= Date.now() ? false : !keepStaleWhileRevalidate;
  });
  const [error, setError] = useState(false);

  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;
  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;
  const ttlRef = useRef(ttlMs);
  ttlRef.current = ttlMs;
  const keepRef = useRef(keepStaleWhileRevalidate);
  keepRef.current = keepStaleWhileRevalidate;

  useEffect(() => {
    const entry = cache.get(key);
    const fresh = !!entry && entry.expiresAt >= Date.now();
    if (fresh) {
      setData(entry.value as T);
      setLoading(false);
      return;
    }
    if (entry && keepRef.current) {
      setData(entry.value as T);
    } else {
      setLoading(true);
    }
    setError(false);
    let cancelled = false;
    fetchCached<T>(key, fetcherRef.current, ttlRef.current)
      .then((value) => {
        if (cancelled) return;
        setData(value);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(true);
        setLoading(false);
        onErrorRef.current?.(err);
      });
    return () => {
      cancelled = true;
    };
  }, [key]);

  const refresh = useCallback((): Promise<T> => {
    setLoading(true);
    setError(false);
    cache.delete(key);
    inflight.delete(key);
    const run = fetchCached<T>(key, fetcherRef.current, ttlRef.current)
      .then((value) => {
        setData(value);
        setLoading(false);
        return value;
      })
      .catch((err: unknown) => {
        setError(true);
        setLoading(false);
        onErrorRef.current?.(err);
        throw err;
      });
    return run;
  }, [key]);

  return { data, loading, error, refresh };
}