import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { TASK_HARD_TIMEOUT_MS, TASK_HIDDEN_POLL_MS } from "@/lib/task-poll";

type TaskPollStatus = "idle" | "pending" | "running" | "completed" | "failed" | "timeout";

interface TaskStatusResponse {
  id?: number | string;
  status?: string;
  result?: string;
  error?: string;
  output?: string;
}

export function useTaskResult(agentId: string, pollMs = 2000, maxAttempts = 45) {
  const { t } = useI18n();
  const [taskId, setTaskId] = useState<string | null>(null);
  const [status, setStatus] = useState<TaskPollStatus>("idle");
  const [result, setResult] = useState<string>("");
  // Stalled (soft timeout): past maxAttempts with no terminal state. The task
  // is still live server-side, so tracking continues at a background cadence
  // instead of reporting a false "timeout" that invites double-execution.
  const [stalled, setStalled] = useState(false);
  const attempts = useRef(0);
  const startedAt = useRef(0);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inFlightSeq = useRef<number | null>(null);
  const seqRef = useRef(0);
  const taskIdRef = useRef<string | null>(null);

  const stop = useCallback(() => {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    seqRef.current += 1;
  }, []);

  const reset = useCallback(() => {
    stop();
    setTaskId(null);
    taskIdRef.current = null;
    setStatus("idle");
    setResult("");
    setStalled(false);
    attempts.current = 0;
  }, [stop]);

  const start = useCallback((id: string | number) => {
    stop();
    attempts.current = 0;
    startedAt.current = Date.now();
    taskIdRef.current = String(id);
    setTaskId(String(id));
    setStatus("pending");
    setResult("");
    setStalled(false);
  }, [stop]);

  useEffect(() => {
    taskIdRef.current = taskId;
  }, [taskId]);

  useEffect(() => {
    if (!agentId || !taskId) return;
    // Invalidate any prior chain (cleanup also clears the timer); bumping the
    // sequence here guarantees stale ticks never reschedule after re-subscribe.
    seqRef.current += 1;
    const seq = seqRef.current;

    const tick = async () => {
      if (seqRef.current !== seq) return;
      if (inFlightSeq.current === seq) return;
      // Hard deadline matches the server stale sweep: only here is "timeout"
      // truthful. The soft deadline (maxAttempts) flips to stalled tracking.
      if (Date.now() - startedAt.current > TASK_HARD_TIMEOUT_MS) {
        if (seqRef.current !== seq) return;
        setStatus("timeout");
        return;
      }
      attempts.current += 1;
      if (attempts.current > maxAttempts) {
        if (seqRef.current !== seq) return;
        setStalled(true);
        setStatus((s) => (s === "pending" ? "pending" : "running"));
      }
      inFlightSeq.current = seq;
      let terminal = false;
      try {
        const data = await api.get<TaskStatusResponse>(paths.agents.task(agentId, taskId));
        if (seqRef.current !== seq) return;
        const st = (data.status || "").toLowerCase();
        if (st === "completed" || st === "success" || st === "done") {
          terminal = true;
          setStatus("completed");
          setResult(data.result || data.output || "");
          return;
        }
        if (st === "failed" || st === "error" || st === "cancelled") {
          terminal = true;
          setStatus("failed");
          setResult(data.error || data.result || t("common.task_failed"));
          return;
        }
        if (st === "running" || st === "sent") {
          setStatus("running");
        } else {
          setStatus("pending");
        }
      } catch {
        // keep polling until the hard timeout
      } finally {
        if (inFlightSeq.current === seq) inFlightSeq.current = null;
        if (!terminal && seqRef.current === seq) {
          // Background cadence once stalled or hidden: keep tracking a live
          // task without hammering the server (or a throttled timer).
          const slow = document.hidden || attempts.current > maxAttempts;
          timer.current = setTimeout(tick, slow ? TASK_HIDDEN_POLL_MS : pollMs);
        }
      }
    };

    const handleVisibility = () => {
      if (!document.hidden && seqRef.current === seq && inFlightSeq.current !== seq) {
        if (timer.current) clearTimeout(timer.current);
        tick();
      }
    };

    tick();
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibility);
      stop();
    };
  }, [agentId, taskId, pollMs, maxAttempts, stop, t]);

  return { taskId, status, result, stalled, start, reset, polling: status === "pending" || status === "running" };
}
