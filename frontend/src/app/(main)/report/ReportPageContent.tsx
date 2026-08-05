"use client";

import { useState } from "react";
import { API_BASE } from "@/lib/constants";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { PageHeader, Spinner, PageSpinner } from "@/components/UI";
import { toast } from "sonner";
import StatCard from "@/components/StatCard";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CircleAlert, Clock, Download, FileText, Info, Inbox, Key, Lightbulb, ListChecks, PieChart, Radio, Bot, ShieldCheck, TriangleAlert, Trash2, Wand2 } from "lucide-react";
import { Accordion, AccordionItem, AccordionHeader, AccordionTrigger, AccordionPanel } from "@/components/ui/accordion";
import { severityColor } from "./_components/types";
import { useReportData } from "./_components/useReportData";

export default function ReportPage() {
  const [activeSection, setActiveSection] = useState("overview");
  const [schedName, setSchedName] = useState("");
  const [schedSchedule, setSchedSchedule] = useState("daily 08:00");
  const [schedFormat, setSchedFormat] = useState("html");
  const [schedDeliveryType, setSchedDeliveryType] = useState("");
  const [schedDeliveryTo, setSchedDeliveryTo] = useState("");
  const [creatingSched, setCreatingSched] = useState(false);
  const [schedActionId, setSchedActionId] = useState<string | null>(null);

  const { t } = useI18n();
  const {
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
    loadScheduledReports,
    generateReport,
    deleteReport,
    pdfExportUrl,
  } = useReportData();

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

  const handleGenerate = async () => {
    let sections: string[];
    if (template === "technical") {
      sections = ["agents", "tasks", "credentials", "network", "recommendations"];
    } else if (template === "executive") {
      sections = ["overview", "recommendations"];
    } else {
      sections = SECTIONS.map((s) => s.key);
    }
    await generateReport(sections);
  };

  const handleExportPDF = () => {
    window.open(`${API_BASE}${pdfExportUrl()}`, "_blank");
  };

  const handleDeleteReport = async (id: string) => {
    await deleteReport(id);
  };

  if (loading) {
    return (
      <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
        <PageSpinner />
      </div>
    );
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
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

      <Tabs value={activeSection} onValueChange={setActiveSection}>
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-4 mb-6">
          <div className="lg:col-span-1">
            <Card className="p-2">
              <TabsList className="flex-col bg-transparent p-0 gap-1 w-full h-auto">
                {SECTIONS.map((s) => (
                  <TabsTrigger key={s.key} value={s.key}
                    className="w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left text-sm font-medium transition-colors data-[selected]:bg-primary/10 data-[selected]:text-primary text-muted-foreground hover:bg-muted/50">
                    {s.icon}
                    {s.label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Card>
          </div>

          <div className="lg:col-span-3">
          <TabsContent value="overview" className="mt-0">
            <Card className="p-4 sm:p-5">
              <h2 className="text-lg font-semibold text-foreground mb-6">{t("report.settings")}</h2>
              <div className="space-y-6">
                <div>
                  <Label className="text-sm font-medium mb-3">{t("report.report_template")}</Label>
                  <RadioGroup value={template} onValueChange={setTemplate} className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    {TEMPLATES.map((tpl) => (
                      <div key={tpl.value} className={`flex flex-col items-start p-4 rounded-xl cursor-pointer transition-colors ${template === tpl.value ? "border-2 border-primary bg-primary/10" : "border border-border hover:bg-muted/50"}`}>
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
                              <a href={`${API_BASE}/report/${id}/download?format=html`} download className="p-2 text-primary hover:bg-primary/10 dark:hover:bg-primary/20 rounded-lg transition-colors">
                                <Download className="w-4 h-4" />
                              </a>
                              <Button variant="ghost" size="icon" onClick={() => handleDeleteReport(id)} className="text-destructive hover:bg-destructive/10" aria-label={t("report.a11y_delete")}>
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
          </TabsContent>

          <TabsContent value="agents" className="mt-0">
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
          </TabsContent>

          <TabsContent value="tasks" className="mt-0">
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
                    ) : taskStats.map((ts) => (
                      <TableRow key={ts.type}>
                        <TableCell className="font-medium">{ts.type || "-"}</TableCell>
                        <TableCell>{ts.total ?? 0}</TableCell>
                        <TableCell className="text-emerald-600 dark:text-emerald-400">{ts.success ?? 0}</TableCell>
                        <TableCell className="text-destructive">{ts.failed ?? 0}</TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <Progress value={ts.success_rate ?? Math.round(((ts.success ?? 0) / (ts.total || 1)) * 100)} className="flex-1 max-w-[100px]" />
                            <span className="text-xs tabular-nums">{ts.success_rate ?? Math.round(((ts.success ?? 0) / (ts.total || 1)) * 100)}%</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </Card>
          </TabsContent>

          <TabsContent value="credentials" className="mt-0">
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
                    ) : creds.map((c) => (
                      <TableRow key={c.type}>
                        <TableCell className="font-medium">{c.type || "-"}</TableCell>
                        <TableCell className="text-primary font-semibold">{c.count ?? 0}</TableCell>
                        <TableCell>{c.source || "-"}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </Card>
          </TabsContent>

          <TabsContent value="network" className="mt-0">
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
          </TabsContent>

          <TabsContent value="recommendations" className="mt-0">
            <Card className="overflow-hidden">
              <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                <h2 className="text-lg font-semibold text-foreground">{t("report.sec_findings_advice")}<span className="text-sm font-normal text-muted-foreground ml-2">{t("report.total_prefix")}{findings.length} {t("report.items")}</span></h2>
              </div>
              {findings.length === 0 ? (
                <div className="py-16 sm:py-20 text-center text-muted-foreground">
                  <ShieldCheck className="w-4 h-4" />
                  <p className="text-sm">{t("report.no_findings")}</p>
                </div>
              ) : (
                <Accordion>
                  {findings.map((f, i) => {
                    const id = f.id || String(i);
                    return (
                      <AccordionItem key={id} value={id}>
                        <AccordionHeader>
                          <AccordionTrigger className="px-4 py-3 hover:bg-muted/50">
                            <div className="flex items-start gap-3 flex-1 text-left">
                              <div className="mt-0.5">
                                {f.severity === "critical" ? <CircleAlert className="w-4 h-4 text-destructive" /> : f.severity === "high" ? <TriangleAlert className="w-4 h-4 text-orange-500" /> : f.severity === "medium" ? <CircleAlert className="w-4 h-4 text-yellow-500" /> : <Info className="w-4 h-4 text-blue-500" />}
                              </div>
                              <div className="flex-1">
                                <div className="flex items-center gap-2 flex-wrap">
                                  <span className="text-sm font-medium text-foreground">{f.title || "-"}</span>
                                  <Badge variant="outline" className={`text-(--fs-micro-sm) ${severityColor(f.severity || "low")}`}>
                                    {(f.severity || "unknown").toUpperCase()}
                                  </Badge>
                                  {f.cve_id && <Badge variant="secondary" className="text-(--fs-micro-sm) font-mono">{f.cve_id}</Badge>}
                                </div>
                              </div>
                            </div>
                          </AccordionTrigger>
                        </AccordionHeader>
                        <AccordionPanel className="px-4 pb-4">
                          <div className="space-y-2">
                            <p className="text-sm text-muted-foreground">{f.description || ""}</p>
                            {f.recommendation && (
                              <p className="text-sm text-primary">
                                <Lightbulb className="w-4 h-4" />{t("report.recommendation_label")} {f.recommendation}
                              </p>
                            )}
                          </div>
                        </AccordionPanel>
                      </AccordionItem>
                    );
                  })}
                </Accordion>
              )}
            </Card>
          </TabsContent>
          <TabsContent value="scheduled" className="mt-0">
            <div className="space-y-4">
              <Card className="p-4 sm:p-5">
                <h2 className="text-lg font-semibold mb-4">{t("report.create_scheduled")}</h2>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <Label className="mb-1">{t("report.name")}</Label>
                    <Input value={schedName} onChange={e => setSchedName(e.target.value)} placeholder={t("report.sched_name_ph")} />
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
                      <Input value={schedDeliveryTo} onChange={e => setSchedDeliveryTo(e.target.value)} placeholder={t("report.dest_ph")} className="flex-1" />
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
                    const d: { success?: boolean; error?: string } = await api.postJson(paths.report.scheduled, { name: schedName, schedule: schedSchedule, format: schedFormat, delivery_type: schedDeliveryType, delivery_to: schedDeliveryTo, include_agents: true, include_tasks: true, include_creds: true, include_audit: true });
                     if (d.success) { setSchedName(""); void loadScheduledReports(); toast.success(t("report.toast.scheduled_created")); }
                   } catch { toast.error(t("report.toast.create_scheduled_failed")); }
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
                          try { await api.post(paths.report.scheduledToggle(r.id)); void loadScheduledReports(); } catch { toast.error(t("report.toast.load_scheduled_failed")); }
                          setSchedActionId(null);
                        }} disabled={schedActionId === r.id} className={r.enabled ? "border-emerald-500 text-emerald-600" : ""}>{schedActionId === r.id ? "Toggling..." : r.enabled ? "Enabled" : "Disabled"}</Button>
                        <Button variant="outline" size="sm" onClick={async () => {
                          setSchedActionId(r.id);
                          try { await api.del(paths.report.scheduledOne(r.id)); void loadScheduledReports(); } catch { toast.error(t("report.toast.load_scheduled_failed")); }
                          setSchedActionId(null);
                        }} disabled={schedActionId === r.id} className="border-destructive/30 text-destructive">{schedActionId === r.id ? "Deleting..." : "Delete"}</Button>
                      </div>
                    </div>
                  ))}
                </div>
              </Card>
            </div>
          </TabsContent>
        </div>
      </div>
      </Tabs>
    </div>
  );
}
