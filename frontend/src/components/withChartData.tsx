"use client";

import { useState, useEffect, useCallback, useRef, memo } from "react";
import { api } from "@/lib/api";
import { fetchCached } from "@/lib/hooks/useCachedData";
import { Spinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface ChartDataProps<T> {
  data: T;
  loading: boolean;
  error: boolean;
  onRefresh: () => void;
}

/** Freshness window for dashboard chart data. Charts trade up to ~30s of
 * staleness for shared, deduped requests across remounts. */
export const CHART_CACHE_TTL_MS = 30_000;

/** Fetch a chart endpoint through the shared TTL cache. Concurrent/lazy
 * charts for the same endpoint collapse into a single in-flight request. */
export async function fetchChartData<T>(
  endpoint: string,
  fetchFn: (endpoint: string) => Promise<unknown>,
  transform?: (raw: unknown) => T,
): Promise<T> {
  return fetchCached<T>(
    `chart:${endpoint}`,
    async () => {
      const raw = await fetchFn(endpoint);
      return transform ? transform(raw) : (raw as T);
    },
    CHART_CACHE_TTL_MS,
  );
}

export function withChartData<T>(
  Wrapped: React.ComponentType<ChartDataProps<T>>,
  endpoint: string,
  transform?: (raw: unknown) => T,
) {
  const ChartDataWrapper = memo(function ChartDataWrapper() {
    const { t } = useI18n();
    const [data, setData] = useState<T | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(false);
    const [visible, setVisible] = useState(false);
    const ref = useRef<HTMLDivElement>(null);
    const transformRef = useRef(transform);
    transformRef.current = transform;

    // IntersectionObserver: only fetch when visible
    useEffect(() => {
      const el = ref.current;
      if (!el) return;
      const obs = new IntersectionObserver(
        ([entry]) => { if (entry.isIntersecting) { setVisible(true); obs.disconnect(); } },
        { rootMargin: "100px" },
      );
      obs.observe(el);
      return () => obs.disconnect();
    }, []);

    const load = useCallback(() => {
      setLoading(true);
      setError(false);
      fetchChartData<T>(endpoint, (e) => api.get<unknown>(e), transformRef.current)
        .then((value) => setData(value))
        .catch(() => setError(true))
        .finally(() => setLoading(false));
    }, []);

    useEffect(() => { if (visible) load(); }, [visible, load]);

    return (
      <div ref={ref}>
        {loading ? (
          <div className="h-24 flex items-center justify-center text-muted-foreground/70 text-xs">
            <Spinner size="sm" />Loading...
          </div>
        ) : error ? (
          <div className="h-24 flex items-center justify-center text-destructive text-xs" role="alert">
            <AlertTriangle className="w-4 h-4 mr-2 inline" />Failed to load
            <Button variant="link" size="sm" onClick={load} className="ml-2">{t("common.retry")}</Button>
          </div>
        ) : data != null ? (
          <Wrapped data={data as T} loading={false} error={false} onRefresh={load} />
        ) : null}
      </div>
    );
  });
  ChartDataWrapper.displayName = `ChartDataWrapper(${Wrapped.displayName || Wrapped.name || "Component"})`;
  return ChartDataWrapper;
}
