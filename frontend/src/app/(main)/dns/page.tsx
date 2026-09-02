"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { POLL } from "@/lib/polling";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { PageContainer } from "@/components/ui/page-container";
import { PageSpinner } from "@/components/ui/spinner";
import { StatCard } from "@/components/ui/animated-stat-card";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { Card, CardContent } from "@/components/ui/card";
import { Banner } from "@/components/ui/banner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Globe,
  Server,
  Play,
  Square,
  Settings,
  ArrowDown,
  ArrowUp,
  Loader2,
} from "lucide-react";

interface DnsStatus {
  running?: boolean;
  domain?: string;
  addr?: string;
  agent_ip?: string;
  beacon_count?: number;
}

export default function DnsPage() {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const [domain, setDomain] = useState("");
  const [addr, setAddr] = useState("");
  const [configDirty, setConfigDirty] = useState(false);
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchStatus = useCallback(
    () => api.get<DnsStatus>(paths.dns.status),
    [],
  );

  const { data: status, loading, error, refresh } = useApiResource<DnsStatus>({
    fetcher: fetchStatus,
    pollMs: POLL.dns,
    toastThrottleMs: POLL.toastThrottle,
    errorMessage: t("dns.toast.load_failed"),
  });

  const s = status ?? {
    running: false,
    domain: "",
    addr: ":53",
    agent_ip: "",
    beacon_count: 0,
  };

  useEffect(() => {
    if (!status || status.running || configDirty) return;
    setDomain(status.domain ?? "");
    setAddr(status.addr ?? ":53");
  }, [configDirty, status]);

  const handleToggle = async (nextRunning: boolean) => {
    if (busy) return;
    setBusy(true);
    try {
      if (nextRunning) {
        await api.postJson(paths.dns.start, { domain, addr });
        setConfigDirty(false);
        toast.success(t("dns.toast.started"));
      } else {
        await api.post(paths.dns.stop);
        setConfigDirty(false);
        toast.success(t("dns.toast.stopped"));
      }
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);
      refreshTimerRef.current = setTimeout(() => {
        refreshTimerRef.current = null;
        void refresh();
      }, 500);
    } catch {
      toast.error(nextRunning ? t("dns.toast.start_failed") : t("dns.toast.stop_failed"));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => () => {
    if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);
  }, []);

  if (loading) return <PageContainer title={t("dns.title")} subtitle={t("dns.subtitle")}><PageSpinner /></PageContainer>;

  const totalBeacons = s.beacon_count ?? 0;
  const running = !!s.running;

  return (
    <PageContainer
      title={t("dns.title")}
      subtitle={t("dns.subtitle")}
      actions={
        <div className="flex items-center gap-3">
          <StatusIndicator
            status={running ? "connected" : "disconnected"}
            variant="dot"
            label={running ? t("common.online") : t("common.offline")}
            pulse={running}
          />
          <Button
            size="sm"
            variant={running ? "destructive" : "default"}
            disabled={busy}
            onClick={() => handleToggle(!running)}
          >
            {busy ? (
              <Loader2 className="size-4 animate-spin" />
            ) : running ? (
              <Square className="size-4" />
            ) : (
              <Play className="size-4" />
            )}
            {running ? t("dns.stop") : t("dns.start")}
          </Button>
        </div>
      }
    >
      {error && <Banner tone="destructive" alert>{error}</Banner>}

      {/* Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 sm:gap-5">
        <StatCard
          label={t("dns.beacon_count")}
          value={totalBeacons}
          color="emerald"
          icon={<Globe className="size-4" />}
          sub={running ? t("common.online") : t("common.offline")}
          dot={running}
          dotTone={running ? "ok" : "crit"}
        />
        <StatCard
          label={t("dns.domain")}
          value={s.domain || "—"}
          color="indigo"
          icon={<Globe className="size-4" />}
          sub={s.agent_ip || "—"}
        />
        <StatCard
          label={t("dns.listen_addr")}
          value={running ? t("common.online") : t("common.offline")}
          color={running ? "emerald" : "slate"}
          icon={<Server className="size-4" />}
          sub={s.addr ?? ":53"}
        />
      </div>

      {/* How it works banner */}
      <Banner tone="info" className="items-start">
        <div className="font-semibold">{t("dns.how_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("dns.how_desc")}</div>
      </Banner>

      {/* Configuration Card */}
      <Card>
        <CardContent>
          <div className="flex items-center justify-between mb-4">
            <span className="text-sm font-semibold text-foreground">{t("dns.config_title")}</span>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <Label className="text-xs font-semibold text-muted-foreground mb-1.5">{t("dns.field_domain")}</Label>
              <Input
                value={domain}
                placeholder="c2.example.com"
                disabled={running}
                onChange={(e) => {
                  setDomain(e.target.value);
                  setConfigDirty(true);
                }}
              />
              <p className="text-(--fs-micro-sm) text-muted-foreground mt-1">{t("dns.domain")}</p>
            </div>
            <div>
              <Label className="text-xs font-semibold text-muted-foreground mb-1.5">{t("dns.field_listen")}</Label>
              <Input
                value={addr}
                placeholder=":53"
                disabled={running}
                onChange={(e) => {
                  setAddr(e.target.value);
                  setConfigDirty(true);
                }}
              />
              <p className="text-(--fs-micro-sm) text-muted-foreground mt-1">{t("dns.listen_addr")}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Protocol Details Card */}
      <Card className="p-(--card-spacing)">
        <h3 className="text-sm font-semibold text-foreground mb-3">
          <Settings className="size-4 inline mr-1.5" />
          {t("dns.how_title")}
        </h3>
        <div className="space-y-2 max-h-64 overflow-y-auto">
          <div className="flex items-start gap-3 p-3 bg-muted border border-border rounded-lg">
            <ArrowDown className="size-4 text-emerald-500 mt-0.5 shrink-0" />
            <div className="min-w-0">
              <code className="text-xs font-semibold text-foreground">{t("dns.query_hint")}</code>
              <p className="text-xs text-muted-foreground mt-0.5">{t("dns.desc_paragraph")}</p>
            </div>
          </div>
          <div className="flex items-start gap-3 p-3 bg-muted border border-border rounded-lg">
            <ArrowUp className="size-4 text-amber-500 mt-0.5 shrink-0" />
            <div className="min-w-0">
              <code className="text-xs font-semibold text-foreground">{t("dns.chunk_hint")}</code>
              <p className="text-xs text-muted-foreground mt-0.5">{t("dns.encoding_hint")}</p>
            </div>
          </div>
        </div>
      </Card>
    </PageContainer>
  );
}
