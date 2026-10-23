import { useI18n } from "@/lib/i18n";
import { SettingsData, PasswordForm } from "./types";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Key, Lock, RotateCw, Save } from "lucide-react";
import { Field } from "@/components/ui/field";

export default function SecuritySection({
  data, passwordForm, setPasswordForm,
  saving, onChangePassword, onRegenerateJWT,
}: {
  data: SettingsData;
  passwordForm: PasswordForm;
  setPasswordForm: React.Dispatch<React.SetStateAction<PasswordForm>>;
  saving: boolean;
  onChangePassword: (e: React.FormEvent) => void;
  onRegenerateJWT: () => void;
}) {
  const { t } = useI18n();

  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={Lock} tone="muted" title={t("settings.security.title")} description={t("settings.security.subtitle")} />
      <div className="p-(--card-spacing) space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Key className="size-4" />{t("settings.security.change_password")}</h3>
          <form onSubmit={onChangePassword} className="max-w-md space-y-4">
            <Field label={t("settings.security.current_password")} htmlFor="sec-current-pw">
              <Input id="sec-current-pw" type="password" value={passwordForm.current} onChange={(e) => setPasswordForm({ ...passwordForm, current: e.target.value })} />
            </Field>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Field label={t("settings.security.new_password")} htmlFor="sec-new-pw">
                <Input id="sec-new-pw" type="password" minLength={8} value={passwordForm.next} onChange={(e) => setPasswordForm({ ...passwordForm, next: e.target.value })} />
              </Field>
              <Field label={t("settings.security.confirm_password")} htmlFor="sec-confirm-pw">
                <Input id="sec-confirm-pw" type="password" minLength={8} value={passwordForm.confirm} onChange={(e) => setPasswordForm({ ...passwordForm, confirm: e.target.value })} />
              </Field>
            </div>
            <Button type="submit" size="lg" disabled={saving} className="px-6 text-sm font-medium transition-colors disabled:opacity-50">
              <Save className="size-4" />{t("settings.security.change_password")}
            </Button>
          </form>
        </div>

        <div className="border-t border-border pt-6">
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Key className="size-4" />{t("settings.security.jwt_secret")}</h3>
          <div className="flex items-center gap-4 p-4 bg-warning/10 rounded-lg border border-warning/20">
            <div className="flex-1">
              <div className="text-xs text-warning-ink font-medium mb-1">{t("settings.security.current_key")}</div>
              <code className="text-sm font-mono text-warning-ink break-all select-all">{data.jwt_masked ?? "????????"}</code>
            </div>
            <Button onClick={onRegenerateJWT} size="lg" disabled={saving} className="shrink-0 px-4 bg-warning/15 text-warning-ink hover:bg-warning/25 text-sm font-medium transition-colors disabled:opacity-50">
              <RotateCw className="size-4" />{t("settings.security.regenerate")}
            </Button>
          </div>
        </div>
      </div>
    </Card>
  );
}
