"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { normalizeListEnvelope } from "@/lib/envelope";
import { useI18n } from "@/lib/i18n";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import {
  computeDateRange,
  type AgentRow,
  type CredRow,
  type FindingRow,
  type ListenerRow,
  type ReportHistoryRow,
  type ReportStats,
  type ScheduledReport,
  type TaskStatRow,
} from "./types";

export function useReportData() {
  const { t } = useI18n();
  const [stats, setStats] = useState<ReportStats>({});
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [datePreset, setDatePreset] = useState("30d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [template, setTemplate] = useState("full");
  const [agents, setAgents] = useState<AgentRow[]>([]);
  const [taskStats, setTaskStats] = useState<TaskStatRow[]>([]);
  const [creds, setCreds] = useState<CredRow[]>([]);
  const [listeners, setListeners] = useState<ListenerRow[]>([]);
  const [findings, setFindings] = useState<FindingRow[]>([]);
  const [history, setHistory] = useState<ReportHistoryRow[]>([]);
  const [scheduledReports, setScheduledReports] = useState<ScheduledReport[]>([]);

  const range = useCallback(
    () => computeDateRange(datePreset, customStart, customEnd),
    [datePreset, customStart, customEnd],
  );

  const loadOverview = useCallback(async () => {
    try {
      const data: ReportStats = await api.get(paths.report.overview);
      setStats(data);
    } catch {
      toast.error(t("report.toast.load_overview_failed"));
    }
  }, [t]);

  const loadPreview = useCallback(async () => {
    try {
      const { start, end } = range();
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
      setAgents(agentsResp.agents || []);
      setTaskStats(tasksResp.stats || []);
      setCreds(credsResp.credentials || []);
      setListeners(netResp.listeners || []);
      setFindings(findResp.findings || []);
    } catch {
      toast.error(t("report.toast.load_preview_failed"));
    }
  }, [range, t]);

  const loadHistory = useCallback(async () => {
    try {
      const data = await api.get(paths.report.history);
      setHistory(normalizeListEnvelope(data, ["reports", "Reports", "data"]) as ReportHistoryRow[]);
    } catch {
      toast.error(t("report.toast.load_history_failed"));
    }
  }, [t]);

  const loadScheduledReports = useCallback(async () => {
    try {
      const d = await api.get(paths.report.scheduled);
      setScheduledReports(normalizeListEnvelope(d, ["reports", "data"]) as ScheduledReport[]);
    } catch {
      toast.error(t("report.toast.load_scheduled_failed"));
    }
  }, [t]);

  const loadAll = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadOverview(), loadPreview(), loadHistory()]);
    setLoading(false);
  }, [loadOverview, loadPreview, loadHistory]);

  useEffect(() => {
    void loadAll();
    void loadScheduledReports();
  }, [loadAll, loadScheduledReports]);

  useVisibleInterval(loadOverview, 30000);

  useEffect(() => {
    if (!loading) void loadPreview();
  }, [datePreset, customStart, customEnd, loading, loadPreview]);

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

  const pdfExportUrl = useCallback(() => {
    const { start, end } = range();
    const params = new URLSearchParams({ format: "json", template });
    if (start) params.set("start", start);
    if (end) params.set("end", end);
    return paths.report.exportPdf(params.toString());
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
    scheduledReports,
    setScheduledReports,
    loadScheduledReports,
    loadHistory,
    generateReport,
    deleteReport,
    pdfExportUrl,
    range,
  };
}
