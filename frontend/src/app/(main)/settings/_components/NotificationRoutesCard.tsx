"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Spinner } from "@/components/ui/spinner";
import { FlaskConical, Route as RouteIcon, Trash2 } from "lucide-react";

interface NotificationRoute {
  id?: number;
  name: string;
  channel: "discord" | "telegram" | "webhook" | string;
  target: string;
  secret: string;
  min_severity: string;
  enabled: boolean;
}

const emptyRoute = (): NotificationRoute => ({
  name: "",
  channel: "discord",
  target: "",
  secret: "",
  min_severity: "info",
  enabled: true,
});

const SEVERITIES = ["info", "warning", "critical"] as const;

export default function NotificationRoutesCard() {
  const { t } = useI18n();
  const { confirm, modal } = useConfirm();
  const [routes, setRoutes] = useState<NotificationRoute[]>([]);
  const [loading, setLoading] = useState(true);
  const [savingId, setSavingId] = useState<string | number | null>(null);
  const [testingId, setTestingId] = useState<string | number | null>(null);

  const load = useCallback(async () => {
    try {
      const d = await api.get<{ routes?: NotificationRoute[] }>(paths.settings.notificationRoutes);
      setRoutes(d.routes || []);
    } catch {
      setRoutes([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const addRoute = () => setRoutes(prev => [...prev, emptyRoute()]);
  const removeLocal = (idx: number) => setRoutes(prev => prev.filter((_, i) => i !== idx));

  const update = (idx: number, patch: Partial<NotificationRoute>) => {
    setRoutes(prev => prev.map((r, i) => (i === idx ? { ...r, ...patch } : r)));
  };

  const handleCreate = async (idx: number) => {
    const r = routes[idx];
    if (!r?.name || !r.target) { toast.error(t("settings.routes.toast_missing_fields")); return; }
    setSavingId(`new-${idx}`);
    try {
      await api.postJson(paths.settings.notificationRoutes, r);
      toast.success(t("settings.routes.toast_created"));
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("settings.routes.toast_save_failed"));
    } finally {
      setSavingId(null);
    }
  };

  const handleUpdate = async (r: NotificationRoute) => {
    if (!r.id) return;
    setSavingId(r.id);
    try {
      await api.putJson(paths.settings.notificationRoute(r.id), r);
      toast.success(t("settings.routes.toast_updated"));
    } catch {
      toast.error(t("settings.routes.toast_save_failed"));
    } finally {
      setSavingId(null);
    }
  };

  const handleDelete = async (r: NotificationRoute) => {
    if (!r.id) return;
    if (!(await confirm({ message: t("settings.routes.confirm_delete", { name: r.name }) }))) return;
    try {
      await api.del(paths.settings.notificationRoute(r.id));
      toast.success(t("settings.routes.toast_deleted"));
      void load();
    } catch {
      toast.error(t("settings.routes.toast_delete_failed"));
    }
  };

  const handleTest = async (r: NotificationRoute) => {
    if (!r.id) return;
    setTestingId(r.id);
    try {
      await api.postJson(paths.settings.notificationRouteTest(r.id), {});
      toast.success(t("settings.routes.toast_test_sent"));
    } catch {
      toast.error(t("settings.routes.toast_test_failed"));
    } finally {
      setTestingId(null);
    }
  };

  return (
    <Card className="overflow-hidden">
      {modal}
      <CardHeaderRow icon={RouteIcon} tone="warning" accent={false}
        title={t("settings.routes.title")}
        description={t("settings.routes.subtitle")} />
      <div className="p-(--card-spacing) space-y-4">
        {loading ? (
          <div className="py-8 text-center"><Spinner /></div>
        ) : (
          <>
            {routes.length === 0 && (
              <p className="text-xs text-muted-foreground text-center py-4">{t("settings.routes.empty")}</p>
            )}
            {routes.map((route, idx) => (
              <div key={route.id ?? `new-${idx}`} className="p-4 bg-muted rounded-lg space-y-3 border border-border">
                {!route.id && (
                  <p className="text-(--fs-micro-sm) font-medium text-warning">{t("settings.routes.new_badge")}</p>
                )}
                <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                  <div>
                    <Label className="text-(--fs-micro-sm) text-muted-foreground">{t("settings.routes.name")}</Label>
                    <Input className="text-xs mt-1" value={route.name} onChange={(e) => update(idx, { name: e.target.value })}
                      placeholder="soc-discord" />
                  </div>
                  <div>
                    <Label className="text-(--fs-micro-sm) text-muted-foreground">{t("settings.routes.channel")}</Label>
                    <Select value={route.channel} onValueChange={(v) => update(idx, { channel: v ?? "webhook" })}>
                      <SelectTrigger className="text-xs mt-1"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectItem value="discord">Discord</SelectItem>
                        <SelectItem value="telegram">Telegram</SelectItem>
                        <SelectItem value="webhook">{t("settings.routes.channel_webhook")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div>
                    <Label className="text-(--fs-micro-sm) text-muted-foreground">{t("settings.routes.min_severity")}</Label>
                    <Select value={route.min_severity} onValueChange={(v) => update(idx, { min_severity: v ?? "info" })}>
                      <SelectTrigger className="text-xs mt-1"><SelectValue /></SelectTrigger>
                      <SelectContent>
                        {SEVERITIES.map((s) => (
                          <SelectItem key={s} value={s}>{t(`settings.routes.sev_${s}`)}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <Label className="text-(--fs-micro-sm) text-muted-foreground">
                      {route.channel === "telegram" ? t("settings.routes.chat_id") : t("settings.routes.webhook_url")}
                    </Label>
                    <Input className="text-xs mt-1 font-mono"
                      placeholder={route.channel === "telegram" ? "-1001234567890" : "https://discord.com/api/webhooks/..."}
                      value={route.target} onChange={(e) => update(idx, { target: e.target.value })} />
                  </div>
                  {route.channel === "telegram" && (
                    <div>
                      <Label className="text-(--fs-micro-sm) text-muted-foreground">{t("settings.routes.bot_token")}</Label>
                      <Input className="text-xs mt-1 font-mono" type="password"
                        placeholder="123456:ABC-DEF..."
                        value={route.secret} onChange={(e) => update(idx, { secret: e.target.value })} />
                    </div>
                  )}
                  <div className="flex items-end gap-2">
                    <label className="flex items-center gap-2 cursor-pointer select-none">
                      <Switch checked={route.enabled} onCheckedChange={(v) => update(idx, { enabled: v === true })} />
                      <span className="text-xs text-muted-foreground">{route.enabled ? t("settings.notifications.enabled_option") : t("settings.notifications.disabled_option")}</span>
                    </label>
                  </div>
                </div>

                <div className="flex items-center justify-end gap-2 pt-1">
                  {route.id ? (
                    <>
                      <Button variant="ghost" size="sm" onClick={() => void handleTest(route)}
                        disabled={testingId === route.id}
                        className="text-info gap-1.5">
                        {testingId === route.id ? <Spinner size="xs" /> : <FlaskConical className="size-4" />}
                        {t("settings.notifications.test")}
                      </Button>
                      <Button variant="destructive" size="sm" onClick={() => void handleDelete(route)} className="gap-1.5">
                        <Trash2 className="size-4" /> {t("common.delete")}
                      </Button>
                      <Button size="sm" onClick={() => void handleUpdate(route)} disabled={savingId === route.id} className="gap-1.5">
                        {savingId === route.id ? <Spinner size="xs" /> : null}
                        {t("common.save")}
                      </Button>
                    </>
                  ) : (
                    <>
                      <Button variant="outline" size="sm" onClick={() => removeLocal(idx)}>{t("common.cancel")}</Button>
                      <Button size="sm" onClick={() => void handleCreate(idx)} disabled={savingId === `new-${idx}`} className="gap-1.5">
                        {savingId === `new-${idx}` ? <Spinner size="xs" /> : null}
                        {t("settings.routes.create_btn")}
                      </Button>
                    </>
                  )}
                </div>
              </div>
            ))}

            <div className="flex justify-start pt-1">
              <Button onClick={addRoute} className="gap-1.5 text-xs">
                + {t("settings.routes.add")}
              </Button>
            </div>
            <p className="text-(--fs-micro-sm) text-muted-foreground">{t("settings.routes.hint")}</p>
          </>
        )}
      </div>
    </Card>
  );
}
