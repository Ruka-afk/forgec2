"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { normalizeListEnvelope } from "@/lib/envelope";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { POLL } from "@/lib/polling";
import {
  computeDateRange,
  type AgentRow,
  type CredRow,
  type FindingRow,
  type ListenerRow,
  type ReportHistoryRow,
  type ReportStats,
  type TaskStatRow,
} from "./types";

export function useReportData() {
  const { t } = useI18n();
  const [generating, setGenerating] = useState(false);
  const [datePreset, setDatePreset] = useState("30d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [template, setTemplate] = useState("full");

  const range = useCallback(
    () => computeDateRange(datePreset, customStart, customEnd),
    [datePreset, customStart, customEnd],
  );

  const { data: statsData, loading: loadingOverview } = useApiResource<ReportStats>({
    fetcher: async () => api.get<ReportStats>(paths.report.overview),
    pollMs: POLL.report,
    toastThrottleMs: POLL.toastThrottleLong,
    errorMessage: t("report.toast.load_overview_failed"),
  });
  const stats = statsData ?? {};

  const dateRef = useRef({ datePreset, customStart, customEnd });
  dateRef.current = { datePreset, customStart, customEnd };

  const { data: preview, loading: loadingPreview, refresh: refreshPreview } = useApiResource<{
    agents: AgentRow[];
    taskStats: TaskStatRow[];
    creds: CredRow[];
    listeners: ListenerRow[];
    findings: FindingRow[];
  }>({
    fetcher: async () => {
      const { start, end } = computeDateRange(
        dateRef.current.datePreset,
        dateRef.current.customStart,
        dateRef.current.customEnd,
      );
      const qs = new URLSearchParams();
      if (start) qs.set("start", start);
      if (end) qs.set("end", end);
      const q = qs.toString();

      const [agentsResp, tasksResp, credsResp, netResp, findResp] = await Promise.all([
        api.get<{ agents?: AgentRow[] }>(paths.report.agents(q)),
        api.get<{ stats?: TaskStatRow[] }>(paths.report.tasks(q)),
        api.get<{ credentials?: CredRow[] }>(paths.report.credentials(q)),
        api.get<{ listeners?: ListenerRow[] }>(paths.report.network(q)),
        api.get<{ findings?: FindingRow[] }>(paths.report.findings(q)),
      ]);
      return {
        agents: agentsResp.agents || [],
        taskStats: tasksResp.stats || [],
        creds: credsResp.credentials || [],
        listeners: netResp.listeners || [],
        findings: findResp.findings || [],
      };
    },
    toastThrottleMs: POLL.toastThrottle,
    errorMessage: t("report.toast.load_preview_failed"),
  });

  const skipFirstDateRef = useRef(true);
  useEffect(() => {
    if (skipFirstDateRef.current) {
      skipFirstDateRef.current = false;
      return;
    }
    void refreshPreview();
  }, [datePreset, customStart, customEnd, refreshPreview]);

  const { data: historyData, loading: loadingHistory, refresh: loadHistory } = useApiResource<ReportHistoryRow[]>({
    fetcher: async () =>
      normalizeListEnvelope(await api.get(paths.report.history), ["reports", "Reports", "data"]) as ReportHistoryRow[],
    toastThrottleMs: POLL.toastThrottle,
    errorMessage: t("report.toast.load_history_failed"),
  });

  const loading = loadingOverview || loadingPreview || loadingHistory;
  const history = historyData ?? [];
  const agents = preview?.agents ?? [];
  const taskStats = preview?.taskStats ?? [];
  const creds = preview?.creds ?? [];
  const listeners = preview?.listeners ?? [];
  const findings = preview?.findings ?? [];

  const generateReport = useCallback(
    async (sections: string[]) => {
      setGenerating(true);
      try {
        const { start, end } = range();
        await api.postJson(paths.report.generate, {
          start_date: start,
          end_date: end,
          template,
          sections,
          format: "html",
        });
        await loadHistory();
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("report.toast.generate_failed"));
      } finally {
        setGenerating(false);
      }
    },
    [range, template, loadHistory, t],
  );

  const deleteReport = useCallback(
    async (id: string) => {
      try {
        await api.del(paths.report.one(id));
        void loadHistory();
      } catch {
        toast.error(t("report.toast.delete_failed"));
      }
    },
    [loadHistory, t],
  );

  const htmlExportUrl = useCallback(() => {
    const { start, end } = range();
    const params = new URLSearchParams({ format: "json", template });
    if (start) params.set("start", start);
    if (end) params.set("end", end);
    return paths.report.exportHtml(params.toString());
  }, [range, template]);

  return {
    stats,
    loading,
    generating,
    datePreset,
    setDatePreset,
    customStart,
    setCustomStart,
    customEnd,
    setCustomEnd,
    template,
    setTemplate,
    agents,
    taskStats,
    creds,
    listeners,
    findings,
    history,
    loadHistory,
    generateReport,
    deleteReport,
    htmlExportUrl,
    range,
  };
}
