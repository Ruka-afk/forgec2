"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { formatTime } from "@/lib/utils";
import { PageHeader } from "@/components/UI";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
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

export default function TrafficPage() {
  const [entries, setEntries] = useState<TrafficEntry[]>([]);
  const [filteredEntries, setFilteredEntries] = useState<TrafficEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoScroll, setAutoScroll] = useState(false);
  const [sourceIpFilter, setSourceIpFilter] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);
  const { t } = useI18n();

  const applyFilter = (data: TrafficEntry[], ip: string) => {
    if (!ip) return data;
    return data.filter(e => {
      const entryIp = e.source_ip || "";
      return entryIp.includes(ip);
    });
  };

  const loadTraffic = useCallback(async () => {
    try {
      const data = await api.get("/traffic");
      const arr = Array.isArray(data)
        ? data
        : (data?.data ?? data?.traffic ?? []);
      setEntries(Array.isArray(arr) ? (arr as TrafficEntry[]) : []);
    } catch {
      setEntries([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => { loadTraffic(); }, [loadTraffic]);
  useVisibleInterval(loadTraffic, autoRefresh ? 5000 : 0);

  const applyFilterToEntries = useCallback((ip: string) => {
    setFilteredEntries(applyFilter(entries, ip));
  }, [entries]);

  useEffect(() => {
    if (sourceIpFilter) {
      applyFilterToEntries(sourceIpFilter);
    } else {
      setFilteredEntries(entries);
    }
  }, [sourceIpFilter, entries, applyFilterToEntries]);

  useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [filteredEntries, autoScroll]);

  const clearLog = () => { setEntries([]); setFilteredEntries([]); };

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
    if (m === "POST") return "bg-emerald-500 text-white";
    if (m === "GET") return "bg-blue-500 text-white";
    if (m === "BEACON") return "bg-purple-500 text-white";
    if (m === "PUT") return "bg-amber-500 text-white";
    if (m === "DELETE") return "bg-red-500 text-white";
    return "bg-muted text-muted-foreground";
  };

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("traffic.title")} subtitle={`${t("traffic.request_log")} · C2 Beacon ${t("traffic.comm_record")}`}>
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
          <Button onClick={loadTraffic} className="h-11">
            <RefreshCw className="w-4 h-4" />
            <span>{t("traffic.refresh")}</span>
          </Button>
          <Button onClick={clearLog} className="bg-destructive hover:bg-destructive/90 text-destructive-foreground h-11">
            <Trash2 className="w-4 h-4" />
            <span>{t("traffic.clear")}</span>
          </Button>
        </div>
      </PageHeader>

      <Card className="p-4 mb-4">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-5">
          <div className="text-center">
            <div className="text-xs text-muted-foreground mb-1">{t("traffic.total_requests")}</div>
            <div className="text-2xl font-bold text-indigo-600 dark:text-indigo-400">{totalRequests}</div>
          </div>
          <div className="text-center">
            <div className="text-xs text-muted-foreground mb-1">Beacons</div>
            <div className="text-2xl font-bold text-purple-600 dark:text-purple-400">{beacons}</div>
          </div>
          <div className="text-center">
            <div className="text-xs text-muted-foreground mb-1">{t("traffic.errors")}</div>
            <div className="text-2xl font-bold text-red-600 dark:text-red-400">{errors}</div>
          </div>
          <div className="text-center">
            <div className="text-xs text-muted-foreground mb-1">{t("traffic.data_transfer")}</div>
            <div className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">{formatBytes(dataTransferred)}</div>
          </div>
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

        <div ref={containerRef} className="max-h-[500px] overflow-y-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted border-b border-border sticky top-0">
                <TableHead className="text-xs font-medium min-w-[80px]">{t("traffic.time")}</TableHead>
                <TableHead className="text-xs font-medium min-w-[80px]">Method</TableHead>
                <TableHead className="text-xs font-medium min-w-[200px]">Path</TableHead>
                <TableHead className="text-xs font-medium min-w-[120px]">Source IP</TableHead>
                <TableHead className="text-xs font-medium min-w-[100px]">Agent</TableHead>
                <TableHead className="text-xs font-medium min-w-[60px] text-center">{t("traffic.status")}</TableHead>
                <TableHead className="text-xs font-medium min-w-[60px] text-right">Size</TableHead>
                <TableHead className="text-xs font-medium min-w-[70px] text-right">Latency</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="font-mono">
              {loading ? (
                [1, 2, 3, 4, 5].map(i => (
                  <TableRow key={i}>
                    {[1, 2, 3, 4, 5, 6, 7, 8].map(j => (
                      <TableCell key={j}><Skeleton className="h-3 w-16" /></TableCell>
                    ))}
                  </TableRow>
                ))
              ) : filteredEntries.length > 0 ? (
                filteredEntries.map((e, i) => {
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
                        <span className={`text-xs font-medium ${status >= 400 ? "text-red-500 dark:text-red-400" : status >= 300 ? "text-amber-500 dark:text-amber-400" : "text-emerald-600 dark:text-emerald-400"}`}>{status}</span>
                      </TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground">{formatBytes(size)}</TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground">{latency}ms</TableCell>
                    </TableRow>
                  );
                })
              ) : (
                <TableRow>
                  <TableCell colSpan={8} className="py-16 text-center text-muted-foreground">
                    <Network className="w-8 h-8 mb-2 text-muted-foreground/70" />
                    <p>{t("traffic.empty")}</p>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </Card>
    </div>
  );
}

