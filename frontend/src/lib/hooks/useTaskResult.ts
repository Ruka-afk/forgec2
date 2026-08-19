"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";

export type TaskPollStatus = "idle" | "pending" | "running" | "completed" | "failed" | "timeout";

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
  const attempts = useRef(0);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inFlight = useRef(false);
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
    attempts.current = 0;
  }, [stop]);

  const start = useCallback((id: string | number) => {
    stop();
    attempts.current = 0;
    taskIdRef.current = String(id);
    setTaskId(String(id));
    setStatus("pending");
    setResult("");
  }, [stop]);

  useEffect(() => {
    taskIdRef.current = taskId;
  }, [taskId]);

  useEffect(() => {
    if (!agentId || !taskId) return;
    stop();
    seqRef.current += 1;
    const seq = seqRef.current;

    const tick = async () => {
      if (seqRef.current !== seq) return;
      if (inFlight.current) return;
      attempts.current += 1;
      if (attempts.current > maxAttempts) {
        if (seqRef.current !== seq) return;
        setStatus("timeout");
        return;
      }
      inFlight.current = true;
      try {
        const data = await api.get<TaskStatusResponse>(paths.agents.task(agentId, taskId));
        if (seqRef.current !== seq) return;
        const st = (data.status || "").toLowerCase();
        if (st === "completed" || st === "success" || st === "done") {
          setStatus("completed");
          setResult(data.result || data.output || "");
          return;
        }
        if (st === "failed" || st === "error" || st === "cancelled") {
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
        // keep polling until timeout
      } finally {
        inFlight.current = false;
        if (seqRef.current === seq && !document.hidden) {
          timer.current = setTimeout(tick, pollMs);
        }
      }
    };

    const handleVisibility = () => {
      if (!document.hidden && seqRef.current === seq && !inFlight.current) {
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

  return { taskId, status, result, start, reset, polling: status === "pending" || status === "running" };
}
