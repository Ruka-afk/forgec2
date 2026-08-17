"use client";

import { useEffect, useState, useRef } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { formatTime } from "@/lib/utils";
import { PageContainer } from "@/components/ui/page-container";
import { DataState } from "@/components/ui/data-state";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { StatTile } from "@/components/ui/stat-tile";
import { Button } from "@/components/ui/button";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Network, RefreshCw, Trash2 } from "lucide-react";

interface TrafficEntry {
  id?: string;
  ID?: string;
  timestamp?: string;
  Timestamp?: string;
  method?: string;
  Method?: string;
  path?: string;
  Path?: string;
  source_ip?: string;
  SourceIP?: string;
  agent_id?: string;
  AgentID?: string;
  status_code?: number;
  StatusCode?: number;
  size?: number;
  Size?: number;
  latency?: number;
  Latency?: number;
  protocol?: string;
  Protocol?: string;
}

const EMPTY_TRAFFIC: TrafficEntry[] = [];

function applyFilter(data: TrafficEntry[], ip: string) {
  if (!ip) return data;
  return data.filter((e) => (e.source_ip || "").includes(ip));
}

export default function TrafficPage() {
  const [filteredEntries, setFilteredEntries] = useState<TrafficEntry[]>([]);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoScroll, setAutoScroll] = useState(false);
  const [sourceIpFilter, setSourceIpFilter] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);
  const { t } = useI18n();

  const { data, loading, error, refresh: loadTraffic, setData } = useApiResource<TrafficEntry[]>({
    fetcher: async () => {
      const data = await api.get(paths.traffic.page);
      const arr = Array.isArray(data) ? data : (data?.data ?? data?.traffic ?? []);
      return Array.isArray(arr) ? (arr as TrafficEntry[]) : [];
    },
    pollMs: autoRefresh ? 5_000 : 0,
    toastThrottleMs: 10_000,
    errorMessage: t("traffic.toast.load_failed"),
  });
  const entries = data ?? EMPTY_TRAFFIC;

  useEffect(() => {
    setFilteredEntries(applyFilter(entries, sourceIpFilter));
  }, [sourceIpFilter, entries]);

  useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [filteredEntries, autoScroll]);

  const clearLog = () => { setData(null); };

  const sourceIps = [...new Set(entries.map(e => e.source_ip || "").filter(Boolean))];

  const totalRequests = entries.length;
  const beacons = entries.filter(e => { const p = e.protocol || ""; return p.toLowerCase().includes("beacon"); }).length;
  const errors = entries.filter(e => { const s = e.status_code ?? 0; return s >= 400; }).length;
  const dataTransferred = entries.reduce((acc, e) => acc + (e.size ?? 0), 0);

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(1) + " " + sizes[i];
  };

  const getMethodStyle = (method: string) => {
    const m = method.toUpperCase();
    if (m === "POST") return "bg-success/10 text-success";
    if (m === "GET") return "bg-info/10 text-info";
    if (m === "BEACON") return "bg-chart-6/10 text-chart-6";
    if (m === "PUT") return "bg-warning/10 text-warning";
    if (m === "DELETE") return "bg-destructive/10 text-destructive";
    return "bg-muted text-muted-foreground";
  };

  return (
    <PageContainer title={t("traffic.title")} subtitle={`${t("traffic.request_log")} · C2 Beacon ${t("traffic.comm_record")}`} contentClassName="space-y-6" actions={<>
        <div className="flex items-center gap-2 flex-wrap">
          <Label className="flex items-center gap-x-2 text-sm text-muted-foreground cursor-pointer">
            <Checkbox checked={autoRefresh} onCheckedChange={setAutoRefresh} />
             {t("traffic.auto_refresh")}
          </Label>
          <Label className="flex items-center gap-x-2 text-sm text-muted-foreground cursor-pointer">
            <Checkbox checked={autoScroll} onCheckedChange={setAutoScroll} />
            {t("traffic.auto_scroll")}
          </Label>
          <Select value={sourceIpFilter} onValueChange={v => setSourceIpFilter(v ?? "")}>
            <SelectTrigger className="w-full"><SelectValue placeholder={t("traffic.all_ip")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("traffic.all_ip")}</SelectItem>
              {sourceIps.map(ip => (
                <SelectItem key={ip} value={ip}>{ip}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button onClick={loadTraffic} size="lg" className="">
            <RefreshCw className="w-4 h-4" />
            <span>{t("traffic.refresh")}</span>
          </Button>
          <Button onClick={clearLog} size="lg" className="bg-destructive hover:bg-destructive/90 text-destructive-foreground">
            <Trash2 className="w-4 h-4" />
            <span>{t("traffic.clear")}</span>
          </Button>
        </div>
      </>}>

      <Card className="p-4">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-5">
          <StatTile centered label={t("traffic.total_requests")} value={totalRequests} tone="primary" />
          <StatTile centered label={t("traffic.beacons")} value={beacons} tone="violet" />
          <StatTile centered label={t("traffic.errors")} value={errors} tone="destructive" />
          <StatTile centered label={t("traffic.data_transfer")} value={formatBytes(dataTransferred)} tone="success" />
        </div>
      </Card>

      <Card className="overflow-hidden">
        <div className="bg-muted border-b border-border px-6 py-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-x-3">
              <Network className="w-4 h-4" />
              <span className="text-sm font-medium text-muted-foreground">{t("traffic.beacon_comm")}</span>
            </div>
            <span className="text-xs text-muted-foreground">{t("traffic.showing")} {filteredEntries.length} / {loading ? "..." : entries.length} {t("traffic.records")}</span>
          </div>
        </div>

        <DataState
          loading={loading}
          error={error}
          onRetry={loadTraffic}
          empty={!loading && !error && filteredEntries.length === 0}
          emptyIcon={Network}
          emptyTitle={t("traffic.empty")}
          loadingSkeleton={
            <Table>
              <TableHeader>
                <TableRow className="bg-muted border-b border-border sticky top-0">
                  <TableHead className="text-xs font-medium min-w-[80px]">{t("traffic.time")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[80px]">{t("traffic.col_method")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[200px]">{t("traffic.col_path")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[120px]">{t("traffic.col_source_ip")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[100px]">{t("traffic.col_agent")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[60px] text-center">{t("traffic.status")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[60px] text-right">{t("traffic.col_size")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[70px] text-right">{t("traffic.col_latency")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="font-mono">
                {[1, 2, 3, 4, 5].map(i => (
                  <TableRow key={i}>
                    {[1, 2, 3, 4, 5, 6, 7, 8].map(j => (
                      <TableCell key={j}><Skeleton className="h-3 w-16" /></TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          }
        >
          <div ref={containerRef} className="max-h-[500px] overflow-y-auto">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted border-b border-border sticky top-0">
                  <TableHead className="text-xs font-medium min-w-[80px]">{t("traffic.time")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[80px]">{t("traffic.col_method")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[200px]">{t("traffic.col_path")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[120px]">{t("traffic.col_source_ip")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[100px]">{t("traffic.col_agent")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[60px] text-center">{t("traffic.status")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[60px] text-right">{t("traffic.col_size")}</TableHead>
                  <TableHead className="text-xs font-medium min-w-[70px] text-right">{t("traffic.col_latency")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="font-mono">
                {filteredEntries.map((e, i) => {
                  const id = e.id || String(i);
                  const ts = e.timestamp || "";
                  const method = e.method || "";
                  const path = e.path || "";
                  const ip = e.source_ip || "";
                  const agent = e.agent_id || "";
                  const status = e.status_code ?? 0;
                  const size = e.size ?? 0;
                  const latency = e.latency ?? 0;

                  return (
                    <TableRow key={id}>
                      <TableCell className="text-xs text-muted-foreground">{formatTime(ts)}</TableCell>
                      <TableCell>
                        <span className={`px-2 py-0.5 rounded text-xs font-bold ${getMethodStyle(method)}`}>{method}</span>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground max-w-[250px] truncate">{path}</TableCell>
                      <TableCell className="text-xs text-muted-foreground">{ip}</TableCell>
                      <TableCell className="text-xs text-muted-foreground">{agent ? agent.substring(0, 8) : "-"}</TableCell>
                      <TableCell className="text-center">
                        <span className={`text-xs font-medium ${status >= 400 ? "text-destructive" : status >= 300 ? "text-warning" : "text-success"}`}>{status}</span>
                      </TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground">{formatBytes(size)}</TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground">{latency}ms</TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        </DataState>
      </Card>
    </PageContainer>
  );
}

