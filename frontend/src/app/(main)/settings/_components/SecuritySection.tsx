import { useI18n } from "@/lib/i18n";
import { SettingsData, PasswordForm } from "./types";
import { Spinner } from "@/components/ui/spinner";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { AlertTriangle, Check, CheckCircle, Key, Lock, QrCode, RotateCw, Save, Shield, X } from "lucide-react";

export default function SecuritySection({
  data, passwordForm, setPasswordForm, totpStatus, totpSecret, totpQR, totpBackupCodes,
  totpCode, setTotpCode, showTotpSetup,
  totpEnablePassword, setTotpEnablePassword,
  totpDisablePassword, setTotpDisablePassword, totpDisableCode, setTotpDisableCode,
  saving, onChangePassword, onRegenerateJWT, onGenerateTOTP, onEnableTOTP, onDisableTOTP,
}: {
  data: SettingsData;
  passwordForm: PasswordForm;
  setPasswordForm: React.Dispatch<React.SetStateAction<PasswordForm>>;
  totpStatus: boolean | null;
  totpSecret: string;
  totpQR: string;
  totpBackupCodes: string;
  totpCode: string;
  setTotpCode: React.Dispatch<React.SetStateAction<string>>;
  showTotpSetup: boolean;
  totpEnablePassword: string;
  setTotpEnablePassword: React.Dispatch<React.SetStateAction<string>>;
  totpDisablePassword: string;
  setTotpDisablePassword: React.Dispatch<React.SetStateAction<string>>;
  totpDisableCode: string;
  setTotpDisableCode: React.Dispatch<React.SetStateAction<string>>;
  saving: boolean;
  onChangePassword: (e: React.FormEvent) => void;
  onRegenerateJWT: () => void;
  onGenerateTOTP: () => void;
  onEnableTOTP: () => void;
  onDisableTOTP: () => void;
}) {
  const { t } = useI18n();
  const { confirm, modal } = useConfirm();

  return (
    <Card className="overflow-hidden">
      <div className="bg-secondary/60 border-b border-border px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-lg flex items-center justify-center"><Lock className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">{t("settings.security.title")}</h2><p className="text-xs text-muted-foreground">{t("settings.security.subtitle")}</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5 space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Key className="w-4 h-4" />{t("settings.security.change_password")}</h3>
          <form onSubmit={onChangePassword} className="max-w-md space-y-4">
            <div>
              <Label htmlFor="sec-current-pw" className="text-xs text-muted-foreground mb-1.5">{t("settings.security.current_password")}</Label>
              <Input id="sec-current-pw" type="password" value={passwordForm.current} onChange={(e) => setPasswordForm({ ...passwordForm, current: e.target.value })} />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <Label htmlFor="sec-new-pw" className="text-xs text-muted-foreground mb-1.5">{t("settings.security.new_password")}</Label>
                <Input id="sec-new-pw" type="password" minLength={8} value={passwordForm.next} onChange={(e) => setPasswordForm({ ...passwordForm, next: e.target.value })} />
              </div>
              <div>
                <Label htmlFor="sec-confirm-pw" className="text-xs text-muted-foreground mb-1.5">{t("settings.security.confirm_password")}</Label>
                <Input id="sec-confirm-pw" type="password" minLength={8} value={passwordForm.confirm} onChange={(e) => setPasswordForm({ ...passwordForm, confirm: e.target.value })} />
              </div>
            </div>
            <Button type="submit" size="lg" disabled={saving} className="px-6 text-sm font-medium transition-colors disabled:opacity-50">
              <Save className="w-4 h-4" />{t("settings.security.change_password")}
            </Button>
          </form>
        </div>

        <div className="border-t border-border pt-6">
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Key className="w-4 h-4" />{t("settings.security.jwt_secret")}</h3>
          <div className="flex items-center gap-4 p-4 bg-warning/10 rounded-lg border border-warning/20">
            <div className="flex-1">
              <div className="text-xs text-warning font-medium mb-1">{t("settings.security.current_key")}</div>
              <code className="text-sm font-mono text-warning-foreground break-all select-all">{data.jwt_masked ?? "????????"}</code>
            </div>
            <Button onClick={onRegenerateJWT} size="lg" disabled={saving} className="shrink-0 px-4 bg-warning/15 text-warning hover:bg-warning/25 text-sm font-medium transition-colors disabled:opacity-50">
              <RotateCw className="w-4 h-4" />{t("settings.security.regenerate")}
            </Button>
          </div>
        </div>

        <div className="border-t border-border pt-6">
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Shield className="w-4 h-4" />{t("settings.security.two_factor")}</h3>

          {totpStatus === null ? (
            <div className="flex items-center gap-3 p-4 bg-muted rounded-lg border border-border">
              <Spinner size="sm" />
              <span className="text-sm text-muted-foreground">{t("settings.security.loading_totp")}</span>
            </div>
          ) : !totpStatus ? (
            <div className="space-y-4">
              <div className="flex items-center justify-between p-4 bg-muted rounded-lg border border-border">
                <div>
                  <div className="text-sm font-medium text-muted-foreground">{t("settings.security.two_factor_name")}</div>
                  <div className="text-xs text-muted-foreground mt-0.5">{t("settings.security.two_factor_desc")}</div>
                </div>
                <Badge variant="secondary" className="px-3 py-1.5 text-xs">{t("settings.security.disabled")}</Badge>
              </div>
              {!showTotpSetup ? (
                <Button onClick={onGenerateTOTP} size="lg" className="px-6 text-sm font-medium transition-colors">
                  <QrCode className="w-4 h-4" />{t("settings.security.setup_2fa")}
                </Button>
              ) : (
                <div className="space-y-4">
                  <div className="bg-primary/10 dark:bg-primary/20 border border-primary/30 dark:border-primary/40 rounded-lg p-4">
                    <div className="text-xs font-medium text-primary mb-2">{t("settings.security.scan_qr")}</div>
                    <div className="flex items-center gap-4">
                      <div className="w-32 h-32 bg-card rounded-lg border border-primary/30 dark:border-primary/40 flex items-center justify-center shrink-0">
                        {totpQR ? <img src={totpQR} alt={t("settings.security.qr_alt")} className="max-w-full max-h-full p-2" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} /> : <QrCode className="w-4 h-4" />}
                      </div>
                      <div>
                        <div className="text-xs text-primary mb-1">{t("settings.security.secret_key")}</div>
                        <code className="text-sm font-mono text-primary dark:text-primary break-all">{totpSecret}</code>
                      </div>
                    </div>
                  </div>
                  <div>
                    <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.security.current_password")}</span>
                    <Input id="totp-enable-pw" type="password" aria-label={t("settings.security.current_password")} placeholder={t("settings.security.confirm_to_enable")} value={totpEnablePassword} onChange={(e) => setTotpEnablePassword(e.target.value)} />
                  </div>
                  <div>
                    <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.security.verification_code")}</span>
                    <Input id="totp-code" type="text" aria-label={t("settings.security.verification_code")} placeholder="123 456" maxLength={8} value={totpCode} onChange={(e) => setTotpCode(e.target.value)} />
                  </div>
                  {totpBackupCodes && (
                    <div className="bg-warning/10 border border-warning/20 rounded-lg p-3 text-xs text-warning-foreground">
                      <AlertTriangle className="w-4 h-4" />
                      {t("settings.security.save_backup_codes")} <div className="mt-2 font-mono font-semibold whitespace-pre-wrap">{totpBackupCodes}</div>
                    </div>
                  )}
                  <Button onClick={onEnableTOTP} size="lg" disabled={saving} className="px-6 text-sm font-medium transition-colors disabled:opacity-50">
                    <Check className="w-4 h-4" />{t("settings.security.enable_2fa")}
                  </Button>
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              <div className="bg-success/15 border border-success/30 rounded-lg p-4">
                <div className="flex items-center gap-2">
                  <CheckCircle className="w-4 h-4" />
                  <span className="text-sm font-medium text-success">{t("settings.security.2fa_enabled")}</span>
                </div>
                <div className="text-xs text-success mt-1">{t("settings.security.2fa_login_hint")}</div>
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.security.enter_pw_disable")}</span>
                <Input id="totp-disable-pw" type="password" aria-label={t("settings.security.enter_pw_disable")} placeholder={t("settings.security.current_password")} value={totpDisablePassword} onChange={(e) => setTotpDisablePassword(e.target.value)} />
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.security.enter_2fa_code")}</span>
                <Input id="totp-disable-code" type="text" aria-label={t("settings.security.enter_2fa_code")} placeholder="123 456" maxLength={8} value={totpDisableCode} onChange={(e) => setTotpDisableCode(e.target.value)} />
              </div>
              <Button onClick={async () => { if (await confirm({ message: t("settings.disable_totp") })) onDisableTOTP(); }} size="lg" disabled={saving} className="px-6 bg-destructive/10 hover:bg-destructive/20 text-destructive text-sm font-medium transition-colors disabled:opacity-50">
                <X className="w-4 h-4" />{t("settings.security.disable_2fa")}
              </Button>
            </div>
          )}
        </div>
      </div>
      {modal}
    </Card>
  );
}

