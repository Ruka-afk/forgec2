"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { EmptyState, PageHeader, PageSpinner } from "@/components/UI";
import { toast } from "sonner";
import { formatTime } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Activity, AlertCircle, AlertTriangle, ArrowLeftRight, ArrowRight, Gauge, History, MoreHorizontal, Radio, RotateCw, ShieldCheck, SlidersHorizontal } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { useI18n } from "@/lib/i18n";

interface ListenerDetail {
  target: string;
  scheme: string;
  host: string;
  port: number;
  status: string;
  consecutive_fails: number;
  last_probe: string;
  fail_reasons: string[];
}

interface BreakerConfig {
  failure_threshold: number;
  cooldown_seconds: number;
  half_open_max_reqs: number;
  health_check_seconds: number;
}

interface BreakerEvent {
  id: number;
  listener_id: string;
  old_state: string;
  new_state: string;
  reason: string;
  created_at: string;
}

const stateConfig: Record<string, { dot: string; bg: string; text: string; label: string }> = {
  healthy: { dot: "bg-emerald-500", bg: "bg-emerald-500/10 border-emerald-500/20", text: "text-emerald-700 dark:text-emerald-400", label: "Closed" },
  unstable: { dot: "bg-amber-500", bg: "bg-amber-500/10 border-amber-500/20", text: "text-amber-700 dark:text-amber-400", label: "Half-Open" },
  burned: { dot: "bg-destructive", bg: "bg-destructive/10 border-destructive/20", text: "text-destructive", label: "Open" },
  unknown: { dot: "bg-muted-foreground", bg: "bg-muted border-border", text: "text-muted-foreground", label: "Unknown" },
};

export default function CircuitBreakerPage() {
  const { t } = useI18n();
  const [listeners, setListeners] = useState<ListenerDetail[]>([]);
  const [config, setConfig] = useState<BreakerConfig>({
    failure_threshold: 3,
    cooldown_seconds: 300,
    half_open_max_reqs: 3,
    health_check_seconds: 60,
  });
  const [events, setEvents] = useState<BreakerEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [showConfigModal, setShowConfigModal] = useState(false);
  const [configForm, setConfigForm] = useState<BreakerConfig>(config);

  const loadData = useCallback(() => {
    let failed = 0;
    Promise.all([
      api.get<{ listeners?: ListenerDetail[] }>("/circuit-breaker/detail").catch(() => { failed++; return { listeners: [] as ListenerDetail[] }; }),
      api.get<{ failure_threshold?: number; cooldown_seconds?: number; half_open_max_reqs?: number; health_check_seconds?: number }>("/circuit-breaker/config").catch((): Partial<BreakerConfig> => { failed++; return {}; }),
      api.get<{ events?: BreakerEvent[] }>("/circuit-breaker/events").catch(() => { failed++; return { events: [] as BreakerEvent[] }; }),
    ]).then(([detData, cfgData, evtData]) => {
      setListeners((detData.listeners || []) as ListenerDetail[]);
      if ((cfgData as Record<string, unknown>).failure_threshold !== undefined) {
        setConfig(cfgData as BreakerConfig);
        setConfigForm(cfgData as BreakerConfig);
      }
      setEvents((evtData.events || []) as BreakerEvent[]);
      if (failed > 0) toast.error(t("cb.load_failed"));
    }).catch(() => toast.error(t("cb.load_failed"))).finally(() => setLoading(false));
  }, [t]);

  useEffect(() => { loadData(); }, [loadData]);
  useVisibleInterval(loadData, 15000);

  const handleSaveConfig = async () => {
    try {
      await api.postJson(paths.circuitBreaker.config, configForm);
      setConfig(configForm);
      setShowConfigModal(false);
      toast.success(t("cb.config_saved"));
    } catch { toast.error(t("cb.config_save_failed")); }
  };

  const handleReset = async (listenerId: string) => {
    try {
      await api.post(paths.circuitBreaker.reset(listenerId));
      toast.success(t("cb.listener_reset", { id: listenerId }));
      loadData();
    } catch { toast.error(t("cb.reset_failed")); }
  };

  const handleToggle = async (listenerId: string, state: string) => {
    try {
      await api.postJson(paths.circuitBreaker.toggle(listenerId), { state });
      toast.success(t("cb.listener_toggled", { id: listenerId, state }));
      loadData();
    } catch { toast.error(t("cb.toggle_failed")); }
  };

  const healthyCount = listeners.filter(l => l.status === "healthy").length;
  const burnedCount = listeners.filter(l => l.status === "burned").length;
  const unstableCount = listeners.filter(l => l.status === "unstable").length;

  if (loading) return <PageSpinner />;

  return (
    <>

      <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 space-y-6 animate-fade-slide-up">
        <PageHeader title={t("cb.title")} subtitle={t("cb.subtitle")}>
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400">
              <span className="w-2 h-2 bg-emerald-500 rounded-full"></span>
              {healthyCount} {t("cb.closed")}
            </span>
            {unstableCount > 0 && (
              <span className="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400">
                <span className="w-2 h-2 bg-amber-500 rounded-full"></span>
                {unstableCount} {t("cb.half_open")}
              </span>
            )}
            {burnedCount > 0 && (
              <span className="flex items-center gap-1.5 text-xs text-destructive">
                <span className="w-2 h-2 bg-destructive rounded-full animate-pulse"></span>
                {burnedCount} {t("cb.open")}
              </span>
            )}
          </div>
        </PageHeader>

        <Card className="p-3 border-amber-500/40 bg-amber-500/10 text-sm text-amber-800 dark:text-amber-200">
          <div className="font-semibold">{t("cb.honesty_title")}</div>
          <div className="text-xs text-muted-foreground mt-0.5">{t("cb.honesty_desc")}</div>
        </Card>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-5">
          <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
            <div className="flex items-center gap-3 mb-2">
              <div className="w-10 h-10 rounded-xl bg-emerald-100 dark:bg-emerald-900/30 flex items-center justify-center">
                <ShieldCheck className="w-4 h-4" />
              </div>
              <div>
                <div className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">{healthyCount}</div>
                <div className="text-xs text-muted-foreground">{t("cb.closed")}</div>
              </div>
            </div>
            <div className="text-(--fs-micro-sm) text-muted-foreground">{t("cb.closed_desc")}</div>
          </Card>
          <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
            <div className="flex items-center gap-3 mb-2">
              <div className="w-10 h-10 rounded-xl bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center">
                <AlertTriangle className="w-4 h-4" />
              </div>
              <div>
                <div className="text-2xl font-bold text-amber-600 dark:text-amber-400">{unstableCount}</div>
                <div className="text-xs text-muted-foreground">{t("cb.half_open")}</div>
              </div>
            </div>
            <div className="text-(--fs-micro-sm) text-muted-foreground">{t("cb.half_open_desc")}</div>
          </Card>
          <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
            <div className="flex items-center gap-3 mb-2">
              <div className="w-10 h-10 rounded-xl bg-destructive/10 flex items-center justify-center">
                <Radio className="w-4 h-4" />
              </div>
              <div>
                <div className="text-2xl font-bold text-destructive">{burnedCount}</div>
                <div className="text-xs text-muted-foreground">{t("cb.open")}</div>
              </div>
            </div>
            <div className="text-(--fs-micro-sm) text-muted-foreground">{t("cb.open_desc")}</div>
          </Card>
          <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
            <div className="flex items-center gap-3 mb-2">
              <div className="w-10 h-10 rounded-xl bg-primary/10 dark:bg-primary/20 flex items-center justify-center">
                <Gauge className="w-4 h-4" />
              </div>
              <div>
                <div className="text-2xl font-bold text-primary">{config.health_check_seconds}s</div>
                <div className="text-xs text-muted-foreground">{t("cb.check_interval")}</div>
              </div>
            </div>
            <div className="text-(--fs-micro-sm) text-muted-foreground">{t("cb.probes_every", { seconds: config.health_check_seconds })}</div>
          </Card>
        </div>

        <Card>
          <CardContent>
            <div className="flex items-center justify-between mb-4">
              <span className="text-sm font-semibold text-foreground">{t("cb.monitored_listeners")}</span>
              <Button variant="ghost" size="sm" onClick={() => { setConfigForm(config); setShowConfigModal(true); }}>
                <SlidersHorizontal className="w-4 h-4" />{t("cb.config")}
              </Button>
            </div>
            <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs uppercase tracking-wider font-semibold">ID</TableHead>
                <TableHead className="text-xs uppercase tracking-wider font-semibold">{t("cb.col_target")}</TableHead>
                <TableHead className="text-xs uppercase tracking-wider font-semibold">{t("cb.col_status")}</TableHead>
                <TableHead className="text-xs uppercase tracking-wider font-semibold">{t("cb.col_fails")}</TableHead>
                <TableHead className="text-xs uppercase tracking-wider font-semibold">{t("cb.col_last_probe")}</TableHead>
                <TableHead className="text-xs uppercase tracking-wider font-semibold text-right">{t("cb.col_controls")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {listeners.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="px-4 py-10 text-center text-xs text-muted-foreground"><EmptyState icon={Activity} title={t("cb.empty_listeners")} /></TableCell>
                </TableRow>
              ) : listeners.map((l) => {
                const sc = stateConfig[l.status] || stateConfig.unknown;
                const idStr = l.target;
                return (
                  <TableRow key={idStr} className="hover:bg-muted transition-colors">
                    <TableCell className="px-4 py-3">
                      <code className="font-semibold text-foreground">{idStr}</code>
                    </TableCell>
                    <TableCell className="px-4 py-3">
                      <span className="text-muted-foreground">{l.scheme}://{l.host}:{l.port}</span>
                    </TableCell>
                    <TableCell className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={`w-2 h-2 rounded-full ${sc.dot} ${l.status === "burned" ? "animate-pulse" : ""}`}></span>
                        <span className={`text-xs font-medium ${sc.text}`}>{sc.label}</span>
                      </div>
                    </TableCell>
                    <TableCell className="px-4 py-3">
                      <span className={`text-xs font-medium ${l.consecutive_fails >= 3 ? "text-destructive" : l.consecutive_fails > 0 ? "text-amber-600 dark:text-amber-400" : "text-muted-foreground"}`}>
                        {l.consecutive_fails}
                      </span>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-xs text-muted-foreground">
                      {l.last_probe ? formatTime(l.last_probe) : t("cb.never")}
                    </TableCell>
                    <TableCell className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Tooltip>
                          <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={() => handleReset(idStr)} aria-label={t("circuit_breaker.reset_healthy")} />}>
                            <RotateCw className="w-4 h-4" />
                          </TooltipTrigger>
                          <TooltipContent>{t("circuit_breaker.reset_healthy")}</TooltipContent>
                        </Tooltip>
                        <Popover>
                          <PopoverTrigger render={<Button variant="ghost" size="icon-sm" aria-label={t("circuit_breaker.force_state")} />}>
                            <MoreHorizontal className="w-4 h-4" />
                          </PopoverTrigger>
                          <PopoverContent align="end" sideOffset={4} className="w-[120px] p-1">
                            <Button variant="ghost" size="xs" onClick={() => handleToggle(idStr, "closed")} className="w-full justify-start text-xs">
                              <span className="w-2 h-2 bg-emerald-500 rounded-full inline-block mr-2"></span>Close
                            </Button>
                            <Button variant="ghost" size="xs" onClick={() => handleToggle(idStr, "half-open")} className="w-full justify-start text-xs">
                              <span className="w-2 h-2 bg-amber-500 rounded-full inline-block mr-2"></span>Half-Open
                            </Button>
                            <Button variant="ghost" size="xs" onClick={() => handleToggle(idStr, "open")} className="w-full justify-start text-xs">
                              <span className="w-2 h-2 bg-destructive rounded-full inline-block mr-2"></span>Open
                            </Button>
                          </PopoverContent>
                        </Popover>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
          </CardContent>
        </Card>

        {/* Fail Reasons */}
        {listeners.some(l => l.fail_reasons.length > 0) && (
          <Card className="p-4 sm:p-5">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t("cb.recent_failures")}</h3>
            <div className="space-y-2 max-h-48 overflow-y-auto">
              {listeners.filter(l => l.fail_reasons.length > 0).map(l => (
              l.fail_reasons.map((reason, idx) => (
                <div key={`${l.target}-${idx}`} className="flex items-start gap-2 p-3 bg-muted border border-border rounded-xl">
                  <AlertCircle className="w-4 h-4" />
                  <div className="min-w-0">
                    <code className="text-xs font-semibold text-foreground">{l.target}</code>
                    <p className="text-xs text-muted-foreground">{reason}</p>
                  </div>
                </div>
              ))))}
            </div>
          </Card>
        )}

        <Card className="p-4 sm:p-5">
          <h3 className="text-sm font-semibold text-foreground mb-3">
            <History className="w-4 h-4" />
            {t("cb.state_history")}
          </h3>
          {events.length === 0 ? (
            <p className="text-xs text-muted-foreground text-center py-6">{t("cb.no_events")}</p>
          ) : (
            <div className="space-y-2 max-h-64 overflow-y-auto">
              {events.map((e) => (
                <div key={e.id} className="flex items-start gap-3 p-3 bg-muted border border-border rounded-xl">
                  <ArrowLeftRight className="w-4 h-4" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 text-xs">
                      <code className="font-semibold text-foreground">{e.listener_id}</code>
                      <span className="text-muted-foreground">{e.old_state}</span>
                      <ArrowRight className="w-4 h-4" />
                      <span className={`font-medium ${e.new_state === "healthy" ? "text-emerald-600" : e.new_state === "open" || e.new_state === "burned" ? "text-destructive" : "text-amber-600"}`}>
                        {e.new_state}
                      </span>
                      {e.reason && <span className="text-muted-foreground">({e.reason})</span>}
                    </div>
                    <p className="text-(--fs-micro-sm) text-muted-foreground mt-0.5">{formatTime(e.created_at)}</p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>

      <Dialog open={showConfigModal} onOpenChange={setShowConfigModal}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("cb.config_title")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>{t("cb.failure_threshold")}</Label>
              <Input aria-label={t("circuit_breaker.threshold_label")} name="input-0" type="number" className="mt-1" value={configForm.failure_threshold} onChange={(e) => setConfigForm({ ...configForm, failure_threshold: parseInt(e.target.value) || 3 })} min={1} />
              <p className="text-(--fs-micro-sm) text-muted-foreground mt-1">{t("cb.failure_threshold_desc")}</p>
            </div>
            <div>
              <Label>{t("cb.cooldown")}</Label>
              <Input aria-label={t("circuit_breaker.cooldown_label")} name="input-1" type="number" className="mt-1" value={configForm.cooldown_seconds} onChange={(e) => setConfigForm({ ...configForm, cooldown_seconds: parseInt(e.target.value) || 300 })} min={10} />
              <p className="text-(--fs-micro-sm) text-muted-foreground mt-1">{t("cb.cooldown_desc")}</p>
            </div>
            <div>
              <Label>{t("cb.half_open_max")}</Label>
              <Input aria-label={t("circuit_breaker.half_open_label")} name="input-2" type="number" className="mt-1" value={configForm.half_open_max_reqs} onChange={(e) => setConfigForm({ ...configForm, half_open_max_reqs: parseInt(e.target.value) || 3 })} min={1} />
              <p className="text-(--fs-micro-sm) text-muted-foreground mt-1">{t("cb.half_open_max_desc")}</p>
            </div>
            <div>
              <Label>{t("cb.health_check_interval")}</Label>
              <Input aria-label={t("circuit_breaker.health_check_label")} name="input-3" type="number" className="mt-1" value={configForm.health_check_seconds} onChange={(e) => setConfigForm({ ...configForm, health_check_seconds: parseInt(e.target.value) || 60 })} min={5} />
              <p className="text-(--fs-micro-sm) text-muted-foreground mt-1">{t("cb.health_check_desc")}</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowConfigModal(false)}>{t("common.cancel")}</Button>
            <Button onClick={handleSaveConfig}>{t("cb.save_config")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
