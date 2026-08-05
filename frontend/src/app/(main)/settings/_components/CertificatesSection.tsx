"use client";

import { useState, useRef, useCallback } from "react";
import { SettingsData } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { AlertTriangle, CheckCircle, Globe, RefreshCw, Shield, Upload } from "lucide-react";

interface CertInfo {
  subject?: string;
  issuer?: string;
  expires_at?: string;
  expires_in?: number;
  dns_names?: string[];
  is_self_signed?: boolean;
}

const MAX_FILE_SIZE = 1 * 1024 * 1024; // 1MB

export default function CertificatesSection({
  data, saving, onRefresh,
}: {
  data: SettingsData;
  saving: boolean;
  onRefresh: () => void;
}) {
  const { t } = useI18n();
  const [certInfo, setCertInfo] = useState<CertInfo | null>(null);
  const [loadingCert, setLoadingCert] = useState(false);
  const [showRegenDialog, setShowRegenDialog] = useState(false);
  const [uploading, setUploading] = useState(false);
  const certInputRef = useRef<HTMLInputElement>(null);
  const keyInputRef = useRef<HTMLInputElement>(null);
  const [pendingCertFile, setPendingCertFile] = useState<File | null>(null);
  const [waitingForKey, setWaitingForKey] = useState(false);

  const loadCertInfo = useCallback(async () => {
    setLoadingCert(true);
    try {
      const info = await api.get(paths.settings.certs) as CertInfo;
      setCertInfo(info);
    } catch {
      toast.error(t("settings.toast.cert_load_failed"));
    } finally {
      setLoadingCert(false);
    }
  }, [t]);

  const handleRegenerate = async () => {
    setShowRegenDialog(false);
    setUploading(true);
    try {
      const result = await api.post(paths.settings.certsRegenerate) as CertInfo;
      setCertInfo(result);
      toast.success(t("settings.toast.cert_regenerated"));
      onRefresh();
    } catch {
      toast.error(t("settings.toast.cert_regenerate_failed"));
    } finally {
      setUploading(false);
    }
  };

  const handleUpload = useCallback(async (certFile: File, keyFile: File) => {
    setUploading(true);
    try {
      const formData = new FormData();
      formData.append("cert", certFile);
      formData.append("key", keyFile);
      await api.postFormData(paths.settings.certsUpload, formData);
      toast.success(t("settings.toast.cert_uploaded"));
      loadCertInfo();
      onRefresh();
    } catch {
      toast.error(t("settings.toast.cert_upload_failed"));
    } finally {
      setUploading(false);
      setPendingCertFile(null);
      setWaitingForKey(false);
      if (certInputRef.current) certInputRef.current.value = "";
      if (keyInputRef.current) keyInputRef.current.value = "";
    }
  }, [t, loadCertInfo, onRefresh]);

  const handleCertFileChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const certFile = e.target.files?.[0];
    if (!certFile) return;
    if (certFile.size > MAX_FILE_SIZE) {
      toast.error(t("settings.certificates.file_too_large"));
      e.target.value = "";
      return;
    }
    setPendingCertFile(certFile);
    setWaitingForKey(true);
    setTimeout(() => keyInputRef.current?.click(), 100);
  }, [t]);

  const handleKeyFileChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const keyFile = e.target.files?.[0];
    if (!keyFile) return;
    if (keyFile.size > MAX_FILE_SIZE) {
      toast.error(t("settings.certificates.file_too_large"));
      e.target.value = "";
      return;
    }
    if (pendingCertFile) {
      handleUpload(pendingCertFile, keyFile);
    }
  }, [pendingCertFile, handleUpload, t]);

  const cert = certInfo || {
    subject: data.cert_subject,
    issuer: data.cert_issuer,
    expires_at: data.cert_expires_at,
    expires_in: data.cert_expires_in,
    dns_names: data.cert_dns_names,
    is_self_signed: data.cert_self_signed,
  };

  const isExpiringSoon = typeof cert.expires_in === "number" && cert.expires_in <= 30;

  return (
    <Card className="overflow-hidden">
      <div className="bg-emerald-500/10 border-b border-emerald-500/20 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Shield className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">{t("settings.certificates.title")}</h2><p className="text-xs text-muted-foreground">{t("settings.certificates.subtitle")}</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        {cert.subject ? (
          <>
            {cert.is_self_signed && (
              <div className="mb-4 px-4 py-3 bg-warning/10 dark:bg-amber-900/20 border border-warning/20 dark:border-amber-800 rounded-xl text-sm text-amber-700 dark:text-amber-300 flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                {t("settings.certificates.self_signed_warning")}
              </div>
            )}
            {isExpiringSoon && (
              <div className="mb-4 px-4 py-3 bg-destructive/10 dark:bg-red-900/20 border border-destructive/20 dark:border-red-800 rounded-xl text-sm text-red-700 dark:text-red-300 flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                {t("settings.certificates.expiring_soon", { days: String(cert.expires_in) })}
              </div>
            )}
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mb-6">
              <div className="bg-muted rounded-xl p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.subject")}</div>
                <div className="font-semibold text-sm text-foreground mt-1">{cert.subject}</div>
              </div>
              <div className="bg-muted rounded-xl p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.issuer")}</div>
                <div className="font-semibold text-sm text-foreground mt-1">{cert.issuer}</div>
              </div>
              <div className="bg-muted rounded-xl p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.expires_in")}</div>
                <div className={`font-semibold text-sm mt-1 ${isExpiringSoon ? "text-red-600" : "text-foreground"}`}>
                  {typeof cert.expires_in === "number" ? <>{cert.expires_in} {t("settings.certificates.days")}</> : "-"}
                </div>
              </div>
              <div className="bg-muted rounded-xl p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.dns_names")}</div>
                <div className="font-semibold text-sm text-foreground mt-1">{cert.dns_names?.join(", ") || "-"}</div>
              </div>
              <div className="bg-muted rounded-xl p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.type")}</div>
                <div className="font-semibold text-sm text-foreground mt-1 flex items-center gap-1.5">
                  {cert.is_self_signed ? <CheckCircle className="w-3.5 h-3.5 text-amber-500" /> : <CheckCircle className="w-3.5 h-3.5 text-emerald-500" />}
                  {cert.is_self_signed ? t("settings.certificates.self_signed") : t("settings.certificates.trusted")}
                </div>
              </div>
            </div>
          </>
        ) : (
          <div className="text-center py-8 text-muted-foreground/70 text-sm">{t("settings.certificates.no_cert")}</div>
        )}
        <div className="flex flex-wrap gap-3">
          <input type="file" ref={certInputRef} className="hidden" accept=".crt,.pem,.cer" onChange={handleCertFileChange} />
          <input type="file" ref={keyInputRef} className="hidden" accept=".key,.pem" onChange={handleKeyFileChange} />
          <Button onClick={() => { setPendingCertFile(null); setWaitingForKey(false); certInputRef.current?.click(); }} disabled={uploading || saving} className="px-4 h-10 bg-primary/10 hover:bg-primary/20 text-primary rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <Upload className="w-4 h-4" />{waitingForKey ? t("settings.certificates.choose_key") : t("settings.certificates.upload")}
          </Button>
          <Button onClick={() => setShowRegenDialog(true)} disabled={uploading || saving} className="px-4 h-10 bg-accent hover:bg-accent/80 text-accent-foreground rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <RefreshCw className="w-4 h-4" />{t("settings.certificates.regenerate")}
          </Button>
          <Button onClick={loadCertInfo} disabled={loadingCert || saving} variant="outline" className="px-4 h-10 rounded-xl text-sm font-medium">
            <Globe className="w-4 h-4" />{t("settings.certificates.refresh")}
          </Button>
        </div>
      </div>

      <Dialog open={showRegenDialog} onOpenChange={(v) => { if (v) setShowRegenDialog(true); }}>
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>{t("settings.certificates.confirm_regen_title")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">{t("settings.certificates.confirm_regen_message")}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowRegenDialog(false)}>{t("common.cancel")}</Button>
            <Button onClick={handleRegenerate} disabled={uploading}>{t("settings.certificates.regenerate")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
