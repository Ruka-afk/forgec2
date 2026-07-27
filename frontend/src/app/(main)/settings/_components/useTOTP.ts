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
  totpDisablePassword: string;
}

export function useTOTP(t: (key: string) => string, saving: boolean, setSaving: (v: boolean) => void, activeSection: string) {
  const [totpStatus, setTotpStatus] = useState<boolean | null>(null);
  const [totpSecret, setTotpSecret] = useState("");
  const [totpQR, setTotpQR] = useState("");
  const [totpBackupCodes, setTotpBackupCodes] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [showTotpSetup, setShowTotpSetup] = useState(false);
  const [totpDisablePassword, setTotpDisablePassword] = useState("");

  useEffect(() => {
    if (activeSection === "security") {
      api.get("/settings/totp/status")
        .then((d: Record<string, unknown>) => setTotpStatus((d.enabled ?? false) as boolean))
        .catch(() => setTotpStatus(false));
    }
  }, [activeSection]);

  const handleGenerateTOTP = useCallback(async () => {
    try {
      const d = await api.post("/settings/totp/generate");
      setTotpSecret((d.secret || "") as string);
      setTotpQR((d.qr || d.qr_code || "") as string);
      setTotpBackupCodes(((d.backup_codes || []) as string[]).join("\n"));
      setShowTotpSetup(true);
    } catch { toast.error(t("settings.toast.totp_generate_failed")); }
  }, [t]);

  const handleEnableTOTP = useCallback(async () => {
    if (!totpCode) { toast.error(t("settings.toast.totp_enter_code")); return; }
    setSaving(true);
    try {
      await api.post("/settings/totp/enable", { code: totpCode, secret: totpSecret });
      toast.success(t("settings.toast.totp_enabled"));
      setTotpStatus(true); setShowTotpSetup(false); setTotpCode("");
    } catch { toast.error(t("settings.toast.totp_enable_failed")); }
    finally { setSaving(false); }
  }, [totpCode, totpSecret, t, setSaving]);

  const handleDisableTOTP = useCallback(async () => {
    if (!totpDisablePassword) { toast.error(t("settings.toast.totp_enter_password")); return; }
    setSaving(true);
    try {
      await api.post("/settings/totp/disable", { password: totpDisablePassword });
      toast.success(t("settings.toast.totp_disabled"));
      setTotpStatus(false); setTotpDisablePassword("");
    } catch { toast.error(t("settings.toast.totp_disable_failed")); }
    finally { setSaving(false); }
  }, [totpDisablePassword, t, setSaving]);

  return {
    totpStatus, totpSecret, totpQR, totpBackupCodes,
    totpCode, setTotpCode, showTotpSetup,
    totpDisablePassword, setTotpDisablePassword,
    handleGenerateTOTP, handleEnableTOTP, handleDisableTOTP,
  };
}
