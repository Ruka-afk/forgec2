import { SettingsData, PasswordForm } from "./types";

export default function SecuritySection({
  data, passwordForm, setPasswordForm, totpStatus, totpSecret, totpQR, totpBackupCodes,
  totpCode, setTotpCode, showTotpSetup, setShowTotpSetup, totpDisablePassword, setTotpDisablePassword,
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
  setShowTotpSetup: React.Dispatch<React.SetStateAction<boolean>>;
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
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-slate-800 to-slate-900 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-lock text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Security Settings</h2><p className="text-xs text-slate-400">Password & two-factor authentication</p></div>
        </div>
      </div>
      <div className="p-6 space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4 flex items-center gap-2"><i className="fa-solid fa-key text-slate-400"></i>Change Password</h3>
          <form onSubmit={onChangePassword} className="max-w-md space-y-4">
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Current Password</label>
              <input type="password" required value={passwordForm.current} onChange={(e) => setPasswordForm({ ...passwordForm, current: e.target.value })} className={inputCls} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">New Password</label>
                <input type="password" required minLength={8} value={passwordForm.next} onChange={(e) => setPasswordForm({ ...passwordForm, next: e.target.value })} className={inputCls} />
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Confirm New Password</label>
                <input type="password" required minLength={8} value={passwordForm.confirm} onChange={(e) => setPasswordForm({ ...passwordForm, confirm: e.target.value })} className={inputCls} />
              </div>
            </div>
            <button type="submit" disabled={saving} className="h-11 px-6 bg-slate-900 hover:bg-black text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
              <i className="fa-solid fa-save mr-2"></i>Change Password
            </button>
          </form>
        </div>

        <div className="border-t border-[var(--border)] pt-6">
          <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4 flex items-center gap-2"><i className="fa-solid fa-key text-slate-400"></i>JWT Secret</h3>
          <div className="flex items-center gap-4 p-4 bg-amber-50 dark:bg-amber-900/20 rounded-xl border border-amber-200 dark:border-amber-800">
            <div className="flex-1">
              <div className="text-xs text-amber-600 dark:text-amber-400 font-medium mb-1">Current Key</div>
              <code className="text-sm font-mono text-amber-800 dark:text-amber-300 break-all select-all">{data.JWTMasked ?? data.jwt_masked ?? "????????"}</code>
            </div>
            <button onClick={onRegenerateJWT} disabled={saving} className="shrink-0 px-4 h-10 bg-amber-600 hover:bg-amber-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
              <i className="fa-solid fa-rotate mr-1"></i>Regenerate
            </button>
          </div>
        </div>

        <div className="border-t border-[var(--border)] pt-6">
          <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4 flex items-center gap-2"><i className="fa-solid fa-shield-halved text-slate-400"></i>Two-Factor Authentication (2FA)</h3>

          {totpStatus === null ? (
            <div className="flex items-center gap-3 p-4 bg-slate-50 dark:bg-slate-700/50 rounded-xl border border-[var(--border)]">
              <i className="fa-solid fa-circle-notch fa-spin text-slate-400"></i>
              <span className="text-sm text-slate-500">Loading TOTP status...</span>
            </div>
          ) : !totpStatus ? (
            <div className="space-y-4">
              <div className="flex items-center justify-between p-4 bg-slate-50 dark:bg-slate-700/50 rounded-xl border border-[var(--border)]">
                <div>
                  <div className="text-sm font-medium text-[var(--text-secondary)]">Two-Factor Authentication</div>
                  <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Use TOTP app for second factor verification</div>
                </div>
                <span className="inline-flex items-center px-3 py-1.5 text-xs font-medium bg-slate-100 dark:bg-slate-600 text-slate-500 dark:text-slate-400 rounded-xl">Disabled</span>
              </div>
              {!showTotpSetup ? (
                <button onClick={onGenerateTOTP} className="h-11 px-6 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm font-medium transition-colors">
                  <i className="fa-solid fa-qrcode mr-2"></i>Setup 2FA
                </button>
              ) : (
                <div className="space-y-4">
                  <div className="bg-indigo-50 dark:bg-indigo-900/30 border border-indigo-200 dark:border-indigo-800 rounded-xl p-4">
                    <div className="text-xs font-medium text-indigo-700 dark:text-indigo-400 mb-2">Scan QR code or enter key manually</div>
                    <div className="flex items-center gap-4">
                      <div className="w-32 h-32 bg-[var(--card-bg)] rounded-xl border border-indigo-200 dark:border-indigo-800 flex items-center justify-center shrink-0">
                        {totpQR ? <img src={totpQR} alt="QR Code" className="max-w-full max-h-full p-2" /> : <i className="fa-solid fa-qrcode text-4xl text-slate-300"></i>}
                      </div>
                      <div>
                        <div className="text-xs text-indigo-600 dark:text-indigo-400 mb-1">Secret Key</div>
                        <code className="text-sm font-mono text-indigo-800 dark:text-indigo-300 break-all">{totpSecret}</code>
                      </div>
                    </div>
                  </div>
                  <div>
                    <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Verification Code</label>
                    <input type="text" placeholder="123 456" maxLength={8} value={totpCode} onChange={(e) => setTotpCode(e.target.value)} className={inputCls} />
                  </div>
                  {totpBackupCodes && (
                    <div className="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-xl p-3 text-xs text-amber-700 dark:text-amber-400">
                      <i className="fa-solid fa-triangle-exclamation mr-1"></i>
                      Save these backup codes: <div className="mt-2 font-mono font-semibold whitespace-pre-wrap">{totpBackupCodes}</div>
                    </div>
                  )}
                  <button onClick={onEnableTOTP} disabled={saving} className="h-11 px-6 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
                    <i className="fa-solid fa-check mr-2"></i>Enable 2FA
                  </button>
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              <div className="bg-emerald-50 dark:bg-emerald-900/30 border border-emerald-200 dark:border-emerald-800 rounded-xl p-4">
                <div className="flex items-center gap-2">
                  <i className="fa-solid fa-check-circle text-emerald-500"></i>
                  <span className="text-sm font-medium text-emerald-700 dark:text-emerald-400">Two-Factor Authentication Enabled</span>
                </div>
                <div className="text-xs text-emerald-600 dark:text-emerald-400 mt-1">Verification code required at login</div>
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Enter password to disable</label>
                <input type="password" placeholder="Current password" value={totpDisablePassword} onChange={(e) => setTotpDisablePassword(e.target.value)} className={inputCls} />
              </div>
              <button onClick={onDisableTOTP} disabled={saving} className="h-11 px-6 bg-red-100 dark:bg-red-900/30 hover:bg-red-200 dark:hover:bg-red-800 text-red-700 dark:text-red-400 rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
                <i className="fa-solid fa-xmark mr-2"></i>Disable 2FA
              </button>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
