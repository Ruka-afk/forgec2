import { useState } from "react";
import { useI18n } from "@/lib/i18n";
import { SettingsData, PasswordForm } from "./types";
import { ConfirmModal, Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { AlertTriangle, Check, CheckCircle, Key, Lock, QrCode, RotateCw, Save, Shield, X } from "lucide-react";

export default function SecuritySection({
  data, passwordForm, setPasswordForm, totpStatus, totpSecret, totpQR, totpBackupCodes,
  totpCode, setTotpCode, showTotpSetup, totpDisablePassword, setTotpDisablePassword,
  saving, onChangePassword, onRegenerateJWT, onGenerateTOTP, onEnableTOTP, onDisableTOTP, inputCls,
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
  totpDisablePassword: string;
  setTotpDisablePassword: React.Dispatch<React.SetStateAction<string>>;
  saving: boolean;
  onChangePassword: (e: React.FormEvent) => void;
  onRegenerateJWT: () => void;
  onGenerateTOTP: () => void;
  onEnableTOTP: () => void;
  onDisableTOTP: () => void;
  inputCls: string;
}) {
  const { t } = useI18n();
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-muted/50 to-muted px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Lock className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">Security Settings</h2><p className="text-xs text-muted-foreground">Password & two-factor authentication</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5 space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Key className="w-4 h-4" />Change Password</h3>
          <form onSubmit={onChangePassword} className="max-w-md space-y-4">
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Current Password</span>
              <Input aria-label="password" name="input-0" type="password" required value={passwordForm.current} onChange={(e) => setPasswordForm({ ...passwordForm, current: e.target.value })} className={inputCls} />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">New Password</span>
                <Input aria-label="password" name="input-1" type="password" required minLength={8} value={passwordForm.next} onChange={(e) => setPasswordForm({ ...passwordForm, next: e.target.value })} className={inputCls} />
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">Confirm New Password</span>
                <Input aria-label="password" name="input-2" type="password" required minLength={8} value={passwordForm.confirm} onChange={(e) => setPasswordForm({ ...passwordForm, confirm: e.target.value })} className={inputCls} />
              </div>
            </div>
            <Button type="submit" disabled={saving} className="h-11 px-6 rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
              <Save className="w-4 h-4" />Change Password
            </Button>
          </form>
        </div>

        <div className="border-t border-border pt-6">
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Key className="w-4 h-4" />JWT Secret</h3>
          <div className="flex items-center gap-4 p-4 bg-amber-50 dark:bg-amber-900/20 rounded-xl border border-amber-200 dark:border-amber-800">
            <div className="flex-1">
              <div className="text-xs text-amber-600 dark:text-amber-400 font-medium mb-1">Current Key</div>
              <code className="text-sm font-mono text-amber-800 dark:text-amber-300 break-all select-all">{data.jwt_masked ?? "????????"}</code>
            </div>
            <Button onClick={onRegenerateJWT} disabled={saving} className="shrink-0 px-4 h-10 bg-amber-600 hover:bg-amber-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
              <RotateCw className="w-4 h-4" />Regenerate
            </Button>
          </div>
        </div>

        <div className="border-t border-border pt-6">
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Shield className="w-4 h-4" />Two-Factor Authentication (2FA)</h3>

          {totpStatus === null ? (
            <div className="flex items-center gap-3 p-4 bg-muted rounded-xl border border-border">
              <Spinner size="sm" />
              <span className="text-sm text-muted-foreground">Loading TOTP status...</span>
            </div>
          ) : !totpStatus ? (
            <div className="space-y-4">
              <div className="flex items-center justify-between p-4 bg-muted rounded-xl border border-border">
                <div>
                  <div className="text-sm font-medium text-muted-foreground">Two-Factor Authentication</div>
                  <div className="text-xs text-muted-foreground mt-0.5">Use TOTP app for second factor verification</div>
                </div>
                <Badge variant="secondary" className="px-3 py-1.5 text-xs">Disabled</Badge>
              </div>
              {!showTotpSetup ? (
                <Button onClick={onGenerateTOTP} className="h-11 px-6 rounded-xl text-sm font-medium transition-colors">
                  <QrCode className="w-4 h-4" />Setup 2FA
                </Button>
              ) : (
                <div className="space-y-4">
                  <div className="bg-indigo-50 dark:bg-indigo-900/30 border border-indigo-200 dark:border-indigo-800 rounded-xl p-4">
                    <div className="text-xs font-medium text-indigo-700 dark:text-indigo-400 mb-2">Scan QR code or enter key manually</div>
                    <div className="flex items-center gap-4">
                      <div className="w-32 h-32 bg-card rounded-xl border border-indigo-200 dark:border-indigo-800 flex items-center justify-center shrink-0">
                        {totpQR ? <img src={totpQR} alt="QR Code" className="max-w-full max-h-full p-2" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} /> : <QrCode className="w-4 h-4" />}
                      </div>
                      <div>
                        <div className="text-xs text-indigo-600 dark:text-indigo-400 mb-1">Secret Key</div>
                        <code className="text-sm font-mono text-indigo-800 dark:text-indigo-300 break-all">{totpSecret}</code>
                      </div>
                    </div>
                  </div>
                  <div>
                    <span className="block text-xs text-muted-foreground mb-1.5">Verification Code</span>
                    <Input aria-label="123 456" name="123-456-3" type="text" placeholder="123 456" maxLength={8} value={totpCode} onChange={(e) => setTotpCode(e.target.value)} className={inputCls} />
                  </div>
                  {totpBackupCodes && (
                    <div className="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-xl p-3 text-xs text-amber-700 dark:text-amber-400">
                      <AlertTriangle className="w-4 h-4" />
                      Save these backup codes: <div className="mt-2 font-mono font-semibold whitespace-pre-wrap">{totpBackupCodes}</div>
                    </div>
                  )}
                  <Button onClick={onEnableTOTP} disabled={saving} className="h-11 px-6 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
                    <Check className="w-4 h-4" />Enable 2FA
                  </Button>
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              <div className="bg-emerald-50 dark:bg-emerald-900/30 border border-emerald-200 dark:border-emerald-800 rounded-xl p-4">
                <div className="flex items-center gap-2">
                  <CheckCircle className="w-4 h-4" />
                  <span className="text-sm font-medium text-emerald-700 dark:text-emerald-400">Two-Factor Authentication Enabled</span>
                </div>
                <div className="text-xs text-emerald-600 dark:text-emerald-400 mt-1">Verification code required at login</div>
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">Enter password to disable</span>
                <Input aria-label="Current password" name="current-password-4" type="password" placeholder="Current password" value={totpDisablePassword} onChange={(e) => setTotpDisablePassword(e.target.value)} className={inputCls} />
              </div>
              <Button onClick={() => setCfm({msg: t("settings.disable_totp"), cb: () => onDisableTOTP()})} disabled={saving} className="h-11 px-6 bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
                <X className="w-4 h-4" />Disable 2FA
              </Button>
            </div>
          )}
        </div>
      </div>
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.disable")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </Card>
  );
}

