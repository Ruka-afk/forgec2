"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";

interface UsePagedFetchOptions<T> {
  endpoint: string;
  pageSize?: number;
  params?: Record<string, string>;
  transform?: (data: unknown) => T[];
  totalCountKey?: string;
  dataKey?: string;
}

interface UsePagedFetchResult<T> {
  data: T[];
  loading: boolean;
  total: number;
  page: number;
  setPage: (p: number) => void;
  refresh: () => Promise<void>;
  error: string | null;
}

export function usePagedFetch<T>({
  endpoint,
  pageSize = 50,
  params = {},
  transform,
  totalCountKey = "total",
  dataKey = "data",
}: UsePagedFetchOptions<T>): UsePagedFetchResult<T> {
  const [data, setData] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const paramsRef = useRef(params);
  paramsRef.current = params;
  const transformRef = useRef(transform);
  transformRef.current = transform;

  const refresh = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);
    try {
      const searchParams = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
      for (const [k, v] of Object.entries(paramsRef.current)) {
        if (v) searchParams.set(k, v);
      }
      const res = await api.get(`${endpoint}?${searchParams}`, { signal: controller.signal }) as Record<string, unknown>;
      if (controller.signal.aborted) return;
      const tf = transformRef.current;
      const items = tf
        ? tf(res[dataKey] ?? res)
        : (Array.isArray(res) ? res : (res[dataKey] ?? res)) as T[];
      setData(items);
      setTotal((res[totalCountKey] as number) ?? items.length);
    } catch (err) {
      if (controller.signal.aborted) return;
      const msg = err instanceof Error ? err.message : "Failed to load data";
      setError(msg);
      toast.error(msg);
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [page, pageSize, endpoint, totalCountKey, dataKey]);

  useEffect(() => {
    void refresh();
    return () => { abortRef.current?.abort(); };
  }, [refresh]);

  return { data, loading, total, page, setPage, refresh, error };
}
