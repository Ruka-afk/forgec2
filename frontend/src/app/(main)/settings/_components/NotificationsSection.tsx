"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Bell, FlaskConical, Plus, Save, Trash2 } from "lucide-react";
import NotificationRoutesCard from "./NotificationRoutesCard";

interface NotificationTarget {
  type: "generic" | "slack" | "discord" | "email";
  name: string;
  url: string;
  secret: string;
  to: string;
  smtp_host: string;
  smtp_port: number;
  smtp_user: string;
  smtp_pass: string;
  from: string;
  enabled: boolean;
}

const emptyTarget = (): NotificationTarget => ({
  type: "generic",
  name: "",
  url: "",
  secret: "",
  to: "",
  smtp_host: "",
  smtp_port: 587,
  smtp_user: "",
  smtp_pass: "",
  from: "",
  enabled: true,
});

export default function NotificationsSection() {
  const { t } = useI18n();
  const [targets, setTargets] = useState<NotificationTarget[]>([]);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [testingIdx, setTestingIdx] = useState<number | null>(null);
  useEffect(() => {
    api.get(paths.settings.webhooks)
      .then((d: Record<string, unknown>) => {
        const dd = d.data as Record<string, unknown> | undefined;
        if (dd?.notifications) {
          setTargets(dd.notifications as NotificationTarget[]);
        }
      })
      .catch(() => { /* no saved config yet */ })
      .finally(() => setLoading(false));
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.postJson(paths.settings.webhooks, { notifications: targets });
      toast.success(t("settings.toast.notifications_saved"));
    } catch {
      toast.error(t("settings.toast.notifications_save_failed"));
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async (idx: number) => {
    const target = targets[idx];
    if (!target) return;
    setTestingIdx(idx);
    try {
      const d = await api.postJson(paths.settings.webhooksTest, {
        type: target.type,
        url: target.url,
        secret: target.secret,
        to: target.to,
        smtp_host: target.smtp_host,
        smtp_port: target.smtp_port,
        smtp_user: target.smtp_user,
        smtp_pass: target.smtp_pass,
        from: target.from,
      });
      if (d.success) { toast.success(t("settings.toast.test_notification_sent")); } else { toast.error(((d.error as string) || t("settings.toast.test_failed"))); }
    } catch {
      toast.error(t("settings.toast.test_failed"));
    } finally {
      setTestingIdx(null);
    }
  };

  const update = (idx: number, field: keyof NotificationTarget, value: unknown) => {
    setTargets((prev) => {
      const next = [...prev];
      next[idx] = { ...next[idx], [field]: value };
      return next;
    });
  };

  const addTarget = () => setTargets((prev) => [...prev, emptyTarget()]);
  const removeTarget = (idx: number) => setTargets((prev) => prev.filter((_, i) => i !== idx));

  return (
    <Card className="overflow-hidden">
      {loading ? (
        <div className="p-5 animate-pulse space-y-3">
          <div className="h-4 bg-muted rounded w-1/3" />
          <div className="h-20 bg-muted rounded-lg" />
        </div>
      ) : (<>
      <CardHeaderRow icon={Bell} tone="info" accent={false} title={t("settings.notifications.title")} description={t("settings.notifications.subtitle")} />

      <div className="p-(--card-spacing) space-y-4">
        {targets.length === 0 && (
          <div className="text-center text-xs text-muted-foreground py-6">
            <EmptyState icon={Bell} title={t("settings.notifications.empty_title")} message={t("settings.notifications.empty_message")} />
          </div>
        )}

        {targets.map((target, i) => (
          <div key={`${target.type}-${i}`} className="p-4 bg-muted rounded-lg space-y-3 border border-border">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Select value={target.type} onValueChange={(v) => update(i, "type", v)}>
                  <SelectTrigger className="text-xs">
                    <SelectValue placeholder={t("settings.notifications.select_type")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="generic">{t("settings.notifications.mode_generic")}</SelectItem>
                    <SelectItem value="slack">Slack</SelectItem>
                    <SelectItem value="discord">Discord</SelectItem>
                    <SelectItem value="email">{t("settings.notifications.mode_email")}</SelectItem>
                  </SelectContent>
                </Select>
                <span className="text-(--fs-micro-sm) text-muted-foreground font-mono">{target.type}</span>
              </div>
              <Button onClick={() => removeTarget(i)} className="text-xs px-2 py-1 bg-destructive/10 text-destructive rounded-lg hover:bg-destructive/20" aria-label={t("settings.notifications.remove")}><Trash2 className="size-4" /></Button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div>
                <Label htmlFor={`notif-name-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.name")}</Label>
                <Input id={`notif-name-${i}`} className="text-xs" placeholder={t("settings.notifications.name_placeholder")} value={target.name} onChange={(e) => update(i, "name", e.target.value)} />
              </div>
              <div>
                <span className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.enabled")}</span>
                <Select value={target.enabled ? "true" : "false"} onValueChange={(v) => update(i, "enabled", v === "true")}>
                  <SelectTrigger className="text-xs">
                    <SelectValue placeholder={t("settings.notifications.select_status")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="true">{t("settings.notifications.enabled_option")}</SelectItem>
                    <SelectItem value="false">{t("settings.notifications.disabled_option")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {target.type !== "email" && (
              <div>
                <Label htmlFor={`notif-url-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.webhook_url")}</Label>
                <Input id={`notif-url-${i}`} className="text-xs" placeholder={t("settings.notifications.webhook_url_placeholder")} value={target.url} onChange={(e) => update(i, "url", e.target.value)} />
              </div>
            )}

            {target.type === "generic" && (
              <div>
                <Label htmlFor={`notif-secret-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.hmac_secret")}</Label>
                <Input id={`notif-secret-${i}`} className="text-xs" placeholder={t("settings.notifications.hmac_placeholder")} value={target.secret} onChange={(e) => update(i, "secret", e.target.value)} />
              </div>
            )}

            {target.type === "email" && (
              <>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor={`notif-smtp-host-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.smtp_server")}</Label>
                    <Input id={`notif-smtp-host-${i}`} className="text-xs" placeholder="smtp.gmail.com" value={target.smtp_host} onChange={(e) => update(i, "smtp_host", e.target.value)} />
                  </div>
                  <div>
                    <Label htmlFor={`notif-smtp-port-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.port")}</Label>
                    <Input id={`notif-smtp-port-${i}`} type="number" className="text-xs" placeholder="587" value={target.smtp_port} onChange={(e) => update(i, "smtp_port", parseInt(e.target.value) || 587)} />
                  </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor={`notif-smtp-user-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.username")}</Label>
                    <Input id={`notif-smtp-user-${i}`} className="text-xs" placeholder="user@gmail.com" value={target.smtp_user} onChange={(e) => update(i, "smtp_user", e.target.value)} />
                  </div>
                  <div>
                    <Label htmlFor={`notif-smtp-pass-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.password")}</Label>
                    <Input id={`notif-smtp-pass-${i}`} type="password" className="text-xs" placeholder={t("settings.notifications.password_placeholder")} value={target.smtp_pass} onChange={(e) => update(i, "smtp_pass", e.target.value)} />
                  </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor={`notif-from-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.from")}</Label>
                    <Input id={`notif-from-${i}`} className="text-xs" placeholder="alerts@example.com" value={target.from} onChange={(e) => update(i, "from", e.target.value)} />
                  </div>
                  <div>
                    <Label htmlFor={`notif-to-${i}`} className="text-(--fs-micro-sm) font-medium text-muted-foreground">{t("settings.notifications.to")}</Label>
                    <Input id={`notif-to-${i}`} className="text-xs" placeholder="admin@example.com" value={target.to} onChange={(e) => update(i, "to", e.target.value)} />
                  </div>
                </div>
              </>
            )}

            <div className="flex justify-end pt-1">
              <Button
                onClick={() => handleTest(i)}
                disabled={testingIdx === i}
                className="text-xs px-3 py-1.5 rounded-lg bg-info/15 hover:bg-info/20 text-info dark:bg-info/20 dark:hover:bg-info/40 dark:text-info flex items-center gap-1.5"
              >
                {testingIdx === i ? <Spinner size="xs" /> : <FlaskConical className="size-4" />}
                {testingIdx === i ? t("settings.notifications.sending") : t("settings.notifications.test")}
              </Button>
            </div>
          </div>
        ))}

        <div className="flex items-center justify-between pt-2">
          <Button onClick={addTarget} className="px-3 py-1.5 rounded-lg text-xs font-medium flex items-center gap-1.5">
            <Plus className="size-4" /> {t("settings.notifications.add_target")}
          </Button>
          {targets.length > 0 && (
            <Button onClick={handleSave} disabled={saving} className="px-4 py-1.5 rounded-lg text-xs font-medium flex items-center gap-1.5">
              {saving ? <Spinner size="xs" /> : <Save className="size-4" />}
              {saving ? t("settings.notifications.saving") : t("settings.notifications.save_all")}
            </Button>
          )}
        </div>
      </div>
      </>)}
      <NotificationRoutesCard />
    </Card>
  );
}

