"use client";
import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { formatTime } from "@/lib/utils";
import { Plus, Pencil, Trash2, RefreshCw } from "lucide-react";

interface SIEMRule {
  id: number;
  name: string;
  enabled: boolean;
  action: string;
  window_sec: number;
  threshold: number;
  alert_action: string;
  alert_details: string;
  updated_at: string;
}

interface SIEMRuleList {
  rules: SIEMRule[];
}

interface RuleForm {
  name: string;
  enabled: boolean;
  action: string;
  window_sec: number;
  threshold: number;
  alert_action: string;
  alert_details: string;
}

const emptyForm: RuleForm = {
  name: "",
  enabled: true,
  action: "",
  window_sec: 300,
  threshold: 5,
  alert_action: "siem_alert",
  alert_details: "",
};

export default function SIEMRulesSection() {
  const { t } = useI18n();
  const [rules, setRules] = useState<SIEMRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<number | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<RuleForm>(emptyForm);
  const [saving, setSaving] = useState(false);
  const { confirm, modal } = useConfirm();

  const fetchRules = useCallback(async () => {
    try {
      const data: SIEMRuleList = await api.get(paths.siem.rules);
      setRules(data.rules || []);
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  useEffect(() => { fetchRules(); }, [fetchRules]);

  const set = (patch: Partial<RuleForm>) => setForm((f) => ({ ...f, ...patch }));

  const startAdd = () => {
    setForm(emptyForm);
    setEditing(null);
    setShowForm(true);
  };

  const startEdit = (rule: SIEMRule) => {
    setForm({
      name: rule.name,
      enabled: rule.enabled,
      action: rule.action,
      window_sec: rule.window_sec,
      threshold: rule.threshold,
      alert_action: rule.alert_action,
      alert_details: rule.alert_details,
    });
    setEditing(rule.id);
    setShowForm(true);
  };

  const handleSave = async () => {
    if (!form.name || !form.action || !form.window_sec || !form.threshold) {
      toast.error(t("settings.toast.siem_required"));
      return;
    }
    setSaving(true);
    try {
      const payload = { ...form, window_sec: Number(form.window_sec), threshold: Number(form.threshold) };
      if (editing == null) {
        await api.postJson(paths.siem.rules, payload);
      } else {
        await api.putJson(paths.siem.rule(editing), payload);
      }
      toast.success(t("settings.toast.siem_saved"));
      setShowForm(false);
      fetchRules();
    } catch {
      toast.error(t("settings.toast.channel_config_failed"));
    }
    setSaving(false);
  };

  const handleToggle = async (rule: SIEMRule) => {
    try {
      await api.postJson(paths.siem.toggle(rule.id), {});
      fetchRules();
    } catch { /* ignore */ }
  };

  const handleDelete = async (id: number) => {
    if (!(await confirm({ message: t("settings.siem.delete_confirm") }))) return;
    try {
      await api.del(paths.siem.rule(id));
      toast.success(t("settings.toast.siem_deleted"));
      fetchRules();
    } catch { /* ignore */ }
  };

  return (
    <Card className="p-(--card-spacing) space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-semibold text-sm">{t("settings.siem")}</h3>
          <p className="text-xs text-muted-foreground">{t("settings.siemDesc")}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={fetchRules} className="rounded-lg">
            <RefreshCw className="size-4" />
          </Button>
          <Button size="sm" onClick={startAdd} className="rounded-lg">
            <Plus className="size-4" /> {t("settings.siem.addRule")}
          </Button>
        </div>
      </div>

      {rules.length === 0 && !loading && (
        <p className="text-xs text-muted-foreground text-center py-4">{t("settings.siem.noRules")}</p>
      )}

      {modal}

      {rules.map(rule => (
        <div key={rule.id} className="flex items-start justify-between p-3 rounded-lg bg-muted/50">
          <div className="flex items-center gap-3">
            <Switch checked={rule.enabled} onCheckedChange={() => handleToggle(rule)} />
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium">{rule.name}</span>
                <Badge variant="outline" className="text-xs">{rule.action}</Badge>
                <Badge className="text-xs">{rule.threshold} / {rule.window_sec}s</Badge>
                {!rule.enabled && <Badge variant="outline" className="text-xs text-muted-foreground">off</Badge>}
              </div>
              {rule.alert_details && (
                <p className="text-xs text-muted-foreground mt-1">{rule.alert_details}</p>
              )}
              <p className="text-xs text-muted-foreground mt-0.5">
                {t("settings.siem.alertAction")}: {rule.alert_action}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5">
                {t("settings.siem.updated")} {formatTime(rule.updated_at)}
              </p>
            </div>
          </div>
          <div className="flex gap-1">
            <Button variant="ghost" size="sm" onClick={() => startEdit(rule)} className="rounded-lg">
              <Pencil className="size-4" />
            </Button>
            <Button variant="ghost" size="sm" onClick={() => handleDelete(rule.id)} className="rounded-lg text-destructive hover:text-destructive">
              <Trash2 className="size-4" />
            </Button>
          </div>
        </div>
      ))}

      {showForm && (
        <div className="space-y-3 p-4 rounded-lg border bg-muted/30">
          <div className="flex items-center justify-between">
            <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {editing == null ? t("settings.siem.addRule") : t("settings.siem.edit")}
            </h4>
            <div className="flex items-center gap-2">
              <Label className="text-xs">{t("settings.siem.enabled")}</Label>
              <Switch checked={form.enabled} onCheckedChange={(v) => set({ enabled: v })} />
            </div>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label required htmlFor="siem-name" className="text-xs">{t("settings.siem.name")}</Label>
              <Input id="siem-name" value={form.name} onChange={(e) => set({ name: e.target.value })} placeholder={t("settings.siem.namePlaceholder")} className="h-8 text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label required htmlFor="siem-action" className="text-xs">{t("settings.siem.action")}</Label>
              <Input id="siem-action" value={form.action} onChange={(e) => set({ action: e.target.value })} placeholder={t("settings.siem.actionPlaceholder")} className="h-8 text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label required htmlFor="siem-window" className="text-xs">{t("settings.siem.windowSec")}</Label>
              <Input id="siem-window" type="number" min={1} value={form.window_sec} onChange={(e) => set({ window_sec: Number(e.target.value) || 0 })} className="h-8 text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label required htmlFor="siem-threshold" className="text-xs">{t("settings.siem.threshold")}</Label>
              <Input id="siem-threshold" type="number" min={1} value={form.threshold} onChange={(e) => set({ threshold: Number(e.target.value) || 0 })} className="h-8 text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="siem-alert-action" className="text-xs">{t("settings.siem.alertAction")}</Label>
              <Input id="siem-alert-action" value={form.alert_action} onChange={(e) => set({ alert_action: e.target.value })} className="h-8 text-xs" />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="siem-alert-details" className="text-xs">{t("settings.siem.alertDetails")}</Label>
              <Input id="siem-alert-details" value={form.alert_details} onChange={(e) => set({ alert_details: e.target.value })} className="h-8 text-xs" />
            </div>
          </div>
          <div className="flex gap-2">
            <Button size="sm" onClick={handleSave} disabled={saving} className="rounded-lg">
              {saving ? t("settings.siem.saving") : t("settings.siem.save")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setShowForm(false)} className="rounded-lg">
              {t("settings.siem.cancel")}
            </Button>
          </div>
        </div>
      )}
    </Card>
  );
}
