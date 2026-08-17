"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadText, downloadJSON } from "@/lib/download";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/ui/spinner";
import { PageContainer } from "@/components/ui/page-container";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Crosshair, FileCode, FileSpreadsheet, History, Inbox, Info, Play, Radar } from "lucide-react";

interface ScanAgent {
  id?: string;
  ID?: string;
  hostname?: string;
  Hostname?: string;
  ip?: string;
  IP?: string;
}

interface ScanResult {
  IP?: string;
  ip?: string;
  Port?: number;
  port?: number;
  Protocol?: string;
  protocol?: string;
  Status?: string;
  status?: string;
  Service?: string;
  service?: string;
  Version?: string;
  version?: string;
  Banner?: string;
  banner?: string;
}

interface ActiveScan {
  ID?: string;
  id?: string;
  Agent?: string;
  agent?: string;
  Target?: string;
  target?: string;
  Type?: string;
  type?: string;
  Progress?: number;
  progress?: number;
  Status?: string;
  status?: string;
  StartedAt?: string;
  started_at?: string;
}

interface ScanHistory {
  ID?: string;
  id?: string;
  Target?: string;
  target?: string;
  Type?: string;
  type?: string;
  Ports?: number;
  ports?: number;
  Results?: number;
  results?: number;
  Status?: string;
  status?: string;
  CreatedAt?: string;
  created_at?: string;
}

interface ScannerData {
  agents?: ScanAgent[];
  results?: ScanResult[];
  active_scans?: ActiveScan[];
  history?: ScanHistory[];
}

export default function ScannerPage() {
  const [data, setData] = useState<ScannerData | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [targetAddr, setTargetAddr] = useState("");
  const [scanType, setScanType] = useState("tcp_connect");
  const [portMode, setPortMode] = useState("top");
  const [customPorts, setCustomPorts] = useState("");
  const [showCustomRange, setShowCustomRange] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [activeTab, setActiveTab] = useState<"results" | "active" | "history">("results");

  const { t } = useI18n();

  const loadData = useCallback(async () => {
    try {
      const result = await api.get(paths.scanner.page);
      setData({
        agents: (result.agents || []) as ScanAgent[],
        results: (result.results || []) as ScanResult[],
        active_scans: (result.active_scans || []) as ActiveScan[],
        history: (result.history || []) as ScanHistory[],
      });
    } catch {
      setData({ agents: [], results: [], active_scans: [], history: [] });
    }
    setLoading(false);
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

  useEffect(() => {
    setShowCustomRange(portMode === "custom");
  }, [portMode]);

  const hasActiveScan = data?.active_scans && data.active_scans.some(s => (s.status) === "running");
  useVisibleInterval(loadData, hasActiveScan ? 3000 : 0);

  const handleStartScan = async () => {
    if (!selectedAgent || !targetAddr) return;
    setScanning(true);
    try {
      const body: Record<string, string> = {
        agent_id: selectedAgent,
        target: targetAddr,
        scan_type: scanType,
      };
      if (portMode === "custom" && customPorts) {
        body.port_range = customPorts;
      } else {
        body.top_ports = "1000";
      }
      await api.post(paths.scanner.scan, body);
      setActiveTab("active");
      loadData();
    } catch { toast.error(t("scanner.toast.start_scan_failed")); }
    setScanning(false);
  };

  const handleExport = (format: "csv" | "json") => {
    const results = data?.results || [];
    if (results.length === 0) return;
    if (format === "csv") {
      const header = "IP,Port,Protocol,State,Service,Version,Banner";
      const rows = results.map(r => {
        const ip = r.ip ?? "";
        const port = r.port ?? "";
        const proto = r.protocol ?? "";
        const state = r.status ?? "";
        const svc = r.service ?? "";
        const ver = r.version ?? "";
        const banner = (r.banner ?? "").replace(/,/g, " ");
        return `${ip},${port},${proto},${state},${svc},${ver},${banner}`;
      });
      downloadText([header, ...rows].join("\n"), "scan_results.csv", "text/csv");
    } else {
      downloadJSON(results, "scan_results.json");
    }
  };

  const getStatusVariant = (status: string) => {
    const s = status?.toLowerCase() ?? "";
    if (s === "open") return "success" as const;
    if (s === "closed") return "destructive" as const;
    if (s === "filtered") return "warning" as const;
    return "secondary" as const;
  };

  return (
    <PageContainer title={t("scanner.title")} icon={<Radar className="w-4 h-4" />} subtitle={t("scanner.subtitle")} contentClassName="space-y-6">
      <Card className="p-3 border-warning/40 bg-warning/10 text-sm text-warning-foreground">
        <div className="font-semibold">{t("scanner.honesty_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("scanner.honesty_desc")}</div>
      </Card>

      <Card className="p-4 sm:p-5 gap-0 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-info/10 dark:bg-info/30 rounded-lg flex items-center justify-center">
            <Radar className="w-4 h-4" />
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("scanner.new_task")}</div>
            <div className="text-xs text-muted-foreground">{t("scanner.new_task_desc")}</div>
          </div>
        </div>

        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <div>
              <Label className="block text-xs font-medium text-muted-foreground mb-1.5">{t("scanner.target_agent")}</Label>
              <Select value={selectedAgent} onValueChange={v => setSelectedAgent(v ?? "")}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("scanner.select_agent")} />
                </SelectTrigger>
                <SelectContent>
                  {data?.agents?.map(a => {
                    const id = a.id || "";
                    const hostname = a.hostname || "";
                    const ip = a.ip || "";
                    return <SelectItem key={id} value={id}>{hostname} ({ip})</SelectItem>;
                  })}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="block text-xs font-medium text-muted-foreground mb-1.5">{t("scanner.target_addr")}</Label>
              <Input aria-label={t("scanner.target_addr_ph")} type="text" value={targetAddr} onChange={e => setTargetAddr(e.target.value)} placeholder={t("scanner.target_addr_ph")} className="font-mono" />
            </div>
            <div>
              <Label className="block text-xs font-medium text-muted-foreground mb-1.5">{t("scanner.scan_type")}</Label>
              <Select value={scanType} onValueChange={v => setScanType(v ?? "tcp_connect")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="tcp_connect">TCP Connect</SelectItem>
                  <SelectItem value="tcp_syn">TCP SYN (Stealth)</SelectItem>
                  <SelectItem value="udp">UDP</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="block text-xs font-medium text-muted-foreground mb-1.5">{t("scanner.port_range")}</Label>
              <Select value={portMode} onValueChange={v => setPortMode(v ?? "top")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="top">{t("scanner.top1000")}</SelectItem>
                  <SelectItem value="top100">{t("scanner.top100")}</SelectItem>
                  <SelectItem value="custom">{t("scanner.custom_range")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {showCustomRange && (
            <div>
              <Label className="block text-xs font-medium text-muted-foreground mb-1.5">{t("scanner.port_range")}</Label>
              <Input aria-label="1-1000,8080,8443" type="text" value={customPorts} onChange={e => setCustomPorts(e.target.value)} placeholder="1-1000,8080,8443" className="font-mono" />
              <p className="text-xs text-muted-foreground mt-1">{t("scanner.port_format_hint")}</p>
            </div>
          )}

          <div className="flex items-center gap-3 pt-2">
            <Button onClick={handleStartScan} disabled={scanning || !selectedAgent || !targetAddr}>
              {scanning ? <Spinner size="xs" className="mr-1" /> : <Play className="w-4 h-4" />}
              <span>{scanning ? t("scanner.scanning") : t("scanner.start_scan")}</span>
            </Button>
            <span className="text-xs text-muted-foreground">
              <Info className="w-4 h-4" />
               {t("scanner.scan_bg_hint")}            </span>
          </div>
        </div>
      </Card>

      {(data?.active_scans?.length ?? 0) > 0 && (
        <Card className="p-4 sm:p-5 gap-0 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
          <div className="flex items-center gap-x-3 mb-4">
            <div className="w-10 h-10 bg-warning/10 rounded-lg flex items-center justify-center">
              <Spinner size="xs" />
            </div>
            <div>
              <div className="text-sm font-semibold text-foreground">{t("scanner.active_scans_title")}</div>
              <div className="text-xs text-muted-foreground">{data?.active_scans?.length || 0} {t("scanner.active_scan_msg")}</div>
            </div>
          </div>
          <div className="space-y-3">
            {data?.active_scans?.map((scan, i) => {
              const scanId = scan.id || String(i);
              const target = scan.target || "";
              const progress = scan.progress ?? 0;
              const status = scan.status || "running";
              const type = scan.type || "";
              const agent = scan.agent || "";
              return (
                <div key={scanId} className="bg-muted rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <Crosshair className="w-4 h-4" />
                      <span className="text-sm font-medium text-foreground font-mono">{target}</span>
                       <Badge variant="outline">{type}</Badge>
                    </div>
                    <span className="text-xs text-muted-foreground">{status} via {agent}</span>
                  </div>
                  <Progress value={progress} />
                  <div className="text-xs text-muted-foreground mt-1 text-right">{progress}%</div>
                </div>
              );
            })}
          </div>
        </Card>
      )}

      <Card className="py-0 gap-0 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
        <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as typeof activeTab)}>
        <div className="flex items-center justify-between px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
            <TabsList>
              {(["results", "active", "history"] as const).map(tab => (
                <TabsTrigger key={tab} value={tab}>
                  {tab === "results" ? t("scanner.tab_results") : tab === "active" ? t("scanner.tab_active") : t("scanner.tab_history")}
                </TabsTrigger>
              ))}
            </TabsList>
          {activeTab === "results" && (
            <div className="flex items-center gap-2">
              <Button variant="secondary" size="sm" onClick={() => handleExport("csv")} className="text-muted-foreground">
                <FileSpreadsheet className="w-4 h-4" />CSV
              </Button>
              <Button variant="secondary" size="sm" onClick={() => handleExport("json")} className="text-muted-foreground">
                <FileCode className="w-4 h-4" />JSON
              </Button>
            </div>
          )}
        </div>

        <div className="p-4">
          <TabsContent value="results" className="mt-0">
            {loading ? (
              <Table>
                <TableHeader><TableRow>
                  {["IP", t("scanner.col_port"), t("scanner.col_protocol"), t("scanner.col_status"), t("scanner.col_service"), t("scanner.col_version"), "Banner"].map(h => (
                    <TableHead key={h}>{h}</TableHead>
                  ))}
                </TableRow></TableHeader>
                <TableBody>{[1,2,3].map(i => (
                  <TableRow key={i}>{[1,2,3,4,5,6,7].map(j => (
                    <TableCell key={j}><Skeleton className="h-3 w-16" /></TableCell>
                  ))}</TableRow>
                ))}</TableBody>
              </Table>
            ) : data?.results && data.results.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>IP</TableHead>
                    <TableHead>{t("scanner.col_port")}</TableHead>
                    <TableHead>{t("scanner.col_protocol")}</TableHead>
                    <TableHead>{t("scanner.col_status")}</TableHead>
                    <TableHead>{t("scanner.col_service")}</TableHead>
                    <TableHead>{t("scanner.col_version")}</TableHead>
                    <TableHead>{t("scanner.col_banner")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.results.map((r) => (
                    <TableRow key={`${r.ip}-${r.port}-${r.protocol}`}>
                      <TableCell className="font-mono text-muted-foreground">{r.ip ?? "-"}</TableCell>
                      <TableCell className="font-mono font-medium text-info dark:text-info">{r.port ?? "-"}</TableCell>
                      <TableCell className="text-muted-foreground">{r.protocol ?? "-"}</TableCell>
                      <TableCell><Badge variant={getStatusVariant(r.status ?? "open")}>{r.status ?? "open"}</Badge></TableCell>
                      <TableCell className="text-muted-foreground">{r.service ?? "-"}</TableCell>
                      <TableCell className="text-muted-foreground text-xs">{r.version ?? "-"}</TableCell>
                      <TableCell className="text-xs text-muted-foreground max-w-xs truncate">{r.banner ?? "-"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className="text-center py-16 sm:py-20 text-muted-foreground dark:text-muted-foreground">
                <Inbox className="w-4 h-4" />
                <p>{t("scanner.no_results")}</p>
              </div>
            )}
          </TabsContent>

          <TabsContent value="active" className="mt-0">
            {data?.active_scans && data.active_scans.length > 0 ? (
              <div className="space-y-3">
                {data.active_scans.map((scan, i) => {
                  const scanId = scan.id || String(i);
                  const progress = scan.progress ?? 0;
                  return (
                    <div key={scanId} className="bg-muted rounded-lg p-4">
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-sm font-medium font-mono text-foreground">{scan.target}</span>
                        <span className="text-xs text-muted-foreground">{progress}%</span>
                      </div>
                      <Progress value={progress} />
                      <div className="flex items-center justify-between mt-2">
                        <Badge variant="outline">{scan.type}</Badge>
                        <span className="text-xs text-muted-foreground">{scan.started_at || ""}</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="text-center py-16 sm:py-20 text-muted-foreground dark:text-muted-foreground">
                <Inbox className="w-4 h-4" />
                <p>{t("scanner.no_active")}</p>
              </div>
            )}
          </TabsContent>

          <TabsContent value="history" className="mt-0">
            {data?.history && data.history.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("scanner.col_target")}</TableHead>
                    <TableHead>{t("scanner.col_type")}</TableHead>
                    <TableHead>{t("scanner.col_port_count")}</TableHead>
                    <TableHead>{t("scanner.col_result")}</TableHead>
                    <TableHead>{t("scanner.col_status")}</TableHead>
                    <TableHead>{t("scanner.col_time")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.history.map((h, i) => (
                    <TableRow key={h.id || i}>
                      <TableCell className="font-mono text-muted-foreground truncate max-w-[200px]">{h.target}</TableCell>
                      <TableCell className="text-muted-foreground">{h.type}</TableCell>
                      <TableCell className="text-muted-foreground">{h.ports ?? 0}</TableCell>
                      <TableCell className="font-medium text-info dark:text-info">{h.results ?? 0}</TableCell>
                      <TableCell>
                        <Badge variant={(h.status ?? "") === "completed" ? "success" : "warning"}>
                          {h.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">{h.created_at ?? "-"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className="text-center py-16 sm:py-20 text-muted-foreground dark:text-muted-foreground">
                <History className="w-4 h-4" />
                <p>{t("scanner.no_history")}</p>
              </div>
            )}
          </TabsContent>
        </div>
        </Tabs>
      </Card>
    </PageContainer>
  );
}

