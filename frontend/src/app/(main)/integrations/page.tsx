"use client";
import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { PageHeader, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Bell, Link2, Mail, MessageCircle, Plug, RefreshCw, Shield, Send } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface Integration {
  type: string;
  name: string;
  enabled: boolean;
  endpoint: string;
  event_count: number;
  last_trigger: string;
  status: string;
}

export default function IntegrationsPage() {
  const { t } = useI18n();
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [loading, setLoading] = useState(true);
  const [testResult, setTestResult] = useState("");

  const [formType, setFormType] = useState("slack");
  const [formUrl, setFormUrl] = useState("");
  const [formSecret, setFormSecret] = useState("");
  const [formTo, setFormTo] = useState("");
  const [formSMTPHost, setFormSMTPHost] = useState("");
  const [formSMTPPort, setFormSMTPPort] = useState("587");
  const [formSMTPUser, setFormSMTPUser] = useState("");
  const [formSMTPPass, setFormSMTPPass] = useState("");
  const [formFrom, setFormFrom] = useState("");

  const fetchIntegrations = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.json("/integrations");
      setIntegrations((data.integrations as Integration[]) || []);
    } catch { setIntegrations([]); }
    setLoading(false);
  }, []);

  useEffect(() => { fetchIntegrations(); }, [fetchIntegrations]);

  async function testNotification() {
    setTestResult(t("integrations.sending"));
    try {
      const data = await api.postJson("/settings/webhooks/test", {
        type: formType, url: formUrl, secret: formSecret, to: formTo,
        smtp_host: formSMTPHost, smtp_port: parseInt(formSMTPPort) || 587,
        smtp_user: formSMTPUser, smtp_pass: formSMTPPass, from: formFrom,
      });
      if (data.success) { setTestResult(""); toast.success("Test sent successfully!"); }
      else { setTestResult((data.error as string) || t("integrations.test_failed")); }
    } catch { setTestResult(t("integrations.network_error")); }
  }

  function getIcon(type: string): React.ReactNode {
    switch (type) {
      case "webhook": return <Link2 className="w-4 h-4" />;
      case "notification": return <Bell className="w-4 h-4" />;
      case "slack": return <MessageCircle className="w-4 h-4" />;
      case "discord": return <MessageCircle className="w-4 h-4" />;
      case "email": return <Mail className="w-4 h-4" />;
      case "telegram": return <Send className="w-4 h-4" />;
      case "jira": return <Shield className="w-4 h-4" />;
      case "thehive": return <Shield className="w-4 h-4" />;
      default: return <Plug className="w-4 h-4" />;
    }
  }

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("integrations.title")} subtitle={t("integrations.subtitle")}>
        <Button variant="ghost" onClick={fetchIntegrations}><RefreshCw className="w-4 h-4" /> {t("integrations.refresh")}</Button>
      </PageHeader>
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Spinner />
        </div>
      ) : (
        <>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 sm:gap-5 mb-6">
            {integrations.length === 0 ? (
              <Card className="col-span-full p-12 text-center">
                <Plug className="w-4 h-4" />
                <h3 className="text-base font-semibold text-foreground mb-1">{t("integrations.empty_title")}</h3>
                <p className="text-sm text-muted-foreground">{t("integrations.empty_desc")}</p>
              </Card>
            ) : (
              integrations.map((intg, i) => (
                <Card key={i} className="p-3.5 flex items-center gap-3">
                   <div className="w-10 h-10 flex items-center justify-center rounded-xl bg-secondary text-lg text-indigo-500">{getIcon(intg.type)}</div>
                  <div className="flex-1 flex flex-col min-w-0">
                    <span className="text-sm font-semibold text-foreground">{intg.name}</span>
                    <span className="text-[11px] uppercase text-muted-foreground">{intg.type}</span>
                    {intg.endpoint && <span className="text-xs text-muted-foreground truncate">{intg.endpoint}</span>}
                  </div>
                  <Badge variant={intg.enabled ? "success" : "secondary"}>
                    {intg.enabled ? t("integrations.active") : t("integrations.disabled")}
                  </Badge>
                </Card>
              ))
            )}
          </div>
<Card className="p-4 sm:p-5">
            <h3 className="text-sm font-semibold text-foreground m-0">{t("integrations.test_title")}</h3>
            <p className="text-xs text-muted-foreground mt-1 mb-0">{t("integrations.test_desc")}</p>
            <div className="mt-3 space-y-2">
              <div className="flex gap-2">
                <Select value={formType} onValueChange={(v) => setFormType(v ?? "slack")}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Slack" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="slack">Slack</SelectItem>
                    <SelectItem value="discord">Discord</SelectItem>
                    <SelectItem value="email">Email</SelectItem>
                    <SelectItem value="telegram">Telegram</SelectItem>
                    <SelectItem value="generic">Generic Webhook</SelectItem>
                    <SelectItem value="jira">JIRA</SelectItem>
                    <SelectItem value="thehive">TheHive</SelectItem>
                  </SelectContent>
                </Select>
                <Input aria-label="Webhook URL" name="input-1" value={formUrl} onChange={e => setFormUrl(e.target.value)} placeholder={t("integrations.webhook_url")} />
              </div>
              <div className="flex gap-2">
                <Input aria-label="Email To (for email)" name="input-2" value={formTo} onChange={e => setFormTo(e.target.value)} placeholder={t("integrations.email_to")} />
                <Input aria-label="Secret / Token" name="input-3" value={formSecret} onChange={e => setFormSecret(e.target.value)} placeholder={t("integrations.secret_token")} />
              </div>
              {formType === "email" && (
                <div className="flex flex-wrap gap-2">
                  <Input aria-label="SMTP Host" name="input-4" value={formSMTPHost} onChange={e => setFormSMTPHost(e.target.value)} placeholder="SMTP Host" />
                  <Input aria-label="SMTP Port" name="input-5" value={formSMTPPort} onChange={e => setFormSMTPPort(e.target.value)} placeholder="SMTP Port" />
                  <Input aria-label="SMTP User" name="input-6" value={formSMTPUser} onChange={e => setFormSMTPUser(e.target.value)} placeholder="SMTP User" />
                  <Input aria-label="SMTP Pass" name="input-7" type="password" value={formSMTPPass} onChange={e => setFormSMTPPass(e.target.value)} placeholder="SMTP Pass" />
                  <Input aria-label="From Address" name="input-8" value={formFrom} onChange={e => setFormFrom(e.target.value)} placeholder="From Address" />
                </div>
              )}
              <div className="flex items-center gap-3">
                <Button onClick={testNotification}>{t("integrations.send_test")}</Button>
                {testResult && (
                  <span className={`text-xs ${testResult === "Sending..." ? "text-muted-foreground" : testResult.includes("success") ? "text-emerald-600" : "text-destructive"}`}>{testResult}</span>
                )}
              </div>
            </div>
          </Card>
        </>
      )}
    </div>
  );
}

