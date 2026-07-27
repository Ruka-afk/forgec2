"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import Toggle from "./Toggle";
import { FlaskConical, LogIn, Save } from "lucide-react";

interface SIEMConfig {
  enabled: boolean;
  type: string;
  endpoint: string;
  token: string;
  index: string;
  source: string;
  level: string;
  tls_enabled: boolean;
  tls_skip_verify: boolean;
}

const defaultConfig: SIEMConfig = {
  enabled: false,
  type: "splunk_hec",
  endpoint: "",
  token: "",
  index: "",
  source: "forgec2",
  level: "info",
  tls_enabled: false,
  tls_skip_verify: false,
};

export default function SIEMSection() {
  const { t } = useI18n();
  const types = [
    { value: "splunk_hec", label: t("settings.siem.typeSplunkHec") },
    { value: "syslog", label: t("settings.siem.typeSyslog") },
    { value: "elastic", label: t("settings.siem.typeElastic") },
    { value: "generic", label: t("settings.siem.typeGeneric") },
  ];

  const levels = [
    { value: "info", label: t("settings.siem.levelInfo") },
    { value: "warn", label: t("settings.siem.levelWarn") },
    { value: "error", label: t("settings.siem.levelError") },
  ];
  const [config, setConfig] = useState<SIEMConfig>({ ...defaultConfig });
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  useEffect(() => {
    api.get("/settings/siem")
      .then((d: Record<string, unknown>) => {
        if (d.data) {
          setConfig({ ...defaultConfig, ...d.data });
        }
      })
      .catch(() => toast.error(t("settings.toast.siem_load_failed")));
  }, [t]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.postJson("/settings/siem", { config });
      toast.success(t("settings.toast.siem_saved"));
    } catch {
      toast.error(t("settings.toast.siem_save_failed"));
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    try {
      const d = await api.postJson("/settings/siem/test", { config });
      if (d.success) { toast.success(t("settings.toast.siem_test_sent")); } else { toast.error(((d.error as string) || t("settings.toast.test_failed"))); }
    } catch {
      toast.error(t("settings.toast.test_failed"));
    } finally {
      setTesting(false);
    }
  };

  const update = (field: keyof SIEMConfig, value: unknown) => {
    setConfig((prev) => ({ ...prev, [field]: value }));
  };

  return (
    <Card className="rounded-2xl overflow-hidden">
      <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border flex items-center gap-3">
        <div className="w-8 h-8 bg-amber-100 dark:bg-amber-900/30 rounded-xl flex items-center justify-center text-amber-600"><LogIn className="w-4 h-4" /></div>
        <div>
          <h2 className="text-sm font-semibold">{t("settings.siem.title")}</h2>
          <p className="text-(--font-size-xs-sm) text-muted-foreground">{t("settings.siem.description")}</p>
        </div>
      </div>

      <div className="p-4 sm:p-5 space-y-5">
        {/* Enable toggle */}
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-medium">{t("settings.siem.enableForwarding")}</div>
            <div className="text-(--font-size-xs-sm) text-muted-foreground">{t("settings.siem.enableDesc")}</div>
          </div>
          <Toggle checked={config.enabled} onChange={(v) => update("enabled", v)} />
        </div>

        {/* Type */}
        <div>
          <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.siem.type")}</span>
          <Select value={config.type} onValueChange={(v) => v && update("type", v)}>
            <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              {types.map((t) => (
                <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* Endpoint */}
        <div>
          <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">
            {config.type === "syslog" ? t("settings.siem.syslogServer") : t("settings.siem.endpointUrl")}
          </span>
          <Input aria-label="SIEM endpoint URL" name="input-1" className="h-9 text-xs" placeholder={
            config.type === "syslog" ? "192.168.1.100:514" :
            config.type === "splunk_hec" ? "https://splunk.example.com:8088" :
            config.type === "elastic" ? "https://elastic.example.com:9200" :
            "https://logstash.example.com:5000"
          } value={config.endpoint} onChange={(e) => update("endpoint", e.target.value)} />
        </div>

        {/* Auth Token */}
        {(config.type === "splunk_hec" || config.type === "elastic" || config.type === "generic") && (
          <div>
            <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">
              {config.type === "splunk_hec" ? t("settings.siem.hecToken") : t("settings.siem.authToken")}
            </span>
            <Input aria-label="????????" name="input-2" type="password" className="h-9 text-xs" placeholder="????????" value={config.token} onChange={(e) => update("token", e.target.value)} />
          </div>
        )}

        {/* Index (Splunk/Elastic only) */}
        {(config.type === "splunk_hec" || config.type === "elastic") && (
          <div>
            <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.siem.indexName")}</span>
            <Input aria-label="Index name" name="input-3" className="h-9 text-xs" placeholder={config.type === "splunk_hec" ? "forgec2" : "forgec2-logs"} value={config.index} onChange={(e) => update("index", e.target.value)} />
          </div>
        )}

        {/* Source name */}
        <div>
          <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.siem.sourceIdentifier")}</span>
          <Input aria-label="forgec2" name="forgec2-4" className="h-9 text-xs" placeholder="forgec2" value={config.source} onChange={(e) => update("source", e.target.value)} />
        </div>

        {/* Log level filter */}
        <div>
          <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.siem.minLogLevel")}</span>
          <Select value={config.level} onValueChange={(v) => v && update("level", v)}>
            <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              {levels.map((l) => (
                <SelectItem key={l.value} value={l.value}>{l.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* TLS settings (for HTTP-based types) */}
        {config.type !== "syslog" && (
          <div className="space-y-3 p-3 bg-muted/30 rounded-2xl">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-xs font-medium">{t("settings.siem.tlsEnabled")}</div>
                <div className="text-(--font-size-micro-sm) text-muted-foreground">{t("settings.siem.tlsEnabledDesc")}</div>
              </div>
              <Toggle checked={config.tls_enabled} onChange={(v) => update("tls_enabled", v)} />
            </div>
            {config.tls_enabled && (
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-xs font-medium">{t("settings.siem.skipTlsVerify")}</div>
                  <div className="text-(--font-size-micro-sm) text-muted-foreground">{t("settings.siem.skipTlsVerifyDesc")}</div>
                </div>
                <Toggle checked={config.tls_skip_verify} onChange={(v) => update("tls_skip_verify", v)} />
              </div>
            )}
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center justify-between pt-2 border-t border-border">
          <Button onClick={handleTest} disabled={testing}
            className="px-3 py-1.5 bg-sky-600 hover:bg-sky-700 text-white rounded-xl text-xs font-medium flex items-center gap-1.5">
            {testing ? <Spinner size="xs" /> : <FlaskConical className="w-4 h-4" />}
            {testing ? t("settings.siem.sending") : t("settings.siem.test")}
          </Button>
          <Button onClick={handleSave} disabled={saving}
            className="px-4 py-1.5 rounded-xl text-xs font-medium flex items-center gap-1.5">
            {saving ? <Spinner size="xs" /> : <Save className="w-4 h-4" />}
            {saving ? t("settings.siem.saving") : t("settings.siem.save")}
          </Button>
        </div>
      </div>
    </Card>
  );
}

