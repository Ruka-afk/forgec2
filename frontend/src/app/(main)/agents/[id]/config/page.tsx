"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import type { AgentDetail } from "@/types/agent";
import { useI18n } from "@/lib/i18n";

import { Spinner, PageSpinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ArrowLeft, Clock, Info, Plus, RotateCcw, Send, SlidersHorizontal, X } from "lucide-react";

const COMMON_UAS = [
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
];

interface EffectiveConfig {
  sleep: number;
  jitter: number;
  user_agent: string;
  headers: Record<string, string> | null;
  beacon_uri: string;
  method: string;
}

interface ConfigResponse {
  agent_id: string;
  effective: EffectiveConfig;
  has_pending: boolean;
  pending_task_id: number;
}

const defaultConfig: EffectiveConfig = {
  sleep: 10,
  jitter: 20,
  user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
  headers: null,
  beacon_uri: "/beacon",
  method: "POST",
};

export default function AgentConfigPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useI18n();
  const [agent, setAgent] = useState<AgentDetail | null>(null);
  const [effective, setEffective] = useState<EffectiveConfig>(defaultConfig);
  const [hasPending, setHasPending] = useState(false);
  const [loading, setLoading] = useState(true);

  const [editSleep, setEditSleep] = useState("");
  const [editJitter, setEditJitter] = useState("");
  const [editUA, setEditUA] = useState("");
  const [editURI, setEditURI] = useState("");
  const [editMethod, setEditMethod] = useState("POST");
  const [editHeaders, setEditHeaders] = useState<{ key: string; value: string }[]>([]);
  const [pushing, setPushing] = useState(false);

  const resetForm = useCallback((cfg: EffectiveConfig) => {
    setEditSleep(String(cfg.sleep));
    setEditJitter(String(cfg.jitter));
    setEditUA(cfg.user_agent);
    setEditURI(cfg.beacon_uri);
    setEditMethod(cfg.method);
    if (cfg.headers) {
      setEditHeaders(Object.entries(cfg.headers).map(([k, v]) => ({ key: k, value: v })));
    } else {
      setEditHeaders([]);
    }
  }, []);

  const loadConfig = useCallback(async () => {
    if (!id) return;
    try {
      const data = await api.get<ConfigResponse>(`/agents/${id}/config`);
      setEffective(data.effective);
      setHasPending(data.has_pending);
      resetForm(data.effective);
      const agentData = await api.get(`/agents/${id}`);
      setAgent(agentData.agent || agentData);
    } catch {
      toast.error(t("agents.config_load_failed"));
    } finally {
      setLoading(false);
    }
  }, [id, resetForm, t]);

  useEffect(() => { loadConfig(); }, [loadConfig]);

  const addHeader = () => setEditHeaders([...editHeaders, { key: "", value: "" }]);
  const removeHeader = (i: number) => setEditHeaders(editHeaders.filter((_, idx) => idx !== i));
  const updateHeader = (i: number, field: "key" | "value", val: string) => {
    const next = [...editHeaders];
    next[i] = { ...next[i], [field]: val };
    setEditHeaders(next);
  };

  const resetField = (field: string) => {
    switch (field) {
      case "sleep": setEditSleep(String(defaultConfig.sleep)); break;
      case "jitter": setEditJitter(String(defaultConfig.jitter)); break;
      case "ua": setEditUA(defaultConfig.user_agent); break;
      case "uri": setEditURI(defaultConfig.beacon_uri); break;
      case "method": setEditMethod(defaultConfig.method); break;
      case "headers": setEditHeaders([]); break;
    }
  };

  const resetAll = () => resetForm(defaultConfig);

  const handlePush = async () => {
    if (!id) return;
    setPushing(true);
    try {
      const headersMap: Record<string, string> = {};
      for (const h of editHeaders) {
        if (h.key.trim()) headersMap[h.key.trim()] = h.value;
      }

      const body: Record<string, unknown> = {};
      const s = parseInt(editSleep);
      if (!isNaN(s) && s > 0) body.sleep = s;
      const j = parseInt(editJitter);
      if (!isNaN(j) && j >= 0) body.jitter = j;
      if (editUA.trim()) body.user_agent = editUA.trim();
      if (Object.keys(headersMap).length > 0) body.headers = headersMap;
      if (editURI.trim()) body.beacon_uri = editURI.trim();
      if (editMethod) body.method = editMethod;

      const res = await api.postJson<{success: boolean; error?: string}>(`/agents/${id}/config`, body);
      if (res.success) {
        toast.success(t("agents.config_push_success"));
        loadConfig();
      } else {
        toast.error(res.error || t("agents.config_push_failed"));
      }
    } catch {
      toast.error(t("agents.config_push_failed"));
    } finally {
      setPushing(false);
    }
  };

  const hostname = agent?.hostname || id;
  const os = agent?.os || "";

  if (loading) {
    return <PageSpinner />;
  }

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 space-y-6 animate-fade-slide-up">
      <div className="flex items-center gap-3 mb-2">
        <Link href={`/agents/${id}`} className="text-sm text-muted-foreground hover:text-foreground transition-colors">
          <ArrowLeft className="w-4 h-4" /> {hostname}
        </Link>
        <span className="text-xs text-muted-foreground/70">/</span>
        <span className="text-sm text-foreground">Config {os ? <span className="text-muted-foreground/70">({os})</span> : null}</span>
      </div>

      {hasPending && (
        <div className="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-xl px-4 py-3 text-sm text-amber-800 dark:text-amber-200 flex items-center gap-2">
          <Clock className="w-4 h-4" />
          {t("agents.config_pending_push")}
        </div>
      )}

      <Card className="p-4 sm:p-5 bg-gradient-to-br from-indigo-50/40 to-transparent dark:from-indigo-900/10">
        <h3 className="text-base font-semibold text-foreground mb-4 flex items-center gap-2">
          <SlidersHorizontal className="w-4 h-4" /> {t("agents.config_hot_config")}
        </h3>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-5">
          <div>
            <Label className="text-xs mb-1.5">{t("agents.config_sleep_seconds")}</Label>
            <div className="flex gap-2">
              <Input aria-label="Sleep interval in seconds" name="input-0" type="number" min="1" value={editSleep} onChange={(e) => setEditSleep(e.target.value)}
                className="flex-1" />
              <Button variant="outline" size="sm" onClick={() => resetField("sleep")} title={t("agents.config_reset_default")} aria-label="Reset sleep to default">
                <RotateCcw className="w-4 h-4" />
              </Button>
            </div>
          </div>
          <div>
            <Label className="text-xs mb-1.5">{t("agents.config_jitter_pct")}</Label>
            <div className="flex gap-2">
              <Input aria-label="Jitter percentage" name="input-1" type="number" min="0" max="100" value={editJitter} onChange={(e) => setEditJitter(e.target.value)}
                className="flex-1" />
              <Button variant="outline" size="sm" onClick={() => resetField("jitter")} title={t("agents.config_reset_default")} aria-label="Reset jitter to default">
                <RotateCcw className="w-4 h-4" />
              </Button>
            </div>
          </div>
        </div>

        <div className="mb-4">
          <Label className="text-xs mb-1.5">{t("agents.config_user_agent")}</Label>
          <div className="flex gap-2">
            <div className="flex-1 flex gap-2">
              <Input aria-label="User-Agent string" name="input-2" type="text" value={editUA} onChange={(e) => setEditUA(e.target.value)}
                className="flex-1 font-mono" />
              <Select value="" onValueChange={(v) => { if (v) setEditUA(v); }}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("agents.config_quick_fill")} />
                </SelectTrigger>
                <SelectContent>
                  {COMMON_UAS.map((ua, i) => (
                    <SelectItem key={i} value={ua}>{ua.slice(0, 60)}...</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
              <Button variant="outline" size="sm" onClick={() => resetField("ua")} title={t("agents.config_reset_default")} aria-label="Reset user-agent to default">
                <RotateCcw className="w-4 h-4" />
              </Button>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <Label className="text-xs mb-1.5">{t("agents.config_beacon_uri")}</Label>
            <div className="flex gap-2">
              <Input aria-label="Beacon URI path" name="input-4" type="text" value={editURI} onChange={(e) => setEditURI(e.target.value)}
                className="flex-1 font-mono" />
              <Button variant="outline" size="sm" onClick={() => resetField("uri")} title={t("agents.config_reset_default")} aria-label="Reset beacon URI to default">
                <RotateCcw className="w-4 h-4" />
              </Button>
            </div>
          </div>
          <div>
            <Label className="text-xs mb-1.5">{t("agents.config_http_method")}</Label>
            <div className="flex gap-2">
              <Select value={editMethod} onValueChange={(v) => { if (v !== null) setEditMethod(v); }}>
                <SelectTrigger className="flex-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="GET">GET</SelectItem>
                  <SelectItem value="POST">POST</SelectItem>
                </SelectContent>
              </Select>
              <Button variant="outline" size="sm" onClick={() => resetField("method")} title={t("agents.config_reset_default")} aria-label="Reset HTTP method to default">
                <RotateCcw className="w-4 h-4" />
              </Button>
            </div>
          </div>
        </div>

        <div className="mb-5">
          <div className="flex items-center justify-between mb-1.5">
            <Label className="text-xs">{t("agents.config_custom_headers")}</Label>
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" onClick={addHeader}>
                <Plus className="w-4 h-4" /> {t("agents.config_add")}
              </Button>
              <Button variant="outline" size="sm" onClick={() => resetField("headers")} title={t("agents.config_reset_default")} aria-label="Reset headers to default">
                <RotateCcw className="w-4 h-4" />
              </Button>
            </div>
          </div>
          <div className="space-y-2">
            {editHeaders.length === 0 ? (
              <p className="text-xs text-muted-foreground/70 italic px-1">{t("agents.config_no_headers")}</p>
            ) : (
              editHeaders.map((h, i) => (
                <div key={i} className="flex gap-2 items-center">
                  <Input aria-label="Header name" name="header-name-6" type="text" placeholder={t("agents.config_header_name")} value={h.key} onChange={(e) => updateHeader(i, "key", e.target.value)}
                    className="flex-1 font-mono" />
                  <Input aria-label="Value" name="value-7" type="text" placeholder={t("agents.config_header_value")} value={h.value} onChange={(e) => updateHeader(i, "value", e.target.value)}
                    className="flex-[2] font-mono" />
                  <Button variant="ghost" size="sm" onClick={() => removeHeader(i)} aria-label="Remove header">
                    <X className="w-4 h-4" />
                  </Button>
                </div>
              ))
            )}
          </div>
        </div>

        <div className="flex items-center gap-3 pt-3 border-t border-border">
          <Button onClick={handlePush} disabled={pushing}
            className="gap-2">
            {pushing ? (
              <Spinner size="sm" color="white" />
            ) : (
              <Send className="w-4 h-4" />
            )}
            {t("agents.config_push")}
          </Button>
          <Button variant="outline" onClick={resetAll} className="gap-1.5">
            <RotateCcw className="w-4 h-4" /> {t("agents.config_reset_all")}
          </Button>
        </div>
      </Card>

      <Card className="p-4 sm:p-5">
        <h3 className="text-base font-semibold text-foreground mb-4 flex items-center gap-2">
          <Info className="w-4 h-4" /> {t("agents.config_current_config")}
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          <div className="p-3 bg-card border border-border rounded-xl">
            <div className="text-xs text-muted-foreground mb-1">{t("agents.config_sleep_seconds")}</div>
            <div className="text-base font-mono font-semibold text-foreground">{effective.sleep}s</div>
          </div>
          <div className="p-3 bg-card border border-border rounded-xl">
            <div className="text-xs text-muted-foreground mb-1">{t("agents.config_jitter_pct")}</div>
            <div className="text-base font-mono font-semibold text-foreground">{effective.jitter}%</div>
          </div>
          <div className="p-3 bg-card border border-border rounded-xl">
            <div className="text-xs text-muted-foreground mb-1">{t("agents.config_user_agent")}</div>
            <div className="text-xs font-mono text-foreground break-all">{effective.user_agent}</div>
          </div>
          <div className="p-3 bg-card border border-border rounded-xl">
            <div className="text-xs text-muted-foreground mb-1">{t("agents.config_beacon_uri")}</div>
            <div className="text-sm font-mono font-semibold text-foreground">{effective.beacon_uri}</div>
          </div>
          <div className="p-3 bg-card border border-border rounded-xl">
            <div className="text-xs text-muted-foreground mb-1">{t("agents.config_http_method")}</div>
            <div className="text-sm font-mono font-semibold text-foreground">{effective.method}</div>
          </div>
          <div className="p-3 bg-card border border-border rounded-xl">
            <div className="text-xs text-muted-foreground mb-1">{t("agents.config_custom_headers")}</div>
            {effective.headers && Object.keys(effective.headers).length > 0 ? (
              <div className="space-y-0.5">
                {Object.entries(effective.headers).map(([k, v]) => (
                  <div key={k} className="text-xs font-mono text-foreground">
                    <span className="text-indigo-500">{k}</span>: {v}
                  </div>
                ))}
              </div>
            ) : (
              <span className="text-xs text-muted-foreground/70">None</span>
            )}
          </div>
        </div>
      </Card>
    </div>
  );
}
