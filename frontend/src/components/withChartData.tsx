"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { apiGet } from "@/lib/api";

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
  return function ChartDataWrapper() {
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
      apiGet<unknown>(endpoint)
        .then((raw) => setData(transform ? transform(raw) : (raw as T)))
        .catch(() => setError(true))
        .finally(() => setLoading(false));
    }, []);

    useEffect(() => { if (visible) load(); }, [visible, load]);

    return (
      <div ref={ref}>
        {loading ? (
          <div className="h-24 flex items-center justify-center text-[var(--text-tertiary)] text-xs">
            <i className="fa-solid fa-circle-notch fa-spin mr-2"></i>Loading...
          </div>
        ) : error ? (
          <div className="h-24 flex items-center justify-center text-[var(--danger-text)] text-xs">
            <i className="fa-solid fa-triangle-exclamation mr-2"></i>Failed to load
            <button onClick={load} className="ml-2 underline hover:no-underline">Retry</button>
          </div>
        ) : data != null ? (
          <Wrapped data={data as T} loading={false} error={false} onRefresh={load} />
        ) : null}
      </div>
    );
  };
}
