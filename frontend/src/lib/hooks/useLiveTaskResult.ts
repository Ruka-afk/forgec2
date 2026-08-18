"use client";

import { useCallback, useRef, useState } from "react";
import { runTask, type TaskStatus } from "@/lib/api";

export type LiveTaskStatus = "idle" | "pending" | "running" | "completed" | "failed" | "timeout";

interface UseLiveTaskResultOptions {
  /** Max time to wait for a task result before failing with the timeout status. */
  timeoutMs?: number;
}

/**
 * Dispatches a task via `runTask` and tracks its lifecycle live over the
 * WebSocket (`task_update`/`task_output` frames) instead of page-side polling.
 * Resolves with the final TaskStatus (full result fetched once completed).
 */
export function useLiveTaskResult(opts: UseLiveTaskResultOptions = {}) {
  const [status, setStatus] = useState<LiveTaskStatus>("idle");
  const [result, setResult] = useState("");
  const [error, setError] = useState("");
  const [taskId, setTaskId] = useState<number | null>(null);
  const seqRef = useRef(0);

  const reset = useCallback(() => {
    seqRef.current += 1;
    setStatus("idle");
    setResult("");
    setError("");
    setTaskId(null);
  }, []);

  const run = useCallback(
    async (agentId: string, path: string, body: Record<string, string>): Promise<TaskStatus | null> => {
      seqRef.current += 1;
      const seq = seqRef.current;
      setStatus("pending");
      setResult("");
      setError("");
      setTaskId(null);
      try {
        const final = await runTask(agentId, path, {
          body,
          timeoutMs: opts.timeoutMs,
          onStatus: (st: TaskStatus) => {
            if (seqRef.current !== seq) return;
            setTaskId(st.id);
            setStatus(st.status);
            if (st.result) setResult(st.result);
            if (st.status === "failed") setError(st.error ?? "Task failed");
          },
        });
        if (seqRef.current !== seq) return null;
        setTaskId(final.id);
        setStatus(final.status);
        if (final.result) setResult(final.result);
        if (final.status === "failed") setError(final.error ?? "Task failed");
        return final;
      } catch (e) {
        if (seqRef.current !== seq) return null;
        const msg = e instanceof Error ? e.message : String(e);
        setError(msg);
        setStatus(msg.includes("did not respond") ? "timeout" : "failed");
        return null;
      }
    },
    [opts.timeoutMs],
  );

  return { taskId, status, result, error, run, reset, running: status === "pending" || status === "running" };
}