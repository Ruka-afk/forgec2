"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Banner } from "@/components/ui/banner";
import { Textarea } from "@/components/ui/textarea";

interface DeviceStart {
  id?: string;
  user_code?: string;
  verification_uri?: string;
  verification_uri_complete?: string;
  message?: string;
  expires_in?: number;
}

interface ConsentStart {
  id?: string;
  authorize_url?: string;
  state?: string;
  note?: string;
}

export function EntraIdentityPanel() {
  const { t } = useI18n();
  const [tenant, setTenant] = useState("organizations");
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [scope, setScope] = useState("https://graph.microsoft.com/.default offline_access");
  const [redirectUri, setRedirectUri] = useState("");
  const [device, setDevice] = useState<DeviceStart | null>(null);
  const [consent, setConsent] = useState<ConsentStart | null>(null);
  const [tokenJson, setTokenJson] = useState("");
  const [busy, setBusy] = useState(false);

  const startDevice = async () => {
    setBusy(true);
    try {
      const res = await api.postJson<DeviceStart>(paths.identity.deviceCode, {
        tenant, client_id: clientId, scope,
      });
      setDevice(res);
      toast.success(t("phishing.entra_device_started"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("phishing.entra_failed"));
    } finally {
      setBusy(false);
    }
  };

  const pollDevice = async () => {
    if (!device?.id) return;
    setBusy(true);
    try {
      const res = await api.postJson<Record<string, unknown>>(paths.identity.deviceCodePoll(device.id), {});
      setTokenJson(JSON.stringify(res, null, 2));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("phishing.entra_failed"));
    } finally {
      setBusy(false);
    }
  };

  const startConsent = async () => {
    setBusy(true);
    try {
      const res = await api.postJson<ConsentStart>(paths.identity.consent, {
        tenant,
        client_id: clientId,
        client_secret: clientSecret,
        redirect_uri: redirectUri,
        scope,
      });
      setConsent(res);
      toast.success(t("phishing.entra_consent_started"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("phishing.entra_failed"));
    } finally {
      setBusy(false);
    }
  };

  const exchangeConsent = async () => {
    if (!consent?.id) return;
    setBusy(true);
    try {
      const res = await api.postJson<Record<string, unknown>>(paths.identity.consentExchange(consent.id), {});
      setTokenJson(JSON.stringify(res, null, 2));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("phishing.entra_failed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <Banner tone="warning">{t("phishing.entra_lab_only")}</Banner>
      <Card className="p-(--card-spacing) space-y-3">
        <div className="text-sm font-semibold">{t("phishing.entra_shared")}</div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <Label>{t("phishing.entra_tenant")}</Label>
            <Input value={tenant} onChange={(e) => setTenant(e.target.value)} />
          </div>
          <div>
            <Label>{t("phishing.entra_client_id")}</Label>
            <Input value={clientId} onChange={(e) => setClientId(e.target.value)} />
          </div>
          <div className="sm:col-span-2">
            <Label>{t("phishing.entra_scope")}</Label>
            <Input value={scope} onChange={(e) => setScope(e.target.value)} />
          </div>
        </div>
      </Card>

      <Card className="p-(--card-spacing) space-y-3">
        <div className="text-sm font-semibold">{t("phishing.entra_device_title")}</div>
        <p className="text-xs text-muted-foreground">{t("phishing.entra_device_hint")}</p>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" disabled={busy || !clientId} onClick={() => { void startDevice(); }}>{t("phishing.entra_device_start")}</Button>
          <Button size="sm" variant="outline" disabled={busy || !device?.id} onClick={() => { void pollDevice(); }}>{t("phishing.entra_device_poll")}</Button>
        </div>
        {device?.user_code && (
          <Banner tone="info">
            {device.message || `${device.verification_uri} — ${device.user_code}`}
          </Banner>
        )}
      </Card>

      <Card className="p-(--card-spacing) space-y-3">
        <div className="text-sm font-semibold">{t("phishing.entra_consent_title")}</div>
        <p className="text-xs text-muted-foreground">{t("phishing.entra_consent_hint")}</p>
        <div>
          <Label>{t("phishing.entra_redirect")}</Label>
          <Input value={redirectUri} onChange={(e) => setRedirectUri(e.target.value)} placeholder={t("phishing.entra_redirect_ph")} />
        </div>
        <div>
          <Label>{t("phishing.entra_client_secret")}</Label>
          <Input type="password" value={clientSecret} onChange={(e) => setClientSecret(e.target.value)} />
        </div>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" disabled={busy || !clientId || !redirectUri} onClick={() => { void startConsent(); }}>{t("phishing.entra_consent_start")}</Button>
          <Button size="sm" variant="outline" disabled={busy || !consent?.id} onClick={() => { void exchangeConsent(); }}>{t("phishing.entra_consent_exchange")}</Button>
        </div>
        {consent?.authorize_url && (
          <Banner tone="info">
            <a className="underline break-all" href={consent.authorize_url} target="_blank" rel="noreferrer">{consent.authorize_url}</a>
          </Banner>
        )}
      </Card>

      {tokenJson && (
        <Card className="p-(--card-spacing)">
          <div className="text-sm font-semibold mb-2">{t("phishing.entra_token")}</div>
          <Textarea readOnly rows={12} className="font-mono text-xs" value={tokenJson} />
        </Card>
      )}
    </div>
  );
}
