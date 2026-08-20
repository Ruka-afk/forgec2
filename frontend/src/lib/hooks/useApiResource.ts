"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { useI18n } from "@/lib/i18n";

interface UseApiResourceOptions<T> {
  fetcher: (signal?: AbortSignal) => Promise<T>;
  /** Poll interval in ms; 0 (default) disables. Pauses when tab hidden. */
  pollMs?: number;
  /** Gate the initial load (e.g. lazy views). Default true. */
  enabled?: boolean;
  /** Keep previously-loaded data on refresh errors (no flicker). Default true. */
  retainOnError?: boolean;
  /** Called with the error whenever a fetch fails. */
  onError?: (err: unknown) => void;
  /** Throttle toast.error(errMsg) to once per N ms. 0 disables toasts. */
  toastThrottleMs?: number;
  /** Message used for `error`/toast when a fetch fails. */
  errorMessage?: string;
}

interface UseApiResourceResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  /** Re-fetch immediately (also used as the poll tick). */
  refresh: () => Promise<void>;
  /** Locally replace data without a fetch (optimistic updates). */
  setData: React.Dispatch<React.SetStateAction<T | null>>;
  /** Clear the current error state (e.g. dismiss banner). */
  setError: React.Dispatch<React.SetStateAction<string | null>>;
}

/**
 * Unified fetch state machine: { data, loading, error, refresh } with
 * visibility-aware polling and silent background revalidation — once data
 * has loaded, refresh errors retain the old value instead of flashing an
 * error/loading state.
 */
export function useApiResource<T>({
  fetcher,
  pollMs = 0,
  enabled = true,
  retainOnError = true,
  onError,
  toastThrottleMs = 0,
  errorMessage: errorMessageOpt,
}: UseApiResourceOptions<T>): UseApiResourceResult<T> {
  const { t } = useI18n();
  const errorMessage = errorMessageOpt ?? t("common.load_failed");
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(enabled);
  const [error, setError] = useState<string | null>(null);

  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;
  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;
  const retainRef = useRef(retainOnError);
  retainRef.current = retainOnError;
  const toastRef = useRef({ throttleMs: toastThrottleMs, last: 0 });
  toastRef.current.throttleMs = toastThrottleMs;
  const errorMsgRef = useRef(errorMessage);
  errorMsgRef.current = errorMessage;
  const hasDataRef = useRef(false);
  const inFlightRef = useRef(false);
  const abortRef = useRef<AbortController | null>(null);

  const refresh = useCallback(async () => {
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    inFlightRef.current = true;
    setError(null);
    setLoading(!hasDataRef.current);
    try {
      const value = await fetcherRef.current(ac.signal);
      if (ac.signal.aborted) return;
      hasDataRef.current = true;
      setData(value);
    } catch (err) {
      if (ac.signal.aborted) return;
      onErrorRef.current?.(err);
      if (!retainRef.current || !hasDataRef.current) {
        setError(errorMsgRef.current);
      }
      const { throttleMs, last } = toastRef.current;
      if (throttleMs > 0 && Date.now() - last >= throttleMs) {
        toastRef.current.last = Date.now();
        toast.error(errorMsgRef.current);
      }
    } finally {
      if (abortRef.current === ac) {
        abortRef.current = null;
        inFlightRef.current = false;
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    if (enabled) refresh();
  }, [enabled, refresh]);

  useEffect(() => () => abortRef.current?.abort(), []);

  useVisibleInterval(() => { if (!inFlightRef.current) void refresh(); }, enabled ? pollMs : 0);

  return { data, loading, error, refresh, setData, setError };
}
