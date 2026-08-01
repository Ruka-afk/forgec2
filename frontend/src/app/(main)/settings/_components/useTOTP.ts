"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";

export interface TOTPState {
  totpStatus: boolean | null;
  totpSecret: string;
  totpQR: string;
  totpBackupCodes: string;
  totpCode: string;
  showTotpSetup: boolean;
  totpEnablePassword: string;
  totpDisablePassword: string;
  totpDisableCode: string;
}

export function useTOTP(t: (key: string) => string, saving: boolean, setSaving: (v: boolean) => void, activeSection: string) {
  const [totpStatus, setTotpStatus] = useState<boolean | null>(null);
  const [totpSecret, setTotpSecret] = useState("");
  const [totpQR, setTotpQR] = useState("");
  const [totpBackupCodes, setTotpBackupCodes] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [showTotpSetup, setShowTotpSetup] = useState(false);
  const [totpEnablePassword, setTotpEnablePassword] = useState("");
  const [totpDisablePassword, setTotpDisablePassword] = useState("");
  const [totpDisableCode, setTotpDisableCode] = useState("");

  useEffect(() => {
    if (activeSection === "security") {
      api.get("/settings/totp/status")
        .then((d: Record<string, unknown>) => setTotpStatus((d.totp_enabled ?? false) as boolean))
        .catch(() => setTotpStatus(false));
    }
  }, [activeSection]);

  const handleGenerateTOTP = useCallback(async () => {
    try {
      const d = await api.post("/settings/totp/generate");
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
      await api.post("/settings/totp/enable", { password: totpEnablePassword, code: totpCode, secret: totpSecret });
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
      await api.post("/settings/totp/disable", { password: totpDisablePassword, code: totpDisableCode });
      toast.success(t("settings.toast.totp_disabled"));
      setTotpStatus(false); setTotpDisablePassword(""); setTotpDisableCode("");
    } catch { toast.error(t("settings.toast.totp_disable_failed")); }
    finally { setSaving(false); }
  }, [totpDisablePassword, totpDisableCode, t, setSaving]);

  return {
    totpStatus, totpSecret, totpQR, totpBackupCodes,
    totpCode, setTotpCode, showTotpSetup,
    totpEnablePassword, setTotpEnablePassword,
    totpDisablePassword, setTotpDisablePassword,
    totpDisableCode, setTotpDisableCode,
    handleGenerateTOTP, handleEnableTOTP, handleDisableTOTP,
  };
}
