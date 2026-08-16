import { MalleableForm } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import Toggle from "./Toggle";
import { AlertTriangle, Save, Shield } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function MalleableSection({
  form, setForm, saving, onSave,
}: {
  form: MalleableForm;
  setForm: React.Dispatch<React.SetStateAction<MalleableForm>>;
  saving: boolean;
  onSave: (e: React.FormEvent) => void;
}) {
  const { t } = useI18n();
  return (
    <Card className="overflow-hidden">
      <div className="bg-chart-6/violet border-b border-chart-6/violet px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-lg flex items-center justify-center"><Shield className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">{t("settings.malleable.title")}</h2><p className="text-xs text-muted-foreground">{t("settings.malleable.subtitle")}</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <form onSubmit={onSave} className="space-y-4">
          <div className="flex items-center gap-3">
            <Toggle checked={form.enabled} onChange={(v) => setForm({ ...form, enabled: v })} />
            <span className="text-sm text-muted-foreground dark:text-muted-foreground">{form.enabled ? t("settings.malleable.enabled") : t("settings.malleable.disabled")}</span>
            <span className="text-xs text-muted-foreground">{t("settings.malleable.override_hint")}</span>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <Label htmlFor="malleable-http-status-code" className="block text-xs text-muted-foreground mb-1.5">{t("settings.malleable.http_status_code")}</Label>
              <Input id="malleable-http-status-code" type="number" min={100} max={599} value={form.status_code} onChange={(e) => setForm({ ...form, status_code: Number(e.target.value) })}  />
            </div>
            <div>
              <Label htmlFor="malleable-content-type" className="block text-xs text-muted-foreground mb-1.5">{t("settings.malleable.content_type")}</Label>
              <Input id="malleable-content-type" type="text" placeholder="application/json" value={form.content_type} onChange={(e) => setForm({ ...form, content_type: e.target.value })} className="font-mono" />
            </div>
          </div>
          <div>
            <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.malleable.custom_headers")}</span>
            <Textarea rows={3} aria-label={t("settings.malleable.custom_headers")} value={form.headers_text} onChange={(e) => setForm({ ...form, headers_text: e.target.value })} placeholder={"Server: nginx/1.24.0\nX-Powered-By: ASP.NET"}  />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.malleable.prepend")}</span>
              <Textarea rows={2} aria-label={t("settings.malleable.prepend")} value={form.prepend} onChange={(e) => setForm({ ...form, prepend: e.target.value })} placeholder="<html><body><!--"  />
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.malleable.append")}</span>
              <Textarea rows={2} aria-label={t("settings.malleable.append")} value={form.append} onChange={(e) => setForm({ ...form, append: e.target.value })} placeholder="--></body></html>"  />
            </div>
          </div>
          <div className="p-3 bg-warning/10 rounded-lg border border-warning/20 text-xs text-warning-foreground">
            <AlertTriangle className="w-4 h-4" />
            {t("settings.malleable.warning")}
          </div>
          <Button type="submit" size="lg" disabled={saving} className="px-6 text-sm font-medium transition-colors disabled:opacity-50">
            <Save className="w-4 h-4" />{t("settings.malleable.save")}
          </Button>
        </form>
      </div>
    </Card>
  );
}
