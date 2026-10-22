import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { normalizeListEnvelope, firstArray } from "@/lib/envelope";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { POLL } from "@/lib/polling";
import {
  aggregateTaskStats,
  computeDateRange,
  type AgentRow,
  type CredRow,
  type FindingRow,
  type ListenerRow,
  type ReportHistoryRow,
  type ReportStats,
  type ReportTaskRow,
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

  const {
    data: statsData,
    loading: loadingOverview,
    error: overviewError,
    refresh: refreshOverview,
  } = useApiResource<ReportStats>({
    // "/report" is the HTML page route (answering JSON with that page's own
    // context, not these counters). The dashboard API is the authoritative
    // source of the summary numbers shown above the tabs.
    fetcher: async () => {
      const d = await api.get<{
        total_agents?: number;
        online_agents?: number;
        total_tasks?: number;
        completed_tasks?: number;
        failed_tasks?: number;
        total_creds?: number;
        total_audits?: number;
        total_listeners?: number;
      }>(paths.dashboard.v1);
      return {
        total_agents: d?.total_agents ?? 0,
        online_agents: d?.online_agents ?? 0,
        total_tasks: d?.total_tasks ?? 0,
        success_tasks: d?.completed_tasks ?? 0,
        failed_tasks: d?.failed_tasks ?? 0,
        total_creds: d?.total_creds ?? 0,
        total_audits: d?.total_audits ?? 0,
        total_listeners: d?.total_listeners ?? 0,
      };
    },
    pollMs: POLL.report,
    toastThrottleMs: POLL.toastThrottleLong,
    errorMessage: t("report.toast.load_overview_failed"),
  });

  const dateRef = useRef({ datePreset, customStart, customEnd });
  dateRef.current = { datePreset, customStart, customEnd };

  const {
    data: preview,
    loading: loadingPreview,
    error: previewError,
    refresh: refreshPreview,
  } = useApiResource<{
    agents: AgentRow[];
    taskStats: TaskStatRow[];
    creds: CredRow[];
    listeners: ListenerRow[];
    findings: FindingRow[];
    partial: boolean;
    failedSections: string[];
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

      // allSettled, not all: one broken section must not blank the whole
      // preview. Failed sections come back empty and are reported so the
      // UI can say "incomplete" instead of implying "no data".
      const [agentsResp, tasksResp, credsResp, netResp, findResp] = await Promise.allSettled([
        api.get<{ agents?: AgentRow[] }>(paths.report.agents(q)),
        api.get<{ stats?: TaskStatRow[] }>(paths.report.tasks(q)),
        api.get<{ credentials?: CredRow[] }>(paths.report.credentials(q)),
        api.get<{ listeners?: ListenerRow[] }>(paths.report.network(q)),
        api.get<{ findings?: FindingRow[] }>(paths.report.findings(q)),
      ]);
      const value = <T,>(r: PromiseSettledResult<T>): T | undefined =>
        r.status === "fulfilled" ? r.value : undefined;
      const failedSections: string[] = [];
      const sectionNames = ["agents", "tasks", "credentials", "listeners", "findings"];
      [agentsResp, tasksResp, credsResp, netResp, findResp].forEach((r, i) => {
        if (r.status === "rejected") failedSections.push(sectionNames[i]);
      });
      return {
        // `firstArray` tolerates both `null` and an unexpected object shape;
        // a null list used to reach `.map` and blank the page behind the
        // error boundary.
        agents: firstArray(value(agentsResp), ["agents", "Agents"]) as AgentRow[],
        // The section API returns raw task rows; the Tasks tab renders one row
        // per task type. `stats` is a single aggregate object, not a list.
        taskStats: aggregateTaskStats(
          firstArray(value(tasksResp), ["tasks", "Tasks"]) as ReportTaskRow[],
        ),
        creds: firstArray(value(credsResp), ["credentials", "Credentials"]) as CredRow[],
        listeners: firstArray(value(netResp), ["listeners", "Listeners"]) as ListenerRow[],
        findings: firstArray(value(findResp), ["findings", "Findings"]) as FindingRow[],
        partial: failedSections.length > 0,
        failedSections,
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

  const {
    data: historyData,
    loading: loadingHistory,
    error: historyError,
    refresh: loadHistory,
  } = useApiResource<ReportHistoryRow[]>({
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
  // A failed overview must not read as a clean bill of health: every figure
  // below is derived from it. Findings are synthesized by the section API
  // (credentials + failed tasks), so their counts come from the preview.
  const stats = overviewError
    ? null
    : statsData
      ? {
          ...statsData,
          total_findings: findings.length,
          critical_findings: findings.filter((f) => f.severity === "critical").length,
          high_findings: findings.filter((f) => f.severity === "high").length,
        }
      : null;
  // One or more preview sections failed: the data is incomplete, which the
  // page must state rather than rendering empty tabs as fact.
  const previewPartial = preview?.partial ?? false;
  const previewFailedSections = preview?.failedSections ?? [];

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
      } catch {
        toast.error(t("report.toast.generate_failed"));
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
    overviewError,
    previewError,
    previewPartial,
    previewFailedSections,
    historyError,
    refreshOverview,
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
