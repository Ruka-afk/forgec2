"use client";
import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { EmptyState } from "@/components/ui/empty-state";
import { FieldError } from "@/components/ui/field-error";
import { PageContainer } from "@/components/ui/page-container";
import { ErrorState } from "@/components/ui/error-state";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Bell, Link2, Mail, MessageCircle, Plug, Power, RefreshCw, Shield, Send, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface Integration {
  id?: number;
  type: string;
  name: string;
  enabled: boolean;
  endpoint: string;
  event_count: number;
  last_trigger: string;
  status: string;
  configured?: boolean;
  readonly?: boolean;
}

export default function IntegrationsPage() {
  const { t } = useI18n();
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testResult, setTestResult] = useState("");
  const [loadError, setLoadError] = useState("");

  const [formType, setFormType] = useState("slack");
  const [formName, setFormName] = useState("");
  const [formUrl, setFormUrl] = useState("");
  const [formSecret, setFormSecret] = useState("");
  const [formTo, setFormTo] = useState("");
  const [formSMTPHost, setFormSMTPHost] = useState("");
  const [formSMTPPort, setFormSMTPPort] = useState("587");
  const [formSMTPUser, setFormSMTPUser] = useState("");
  const [formSMTPPass, setFormSMTPPass] = useState("");
  const [formFrom, setFormFrom] = useState("");
  const [formErrors, setFormErrors] = useState<{ name?: string; url?: string; smtp?: string }>({});

  const fetchIntegrations = useCallback(async () => {
    setLoading(true);
    setLoadError("");
    try {
      const data = await api.get<{ integrations?: Integration[] }>("/integrations");
      setIntegrations(data.integrations || []);
    } catch {
      setIntegrations([]);
      setLoadError(t("integrations.load_failed"));
      toast.error(t("integrations.load_failed"));
    }
    setLoading(false);
  }, [t]);

  useEffect(() => { fetchIntegrations(); }, [fetchIntegrations]);

  function resetForm() {
    setFormName("");
    setFormUrl("");
    setFormSecret("");
    setFormTo("");
    setFormSMTPHost("");
    setFormSMTPPort("587");
    setFormSMTPUser("");
    setFormSMTPPass("");
    setFormFrom("");
    setFormErrors({});
  }

  async function saveIntegration() {
    const errors: { name?: string; url?: string; smtp?: string } = {};
    if (!formName.trim()) errors.name = t("integrations.name_required");
    if (formType !== "email" && !formUrl.trim()) errors.url = t("integrations.url_required");
    if (formType === "email") {
      const port = parseInt(formSMTPPort, 10);
      if (!formSMTPHost.trim() || !port || port < 1 || port > 65535) {
        errors.smtp = t("integrations.err_smtp_invalid");
      }
    }
    setFormErrors(errors);
    if (Object.keys(errors).length > 0) return;
    setSaving(true);
    try {
      await api.postJson(paths.integrations.list, {
        type: formType,
        name: formName.trim(),
        url: formUrl,
        secret: formSecret,
        to: formTo,
        smtp_host: formSMTPHost,
        smtp_port: parseInt(formSMTPPort) || 587,
        smtp_user: formSMTPUser,
        smtp_pass: formSMTPPass,
        from: formFrom,
        enabled: true,
        event_type: "all",
      });
      toast.success(t("integrations.toast.saved"));
      resetForm();
      fetchIntegrations();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("integrations.toast.save_failed"));
    }
    setSaving(false);
  }

  async function toggleIntegration(id?: number) {
    if (!id) return;
    try {
      await api.postJson(paths.integrations.toggle(id), {});
      fetchIntegrations();
    } catch {
      toast.error(t("integrations.toast.toggle_failed"));
    }
  }

  async function deleteIntegration(id?: number) {
    if (!id) return;
    try {
      await api.del(paths.integrations.one(id));
      toast.success(t("integrations.toast.deleted"));
      fetchIntegrations();
    } catch {
      toast.error(t("integrations.toast.delete_failed"));
    }
  }

  async function testNotification() {
    setTestResult(t("integrations.sending"));
    try {
      const data = await api.postJson<{ success?: boolean; error?: string }>(paths.settings.webhooksTest, {
        type: formType, url: formUrl, secret: formSecret, to: formTo,
        smtp_host: formSMTPHost, smtp_port: parseInt(formSMTPPort) || 587,
        smtp_user: formSMTPUser, smtp_pass: formSMTPPass, from: formFrom,
      });
      if (data.success) { setTestResult(""); toast.success(t("integrations.toast.test_sent")); }
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
    <PageContainer title={t("integrations.title")} subtitle={t("integrations.subtitle")} actions={<>
        <Button variant="ghost" onClick={fetchIntegrations}><RefreshCw className="w-4 h-4" /> {t("integrations.refresh")}</Button>
      </>}>
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Spinner />
        </div>
      ) : (
        <>
          {loadError && (
            <ErrorState message={loadError} className="mb-4" />
          )}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 sm:gap-5 mb-6">
            {integrations.length === 0 ? (
              <Card className="col-span-full p-12 text-center">
                <EmptyState icon={Plug} title={t("integrations.empty_title")} message={t("integrations.empty_desc")} />
              </Card>
            ) : (
              integrations.map((intg, i) => (
                <Card key={intg.id ?? `ro-${i}`} className="p-3.5 flex items-center gap-3">
                  <div className="w-10 h-10 flex items-center justify-center rounded-xl bg-secondary ring-1 ring-border/50 text-primary">{getIcon(intg.type)}</div>
                  <div className="flex-1 flex flex-col min-w-0">
                    <span className="text-sm font-semibold text-foreground">{intg.name}</span>
                    <span className="text-(--fs-xs-sm) uppercase text-muted-foreground">{intg.type}</span>
                    {intg.endpoint && <span className="text-xs text-muted-foreground truncate">{intg.endpoint}</span>}
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    <Badge variant={intg.enabled ? "success" : "secondary"}>
                      {intg.enabled ? t("integrations.active") : t("integrations.disabled")}
                    </Badge>
                    {!intg.readonly && intg.id ? (
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon-sm" onClick={() => toggleIntegration(intg.id)} aria-label={t("integrations.a11y_toggle")}>
                          <Power className="w-3.5 h-3.5" />
                        </Button>
                        <Button variant="ghost" size="icon-sm" onClick={() => deleteIntegration(intg.id)} aria-label={t("common.delete")}>
                          <Trash2 className="w-3.5 h-3.5 text-destructive" />
                        </Button>
                      </div>
                    ) : null}
                  </div>
                </Card>
              ))
            )}
          </div>

          <Card className="p-4 sm:p-5 mb-6">
            <h3 className="text-sm font-semibold text-foreground m-0">{t("integrations.save_title")}</h3>
            <p className="text-xs text-muted-foreground mt-1 mb-3">{t("integrations.save_desc")}</p>
            <div className="space-y-2">
              <div className="flex flex-col sm:flex-row gap-2">
                <Select value={formType} onValueChange={(v) => setFormType(v ?? "slack")}>
                  <SelectTrigger className="w-full sm:w-48">
                    <SelectValue placeholder="Slack" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="slack">Slack</SelectItem>
                    <SelectItem value="discord">Discord</SelectItem>
                    <SelectItem value="email">{t("integrations.type_email")}</SelectItem>
                    <SelectItem value="telegram">Telegram</SelectItem>
                    <SelectItem value="generic">{t("integrations.type_generic")}</SelectItem>
                    <SelectItem value="webhook">{t("integrations.type_webhook")}</SelectItem>
                    <SelectItem value="jira">JIRA</SelectItem>
                    <SelectItem value="thehive">TheHive</SelectItem>
                  </SelectContent>
                </Select>
                <Input id="int-name" aria-label={t("integrations.a11y_name")} name="int-name" value={formName} onChange={e => { setFormName(e.target.value); if (formErrors.name) setFormErrors({ ...formErrors, name: undefined }); }} placeholder={t("integrations.name_placeholder")} aria-invalid={!!formErrors.name} aria-describedby={formErrors.name ? "int-name-error" : undefined} />
                <Input id="int-url" aria-label={t("integrations.webhook_url")} name="int-url" value={formUrl} onChange={e => { setFormUrl(e.target.value); if (formErrors.url) setFormErrors({ ...formErrors, url: undefined }); }} placeholder={t("integrations.webhook_url")} aria-invalid={!!formErrors.url} aria-describedby={formErrors.url ? "int-name-error" : undefined} />
              </div>
              <FieldError id="int-name-error">{formErrors.name || formErrors.url}</FieldError>
              <div className="flex flex-col sm:flex-row gap-2">
                <Input aria-label={t("integrations.email_to")} name="int-to" value={formTo} onChange={e => setFormTo(e.target.value)} placeholder={t("integrations.email_to")} />
                <Input aria-label={t("integrations.a11y_secret")} name="int-secret" value={formSecret} onChange={e => setFormSecret(e.target.value)} placeholder={t("integrations.secret_token")} />
              </div>
              {formType === "email" && (
                <div className="flex flex-wrap gap-2">
                  <Input id="int-smtp-host" aria-label={t("integrations.smtp_host")} name="smtp-host" value={formSMTPHost} onChange={e => { setFormSMTPHost(e.target.value); if (formErrors.smtp) setFormErrors({ ...formErrors, smtp: undefined }); }} placeholder={t("integrations.smtp_host")} aria-invalid={!!formErrors.smtp} aria-describedby={formErrors.smtp ? "int-smtp-error" : undefined} />
                  <Input id="int-smtp-port" aria-label={t("integrations.smtp_port")} name="smtp-port" value={formSMTPPort} onChange={e => { setFormSMTPPort(e.target.value); if (formErrors.smtp) setFormErrors({ ...formErrors, smtp: undefined }); }} placeholder={t("integrations.smtp_port")} aria-invalid={!!formErrors.smtp} aria-describedby={formErrors.smtp ? "int-smtp-error" : undefined} />
                  <Input aria-label={t("integrations.smtp_user")} name="smtp-user" value={formSMTPUser} onChange={e => setFormSMTPUser(e.target.value)} placeholder={t("integrations.smtp_user")} />
                  <Input aria-label={t("integrations.smtp_pass")} name="smtp-pass" type="password" value={formSMTPPass} onChange={e => setFormSMTPPass(e.target.value)} placeholder={t("integrations.smtp_pass")} />
                  <Input aria-label={t("integrations.from_address")} name="smtp-from" value={formFrom} onChange={e => setFormFrom(e.target.value)} placeholder={t("integrations.from_address")} />
                </div>
              )}
              {formErrors.smtp && <FieldError id="int-smtp-error">{formErrors.smtp}</FieldError>}
              <Button onClick={saveIntegration} disabled={saving}>
                {saving ? <Spinner size="xs" /> : null}
                {t("integrations.save_btn")}
              </Button>
            </div>
          </Card>

          <Card className="p-4 sm:p-5">
            <h3 className="text-sm font-semibold text-foreground m-0">{t("integrations.test_title")}</h3>
            <p className="text-xs text-muted-foreground mt-1 mb-0">{t("integrations.test_desc")}</p>
            <div className="mt-3 flex items-center gap-3">
              <Button variant="outline" onClick={testNotification}>{t("integrations.send_test")}</Button>
              {testResult && (
                <span className="text-xs text-muted-foreground">{testResult}</span>
              )}
            </div>
          </Card>
        </>
      )}
    </PageContainer>
  );
}
