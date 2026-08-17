"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadText, downloadJSON } from "@/lib/download";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";
import { Spinner } from "@/components/ui/spinner";
import { PageContainer } from "@/components/ui/page-container";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Bot, ChevronDown, CircleAlert, Info, CircleQuestionMark, Eye, FileCode, FileSpreadsheet, History, Lightbulb, Play, ShieldAlert, ShieldCheck, TriangleAlert, TrendingUp, Zap } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";

interface PrivescAgent {
  id?: string;
  hostname?: string;
  ip?: string;
  os?: string;
}

interface PrivescHistory {
  id?: string;
  agent_id?: string;
  check_type?: string;
  status?: string;
  result?: string;
  created_at?: string;
  findings_count?: number;
}

interface PrivescFinding {
  id?: string;
  title?: string;
  severity?: string;
  cve_id?: string;
  description?: string;
  exploit_command?: string;
  recommendation?: string;
}

function severityBadge(severity: string) {
  switch (severity) {
    case "critical": return "bg-destructive/10 text-destructive border-destructive/30";
    case "high": return "bg-warning/10 text-warning dark:text-warning border-warning/30";
    case "medium": return "bg-warning/10 text-warning dark:text-warning border-warning/30";
    case "low": return "bg-primary/10 text-primary border-primary/30";
    default: return "bg-secondary/50 text-muted-foreground border-border";
  }
}

function severityIcon(severity: string): React.ReactNode {
  switch (severity) {
    case "critical": return <CircleAlert className="w-4 h-4 text-destructive" />;
    case "high": return <TriangleAlert className="w-4 h-4 text-warning" />;
    case "medium": return <CircleAlert className="w-4 h-4 text-warning" />;
    case "low": return <Info className="w-4 h-4 text-primary" />;
    default: return <CircleQuestionMark className="w-4 h-4 text-muted-foreground" />;
  }
}

export default function PrivescPage() {
  const [agents, setAgents] = useState<PrivescAgent[]>([]);
  const [history, setHistory] = useState<PrivescHistory[]>([]);
  const [findings, setFindings] = useState<PrivescFinding[]>([]);
  const [, setLoading] = useState(true);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [checkType, setCheckType] = useState("all");
  const [running, setRunning] = useState(false);
  const [expandedFinding, setExpandedFinding] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("all");

  const { confirm, modal } = useConfirm();

  const { t } = useI18n();

  const CHECK_TYPES = [
    { value: "all", icon: "??", label: t("privesc.ct_all"), desc: t("privesc.ct_all_desc") },
    { value: "printnightmare", icon: "???", label: "PrintNightmare", desc: t("privesc.ct_printnightmare_desc") },
    { value: "elevate", icon: "??", label: "Elevate", desc: t("privesc.ct_elevate_desc") },
    { value: "uac_bypass", icon: "???", label: "UAC Bypass", desc: t("privesc.ct_uac_desc") },
    { value: "amsi_bypass", icon: "???", label: "AMSI Bypass", desc: t("privesc.ct_amsi_desc") },
    { value: "etw_bypass", icon: "??", label: "ETW Bypass", desc: t("privesc.ct_etw_desc") },
    { value: "cvescan", icon: "??", label: "CVE Scan", desc: t("privesc.ct_cvescan_desc") },
    { value: "binary_abuse", icon: "??", label: "Binary Abuse", desc: t("privesc.ct_binary_desc") },
    { value: "service_exploit", icon: "??", label: "Service Exploit", desc: t("privesc.ct_service_desc") },
    { value: "token_abuse", icon: "??", label: "Token Abuse", desc: t("privesc.ct_token_desc") },
    { value: "kernel_exploit", icon: "??", label: "Kernel Exploit", desc: t("privesc.ct_kernel_desc") },
    { value: "password_finder", icon: "??", label: "Password Finder", desc: t("privesc.ct_password_desc") },
  ];

  const loadBusyRef = useRef(false);
  const delayedReloadRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadData = useCallback(async () => {
    if (loadBusyRef.current) return;
    loadBusyRef.current = true;
    try {
      const data = await api.get(paths.privesc.page);
      setAgents((data.agents || []) as PrivescAgent[]);
      setHistory((data.history || []) as PrivescHistory[]);
      setFindings((data.findings || []) as PrivescFinding[]);
    } catch {
      setAgents([]);
      setHistory([]);
      setFindings([]);
    }
    setLoading(false);
    loadBusyRef.current = false;
  }, []);

  useEffect(() => {
    return () => {
      if (delayedReloadRef.current) clearTimeout(delayedReloadRef.current);
      loadBusyRef.current = false;
    };
  }, []);

  useEffect(() => { loadData(); }, [loadData]);
  useVisibleInterval(loadData, 10000);

  const handleRun = async () => {
    if (!selectedAgent) {
      toast.error(t("privesc.toast_select_agent"));
      return;
    }
    setRunning(true);
    try {
      await api.postJson(paths.privesc.run, { agent_id: selectedAgent, check_type: checkType });
      if (delayedReloadRef.current) clearTimeout(delayedReloadRef.current);
      delayedReloadRef.current = setTimeout(loadData, 2000);
    } catch { toast.error(t("privesc.toast.start_check_failed")); }
    setRunning(false);
  };

  const handleViewHistory = async (historyId: string) => {
    try {
      const data = await api.get(paths.privesc.history(historyId));
      setFindings((data.findings || data.tasks || []) as PrivescFinding[]);
    } catch { toast.error(t("privesc.toast.load_history_failed")); }
  };

  const handleExecuteExploit = async (finding: PrivescFinding) => {
    if (!(await confirm({ message: `${t("privesc.confirm_exploit")}\n\n${finding.title || t("privesc.unknown")}` }))) return;
    try {
      await api.postJson(paths.privesc.execute, { agent_id: selectedAgent, check_type: checkType, exploit_command: finding.exploit_command });
    } catch { toast.error(t("privesc.toast.execute_exploit_failed")); }
  };

  const handleExportJSON = () => {
    downloadJSON(findings, `privesc_findings_${new Date().toISOString().split("T")[0]}.json`);
  };

  const handleExportCSV = () => {
    const headers = ["Title", "Severity", "CVE", "Description", "Recommendation"];
    const rows = findings.map((f) => [
      f.title || "", f.severity || "", f.cve_id || "", (f.description || "").replace(/,/g, ";"), (f.recommendation || "").replace(/,/g, ";"),
    ]);
    const csv = [headers, ...rows].map((r) => r.map((c) => `"${c}"`).join(",")).join("\n");
    downloadText(csv, `privesc_findings_${new Date().toISOString().split("T")[0]}.csv`, "text/csv");
  };

  const totalChecks = history.length;
  const criticalCount = findings.filter((f) => f.severity === "critical").length;
  const highCount = findings.filter((f) => f.severity === "high").length;
  const mediumCount = findings.filter((f) => f.severity === "medium").length;
  const lowCount = findings.filter((f) => f.severity === "low").length;

  return (
    <PageContainer title={<><TrendingUp className="w-4 h-4" />{t("privesc.title")}</>} subtitle={t("privesc.subtitle")} actions={<>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" onClick={handleExportJSON}>
            <FileCode className="w-4 h-4" /> JSON
          </Button>
          <Button variant="secondary" size="sm" onClick={handleExportCSV}>
            <FileSpreadsheet className="w-4 h-4" /> CSV
          </Button>
        </div>
      </>}>

      <Card className="p-3 mb-4 border-warning/40 bg-warning/10 text-sm text-warning dark:text-warning">
        <div className="font-semibold">{t("privesc.honesty_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("privesc.honesty_desc")}</div>
      </Card>

      <div className="grid grid-cols-2 sm:grid-cols-5 gap-4 mb-6">
        <Card className="p-4">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("privesc.stat_total_checks")}</div>
          <div className="text-2xl font-bold mt-1 text-primary">{totalChecks}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("privesc.stat_critical")}</div>
          <div className="text-2xl font-bold mt-1 text-destructive">{criticalCount}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("privesc.stat_high")}</div>
          <div className="text-2xl font-bold mt-1 text-warning dark:text-warning">{highCount}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("privesc.stat_medium")}</div>
          <div className="text-2xl font-bold mt-1 text-warning dark:text-warning">{mediumCount}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("privesc.stat_low")}</div>
          <div className="text-2xl font-bold mt-1 text-primary">{lowCount}</div>
        </Card>
      </div>

      <Card className="px-6 py-6 mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-primary/10 dark:bg-primary/20 rounded-lg flex items-center justify-center">
            <ShieldAlert className="w-4 h-4" />
          </div>
          <div>
            <div className="text-sm font-semibold">{t("privesc.new_task")}</div>
            <div className="text-xs text-muted-foreground">{t("privesc.new_task_desc")}</div>
          </div>
        </div>

        <div className="space-y-5">
          <div>
            <Label className="text-sm font-medium mb-2 block">
              <Bot className="w-4 h-4" />{t("privesc.target_agent")}
            </Label>
            <Select value={selectedAgent || "placeholder"} onValueChange={(v) => setSelectedAgent(v === "placeholder" ? "" : v ?? "")}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("privesc.select_target_implant")} />
              </SelectTrigger>
              <SelectContent>
                {agents.map((a) => {
                  const id = a.id || "";
                  const hostname = a.hostname || "";
                  const ip = a.ip || "";
                  const os = a.os || "";
                  return <SelectItem key={id} value={id}>{hostname} ({ip}) - {os}</SelectItem>;
                })}
                {!agents.length && <SelectItem value="placeholder" disabled>{t("privesc.select_target_implant")}</SelectItem>}
              </SelectContent>
            </Select>
          </div>

          <div>
            <span className="block text-sm font-semibold mb-3">{t("privesc.check_type_label")}</span>
            <RadioGroup value={checkType} onValueChange={setCheckType} className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {CHECK_TYPES.map((ct) => (
                <div key={ct.value} className={`flex items-center gap-3 p-3 rounded-lg cursor-pointer transition-colors ${checkType === ct.value ? "border-2 border-primary bg-primary/10" : "border border-border hover:bg-muted/50"}`}>
                  <RadioGroupItem value={ct.value} id={`ct-${ct.value}`} />
                  <Label htmlFor={`ct-${ct.value}`} className="min-w-0 cursor-pointer">
                    <div className="text-sm font-medium">{ct.icon} {ct.label}</div>
                    <div className="text-xs text-muted-foreground truncate">{ct.desc}</div>
                  </Label>
                </div>
              ))}
            </RadioGroup>
          </div>

          <div className="flex gap-3 pt-3 border-t border-border">
            <Button onClick={handleRun} disabled={running} className="flex-1 h-11">
              {running ? <><Spinner size="xs" className="mr-2" />{t("privesc.running")}</> : <><Play className="w-4 h-4" />{t("privesc.exec_privesc")}</>}
            </Button>
          </div>
        </div>
      </Card>

      <Card className="mb-6 overflow-hidden p-0">
        <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border flex items-center justify-between flex-wrap gap-3">
          <h2 className="text-lg font-semibold">{t("privesc.findings_title")} <span className="text-sm font-normal text-muted-foreground ml-2">{findings.length} </span></h2>
          <div className="flex items-center gap-2">
            {["all", "critical", "high", "medium", "low"].map((s) => (
              <Button key={s} variant={statusFilter === s ? "default" : "secondary"} size="xs" onClick={() => setStatusFilter(s)}>
                {s === "all" ? t("privesc.filter_all") : s.charAt(0).toUpperCase() + s.slice(1)}
              </Button>
            ))}
          </div>
        </div>
        <div className="divide-y divide-border max-h-[400px] overflow-y-auto">
          {findings.length === 0 ? (
            <div className="py-16 sm:py-20 text-center text-muted-foreground">
              <ShieldCheck className="w-4 h-4" />
              <p className="text-sm">{t("privesc.no_findings")}</p>
              <p className="text-xs mt-1 text-muted-foreground">{t("privesc.no_findings_hint")}</p>
            </div>
          ) : findings.filter((f) => statusFilter === "all" || f.severity === statusFilter).map((f, i) => {
            const fid = f.id || String(i);
            const isExpanded = expandedFinding === fid;
            return (
              <div key={fid} className="p-4 hover:bg-muted/50 transition-colors">
                <Collapsible open={isExpanded} onOpenChange={(open) => setExpandedFinding(open ? fid : null)}>
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-start gap-3 flex-1">
                      {severityIcon(f.severity || "low")}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-sm font-medium">{f.title || "-"}</span>
                          <Badge variant="secondary" className={`px-2 py-0.5 text-(--fs-micro-sm) rounded-full border font-medium ${severityBadge(f.severity || "low")}`}>
                            {(f.severity || "unknown").toUpperCase()}
                          </Badge>
                          {f.cve_id && <Badge variant="secondary" className="font-mono text-(--fs-micro-sm)">{f.cve_id}</Badge>}
                        </div>
                        <CollapsibleContent>
                          <div className="mt-3 space-y-2">
                            {f.description && <p className="text-sm text-muted-foreground">{f.description}</p>}
                            {f.recommendation && (
                              <p className="text-sm text-primary">
                                <Lightbulb className="w-4 h-4" />{t("privesc.recommendation_label")} {f.recommendation}
                              </p>
                            )}
                            {f.exploit_command && (
                              <div className="flex items-center gap-2">
                                <code className="text-xs font-mono bg-card text-success px-3 py-1.5 rounded-lg flex-1 overflow-x-auto">{f.exploit_command}</code>
                                <Button variant="destructive" size="sm" onClick={() => handleExecuteExploit(f)} className="shrink-0">
                                  <Zap className="w-4 h-4" /> {t("privesc.execute")}
                                </Button>
                              </div>
                            )}
                          </div>
                        </CollapsibleContent>
                      </div>
                    </div>
                    <CollapsibleTrigger render={<Button variant="ghost" size="icon-xs" aria-label={t("common.expand")} />}>
                      <ChevronDown className="w-4 h-4" />
                    </CollapsibleTrigger>
                  </div>
                </Collapsible>
              </div>
            );
          })}
        </div>
      </Card>

      <Card className="overflow-hidden p-0">
        <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
          <h2 className="text-lg font-semibold">{t("privesc.history_title")} <span className="text-sm font-normal text-muted-foreground ml-2">{history.length} </span></h2>
        </div>
        {history.length === 0 ? (
          <div className="py-16 sm:py-20 text-center text-muted-foreground">
            <History className="w-4 h-4" />
              <p className="text-sm">{t("privesc.no_history")}</p>
          </div>
        ) : (
          <Table>
            <TableHeader className="bg-muted">
              <TableRow>
                <TableHead className="text-xs">{t("privesc.col_time")}</TableHead>
                <TableHead className="text-xs">{t("privesc.col_agent")}</TableHead>
                <TableHead className="text-xs">{t("privesc.check_type_label")}</TableHead>
                <TableHead className="text-xs">{t("privesc.col_status")}</TableHead>
                <TableHead className="text-xs">{t("privesc.col_result")}</TableHead>
                <TableHead className="text-xs">{t("privesc.col_action")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {history.map((item, i) => {
                const hid = item.id || String(i);
                return (
                  <TableRow key={hid}>
                    <TableCell className="text-xs font-mono text-muted-foreground">{formatTime(item.created_at || "")}</TableCell>
                    <TableCell className="text-xs font-mono">{(item.agent_id || "").substring(0, 8)}</TableCell>
                    <TableCell className="text-xs">{item.check_type || "-"}</TableCell>
                    <TableCell>
                      {(item.status) === "completed" ? (
                        <Badge variant="success">{item.status || "-"}</Badge>
                      ) : (item.status) === "running" ? (
                        <Badge variant="default">{item.status || "-"}</Badge>
                      ) : (
                        <Badge variant="secondary">{item.status || "-"}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-xs text-primary font-medium">{item.findings_count ?? 0}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                         <Tooltip>
                           <TooltipTrigger render={<Button variant="ghost" size="icon-xs" onClick={() => handleViewHistory(hid)} aria-label={t("privesc.view_history")} />}>
                              <Eye className="w-4 h-4" />
                            </TooltipTrigger>
                           <TooltipContent>{t("privesc.view_result")}</TooltipContent>
                         </Tooltip>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        )}
      </Card>

      {modal}
    </PageContainer>
  );
}


