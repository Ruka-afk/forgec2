"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { API_BASE } from "@/lib/constants";
import { downloadFromResponse } from "@/lib/download";
import { api, getCsrfToken } from "@/lib/api";
import { toast } from "sonner";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { Spinner, PageHeader } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Progress } from "@/components/ui/progress";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Activity, AlertCircle, AlertTriangle, Apple, Calendar, CheckCircle, Clock, Cpu, Download, File, Filter, Hammer, Monitor, Plus, RefreshCw, Terminal, Timer, User, X, XCircle } from "lucide-react";

interface BuildLog {
  id?: string;
  ID?: string;
  created_at?: string;
  CreatedAt?: string;
  platform?: string;
  Platform?: string;
  format?: string;
  Format?: string;
  c2_url?: string;
  C2URL?: string;
  filename?: string;
  Filename?: string;
  user?: string;
  User?: string;
  status?: string;
  Status?: string;
  error?: string;
  Error?: string;
  stdout?: string;
  Stdout?: string;
  stderr?: string;
  Stderr?: string;
  started_at?: string;
  StartedAt?: string;
  completed_at?: string;
  CompletedAt?: string;
  duration?: string;
  Duration?: string;
  artifact_path?: string;
  ArtifactPath?: string;
}

const PLATFORMS = ["all", "windows", "linux", "macos"];

function getDuration(start?: string, end?: string): string {
  if (!start) return "-";
  try {
    const s = new Date(start).getTime();
    const e = end ? new Date(end).getTime() : Date.now();
    const diff = Math.max(0, e - s);
    const min = Math.floor(diff / 60000);
    const sec = Math.floor((diff % 60000) / 1000);
    if (min > 0) return `${min}m ${sec}s`;
    return `${sec}s`;
  } catch { return "-"; }
}

function getStatusInfo(status: string) {
  switch (status) {
    case "success": return { icon: <CheckCircle className="w-4 h-4 text-primary" />, label: "Success", bg: "bg-emerald-500" };
    case "failed": return { icon: <XCircle className="w-4 h-4 text-destructive" />, label: "Failed", bg: "bg-destructive" };
    case "building": return { icon: <Spinner size="sm" />, label: "Building", bg: "bg-primary" };
    default: return { icon: <Clock className="w-4 h-4 text-muted-foreground" />, label: "Pending", bg: "bg-muted-foreground" };
  }
}

export default function BuildsPage() {
  const [builds, setBuilds] = useState<BuildLog[]>([]);
  const [total, setTotal] = useState(0);
  const [successCount, setSuccessCount] = useState(0);
  const [failedCount, setFailedCount] = useState(0);
  const [avgDuration, setAvgDuration] = useState(0);
  const [loading, setLoading] = useState(true);
  const [filterPlatform, setFilterPlatform] = useState("all");
  const [filterStatus, setFilterStatus] = useState("");
  const [expandedBuild, setExpandedBuild] = useState<string | null>(null);
  const [versionDist, setVersionDist] = useState<{ version: string; count: number }[]>([]);
  const { t } = useI18n();

  const loadBuilds = useCallback(async () => {
    try {
      const params = new URLSearchParams();
      if (filterPlatform !== "all") params.set("platform", filterPlatform);
      if (filterStatus) params.set("status", filterStatus);
      const [data, agents] = await Promise.all([
        api.get(`/builds?${params}`),
        api.get("/api/agents").catch(() => null),
      ]);
      const logs: BuildLog[] = (data.logs || []) as BuildLog[];
      setBuilds(logs);
      setTotal((data.total ?? logs.length) as number);
      const success = (data.success_count ?? logs.filter((l: BuildLog) => (l.status) === "success").length) as number;
      const failed = (data.failed_count ?? logs.filter((l: BuildLog) => (l.status) === "failed").length) as number;
      setSuccessCount(success);
      setFailedCount(failed);
      const durations = logs.filter((l: BuildLog) => l.duration).map((l: BuildLog) => {
        const d = (l.duration || "0").replace("s", "");
        return parseFloat(d) || 0;
      });
      setAvgDuration(durations.length > 0 ? Math.round(durations.reduce((a: number, b: number) => a + b, 0) / durations.length) : 0);
      if (agents) {
        const list = (Array.isArray(agents) ? agents : agents.agents || []) as { version?: string; Version?: string }[];
        const counts: Record<string, number> = {};
        list.forEach(a => { const v = a.version || a.Version || "unknown"; counts[v] = (counts[v] || 0) + 1; });
        setVersionDist(Object.entries(counts).map(([version, count]) => ({ version, count })).sort((a, b) => b.count - a.count));
      }
    } catch {
      setBuilds([]);
      setTotal(0);
      setSuccessCount(0);
      setFailedCount(0);
      setAvgDuration(0);
      toast.error(t("builds.toast.load_failed"));
    }
    setLoading(false);
  }, [filterPlatform, filterStatus, t]);

  useEffect(() => { loadBuilds(); }, [loadBuilds]);
  useVisibleInterval(loadBuilds, 5000);

  const handleDownload = async (build: BuildLog) => {
    const buildId = build.id;
    if (!buildId) return;
    try {
      const resp = await fetch(`${API_BASE}/builds/${buildId}/download`, { headers: { "X-CSRF-Token": getCsrfToken() } });
      if (!resp.ok) {
        toast.error(t("builds.toast.download_failed"));
        return;
      }
      await downloadFromResponse(resp, build.filename || `build-${buildId}`);
    } catch {
      toast.error(t("builds.toast.download_failed"));
    }
  };

  const platformIcon = (p: string) => {
    switch (p) {
      case "windows": return <Monitor className="w-3 h-3 text-muted-foreground" />;
      case "linux": return <Terminal className="w-3 h-3 text-muted-foreground" />;
      case "macos": return <Apple className="w-3 h-3 text-muted-foreground" />;
      default: return <Cpu className="w-3 h-3 text-muted-foreground" />;
    }
  };

  const platformColor = (p: string) => {
    switch (p) {
      case "linux": return "success";
      case "windows": return "default";
      case "macos": return "secondary";
      default: return "outline";
    }
  };

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
        <PageHeader title={t("builds.title")} subtitle={`${t("builds.subtitle")} · ${successCount} ${t("builds.success")} · ${failedCount} ${t("builds.failed")} · Avg ${avgDuration}s`} />
        <div className="flex items-center gap-2">
          <Button render={<Link href="/generate" />}>
            <Plus className="w-4 h-4" /> {t("builds.new_build")}
          </Button>
          <Button variant="secondary" onClick={loadBuilds}>
            <RefreshCw className="w-4 h-4" /> {t("builds.refresh")}
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-4 gap-4 mb-4">
        <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("builds.total_builds")}</div>
              <div className="text-2xl font-bold tabular-nums text-primary mt-2">{total}</div>
            </div>
            <div className="w-12 h-12 bg-indigo-50 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center">
              <Hammer className="w-4 h-4" />
            </div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("builds.success")}</div>
              <div className="text-2xl font-bold tabular-nums mt-2 text-primary">{successCount}</div>
            </div>
            <div className="w-12 h-12 bg-emerald-50 dark:bg-emerald-900/30 rounded-xl flex items-center justify-center">
              <CheckCircle className="w-4 h-4" />
            </div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("builds.failed")}</div>
              <div className="text-2xl font-bold tabular-nums mt-2 text-destructive">{failedCount}</div>
            </div>
            <div className="w-12 h-12 bg-destructive/10 rounded-xl flex items-center justify-center">
              <AlertCircle className="w-4 h-4" />
            </div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("builds.success_rate")}</div>
              <div className="text-2xl font-bold tabular-nums mt-2 text-amber-600 dark:text-amber-400">{total > 0 ? Math.round((successCount / total) * 100) : 0}%</div>
            </div>
            <div className="w-12 h-12 bg-amber-50 dark:bg-amber-900/30 rounded-xl flex items-center justify-center">
              <Activity className="w-4 h-4" />
            </div>
          </div>
        </Card>
      </div>

      {versionDist.length > 0 && (
        <Card className="p-4 sm:p-5 mb-4">
          <div className="flex items-center justify-between mb-3">
            <span className="text-sm font-semibold text-foreground flex items-center gap-2">
              <Cpu className="w-4 h-4" /> {t("builds.version_dist")}
            </span>
            <span className="text-xs text-muted-foreground">{versionDist.reduce((s, v) => s + v.count, 0)} {t("builds.agents_unit")}</span>
          </div>
          <div className="space-y-2">
            {versionDist.map(v => {
              const pct = Math.round((v.count / versionDist.reduce((s, x) => s + x.count, 0)) * 100);
              return (
                <div key={v.version} className="flex items-center gap-3">
                   <Tooltip>
                     <TooltipTrigger>
                       <span className="text-xs font-mono text-foreground w-32 truncate">{v.version}</span>
                     </TooltipTrigger>
                     <TooltipContent>{v.version}</TooltipContent>
                   </Tooltip>
                  <Progress value={pct} className="h-4 flex-1" />
                  <span className="text-xs text-muted-foreground w-16 text-right">{v.count} ({pct}%)</span>
                </div>
              );
            })}
          </div>
        </Card>
      )}

      <Card className="p-4 sm:p-5 mb-4">
        <div className="flex flex-wrap items-center gap-3">
          <Filter className="w-4 h-4" />
          <span className="text-sm font-semibold text-muted-foreground">{t("builds.platform")}</span>
          <div className="flex flex-wrap gap-2">
            {PLATFORMS.map((p) => (
              <Button key={p} variant={filterPlatform === p ? "default" : "outline"} size="sm"
                onClick={() => setFilterPlatform(p)}
                className="rounded-xl gap-1.5">
                {platformIcon(p)}
                {p === "all" ? t("builds.filter_all") : p.charAt(0).toUpperCase() + p.slice(1)}
              </Button>
            ))}
          </div>
          <span className="text-sm font-semibold text-muted-foreground ml-2">{t("builds.status")}</span>
          <Select value={filterStatus} onValueChange={(v) => setFilterStatus(v ?? "")}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="All" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("builds.filter_all")}</SelectItem>
              <SelectItem value="success">{t("builds.success")}</SelectItem>
              <SelectItem value="failed">{t("builds.failed")}</SelectItem>
              <SelectItem value="building">{t("builds.filter_building")}</SelectItem>
            </SelectContent>
          </Select>
          {(filterStatus || filterPlatform !== "all") && (
            <Button variant="outline" size="sm" onClick={() => { setFilterStatus(""); setFilterPlatform("all"); }}
              className="rounded-xl">
              <X className="w-4 h-4" /> {t("builds.clear")}
            </Button>
          )}
        </div>
      </Card>

      <div className="space-y-3">
        {loading ? (
          [1, 2, 3].map((i) => (
            <Card key={i} className="p-4 sm:p-5">
              <div className="flex items-center gap-4">
                <Skeleton className="w-10 h-10 rounded-xl" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-3 w-40" />
                  <Skeleton className="h-2 w-60" />
                </div>
              </div>
            </Card>
          ))
        ) : builds.length === 0 ? (
          <Card className="p-12 text-center">
            <Hammer className="w-4 h-4" />
            <p className="text-sm font-medium text-muted-foreground">{t("builds.empty")}</p>
            <p className="text-xs mt-2 text-muted-foreground">
              <Link href="/generate" className="text-primary hover:underline">{t("builds.go_generate")}</Link> {t("builds.generate_new_implant")}
            </p>
          </Card>
        ) : (
          builds.map((build, idx) => {
            const id = build.id || String(idx);
            const status = build.status || "unknown";
            const platform = build.platform || "";
            const info = getStatusInfo(status);
            const startedAt = build.started_at || build.created_at || "";
            const completedAt = build.completed_at || "";
            const stderr = build.stderr || "";
            const stdout = build.stdout || "";
            const error = build.error || "";
            const isExpanded = expandedBuild === id;
            const isBuilding = status === "building";

            return (
              <Card key={id} className="overflow-hidden">
                <div className="p-4 sm:p-5">
                  <div className="flex items-center gap-4">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${status === "success" ? "bg-emerald-500/10" : status === "failed" ? "bg-destructive/10" : status === "building" ? "bg-primary/10" : "bg-secondary"}`}>
                      {isBuilding ? <Spinner size="xs" /> : info.icon}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-semibold text-foreground">#{String(id).substring(0, 8)}</span>
                        <Badge variant={platformColor(platform) as "success" | "outline"} className="px-2 py-0.5 text-(--font-size-micro-sm) rounded-lg">
                          {platformIcon(platform)} {platform || "unknown"}
                        </Badge>
                        <Badge variant="outline" className="px-2 py-0.5 text-(--font-size-micro-sm) rounded-lg">{build.format || "-"}</Badge>
                        <Badge variant={status === "success" ? "success" : status === "failed" ? "destructive" : "outline"}
                          className="px-2 py-0.5 text-(--font-size-micro-sm) rounded-full">
                          <span className={`w-1.5 h-1.5 rounded-full inline-block mr-1 ${info.bg} ${isBuilding ? "animate-pulse" : ""}`}></span>
                          {info.label}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-4 mt-1.5 text-xs text-muted-foreground">
                        <span><Calendar className="w-4 h-4" />{formatTime(startedAt)}</span>
                        <span><Timer className="w-4 h-4" />{getDuration(startedAt, completedAt || (isBuilding ? undefined : completedAt))}</span>
                        {build.user ? <span><User className="w-4 h-4" />{build.user}</span> : null}
                        {build.filename ? <span className="truncate max-w-[200px]"><File className="w-4 h-4" />{build.filename}</span> : null}
                      </div>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <Tooltip>
                        <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={() => setExpandedBuild(isExpanded ? null : id)}
                            aria-label="Toggle logs" />}>
                          <Terminal className="w-4 h-4" />
                        </TooltipTrigger>
                        <TooltipContent>Logs</TooltipContent>
                      </Tooltip>
                      {(status === "success" || build.artifact_path) && (
                        <Tooltip>
                          <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={() => handleDownload(build)}
                              className="text-primary hover:bg-indigo-50 dark:hover:bg-indigo-900/30"
                              aria-label="Download artifact" />}>
                            <Download className="w-4 h-4" />
                          </TooltipTrigger>
                          <TooltipContent>Download Artifact</TooltipContent>
                        </Tooltip>
                      )}
                    </div>
                  </div>
                </div>

                {isExpanded && (
                  <div className="border-t border-border">
                    {(status === "failed" || error || stderr) && (
                      <div className="p-4 bg-destructive/10 border-b border-destructive/20">
                        <div className="flex items-center gap-2 mb-2">
                          <AlertTriangle className="w-4 h-4" />
                          <span className="text-xs font-semibold text-destructive">{t("builds.error")}</span>
                        </div>
                        <pre className="text-xs font-mono text-destructive bg-destructive/10 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap break-all">{error || stderr || t("builds.unknown_error")}</pre>
                      </div>
                    )}
                    <div className="bg-card p-4 max-h-[400px] overflow-y-auto">
                      <div className="flex items-center justify-between mb-3">
                        <span className="text-xs font-semibold text-muted-foreground flex items-center gap-1.5">
                          <Terminal className="w-4 h-4" /> {t("builds.log_title")}
                        </span>
                        <span className="text-(--font-size-micro-sm) text-muted-foreground font-mono">{stdout.split("\n").length} {t("builds.lines")}</span>
                      </div>
                      <pre className="text-xs font-mono text-muted-foreground whitespace-pre-wrap leading-relaxed">
                        {stdout || <span className="text-muted-foreground italic">{t("builds.no_output")}</span>}
                      </pre>
                    </div>
                    {build.c2_url ? (
                      <div className="px-4 py-3 bg-muted border-t border-border flex items-center justify-between">
                        <span className="text-xs text-muted-foreground font-mono truncate">{build.c2_url}</span>
                      </div>
                    ) : null}
                  </div>
                )}
              </Card>
            );
          })
        )}
      </div>
    </div>
  );
}

