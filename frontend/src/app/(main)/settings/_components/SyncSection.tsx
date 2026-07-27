"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import Toggle from "./Toggle";
import { Check, Circle, Clock, Cloud, CloudUpload, FlaskConical, FolderOpen, RefreshCw, Save, Server, X } from "lucide-react";

interface S3Config {
  bucket: string;
  region: string;
  endpoint: string;
  access_key: string;
  secret_key: string;
  path_prefix: string;
}

interface WebDAVConfig {
  url: string;
  username: string;
  password: string;
  path_prefix: string;
}

interface SyncConfig {
  enabled: boolean;
  interval: number;
  s3: S3Config;
  webdav: WebDAVConfig;
}

interface SyncStatus {
  enabled: boolean;
  last_sync_at: string;
  files_synced: number;
  files_failed: number;
  last_errors: string[];
  running: boolean;
  backend_status: string[];
}

const defaultS3: S3Config = { bucket: "", region: "us-east-1", endpoint: "", access_key: "", secret_key: "", path_prefix: "" };
const defaultWebDAV: WebDAVConfig = { url: "", username: "", password: "", path_prefix: "" };

type Tab = "s3" | "webdav";

export default function SyncSection() {
  const { t } = useI18n();
  const [config, setConfig] = useState<SyncConfig>({ enabled: false, interval: 0, s3: { ...defaultS3 }, webdav: { ...defaultWebDAV } });
  const [status, setStatus] = useState<SyncStatus | null>(null);
  const [saving, setSaving] = useState(false);
  const syncTimersRef = useRef<{ poll: ReturnType<typeof setInterval>; timeout: ReturnType<typeof setTimeout> } | null>(null);

  useEffect(() => {
    return () => {
      if (syncTimersRef.current) {
        clearInterval(syncTimersRef.current.poll);
        clearTimeout(syncTimersRef.current.timeout);
        syncTimersRef.current = null;
      }
    };
  }, []);
  const [syncing, setSyncing] = useState(false);
  const [tab, setTab] = useState<Tab>("s3");
  const [testing, setTesting] = useState(false);
  const loadConfig = useCallback(async () => {
    try {
      const d = await api.get("/settings/sync") as Record<string, unknown>;
      if (d.data) {
        const dd = d.data as Record<string, unknown>;
        setConfig({
          enabled: !!dd.enabled,
          interval: Number(dd.interval) || 0,
          s3: { ...defaultS3, ...(dd.s3 as Record<string, string>) },
          webdav: { ...defaultWebDAV, ...(dd.webdav as Record<string, string>) },
        });
      }
    } catch {
      toast.error(t("settings.toast.sync_load_failed"));
    }
  }, [t]);

  const loadStatus = useCallback(async () => {
    try {
      const d = await api.get("/settings/sync/status") as Record<string, unknown>;
      if (d.data) setStatus(d.data as SyncStatus);
    } catch {
      toast.error(t("settings.toast.sync_load_failed"));
    }
  }, [t]);

  useEffect(() => {
    Promise.all([loadConfig(), loadStatus()]);
  }, [loadConfig, loadStatus]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.postJson("/settings/sync", { config });
      toast.success(t("settings.toast.sync_saved"));
      loadStatus();
    } catch {
      toast.error(t("settings.toast.sync_save_failed"));
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    const backendType = tab === "s3" ? "s3" : "webdav";
    const testPayload: Record<string, unknown> = { type: backendType };
    if (backendType === "s3") testPayload.s3 = config.s3;
    else testPayload.webdav = config.webdav;

    setTesting(true);
    try {
      const d = await api.postJson("/settings/sync/test", testPayload) as { success?: boolean; error?: string };
      if (d.success) { toast.success(t("settings.toast.connection_ok")); } else { toast.error(d.error || t("settings.toast.connection_failed")); }
    } catch {
      toast.error(t("settings.toast.sync_test_failed"));
    } finally {
      setTesting(false);
    }
  };

  const handleSyncNow = async () => {
    if (syncTimersRef.current) {
      clearInterval(syncTimersRef.current.poll);
      clearTimeout(syncTimersRef.current.timeout);
      syncTimersRef.current = null;
    }
    setSyncing(true);
    try {
      await api.post("/settings/sync/trigger");
      toast.success(t("settings.toast.sync_triggered"));
      const poll = setInterval(async () => {
        try {
          const sd = await api.get("/settings/sync/status") as Record<string, unknown>;
          const sdData = sd.data as SyncStatus | undefined;
          if (sdData && !sdData.running) {
            clearInterval(poll);
            clearTimeout(timeout);
            syncTimersRef.current = null;
            setStatus(sdData);
            setSyncing(false);
            toast.success(t("settings.toast.sync_completed"));
          }
        } catch {
          clearInterval(poll);
          clearTimeout(timeout);
          syncTimersRef.current = null;
          setSyncing(false);
          toast.error(t("settings.toast.sync_status_lost"));
        }
      }, 2000);
      const timeout = setTimeout(() => { clearInterval(poll); syncTimersRef.current = null; setSyncing(false); }, 120000);
      syncTimersRef.current = { poll, timeout };
    } catch {
      toast.error(t("settings.toast.sync_trigger_failed"));
      setSyncing(false);
    }
  };

  const updateS3 = (field: keyof S3Config, value: string) => {
    setConfig((prev) => ({ ...prev, s3: { ...prev.s3, [field]: value } }));
  };

  const updateWebDAV = (field: keyof WebDAVConfig, value: string) => {
    setConfig((prev) => ({ ...prev, webdav: { ...prev.webdav, [field]: value } }));
  };

  return (
    <Card className="rounded-2xl overflow-hidden">
      <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border flex items-center gap-3">
        <div className="w-8 h-8 bg-primary/10 rounded-xl flex items-center justify-center text-primary"><CloudUpload className="w-4 h-4" /></div>
        <div>
          <h2 className="text-sm font-semibold">{t("settings.sync.title")}</h2>
          <p className="text-(--font-size-xs-sm) text-muted-foreground">{t("settings.sync.description")}</p>
        </div>
      </div>

      <div className="p-4 sm:p-5 space-y-5">
        {/* Enable toggle */}
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-medium">{t("settings.sync.enableSync")}</div>
            <div className="text-(--font-size-xs-sm) text-muted-foreground">{t("settings.sync.enableDesc")}</div>
          </div>
          <Toggle checked={config.enabled} onChange={(v) => setConfig((prev) => ({ ...prev, enabled: v }))} />
        </div>

        {/* Interval */}
        <div>
          <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.interval")}</span>
          <Input aria-label="0" name="0-0" type="number" min="0" max="1440" className="h-9 text-xs w-48" placeholder="0"
            value={config.interval} onChange={(e) => setConfig((prev) => ({ ...prev, interval: parseInt(e.target.value) || 0 }))} />
        </div>

        {/* Tabs */}
        <div>
          <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)}>
            <TabsList className="mb-4">
              <TabsTrigger value="s3" className="gap-1.5">
                <Cloud className="w-4 h-4" />{t("settings.sync.s3Compatible")}
              </TabsTrigger>
              <TabsTrigger value="webdav" className="gap-1.5">
                <FolderOpen className="w-4 h-4" />WebDAV
              </TabsTrigger>
            </TabsList>

          <TabsContent value="s3">
            <div className="space-y-3">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.bucket")}</span>
                  <Input aria-label="my-bucket" name="my-bucket-1" className="h-9 text-xs" placeholder="my-bucket" value={config.s3.bucket} onChange={(e) => updateS3("bucket", e.target.value)} />
                </div>
                <div>
                  <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.region")}</span>
                  <Input aria-label="us-east-1" name="us-east-1-2" className="h-9 text-xs" placeholder="us-east-1" value={config.s3.region} onChange={(e) => updateS3("region", e.target.value)} />
                </div>
              </div>
              <div>
                <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.endpointUrl")}</span>
                <Input aria-label="https://s3.amazonaws.com" name="https-s3-amazonaws-com-3" className="h-9 text-xs" placeholder="https://s3.amazonaws.com" value={config.s3.endpoint} onChange={(e) => updateS3("endpoint", e.target.value)} />
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.accessKey")}</span>
                  <Input aria-label="AKIAIOSFODNN7EXAMPLE" name="akiaiosfodnn7example-4" className="h-9 text-xs" placeholder="AKIAIOSFODNN7EXAMPLE" value={config.s3.access_key} onChange={(e) => updateS3("access_key", e.target.value)} />
                </div>
                <div>
                  <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.secretKey")}</span>
                  <Input aria-label="????????" name="input-5" type="password" className="h-9 text-xs" placeholder="????????" value={config.s3.secret_key} onChange={(e) => updateS3("secret_key", e.target.value)} />
                </div>
              </div>
              <div>
                <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.pathPrefix")}</span>
                <Input aria-label="forgec2/screenshots" name="forgec2-screenshots-6" className="h-9 text-xs" placeholder="forgec2/screenshots" value={config.s3.path_prefix} onChange={(e) => updateS3("path_prefix", e.target.value)} />
              </div>
            </div>
          </TabsContent>

          <TabsContent value="webdav">
            <div className="space-y-3">
              <div>
                <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.webdavUrl")}</span>
                <Input aria-label="https://webdav.example.com/remote.php/dav/files/user" name="https-webdav-example-com-remote-php-dav--7" className="h-9 text-xs" placeholder="https://webdav.example.com/remote.php/dav/files/user" value={config.webdav.url} onChange={(e) => updateWebDAV("url", e.target.value)} />
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.username")}</span>
                  <Input aria-label="user" name="user-8" className="h-9 text-xs" placeholder="user" value={config.webdav.username} onChange={(e) => updateWebDAV("username", e.target.value)} />
                </div>
                <div>
                  <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.password")}</span>
                  <Input aria-label="????????" name="input-9" type="password" className="h-9 text-xs" placeholder="????????" value={config.webdav.password} onChange={(e) => updateWebDAV("password", e.target.value)} />
                </div>
              </div>
              <div>
                <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">{t("settings.sync.pathPrefix")}</span>
                <Input aria-label="forgec2/uploads" name="forgec2-uploads-10" className="h-9 text-xs" placeholder="forgec2/uploads" value={config.webdav.path_prefix} onChange={(e) => updateWebDAV("path_prefix", e.target.value)} />
              </div>
            </div>
          </TabsContent>
          </Tabs>
        </div>

        {/* Test & Save */}
        <div className="flex items-center justify-between pt-2 border-t border-border">
          <Button onClick={handleTest} disabled={testing}
            className="px-3 py-1.5 bg-sky-600 hover:bg-sky-700 text-white rounded-xl text-xs font-medium flex items-center gap-1.5">
            {testing ? <Spinner size="xs" /> : <FlaskConical className="w-4 h-4" />}
            {testing ? t("settings.sync.testing") : t("settings.sync.testConnection")}
          </Button>
          <Button onClick={handleSave} disabled={saving}
            className="px-4 py-1.5 rounded-xl text-xs font-medium flex items-center gap-1.5">
            {saving ? <Spinner size="xs" /> : <Save className="w-4 h-4" />}
            {saving ? t("settings.sync.saving") : t("settings.sync.save")}
          </Button>
        </div>

        {/* Sync Now & Status */}
        <div className="border-t border-border pt-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium">{t("settings.sync.manualSync")}</div>
              <div className="text-(--font-size-xs-sm) text-muted-foreground">{t("settings.sync.manualSyncDesc")}</div>
            </div>
            <Button onClick={handleSyncNow} disabled={syncing || !config.enabled}
              className="px-4 py-1.5 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white rounded-xl text-xs font-medium flex items-center gap-1.5">
              {syncing ? <Spinner size="xs" /> : <RefreshCw className="w-4 h-4" />}
              {syncing ? t("settings.sync.syncing") : t("settings.sync.syncNow")}
            </Button>
          </div>

          {status && (
            <div className="p-3 bg-muted/30 rounded-2xl space-y-2 text-xs">
              <div className="flex items-center gap-4 text-muted-foreground">
                <span><Clock className="w-4 h-4" />{t("settings.sync.lastSync")} {status.last_sync_at ? new Date(status.last_sync_at).toLocaleString() : t("settings.sync.never")}</span>
                <span className={status.running ? "text-amber-600" : "text-emerald-600"}>
                  {status.running ? <Spinner size="xs" className="mr-1" /> : <Circle className="w-4 h-4" />}
                  {status.running ? t("settings.sync.syncing") : t("settings.sync.idle")}
                </span>
              </div>
              <div className="flex items-center gap-4">
                <span className="text-emerald-600"><Check className="w-4 h-4" />{t("settings.sync.synced")} {status.files_synced}</span>
                <span className={status.files_failed > 0 ? "text-destructive" : "text-muted-foreground"}><X className="w-4 h-4" />{t("settings.sync.failed")} {status.files_failed}</span>
              </div>
              {status.backend_status && status.backend_status.length > 0 && (
                <div className="text-muted-foreground">
                  <Server className="w-4 h-4" />{t("settings.sync.backends")} {status.backend_status.join(", ")}
                </div>
              )}
              {status.last_errors && status.last_errors.length > 0 && (
                <div className="text-destructive bg-destructive/10 rounded-xl p-2 space-y-1">
                  {status.last_errors.map((err, i) => (
                    <div key={i} className="font-mono text-(--font-size-micro-sm) leading-tight">{err}</div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}

