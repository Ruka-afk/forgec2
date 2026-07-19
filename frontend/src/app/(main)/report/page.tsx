"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { api } from "@/lib/api";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { useI18n } from "@/lib/i18n";
import { PageHeader, Spinner, PageSpinner } from "@/components/UI";
import { toast } from "sonner";
import StatCard from "@/components/StatCard";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ChevronDown, ChevronUp, CircleAlert, Clock, Download, FileText, Info, Inbox, Key, Lightbulb, ListChecks, PieChart, Radio, Bot, ShieldCheck, TriangleAlert, Trash2, Wand2 } from "lucide-react";

interface ReportStats {
  total_agents?: number;
  online_agents?: number;
  total_tasks?: number;
  success_tasks?: number;
  failed_tasks?: number;
  total_creds?: number;
  total_audits?: number;
  total_listeners?: number;
  total_findings?: number;
  critical_findings?: number;
  high_findings?: number;
  medium_findings?: number;
}

interface AgentRow {
  id?: string;
  hostname?: string;
  ip?: string;
  os?: string;
  last_seen?: string;
  status?: string;
}

interface TaskStatRow {
  type?: string;
  total?: number;
  success?: number;
  failed?: number;
  success_rate?: number;
}

interface CredRow {
  type?: string;
  count?: number;
  source?: string;
}

interface ListenerRow {
  id?: string;
  name?: string;
  protocol?: string;
  status?: string;
  agent_count?: number;
  traffic?: string;
}

interface FindingRow {
  id?: string;
  title?: string;
  severity?: string;
  cve_id?: string;
  description?: string;
  recommendation?: string;
}

interface ReportHistoryRow {
  id?: string;
  template?: string;
  format?: string;
  created_at?: string;
  sections?: string[];
  size?: string;
}

interface ScheduledReport {
  id: string;
  name: string;
  enabled: boolean;
  schedule: string;
  format: string;
  last_run: string;
  next_run: string;
  run_count: number;
  delivery_type: string;
}

export default function ReportPage() {
  const [activeSection, setActiveSection] = useState("overview");
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
  const [expandedFinding, setExpandedFinding] = useState<string | null>(null);
  const [scheduledReports, setScheduledReports] = useState<ScheduledReport[]>([]);
  const [schedName, setSchedName] = useState("");
  const [schedSchedule, setSchedSchedule] = useState("daily 08:00");
  const [schedFormat, setSchedFormat] = useState("html");
  const [schedDeliveryType, setSchedDeliveryType] = useState("");
  const [schedDeliveryTo, setSchedDeliveryTo] = useState("");
  const [creatingSched, setCreatingSched] = useState(false);
  const [schedActionId, setSchedActionId] = useState<string | null>(null);

  const { t } = useI18n();

  const SECTIONS = [
    { key: "overview", label: t("report.sec_overview"), icon: <PieChart className="w-5 h-5" /> },
    { key: "agents", label: "Implant", icon: <Bot className="w-5 h-5" /> },
    { key: "tasks", label: t("report.sec_tasks"), icon: <ListChecks className="w-5 h-5" /> },
    { key: "credentials", label: t("report.sec_credentials"), icon: <Key className="w-5 h-5" /> },
    { key: "network", label: t("report.sec_network"), icon: <Radio className="w-5 h-5" /> },
    { key: "recommendations", label: t("report.sec_recommendations"), icon: <Lightbulb className="w-5 h-5" /> },
    { key: "scheduled", label: t("report.sec_scheduled"), icon: <Clock className="w-5 h-5" /> },
  ];

  const TEMPLATES = [
    { value: "full", label: t("report.tpl_full_label"), desc: t("report.tpl_full_desc") },
    { value: "executive", label: t("report.tpl_exec_label"), desc: t("report.tpl_exec_desc") },
    { value: "technical", label: t("report.tpl_tech_label"), desc: t("report.tpl_tech_desc") },
  ];

  const DATE_PRESETS = [
    { value: "7d", label: t("report.preset_7d") },
    { value: "30d", label: t("report.preset_30d") },
    { value: "90d", label: t("report.preset_90d") },
    { value: "custom", label: t("report.preset_custom") },
  ];

  const computeDateRange = useCallback(() => {
    if (datePreset === "custom") {
      return { start: customStart, end: customEnd };
    }
    const days = parseInt(datePreset);
    const end = new Date();
    const start = new Date();
    start.setDate(start.getDate() - days);
    return {
      start: start.toISOString().split("T")[0],
      end: end.toISOString().split("T")[0],
    };
  }, [datePreset, customStart, customEnd]);

  const loadOverview = useCallback(async () => {
    try {
      const data: ReportStats = await api.get("/report");
      setStats(data);
    } catch {
      toast.error("Failed to load report overview");
    }
  }, []);

  const loadPreview = useCallback(async () => {
    try {
      const { start, end } = computeDateRange();
      const qs = new URLSearchParams();
      if (start) qs.set("start", start);
      if (end) qs.set("end", end);
      const q = qs.toString();

      const agentsData: { agents?: AgentRow[]; Agents?: AgentRow[] } = await api.get(`/api/report/agents${q ? "?" + q : ""}`);
      setAgents(agentsData.agents || []);

      const tasksData: { stats?: TaskStatRow[]; Stats?: TaskStatRow[] } = await api.get(`/api/report/tasks${q ? "?" + q : ""}`);
      setTaskStats(tasksData.stats || []);

      const credsData: { credentials?: CredRow[]; Credentials?: CredRow[] } = await api.get(`/api/report/credentials${q ? "?" + q : ""}`);
      setCreds(credsData.credentials || []);

      const netData: { listeners?: ListenerRow[]; Listeners?: ListenerRow[] } = await api.get(`/api/report/network${q ? "?" + q : ""}`);
      setListeners(netData.listeners || []);

      const findData: { findings?: FindingRow[]; Findings?: FindingRow[] } = await api.get(`/api/report/findings${q ? "?" + q : ""}`);
      setFindings(findData.findings || []);
    } catch {
      toast.error("Failed to load report preview");
    }
  }, [computeDateRange]);

  const loadHistory = useCallback(async () => {
    try {
      const data: { reports?: ReportHistoryRow[]; Reports?: ReportHistoryRow[] } = await api.get("/api/report/history");
      setHistory(data.reports || []);
    } catch {
      toast.error("Failed to load report history");
    }
  }, []);

  const loadAll = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadOverview(), loadPreview(), loadHistory()]);
    setLoading(false);
  }, [loadOverview, loadPreview, loadHistory]);

  const loadScheduledReports = useCallback(async () => {
    try {
      const d: { reports?: ScheduledReport[] } = await api.json<{ reports?: ScheduledReport[] }>("/scheduled-reports");
      setScheduledReports(d.reports || []);
    } catch { toast.error("Failed to load scheduled reports"); }
  }, []);

  useEffect(() => {
    loadAll();
    loadScheduledReports();
  }, [loadAll, loadScheduledReports]);
  useVisibleInterval(loadOverview, 30000);

  useEffect(() => {
    if (!loading) loadPreview();
  }, [datePreset, customStart, customEnd, loading, loadPreview]);

  const handleGenerate = async () => {
    setGenerating(true);
    try {
      const { start, end } = computeDateRange();
      let sections: string[];
      if (template === "technical") {
        sections = ["agents", "tasks", "credentials", "network", "recommendations"];
      } else if (template === "executive") {
        sections = ["overview", "recommendations"];
      } else {
        sections = SECTIONS.map((s) => s.key);
      }
      await api.postJson("/api/report/generate", { start_date: start, end_date: end, template, sections, format: "html" });
      await loadHistory();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to generate report");
    } finally {
      setGenerating(false);
    }
  };

  const handleExportPDF = () => {
    const { start, end } = computeDateRange();
    const params = new URLSearchParams({ format: "json" });
    if (start) params.set("start", start);
    if (end) params.set("end", end);
    params.set("template", template);
    window.open(`${API_BASE}/report/export/pdf?${params}`, "_blank");
  };

  const handleDeleteReport = async (id: string) => {
    try {
      await api.del(`/api/report/${id}`);
      loadHistory();
    } catch {
      toast.error("Failed to delete report");
    }
  };

  const severityColor = (s: string) => {
    if (s === "critical") return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20";
    if (s === "high") return "bg-orange-500/10 text-orange-600 dark:text-orange-400 border-orange-500/20";
    if (s === "medium") return "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400 border-yellow-500/20";
    if (s === "low") return "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20";
    return "bg-secondary/50 text-muted-foreground border-border";
  };

  if (loading) {
    return (
      <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
        <PageSpinner />
      </div>
    );
  }

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={<><FileText className="w-4 h-4" />{t("report.title")}</>} subtitle={t("report.subtitle")}>
        <Button onClick={handleExportPDF} variant="destructive" className="gap-x-2">
          <FileText className="w-4 h-4" />{t("report.export_pdf")}
        </Button>
        <Button onClick={handleGenerate} disabled={generating} className="gap-x-2">
          {generating ? <Spinner size="xs" /> : <Wand2 className="w-4 h-4" />}
          {generating ? t("report.generating") : t("report.generate")}
        </Button>
      </PageHeader>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <StatCard color="indigo" label={t("report.stat_agents_total")} value={stats.total_agents || 0} sub={`${stats.online_agents || 0} ${t("report.online")}`} subColor="text-emerald-600" />
        <StatCard color="emerald" label={t("report.stat_task_exec")} value={stats.total_tasks || 0} sub={`${stats.success_tasks || 0} ${t("report.success")} / ${stats.failed_tasks || 0} ${t("report.failed")}`} subColor="text-muted-foreground" />
        <StatCard color="amber" label={t("report.stat_creds")} value={stats.total_creds || 0} sub={t("report.collected")} subColor="text-muted-foreground" />
        <StatCard color="red" label={t("report.stat_findings")} value={stats.total_findings || 0} sub={`${t("report.critical")}: ${stats.critical_findings || 0} | ${t("report.high")} ${stats.high_findings || 0}`} subColor="text-destructive" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-4 mb-6">
        <div className="lg:col-span-1">
          <Card className="p-2">
            {SECTIONS.map((s) => (
              <Button key={s.key} variant="ghost" onClick={() => setActiveSection(s.key)}
                className={`w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left text-sm font-medium transition-colors ${activeSection === s.key ? "bg-indigo-50 dark:bg-indigo-900/20 text-indigo-700 dark:text-indigo-300 hover:bg-indigo-50 dark:hover:bg-indigo-900/20" : "text-muted-foreground hover:bg-muted/50"}`}>
                {s.icon}
                {s.label}
              </Button>
            ))}
          </Card>
        </div>

        <div className="lg:col-span-3">
          {activeSection === "overview" && (
            <Card className="p-4 sm:p-5">
              <h2 className="text-lg font-semibold text-foreground mb-6">{t("report.settings")}</h2>
              <div className="space-y-6">
                <div>
                  <Label className="text-sm font-medium mb-3">{t("report.report_template")}</Label>
                  <RadioGroup value={template} onValueChange={setTemplate} className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    {TEMPLATES.map((tpl) => (
                      <div key={tpl.value} className={`flex flex-col items-start p-4 rounded-xl cursor-pointer transition-colors ${template === tpl.value ? "border-2 border-indigo-500 bg-indigo-50 dark:bg-indigo-900/20" : "border border-border hover:bg-muted/50"}`}>
                        <div className="flex items-center space-x-2 mb-2">
                          <RadioGroupItem value={tpl.value} id={`tpl-${tpl.value}`} />
                          <Label htmlFor={`tpl-${tpl.value}`} className="text-sm font-medium text-foreground cursor-pointer">{tpl.label}</Label>
                        </div>
                        <div className="text-xs text-muted-foreground mt-1">{tpl.desc}</div>
                      </div>
                    ))}
                  </RadioGroup>
                </div>

                <div>
                  <Label className="text-sm font-medium mb-3">{t("report.date_range")}</Label>
                  <div className="flex flex-wrap gap-2 mb-3">
                    {DATE_PRESETS.map((p) => (
                      <Button key={p.value} variant={datePreset === p.value ? "default" : "secondary"} size="sm" onClick={() => setDatePreset(p.value)}>
                        {p.label}
                      </Button>
                    ))}
                  </div>
                  {datePreset === "custom" && (
                    <div className="grid grid-cols-2 gap-3">
                      <Input aria-label={t("report.start_date")} type="date" value={customStart} onChange={(e) => setCustomStart(e.target.value)} placeholder={t("report.start_date")} />
                      <Input aria-label={t("report.end_date")} type="date" value={customEnd} onChange={(e) => setCustomEnd(e.target.value)} placeholder={t("report.end_date")} />
                    </div>
                  )}
                </div>

                <div className="border-t border-border pt-4">
                  <h3 className="text-sm font-semibold text-foreground mb-3">{t("report.history_title")}</h3>
                  {history.length === 0 ? (
                    <div className="text-center py-6 text-muted-foreground">
                      <Inbox className="w-4 h-4" />
                      <p className="text-sm">{t("report.no_history")}</p>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {history.map((r, i) => {
                        const id = r.id || String(i);
                        return (
                          <div key={id} className="flex items-center justify-between p-3 bg-muted rounded-xl hover:bg-secondary transition-colors">
                            <div className="flex items-center gap-3">
                              <FileText className="w-4 h-4" />
                              <div>
                                <div className="text-sm font-medium text-muted-foreground">{r.template || t("report.unknown_template")} - {r.format?.toUpperCase() || "HTML"}</div>
                                <div className="text-xs text-muted-foreground">{r.created_at || "-"} {r.size ? `· ${r.size}` : ""}</div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <a href={`${API_BASE}/report/${id}/download?format=html`} download className="p-2 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 rounded-lg transition-colors">
                                <Download className="w-4 h-4" />
                              </a>
                              <Button variant="ghost" size="icon" onClick={() => handleDeleteReport(id)} className="text-destructive hover:bg-destructive/10" aria-label="Delete report">
                                <Trash2 className="w-4 h-4" />
                              </Button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>
            </Card>
          )}

          {activeSection === "agents" && (
            <Card className="overflow-hidden">
              <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                <h2 className="text-lg font-semibold text-foreground">{t("report.agents_detail")} <span className="text-sm font-normal text-muted-foreground ml-2">{t("report.total")} {agents.length} {t("report.units")}</span></h2>
              </div>
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow className="text-xs">
                      <TableHead className="font-medium">{t("report.col_hostname")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_ip")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_os")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_lastseen")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_status")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {agents.length === 0 ? (
                      <TableRow><TableCell colSpan={5} className="py-16 sm:py-20 text-center text-muted-foreground"><Inbox className="w-4 h-4" />{t("report.no_data")}</TableCell></TableRow>
                    ) : agents.map((a, i) => (
                      <TableRow key={a.id || i}>
                        <TableCell className="font-medium truncate max-w-[200px]">{a.hostname || "-"}</TableCell>
                        <TableCell className="font-mono text-xs truncate max-w-[200px]">{a.ip || "-"}</TableCell>
                        <TableCell>{a.os || "-"}</TableCell>
                        <TableCell className="text-xs">{a.last_seen || "-"}</TableCell>
                        <TableCell>
                          <Badge variant={a.status === "online" ? "success" : "secondary"} className="gap-1">
                            <span className={`w-1.5 h-1.5 rounded-full ${a.status === "online" ? "bg-emerald-500" : "bg-muted-foreground"}`}></span>
                            {a.status || "unknown"}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </Card>
          )}

          {activeSection === "tasks" && (
            <Card className="overflow-hidden">
              <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                <h2 className="text-lg font-semibold text-foreground">{t("report.task_stats")} <span className="text-sm font-normal text-muted-foreground ml-2">{t("report.by_type")}</span></h2>
              </div>
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow className="text-xs">
                      <TableHead className="font-medium">{t("report.col_task_type")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_total")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_success")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_failed")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_success_rate")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {taskStats.length === 0 ? (
                      <TableRow><TableCell colSpan={5} className="py-16 sm:py-20 text-center text-muted-foreground"><Inbox className="w-4 h-4" />{t("report.no_data")}</TableCell></TableRow>
                    ) : taskStats.map((ts, i) => (
                      <TableRow key={i}>
                        <TableCell className="font-medium">{ts.type || "-"}</TableCell>
                        <TableCell>{ts.total ?? 0}</TableCell>
                        <TableCell className="text-emerald-600 dark:text-emerald-400">{ts.success ?? 0}</TableCell>
                        <TableCell className="text-destructive">{ts.failed ?? 0}</TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <div className="flex-1 h-2 bg-secondary rounded-full overflow-hidden max-w-[100px]">
                              <div className="h-full bg-indigo-500 rounded-full" style={{ width: `${ts.success_rate ?? Math.round(((ts.success ?? 0) / (ts.total || 1)) * 100)}%` }}></div>
                            </div>
                            <span className="text-xs tabular-nums">{ts.success_rate ?? Math.round(((ts.success ?? 0) / (ts.total || 1)) * 100)}%</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </Card>
          )}

          {activeSection === "credentials" && (
            <Card className="overflow-hidden">
              <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                <h2 className="text-lg font-semibold text-foreground">{t("report.cred_summary")}<span className="text-sm font-normal text-muted-foreground ml-2">{t("report.total_prefix")}{creds.reduce((s, c) => s + (c.count ?? 0), 0)} {t("report.items")}</span></h2>
              </div>
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow className="text-xs">
                      <TableHead className="font-medium">{t("report.col_cred_type")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_count")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_source")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {creds.length === 0 ? (
                      <TableRow><TableCell colSpan={3} className="py-16 sm:py-20 text-center text-muted-foreground"><Inbox className="w-4 h-4" />{t("report.no_data")}</TableCell></TableRow>
                    ) : creds.map((c, i) => (
                      <TableRow key={i}>
                        <TableCell className="font-medium">{c.type || "-"}</TableCell>
                        <TableCell className="text-indigo-600 dark:text-indigo-400 font-semibold">{c.count ?? 0}</TableCell>
                        <TableCell>{c.source || "-"}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </Card>
          )}

          {activeSection === "network" && (
            <Card className="overflow-hidden">
              <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                <h2 className="text-lg font-semibold text-foreground">{t("report.network_status")} <span className="text-sm font-normal text-muted-foreground ml-2">{t("report.listener_overview")}</span></h2>
              </div>
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow className="text-xs">
                      <TableHead className="font-medium">{t("report.col_listener")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_protocol")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_status")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_agent_count")}</TableHead>
                      <TableHead className="font-medium">{t("report.col_traffic")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {listeners.length === 0 ? (
                      <TableRow><TableCell colSpan={5} className="py-16 sm:py-20 text-center text-muted-foreground"><Inbox className="w-4 h-4" />{t("report.no_data")}</TableCell></TableRow>
                    ) : listeners.map((l, i) => (
                      <TableRow key={l.id || i}>
                        <TableCell className="font-medium">{l.name || "-"}</TableCell>
                        <TableCell><Badge variant="secondary" className="font-mono">{l.protocol || "-"}</Badge></TableCell>
                        <TableCell>
                          <span className={`inline-flex items-center gap-1 text-xs font-medium ${l.status === "active" ? "text-emerald-600 dark:text-emerald-400" : "text-muted-foreground"}`}>
                            <span className={`w-1.5 h-1.5 rounded-full ${l.status === "active" ? "bg-emerald-500" : "bg-muted-foreground"}`}></span>
                            {l.status || "-"}
                          </span>
                        </TableCell>
                        <TableCell>{l.agent_count ?? 0}</TableCell>
                        <TableCell>{l.traffic || "-"}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </Card>
          )}

          {activeSection === "recommendations" && (
            <Card className="overflow-hidden">
              <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                <h2 className="text-lg font-semibold text-foreground">{t("report.sec_findings_advice")}<span className="text-sm font-normal text-muted-foreground ml-2">{t("report.total_prefix")}{findings.length} {t("report.items")}</span></h2>
              </div>
              <div className="divide-y divide-border">
                {findings.length === 0 ? (
                  <div className="py-16 sm:py-20 text-center text-muted-foreground">
                    <ShieldCheck className="w-4 h-4" />
                    <p className="text-sm">{t("report.no_findings")}</p>
                  </div>
                ) : findings.map((f, i) => {
                  const id = f.id || String(i);
                  return (
                    <div key={id} className="p-4 hover:bg-muted/50 transition-colors">
                      <div className="flex items-start justify-between gap-3">
                        <div className="flex items-start gap-3 flex-1">
                          <div className="mt-0.5">
                            {f.severity === "critical" ? <CircleAlert className="w-4 h-4 text-destructive" /> : f.severity === "high" ? <TriangleAlert className="w-4 h-4 text-orange-500" /> : f.severity === "medium" ? <CircleAlert className="w-4 h-4 text-yellow-500" /> : <Info className="w-4 h-4 text-blue-500" />}
                          </div>
                          <div className="flex-1">
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className="text-sm font-medium text-foreground">{f.title || "-"}</span>
                              <Badge variant="outline" className={`text-[10px] ${severityColor(f.severity || "low")}`}>
                                {(f.severity || "unknown").toUpperCase()}
                              </Badge>
                              {f.cve_id && <Badge variant="secondary" className="text-[10px] font-mono">{f.cve_id}</Badge>}
                            </div>
                            {expandedFinding === id && (
                              <div className="mt-3 space-y-2">
                                <p className="text-sm text-muted-foreground">{f.description || ""}</p>
                                {f.recommendation && (
                                  <p className="text-sm text-indigo-600 dark:text-indigo-400">
                                    <Lightbulb className="w-4 h-4" />{t("report.recommendation_label")} {f.recommendation}
                                  </p>
                                )}
                              </div>
                            )}
                          </div>
                        </div>
                        <Button variant="ghost" size="icon" onClick={() => setExpandedFinding(expandedFinding === id ? null : id)} className="text-muted-foreground" aria-label={expandedFinding === id ? "Collapse finding" : "Expand finding"}>
                          {expandedFinding === id ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </Card>
          )}
          {activeSection === "scheduled" && (
            <div className="space-y-4">
              <Card className="p-4 sm:p-5">
                <h2 className="text-lg font-semibold mb-4">{t("report.create_scheduled")}</h2>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <Label className="mb-1">{t("report.name")}</Label>
                    <Input value={schedName} onChange={e => setSchedName(e.target.value)} placeholder="Daily Summary Report" />
                  </div>
                  <div>
                    <Label className="mb-1">{t("report.schedule")}</Label>
                    <Select value={schedSchedule} onValueChange={(v) => v && setSchedSchedule(v)}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="daily 08:00">{t("report.daily_0800")}</SelectItem>
                        <SelectItem value="daily 18:00">{t("report.daily_1800")}</SelectItem>
                        <SelectItem value="hour">{t("report.hourly")}</SelectItem>
                        <SelectItem value="minutes 30">{t("report.every_30min")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div>
                    <Label className="mb-1">{t("report.format")}</Label>
                    <Select value={schedFormat} onValueChange={(v) => v && setSchedFormat(v)}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="html">HTML</SelectItem>
                        <SelectItem value="json">JSON</SelectItem>
                        <SelectItem value="pdf">PDF</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div>
                    <Label className="mb-1">{t("report.delivery_method")}</Label>
                    <div className="flex gap-2">
                      <Input value={schedDeliveryTo} onChange={e => setSchedDeliveryTo(e.target.value)} placeholder="Email / Webhook URL" className="flex-1" />
                      <Select value={schedDeliveryType} onValueChange={(v) => setSchedDeliveryType(v ?? "")}>
                        <SelectTrigger className="w-28">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">None</SelectItem>
                          <SelectItem value="email">Email</SelectItem>
                          <SelectItem value="webhook">Webhook</SelectItem>
                          <SelectItem value="file">Save File</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
                <Button onClick={async () => {
                  if (!schedName.trim()) return;
                  setCreatingSched(true);
                  try {
                    const d: { success?: boolean; error?: string } = await api.postJson("/scheduled-reports", { name: schedName, schedule: schedSchedule, format: schedFormat, delivery_type: schedDeliveryType, delivery_to: schedDeliveryTo, include_agents: true, include_tasks: true, include_creds: true, include_audit: true });
                    if (d.success) { setSchedName(""); loadScheduledReports(); toast.success("Scheduled report created"); }
                  } catch { toast.error("Failed to create scheduled report"); }
                  setCreatingSched(false);
                }} disabled={creatingSched} className="mt-4">{creatingSched ? "Creating..." : t("report.create_scheduled")}</Button>
              </Card>
              <Card className="overflow-hidden">
                <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                  <h2 className="text-lg font-semibold">{t("report.configured_reports")}</h2>
                </div>
                <div className="divide-y divide-border">
                  {scheduledReports.length === 0 ? (
                    <div className="py-16 sm:py-20 text-center text-muted-foreground">
                      <Clock className="w-4 h-4" />
                      <p className="text-sm">{t("report.no_scheduled")}</p>
                    </div>
                  ) : scheduledReports.map(r => (
                    <div key={r.id} className="p-4 flex items-center justify-between">
                      <div>
                        <span className="font-medium">{r.name}</span>
                        <div className="text-xs text-muted-foreground mt-1">
                          {r.schedule} · {r.format.toUpperCase()} · Next: {r.next_run ? new Date(r.next_run).toLocaleString() : "N/A"} · Runs: {r.run_count}
                          {r.delivery_type && ` · Deliver via ${r.delivery_type}`}
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <Button variant="outline" size="sm" onClick={async () => {
                          setSchedActionId(r.id);
                          try { await api.post(`/scheduled-reports/${r.id}/toggle`); loadScheduledReports(); } catch { toast.error("Failed to load scheduled reports"); }
                          setSchedActionId(null);
                        }} disabled={schedActionId === r.id} className={r.enabled ? "border-emerald-500 text-emerald-600" : ""}>{schedActionId === r.id ? "Toggling..." : r.enabled ? "Enabled" : "Disabled"}</Button>
                        <Button variant="outline" size="sm" onClick={async () => {
                          setSchedActionId(r.id);
                          try { await api.del(`/scheduled-reports/${r.id}`); loadScheduledReports(); } catch { toast.error("Failed to load scheduled reports"); }
                          setSchedActionId(null);
                        }} disabled={schedActionId === r.id} className="border-destructive/30 text-destructive">{schedActionId === r.id ? "Deleting..." : "Delete"}</Button>
                      </div>
                    </div>
                  ))}
                </div>
              </Card>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
