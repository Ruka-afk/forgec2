"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";

import { Spinner, PageSpinner } from "@/components/ui/spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import { PageContainer } from "@/components/ui/page-container";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Activity, Wand2 } from "lucide-react";
import { formatTime } from "@/lib/utils";

interface BeaconRecord {
  time: string;
  method: string;
  path: string;
  status: number;
  size: number;
}

interface AdaptationSuggestion {
  desired_interval: number;
  desired_jitter: number;
  pad_size: number;
  reason: string;
  confidence: string;
}

interface BaselineReport {
  agent_id: string;
  sample_count: number;
  baseline_interval: number;
  baseline_jitter: number;
  baseline_packet_size: number;
  mean_interval: number;
  stddev_interval: number;
  mean_packet_size: number;
  cv: number;
  auto_adapt: boolean;
  recent_records: BeaconRecord[];
  suggestion: AdaptationSuggestion | null;
}

export default function AgentTrafficPage() {
  const { t } = useI18n();
  const params = useParams();
  const id = params?.id as string;
  const [report, setReport] = useState<BaselineReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [adapting, setAdapting] = useState(false);

  const [autoAdapt, setAutoAdapt] = useState(false);

  const loadReport = useCallback(async () => {
    if (!id) return;
    try {
      const data = await api.get(paths.agents.trafficProfile(id));
      setReport((data.data as BaselineReport) || null);
      setLoadError(false);
      if (data.data) {
        setAutoAdapt((data.data as BaselineReport).auto_adapt);
      }
    } catch {
      setLoadError(true);
      toast.error(t("agents.traffic_load_failed"));
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  useEffect(() => { loadReport(); }, [loadReport]);

  const handleAdapt = async () => {
    if (!id) return;
    setAdapting(true);
    try {
      const data = await api.postJson(paths.agents.trafficAdapt(id), {});
      if (data.success) {
        toast.success(t("agents.traffic_adapt_queued").replace("{message}", String(data.message || "")));
        loadReport();
      } else {
        toast.error(t("agents.traffic_adapt_error").replace("{error}", String(data.error || t("agents.unknown"))));
      }
    } catch {
      toast.error(t("agents.traffic_adapt_failed"));
    } finally {
      setAdapting(false);
    }
  };

  const toggleAutoAdapt = async () => {
    if (!id) return;
    const next = !autoAdapt;
    try {
      const data = await api.postJson(paths.agents.trafficAutoAdapt(id), { enabled: next });
      if (data.success) {
        setAutoAdapt(next);
        toast(next ? t("agents.traffic_auto_adapt_enabled") : t("agents.traffic_auto_adapt_disabled"));
      }
    } catch {
      toast.error(t("agents.traffic_adapt_failed"));
    }
  };

  if (loading) {
    return <PageContainer><PageSpinner /></PageContainer>;
  }

  if (loadError) {
    return (
      <PageContainer className="space-y-6">
        <Card className="p-(--card-spacing)">
          <ErrorState
            title={t("agents.traffic_load_failed")}
            message={t("agents.traffic_load_failed_hint")}
            action={<Button variant="outline" size="sm" onClick={() => { setLoading(true); setLoadError(false); loadReport(); }}>{t("agents.detail_retry")}</Button>}
            className="mx-auto max-w-md"
          />
        </Card>
      </PageContainer>
    );
  }

  return (
    <PageContainer className="space-y-6">
      <Card className="p-(--card-spacing)">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-semibold text-foreground">{t("agents.traffic_title")}</h2>
          <div className="flex items-center gap-3">
            <Label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
              <Checkbox
                checked={autoAdapt}
                onCheckedChange={() => toggleAutoAdapt()}
              />
              {t("agents.traffic_auto_adapt")}
            </Label>
            <Button
              onClick={handleAdapt}
              disabled={adapting || !report || !report.suggestion}
              className="h-9 text-xs"
            >
              {adapting ? (
                <Spinner size="xs" color="white" />
              ) : (
                <Wand2 className="w-4 h-4" />
              )}
              {t("agents.traffic_adapt")}
            </Button>
          </div>
        </div>

        {!report ? (
          <EmptyState icon={Activity} title={t("agents.traffic_no_data")} />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 sm:gap-5 mb-6">
              <Card>
                <CardContent className="p-4">
                  <div className="text-xs text-muted-foreground mb-1">{t("agents.traffic_samples")}</div>
                  <div className="text-2xl font-bold text-foreground">{report.sample_count}</div>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="p-4">
                  <div className="text-xs text-muted-foreground mb-1">{t("agents.traffic_baseline_interval")}</div>
                  <div className="text-2xl font-bold text-foreground">{report.baseline_interval || "—"}s</div>
                  <div className="text-xs text-muted-foreground mt-1">{t("agents.traffic_mean", { value: report.mean_interval ? report.mean_interval.toFixed(1) : "—" })}</div>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="p-4">
                  <div className="text-xs text-muted-foreground mb-1">{t("agents.traffic_baseline_jitter")}</div>
                  <div className="text-2xl font-bold text-foreground">{report.baseline_jitter || "—"}%</div>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="p-4">
                  <div className="text-xs text-muted-foreground mb-1">{t("agents.traffic_baseline_packet")}</div>
                  <div className="text-2xl font-bold text-foreground">{report.mean_packet_size ? Math.round(report.mean_packet_size).toLocaleString() : "—"}b</div>
                  <div className="text-xs text-muted-foreground mt-1">{t("agents.traffic_baseline_val", { value: report.baseline_packet_size ? report.baseline_packet_size.toLocaleString() : "—" })}</div>
                </CardContent>
              </Card>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-6">
              <Card>
                <CardContent className="p-4">
                  <div className="text-xs text-muted-foreground mb-2">{t("agents.traffic_timing_regularity")}</div>
                  <div className="flex items-center gap-2">
                    <div className="text-2xl font-bold text-foreground">{report.cv ? report.cv.toFixed(3) : "—"}</div>
                    <Badge variant={report.cv < 0.15 ? "destructive" : report.cv < 0.3 ? "secondary" : "success"} className="px-2 py-0.5 rounded-full text-xs font-medium">
                      {report.cv < 0.15 ? t("agents.traffic_too_regular") : report.cv < 0.3 ? t("agents.traffic_moderate") : t("agents.traffic_natural")}
                    </Badge>
                  </div>
                  <div className="text-xs text-muted-foreground mt-1">{t("agents.traffic_stddev", { value: report.stddev_interval ? report.stddev_interval.toFixed(2) : "—" })}</div>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="p-4">
                  <div className="text-xs text-muted-foreground mb-2">{t("agents.traffic_current_suggestion")}</div>
                  {report.suggestion ? (
                    <div>
                      <div className="text-sm font-medium text-foreground">{report.suggestion.reason}</div>
                      <div className="flex items-center gap-2 mt-1">
                        <Badge variant={report.suggestion.confidence === "high" ? "success" : "secondary"} className="px-2 py-0.5 rounded-full text-xs font-medium">
                          {report.suggestion.confidence}
                        </Badge>
                        {report.suggestion.desired_interval > 0 && (
                          <span className="text-xs text-muted-foreground">{t("agents.traffic_suggest_interval", { value: report.suggestion.desired_interval })}</span>
                        )}
                        {report.suggestion.desired_jitter > 0 && (
                          <span className="text-xs text-muted-foreground">{t("agents.traffic_suggest_jitter", { value: report.suggestion.desired_jitter })}</span>
                        )}
                        {report.suggestion.pad_size > 0 && (
                          <span className="text-xs text-muted-foreground">{t("agents.traffic_suggest_pad", { value: report.suggestion.pad_size })}</span>
                        )}
                      </div>
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">{t("agents.traffic_collecting_baseline")}</div>
                  )}
                </CardContent>
              </Card>
            </div>

            {report.recent_records && report.recent_records.length > 0 && (
              <div>
                <div className="text-sm font-semibold text-foreground mb-3">{t("agents.traffic_beacon_timeline", { count: report.recent_records.length })}</div>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="text-left font-medium">{t("agents.traffic_col_time")}</TableHead>
                      <TableHead className="text-right font-medium">{t("agents.traffic_col_size")}</TableHead>
                      <TableHead className="text-right font-medium">{t("agents.traffic_col_interval")}</TableHead>
                      <TableHead className="text-center font-medium">{t("agents.traffic_col_transport")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {report.recent_records.map((rec, i) => {
                      const prev = i > 0 ? report.recent_records[i - 1] : null;
                      const interval = prev
                        ? ((new Date(rec.time).getTime() - new Date(prev.time).getTime()) / 1000).toFixed(1)
                        : "—";
                      return (
                        <TableRow key={i}>
                          <TableCell className="text-foreground">{formatTime(rec.time)}</TableCell>
                          <TableCell className="text-right text-foreground">{rec.size.toLocaleString()}b</TableCell>
                          <TableCell className="text-right text-foreground">{interval}s</TableCell>
                          <TableCell className="text-center text-muted-foreground">{rec.method} {rec.path}</TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
            )}
          </>
        )}
      </Card>


    </PageContainer>
  );
}
