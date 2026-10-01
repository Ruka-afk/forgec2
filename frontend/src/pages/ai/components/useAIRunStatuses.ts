import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { normalizeListEnvelope } from "@/lib/envelope";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";

export type AIRunStatus = "queued" | "running" | "waiting_approval" | "completed" | "failed" | "cancelled" | string;

type ActiveRun = { session_id: number; status: AIRunStatus };

/** Keeps session run badges in sync without coupling the page to polling. */
export function useAIRunStatuses() {
  const [statuses, setStatuses] = useState<Record<number, AIRunStatus>>({});

  const refresh = useCallback(async () => {
    try {
      const payload = await api.get<unknown>(`${paths.ai.runs}?status=active`);
      const runs = normalizeListEnvelope(payload, ["runs", "data"]) as ActiveRun[];
      const next: Record<number, AIRunStatus> = {};
      for (const run of runs) {
        if (Number.isFinite(run.session_id) && run.status) next[run.session_id] = run.status;
      }
      setStatuses(next);
    } catch {
      // Keep the last known state; the global connection banner reports errors.
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);
  useVisibleInterval(() => { void refresh(); }, 5000);

  return { runStatuses: statuses, setRunStatuses: setStatuses, refreshRunStatuses: refresh };
}
