"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface CacheEntry<T> {
  value: T;
  expiresAt: number;
}

const cache = new Map<string, CacheEntry<unknown>>();
const inflight = new Map<string, Promise<unknown>>();
const generations = new Map<string, number>();
let cacheEpoch = 0;

/** Clear the shared module-level cache (used by tests and logout). */
export function clearDataCache(): void {
  cacheEpoch += 1;
  cache.clear();
  inflight.clear();
  generations.clear();
}

function invalidateKey(key: string): void {
  generations.set(key, (generations.get(key) ?? 0) + 1);
  cache.delete(key);
  inflight.delete(key);
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
  const epoch = cacheEpoch;
  const generation = generations.get(key) ?? 0;
  const run = fetcher()
    .then((value) => {
      if (cacheEpoch === epoch && (generations.get(key) ?? 0) === generation) {
        cache.set(key, { value, expiresAt: Date.now() + ttlMs });
      }
      return value;
    })
    .finally(() => {
      if (inflight.get(key) === run) inflight.delete(key);
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
    if (!entry) return true;
    return entry.expiresAt >= Date.now() ? false : !keepStaleWhileRevalidate;
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
  const requestSeqRef = useRef(0);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      requestSeqRef.current += 1;
    };
  }, []);

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
    const seq = ++requestSeqRef.current;
    fetchCached<T>(key, fetcherRef.current, ttlRef.current)
      .then((value) => {
        if (cancelled || requestSeqRef.current !== seq) return;
        setData(value);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled || requestSeqRef.current !== seq) return;
        setError(true);
        setLoading(false);
        onErrorRef.current?.(err);
      });
    return () => {
      cancelled = true;
    };
  }, [key]);

  const refresh = useCallback((): Promise<T> => {
    const seq = ++requestSeqRef.current;
    setLoading(true);
    setError(false);
    invalidateKey(key);
    const run = fetchCached<T>(key, fetcherRef.current, ttlRef.current)
      .then((value) => {
        if (mountedRef.current && requestSeqRef.current === seq) {
          setData(value);
          setLoading(false);
        }
        return value;
      })
      .catch((err: unknown) => {
        if (mountedRef.current && requestSeqRef.current === seq) {
          setError(true);
          setLoading(false);
          onErrorRef.current?.(err);
        }
        throw err;
      });
    return run;
  }, [key]);

  return { data, loading, error, refresh };
}
