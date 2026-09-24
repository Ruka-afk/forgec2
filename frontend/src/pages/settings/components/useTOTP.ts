import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";

export function useTOTP(t: (key: string) => string, setSaving: (v: boolean) => void, activeSection: string) {
  const [totpStatus, setTotpStatus] = useState<boolean | null>(null);
  const [totpStatusError, setTotpStatusError] = useState<string | null>(null);
  const [totpSecret, setTotpSecret] = useState("");
  const [totpQR, setTotpQR] = useState("");
  const [totpBackupCodes, setTotpBackupCodes] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [showTotpSetup, setShowTotpSetup] = useState(false);
  const [totpEnablePassword, setTotpEnablePassword] = useState("");
  const [totpDisablePassword, setTotpDisablePassword] = useState("");
  const [totpDisableCode, setTotpDisableCode] = useState("");

  const loadTotpStatus = useCallback(async (signal?: AbortSignal) => {
    try {
      const d = await api.get(paths.settings.totpStatus, { signal });
      if (signal?.aborted) return;
      setTotpStatus((d.totp_enabled ?? false) as boolean);
      setTotpStatusError(null);
    } catch (error: unknown) {
      if (signal?.aborted) return;
      if (error instanceof Error && error.name === "AbortError") return;
      // Leave totpStatus null. A failed query means "unknown" — reporting it as
      // false rendered the "2FA is not enabled" panel and invited the operator
      // to believe their account is unprotected.
      setTotpStatusError(t("settings.toast.totp_status_failed"));
    }
  }, [t]);

  useEffect(() => {
    if (activeSection !== "security") return;
    const controller = new AbortController();
    void loadTotpStatus(controller.signal);
    return () => controller.abort();
  }, [activeSection, loadTotpStatus]);

  const handleGenerateTOTP = useCallback(async () => {
    try {
      const d = await api.post(paths.settings.totpGenerate);
      setTotpSecret((d.secret || "") as string);
      setTotpQR((d.qr_url || "") as string);
      setTotpBackupCodes(((d.backup_codes || []) as string[]).join("\n"));
      setShowTotpSetup(true);
    } catch { toast.error(t("settings.toast.totp_generate_failed")); }
  }, [t]);

  const handleEnableTOTP = useCallback(async () => {
    if (!totpEnablePassword) { toast.error(t("settings.toast.totp_enter_password")); return; }
    if (!totpCode) { toast.error(t("settings.toast.totp_enter_code")); return; }
    setSaving(true);
    try {
      await api.post(paths.settings.totpEnable, { password: totpEnablePassword, code: totpCode, secret: totpSecret });
      toast.success(t("settings.toast.totp_enabled"));
      setTotpStatus(true); setShowTotpSetup(false); setTotpCode(""); setTotpEnablePassword("");
    } catch { toast.error(t("settings.toast.totp_enable_failed")); }
    finally { setSaving(false); }
  }, [totpCode, totpSecret, totpEnablePassword, t, setSaving]);

  const handleDisableTOTP = useCallback(async () => {
    if (!totpDisablePassword) { toast.error(t("settings.toast.totp_enter_password")); return; }
    if (!totpDisableCode) { toast.error(t("settings.toast.totp_enter_code")); return; }
    setSaving(true);
    try {
      await api.post(paths.settings.totpDisable, { password: totpDisablePassword, code: totpDisableCode });
      toast.success(t("settings.toast.totp_disabled"));
      setTotpStatus(false); setTotpDisablePassword(""); setTotpDisableCode("");
    } catch { toast.error(t("settings.toast.totp_disable_failed")); }
    finally { setSaving(false); }
  }, [totpDisablePassword, totpDisableCode, t, setSaving]);

  return {
    totpStatus, totpStatusError, reloadTotpStatus: loadTotpStatus,
    totpSecret, totpQR, totpBackupCodes,
    totpCode, setTotpCode, showTotpSetup,
    totpEnablePassword, setTotpEnablePassword,
    totpDisablePassword, setTotpDisablePassword,
    totpDisableCode, setTotpDisableCode,
    handleGenerateTOTP, handleEnableTOTP, handleDisableTOTP,
  };
}
