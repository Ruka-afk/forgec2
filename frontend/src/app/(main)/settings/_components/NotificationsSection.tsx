"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { EmptyState, Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Bell, FlaskConical, Plus, Save, Trash2 } from "lucide-react";

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
    api.get("/settings/webhooks")
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
      await api.postJson("/settings/webhooks", { notifications: targets });
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
      const d = await api.postJson("/settings/webhooks/test", {
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
    <Card className="rounded-2xl overflow-hidden">
      {loading ? (
        <div className="p-5 animate-pulse space-y-3">
          <div className="h-4 bg-muted rounded w-1/3" />
          <div className="h-20 bg-muted rounded-xl" />
        </div>
      ) : (<>
      <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border flex items-center gap-3">
        <div className="w-8 h-8 bg-sky-100 dark:bg-sky-900/30 rounded-xl flex items-center justify-center text-sky-600"><Bell className="w-4 h-4" /></div>
        <div>
          <h2 className="text-sm font-semibold">Notification Targets</h2>
          <p className="text-(--font-size-xs-sm) text-muted-foreground">Configure default webhook targets for Slack, Discord, Email</p>
        </div>
      </div>

      <div className="p-4 sm:p-5 space-y-4">
        {targets.length === 0 && (
          <div className="text-center text-xs text-muted-foreground py-6">
            <EmptyState icon={Bell} title={t("settings.notifications.empty_title")} message={t("settings.notifications.empty_message")} />
          </div>
        )}

        {targets.map((t, i) => (
          <div key={`${t.type}-${i}`} className="p-4 bg-muted rounded-xl space-y-3 border border-border">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Select value={t.type} onValueChange={(v) => update(i, "type", v)}>
                  <SelectTrigger className="h-9 text-xs">
                    <SelectValue placeholder="Select type" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="generic">Generic</SelectItem>
                    <SelectItem value="slack">Slack</SelectItem>
                    <SelectItem value="discord">Discord</SelectItem>
                    <SelectItem value="email">Email</SelectItem>
                  </SelectContent>
                </Select>
                <span className="text-(--font-size-micro-sm) text-muted-foreground font-mono">{t.type}</span>
              </div>
              <Button onClick={() => removeTarget(i)} className="text-xs px-2 py-1 bg-destructive/10 text-destructive rounded-xl hover:bg-destructive/20" aria-label="Remove"><Trash2 className="w-4 h-4" /></Button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div>
                <Label htmlFor={`notif-name-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">Name</Label>
                <Input id={`notif-name-${i}`} className="h-9 text-xs" placeholder="e.g. Slack Ops" value={t.name} onChange={(e) => update(i, "name", e.target.value)} />
              </div>
              <div>
                <span className="text-(--font-size-micro-sm) font-medium text-muted-foreground">Enabled</span>
                <Select value={t.enabled ? "true" : "false"} onValueChange={(v) => update(i, "enabled", v === "true")}>
                  <SelectTrigger className="h-9 text-xs">
                    <SelectValue placeholder="Select status" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="true">Enabled</SelectItem>
                    <SelectItem value="false">Disabled</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {t.type !== "email" && (
              <div>
                <Label htmlFor={`notif-url-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">Webhook URL</Label>
                <Input id={`notif-url-${i}`} className="h-9 text-xs" placeholder="https://hooks.example.com/..." value={t.url} onChange={(e) => update(i, "url", e.target.value)} />
              </div>
            )}

            {t.type === "generic" && (
              <div>
                <Label htmlFor={`notif-secret-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">HMAC Secret (optional)</Label>
                <Input id={`notif-secret-${i}`} className="h-9 text-xs" placeholder="Signing key" value={t.secret} onChange={(e) => update(i, "secret", e.target.value)} />
              </div>
            )}

            {t.type === "email" && (
              <>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor={`notif-smtp-host-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">SMTP Server</Label>
                    <Input id={`notif-smtp-host-${i}`} className="h-9 text-xs" placeholder="smtp.gmail.com" value={t.smtp_host} onChange={(e) => update(i, "smtp_host", e.target.value)} />
                  </div>
                  <div>
                    <Label htmlFor={`notif-smtp-port-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">Port</Label>
                    <Input id={`notif-smtp-port-${i}`} type="number" className="h-9 text-xs" placeholder="587" value={t.smtp_port} onChange={(e) => update(i, "smtp_port", parseInt(e.target.value) || 587)} />
                  </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor={`notif-smtp-user-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">Username</Label>
                    <Input id={`notif-smtp-user-${i}`} className="h-9 text-xs" placeholder="user@gmail.com" value={t.smtp_user} onChange={(e) => update(i, "smtp_user", e.target.value)} />
                  </div>
                  <div>
                    <Label htmlFor={`notif-smtp-pass-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">Password</Label>
                    <Input id={`notif-smtp-pass-${i}`} type="password" className="h-9 text-xs" placeholder="App password" value={t.smtp_pass} onChange={(e) => update(i, "smtp_pass", e.target.value)} />
                  </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <Label htmlFor={`notif-from-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">From Address</Label>
                    <Input id={`notif-from-${i}`} className="h-9 text-xs" placeholder="alerts@example.com" value={t.from} onChange={(e) => update(i, "from", e.target.value)} />
                  </div>
                  <div>
                    <Label htmlFor={`notif-to-${i}`} className="text-(--font-size-micro-sm) font-medium text-muted-foreground">To Address(es)</Label>
                    <Input id={`notif-to-${i}`} className="h-9 text-xs" placeholder="admin@example.com" value={t.to} onChange={(e) => update(i, "to", e.target.value)} />
                  </div>
                </div>
              </>
            )}

            <div className="flex justify-end pt-1">
              <Button
                onClick={() => handleTest(i)}
                disabled={testingIdx === i}
                className="text-xs px-3 py-1.5 rounded-xl bg-sky-100 hover:bg-sky-200 text-sky-700 dark:bg-sky-900/30 dark:hover:bg-sky-800/40 dark:text-sky-300 flex items-center gap-1.5"
              >
                {testingIdx === i ? <Spinner size="xs" /> : <FlaskConical className="w-4 h-4" />}
                {testingIdx === i ? "Sending..." : "Test"}
              </Button>
            </div>
          </div>
        ))}

        <div className="flex items-center justify-between pt-2">
          <Button onClick={addTarget} className="px-3 py-1.5 bg-sky-600 hover:bg-sky-700 text-white rounded-xl text-xs font-medium flex items-center gap-1.5">
            <Plus className="w-4 h-4" /> Add Target
          </Button>
          {targets.length > 0 && (
            <Button onClick={handleSave} disabled={saving} className="px-4 py-1.5 rounded-xl text-xs font-medium flex items-center gap-1.5">
              {saving ? <Spinner size="xs" /> : <Save className="w-4 h-4" />}
              {saving ? "Saving..." : "Save All"}
            </Button>
          )}
        </div>
      </div>
      </>)}
    </Card>
  );
}

