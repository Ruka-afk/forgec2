"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

export type TaskPollStatus = "idle" | "pending" | "running" | "completed" | "failed" | "timeout";

interface TaskStatusResponse {
  id?: number | string;
  status?: string;
  result?: string;
  error?: string;
  output?: string;
}

export function useTaskResult(agentId: string, pollMs = 2000, maxAttempts = 45) {
  const [taskId, setTaskId] = useState<string | null>(null);
  const [status, setStatus] = useState<TaskPollStatus>("idle");
  const [result, setResult] = useState<string>("");
  const attempts = useRef(0);
  const timer = useRef<ReturnType<typeof setInterval> | null>(null);

  const stop = useCallback(() => {
    if (timer.current) {
      clearInterval(timer.current);
      timer.current = null;
    }
  }, []);

  const reset = useCallback(() => {
    stop();
    setTaskId(null);
    setStatus("idle");
    setResult("");
    attempts.current = 0;
  }, [stop]);

  const start = useCallback((id: string | number) => {
    stop();
    attempts.current = 0;
    setTaskId(String(id));
    setStatus("pending");
    setResult("");
  }, [stop]);

  useEffect(() => {
    if (!agentId || !taskId) return;
    stop();
    timer.current = setInterval(async () => {
      attempts.current += 1;
      if (attempts.current > maxAttempts) {
        setStatus("timeout");
        stop();
        return;
      }
      try {
        const data = await api.get<TaskStatusResponse>(paths.agents.task(agentId, taskId));
        const st = (data.status || "").toLowerCase();
        if (st === "completed" || st === "success" || st === "done") {
          setStatus("completed");
          setResult(data.result || data.output || "");
          stop();
        } else if (st === "failed" || st === "error" || st === "cancelled") {
          setStatus("failed");
          setResult(data.error || data.result || "Task failed");
          stop();
        } else if (st === "running" || st === "sent") {
          setStatus("running");
        } else {
          setStatus("pending");
        }
      } catch {
        // keep polling until timeout
      }
    }, pollMs);
    return () => stop();
  }, [agentId, taskId, pollMs, maxAttempts, stop]);

  return { taskId, status, result, start, reset, polling: status === "pending" || status === "running" };
}
