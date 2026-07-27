"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { PageHeader, Spinner } from "@/components/UI";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { Globe, Play, Square, Activity, Server, Wifi, WifiOff } from "lucide-react";

interface DNSStatus {
  running: boolean;
  domain: string;
  addr: string;
  agent_ip: string;
  beacon_count: number;
}

export default function DNSPage() {
  const { t } = useI18n();
  const [status, setStatus] = useState<DNSStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [domain, setDomain] = useState("");
  const [addr, setAddr] = useState(":53");
  const [server, setServer] = useState("8.8.8.8:53");
  const [txtPrefix, setTxtPrefix] = useState(".dns");

  const fetchStatus = useCallback(async () => {
    try {
      const data = await api.get<DNSStatus>("/api/dns/status");
      setStatus(data);
      if (data.domain) setDomain(data.domain);
      if (data.addr) setAddr(data.addr);
    } catch {
      toast.error(t("dns.toast.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  useEffect(() => {
    const interval = setInterval(fetchStatus, 5000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  const handleStart = async () => {
    setActionLoading(true);
    try {
      await api.postJson<{ status: string }>("/api/dns/start", {
        domain,
        addr,
        server,
        txt_prefix: txtPrefix,
      });
      toast.success(t("dns.toast.started"));
      fetchStatus();
    } catch (e) {
      toast.error(`${t("dns.toast.start_failed")}: ${e instanceof Error ? e.message : String(e)}`);
    }
    setActionLoading(false);
  };

  const handleStop = async () => {
    setActionLoading(true);
    try {
      await api.postJson<{ status: string }>("/api/dns/stop", {});
      toast.success(t("dns.toast.stopped"));
      fetchStatus();
    } catch (e) {
      toast.error(`${t("dns.toast.stop_failed")}: ${e instanceof Error ? e.message : String(e)}`);
    }
    setActionLoading(false);
  };

  if (loading) {
    return (
      <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
        <PageHeader title={t("dns.title")} subtitle={t("dns.subtitle")} />
        <div className="flex items-center justify-center py-20">
          <Spinner size="lg" />
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={<><Globe className="w-4 h-4" />{t("dns.title")}</>} subtitle={t("dns.subtitle")}>
        {status?.running ? (
          <Button variant="destructive" size="sm" onClick={handleStop} disabled={actionLoading}>
            {actionLoading ? <Spinner size="xs" /> : <Square className="w-3.5 h-3.5" />}
            <span>{actionLoading ? t("dns.stopping") : t("dns.stop")}</span>
          </Button>
        ) : (
          <Button size="sm" onClick={handleStart} disabled={actionLoading}>
            {actionLoading ? <Spinner size="xs" /> : <Play className="w-3.5 h-3.5" />}
            <span>{actionLoading ? t("dns.starting") : t("dns.start")}</span>
          </Button>
        )}
      </PageHeader>

      {/* Status Card */}
      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${status?.running ? 'bg-emerald-100 dark:bg-emerald-900/30' : 'bg-red-100 dark:bg-red-900/30'}`}>
            {status?.running ? <Wifi className="w-4 h-4 text-emerald-600 dark:text-emerald-400" /> : <WifiOff className="w-4 h-4 text-red-600 dark:text-red-400" />}
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("dns.status_title")}</div>
            <div className="text-xs text-muted-foreground">{t("dns.status_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <Card className="p-3">
            <CardContent className="p-0">
              <div className="text-(--font-size-micro-sm) uppercase tracking-wider text-muted-foreground/70 mb-1">{t("dns.status_title")}</div>
              <div className="flex items-center gap-x-2">
                <span className={`w-2 h-2 rounded-full ${status?.running ? 'bg-emerald-500 animate-pulse' : 'bg-red-500'}`} />
                <span className="text-sm font-semibold text-foreground">{status?.running ? t("common.online") : t("common.offline")}</span>
              </div>
            </CardContent>
          </Card>
          <Card className="p-3">
            <CardContent className="p-0">
              <div className="text-(--font-size-micro-sm) uppercase tracking-wider text-muted-foreground/70 mb-1">{t("dns.domain")}</div>
              <div className="text-sm font-semibold text-foreground font-mono">{status?.domain || '\u2014'}</div>
            </CardContent>
          </Card>
          <Card className="p-3">
            <CardContent className="p-0">
              <div className="text-(--font-size-micro-sm) uppercase tracking-wider text-muted-foreground/70 mb-1">{t("dns.listen_addr")}</div>
              <div className="text-sm font-semibold text-foreground font-mono">{status?.addr || '\u2014'}</div>
            </CardContent>
          </Card>
          <Card className="p-3">
            <CardContent className="p-0">
              <div className="text-(--font-size-micro-sm) uppercase tracking-wider text-muted-foreground/70 mb-1">{t("dns.beacon_count")}</div>
              <div className="flex items-center gap-x-2">
                <Activity className="w-3.5 h-3.5 text-primary" />
                <span className="text-sm font-semibold text-foreground">{status?.beacon_count ?? 0}</span>
              </div>
            </CardContent>
          </Card>
        </div>
      </Card>

      {/* Configuration Card */}
      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-primary/10 rounded-xl flex items-center justify-center">
            <Server className="w-4 h-4" />
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("dns.config_title")}</div>
            <div className="text-xs text-muted-foreground">{t("dns.config_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("dns.field_domain")}</label>
            <Input
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="c2.example.com"
              className="font-mono"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("dns.field_listen")}</label>
            <Input
              value={addr}
              onChange={(e) => setAddr(e.target.value)}
              placeholder=":53"
              className="font-mono"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("dns.field_upstream")}</label>
            <Input
              value={server}
              onChange={(e) => setServer(e.target.value)}
              placeholder="8.8.8.8:53"
              className="font-mono"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("dns.field_prefix")}</label>
            <Input
              value={txtPrefix}
              onChange={(e) => setTxtPrefix(e.target.value)}
              placeholder=".dns"
              className="font-mono"
            />
          </div>
        </div>
        <div className="mt-4 flex items-center gap-x-2">
          <Badge variant="outline" className="text-xs">
            {t("dns.query_hint")}
          </Badge>
          <Badge variant="outline" className="text-xs">
            {t("dns.chunk_hint")}
          </Badge>
          <Badge variant="outline" className="text-xs">
            {t("dns.encoding_hint")}
          </Badge>
        </div>
      </Card>

      {/* How It Works */}
      <Card className="p-4 sm:p-5">
        <div className="flex items-center gap-x-3 mb-4">
          <div className="w-10 h-10 bg-amber-100 dark:bg-amber-900/30 rounded-xl flex items-center justify-center">
            <Globe className="w-4 h-4 text-amber-600 dark:text-amber-400" />
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("dns.how_title")}</div>
            <div className="text-xs text-muted-foreground">{t("dns.how_desc")}</div>
          </div>
        </div>
        <div className="space-y-3 text-xs text-muted-foreground">
          <div className="flex gap-x-3">
            <Badge variant="secondary" className="shrink-0">{t("dns.a_records")}</Badge>
            <span>{t("dns.desc_paragraph")}</span>
          </div>
          <div className="flex gap-x-3">
            <Badge variant="secondary" className="shrink-0">{t("dns.txt_records")}</Badge>
            <span>{t("dns.desc_paragraph")}</span>
          </div>
          <div className="flex gap-x-3">
            <Badge variant="secondary" className="shrink-0">{t("dns.query_format")}</Badge>
            <span className="font-mono">{"<agent-uuid>[.<base32-data>].dns.<domain>"}</span>
          </div>
          <div className="flex gap-x-3">
            <Badge variant="secondary" className="shrink-0">{t("dns.cache_bypass")}</Badge>
            <span>{t("dns.desc_paragraph")}</span>
          </div>
        </div>
      </Card>
    </div>
  );
}
