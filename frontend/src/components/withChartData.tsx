"use client";

import { useState, useEffect, useCallback, useRef, memo } from "react";
import { api } from "@/lib/api";
import { Spinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

interface ChartDataProps<T> {
  data: T;
  loading: boolean;
  error: boolean;
  onRefresh: () => void;
}

export function withChartData<T>(
  Wrapped: React.ComponentType<ChartDataProps<T>>,
  endpoint: string,
  transform?: (raw: unknown) => T,
) {
  const ChartDataWrapper = memo(function ChartDataWrapper() {
    const [data, setData] = useState<T | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(false);
    const [visible, setVisible] = useState(false);
    const ref = useRef<HTMLDivElement>(null);

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
      api.get<unknown>(endpoint)
        .then((raw) => setData(transform ? transform(raw) : (raw as T)))
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
          <div className="h-24 flex items-center justify-center text-destructive text-xs">
            <AlertTriangle className="w-4 h-4 mr-2 inline" />Failed to load
            <Button variant="link" size="sm" onClick={load} className="ml-2">Retry</Button>
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
