"use client";

import { useCallback, useEffect, useState } from "react";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { AlertTriangle, Power, ShieldCheck } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";

type KillSwitchStatus = {
  armed: boolean;
  triggered_at?: string | null;
  triggered_by?: string | null;
  disarmed_at?: string | null;
  disarmed_by?: string | null;
};

export default function EmergencySection() {
  const { t } = useI18n();
  const [status, setStatus] = useState<KillSwitchStatus | null>(null);
  const [saving, setSaving] = useState(false);
  const [password, setPassword] = useState("");

  const refresh = useCallback(async () => {
    try {
      const d = await api.get(paths.settings.killSwitchStatus);
      setStatus(d as KillSwitchStatus);
    } catch { /* settings is best-effort */ }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const send = useCallback(async (action: "arm" | "disarm") => {
    if (!password) { toast.error(t("settings.emergency.password_required")); return; }
    setSaving(true);
    try {
      await api.post(paths.settings.killSwitch, { action, password });
      toast.success(action === "arm"
        ? t("settings.emergency.armed_toast")
        : t("settings.emergency.disarmed_toast"));
      setPassword("");
      await refresh();
    } catch {
      toast.error(t("settings.emergency.failed", { action }));
    } finally { setSaving(false); }
  }, [password, refresh, t]);

  const armed = !!status?.armed;

  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={AlertTriangle} tone="destructive" title={t("settings.emergency.title")} description={t("settings.emergency.subtitle")} />
      <div className="p-4 sm:p-5 space-y-4">
        <div className="flex items-center justify-between p-4 bg-muted rounded-lg border border-border">
          <div>
            <div className="text-sm font-medium text-muted-foreground">{t("settings.emergency.status_label")}</div>
            <div className="text-xs text-muted-foreground mt-0.5">
              {armed
                ? t("settings.emergency.status_armed")
                : t("settings.emergency.status_safe")}
            </div>
            {armed && status?.triggered_by && (
              <div className="text-xs text-muted-foreground mt-1">
                {t("settings.emergency.triggered_by", { who: status.triggered_by })}
              </div>
            )}
            {!armed && status?.disarmed_by && (
              <div className="text-xs text-muted-foreground mt-1">
                {t("settings.emergency.disarmed_by", { who: status.disarmed_by })}
              </div>
            )}
          </div>
          <div className={`text-xs font-semibold px-3 py-1 rounded-full ${armed ? "bg-destructive/20 text-destructive" : "bg-success/15 text-success"}`}>
            {armed ? t("settings.emergency.badge_armed") : t("settings.emergency.badge_safe")}
          </div>
        </div>

        <div className="p-4 bg-muted rounded-lg border border-border space-y-3">
          <Input
            type="password"
            placeholder={t("settings.emergency.password_placeholder")}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="h-9"
          />
          <div className="flex flex-col sm:flex-row gap-2">
            <Button
              onClick={() => send("arm")}
              disabled={saving || armed}
              variant="destructive"
              className="flex-1"
            >
              <Power className="w-4 h-4" />{t("settings.emergency.arm")}
            </Button>
            <Button
              onClick={() => send("disarm")}
              disabled={saving || !armed}
              variant="outline"
              className="flex-1"
            >
              <ShieldCheck className="w-4 h-4" />{t("settings.emergency.disarm")}
            </Button>
          </div>
          <div className="text-xs text-muted-foreground">{t("settings.emergency.hint")}</div>
        </div>
      </div>
    </Card>
  );
}