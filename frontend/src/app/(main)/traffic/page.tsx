"use client";

import { useEffect, useMemo, useState, useRef } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { formatTime, formatBytes } from "@/lib/utils";
import { PageContainer } from "@/components/ui/page-container";
import { CardHeaderRow } from "@/components/ui/card-header-row";
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
  time?: string;
  method?: string;
  Method?: string;
  path?: string;
  Path?: string;
  source_ip?: string;
  SourceIP?: string;
  remote_ip?: string;
  agent_id?: string;
  AgentID?: string;
  status_code?: number;
  StatusCode?: number;
  status?: number;
  size?: number;
  Size?: number;
  latency?: number | string;
  Latency?: number | string;
  protocol?: string;
  Protocol?: string;
}

const EMPTY_TRAFFIC: TrafficEntry[] = [];

function normalizeTrafficEntry(raw: TrafficEntry): TrafficEntry {
  // Backend: {time, remote_ip, status, latency:"12ms", size}  Front expects {timestamp, source_ip, status_code, latency:number}
  const e = raw as Record<string, unknown>;
  return {
    id: String(e.id || e.ID || e.time || e.timestamp || ""),
    timestamp: String(e.timestamp || e.Timestamp || e.time || ""),
    method: String(e.method || e.Method || ""),
    path: String(e.path || e.Path || ""),
    source_ip: String(e.source_ip || e.SourceIP || e.remote_ip || ""),
    agent_id: String(e.agent_id || e.AgentID || ""),
    status_code: Number(e.status_code ?? e.StatusCode ?? e.status ?? 0),
    size: Number(e.size ?? e.Size ?? 0),
    latency: typeof e.latency === "string" ? parseInt(String(e.latency), 10) || 0 : Number(e.latency ?? e.Latency ?? 0),
    protocol: String(e.protocol || e.Protocol || ""),
  };
}

export default function TrafficPage() {
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoScroll, setAutoScroll] = useState(false);
  const [sourceIpFilter, setSourceIpFilter] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);
  const { t } = useI18n();

  const { data, loading, error, refresh: loadTraffic, setData } = useApiResource<TrafficEntry[]>({
    fetcher: async () => {
      const data = await api.get(paths.traffic.api);
      const arr = Array.isArray(data) ? data : (data?.data ?? data?.traffic ?? []);
      const list = Array.isArray(arr) ? (arr as TrafficEntry[]) : [];
      return list.map(normalizeTrafficEntry);
    },
    pollMs: autoRefresh ? 15000 : 0,
    toastThrottleMs: 10000,
    errorMessage: t("traffic.toast.load_failed"),
  });
  const entries = data ?? EMPTY_TRAFFIC;

  const { filteredEntries, sourceIps, totalRequests, beacons, errors, dataTransferred } = useMemo(() => {
    const filtered = sourceIpFilter ? entries.filter((e) => (e.source_ip || "").includes(sourceIpFilter)) : entries;
    const ips = [...new Set(entries.map((e) => e.source_ip || "").filter(Boolean))];
    const total = entries.length;
    const b = entries.filter((e) => (e.protocol || "").toLowerCase().includes("beacon")).length;
    const err = entries.filter((e) => (e.status_code ?? 0) >= 400).length;
    const bytes = entries.reduce((acc, e) => acc + (e.size ?? 0), 0);
    return { filteredEntries: filtered, sourceIps: ips, totalRequests: total, beacons: b, errors: err, dataTransferred: bytes };
  }, [entries, sourceIpFilter]);

  const isUserScrolledRef = useRef(false);
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const onScroll = () => {
      isUserScrolledRef.current = el.scrollTop + el.clientHeight < el.scrollHeight - 20;
    };
    el.addEventListener("scroll", onScroll);
    return () => el.removeEventListener("scroll", onScroll);
  }, []);
  useEffect(() => {
    if (autoScroll && !isUserScrolledRef.current && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [filteredEntries, autoScroll]);

  // View-only clear: the server keeps no deletable log, so pause
  // auto-refresh as well — otherwise the next poll silently repopulates.
  const clearLog = () => { setData([]); setAutoRefresh(false); };

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
    <PageContainer title={t("traffic.title")} subtitle={`${t("traffic.request_log")} · C2 Beacon ${t("traffic.comm_record")}`} actions={<>
        <div className="flex items-center gap-2 flex-wrap">
          <Label className="flex items-center gap-x-2 text-sm text-muted-foreground cursor-pointer">
            <Checkbox checked={autoRefresh} onCheckedChange={setAutoRefresh} />
             {t("traffic.auto_refresh")}
          </Label>
          <Label className="flex items-center gap-x-2 text-sm text-muted-foreground cursor-pointer">
            <Checkbox checked={autoScroll} onCheckedChange={setAutoScroll} />
            {t("traffic.auto_scroll")}
          </Label>
          <Select value={sourceIpFilter || "__all"} onValueChange={v => setSourceIpFilter(v === "__all" ? "" : v ?? "")}>
            <SelectTrigger className="w-full"><SelectValue placeholder={t("traffic.all_ip")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all">{t("traffic.all_ip")}</SelectItem>
              {sourceIps.map(ip => (
                <SelectItem key={ip} value={ip}>{ip}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button onClick={loadTraffic} size="lg" className="">
            <RefreshCw className="size-4" />
            <span>{t("traffic.refresh")}</span>
          </Button>
          <Button onClick={clearLog} size="lg" className="bg-destructive hover:bg-destructive/90 text-destructive-foreground">
            <Trash2 className="size-4" />
            <span>{t("traffic.clear")}</span>
          </Button>
        </div>
      </>}>

      <Card className="p-4 shadow-sm hover:shadow-md transition-shadow duration-200">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-5">
          <StatTile centered label={t("traffic.total_requests")} value={totalRequests} tone="primary" />
          <StatTile centered label={t("traffic.beacons")} value={beacons} tone="violet" />
          <StatTile centered label={t("traffic.errors")} value={errors} tone="destructive" />
          <StatTile centered label={t("traffic.data_transfer")} value={formatBytes(dataTransferred)} tone="success" />
        </div>
      </Card>

      <Card className="overflow-hidden">
<CardHeaderRow accent={false} icon={Network} title={t("traffic.beacon_comm")} action={<span className="text-xs text-muted-foreground">{t("traffic.showing")} {filteredEntries.length} / {loading ? "..." : entries.length} {t("traffic.records")}</span>} />

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
                  const id = e.id || `${e.timestamp || ""}-${e.agent_id || ""}-${i}`;
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

