"use client";

import { useState, useRef, useCallback } from "react";
import { SettingsData } from "./types";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { ErrorState } from "@/components/ui/error-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
      <CardHeaderRow icon={Shield} tone="success" title={t("settings.certificates.title")} description={t("settings.certificates.subtitle")} />
      <div className="p-(--card-spacing)">
        {cert.subject ? (
          <>
            {cert.is_self_signed && (
              <div className="mb-4 px-4 py-3 bg-warning/10 dark:bg-warning/20 border border-warning/20 dark:border-warning/40 rounded-lg text-sm text-warning flex items-center gap-2">
                <AlertTriangle className="size-4 shrink-0" />
                {t("settings.certificates.self_signed_warning")}
              </div>
            )}
            {isExpiringSoon && (
              <ErrorState message={t("settings.certificates.expiring_soon", { days: String(cert.expires_in) })} icon={AlertTriangle} className="mb-4" />
            )}
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mb-6">
              <div className="bg-muted rounded-lg p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.subject")}</div>
                <div className="font-semibold text-sm text-foreground mt-1">{cert.subject}</div>
              </div>
              <div className="bg-muted rounded-lg p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.issuer")}</div>
                <div className="font-semibold text-sm text-foreground mt-1">{cert.issuer}</div>
              </div>
              <div className="bg-muted rounded-lg p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.expires_in")}</div>
                <div className={`font-semibold text-sm mt-1 ${isExpiringSoon ? "text-destructive" : "text-foreground"}`}>
                  {typeof cert.expires_in === "number" ? <>{cert.expires_in} {t("settings.certificates.days")}</> : "-"}
                </div>
              </div>
              <div className="bg-muted rounded-lg p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.dns_names")}</div>
                <div className="font-semibold text-sm text-foreground mt-1">{cert.dns_names?.join(", ") || "-"}</div>
              </div>
              <div className="bg-muted rounded-lg p-4 border border-border">
                <div className="text-xs text-muted-foreground">{t("settings.certificates.type")}</div>
                <div className="font-semibold text-sm text-foreground mt-1 flex items-center gap-1.5">
                  {cert.is_self_signed ? <CheckCircle className="size-3.5 text-warning" /> : <CheckCircle className="size-3.5 text-success" />}
                  {cert.is_self_signed ? t("settings.certificates.self_signed") : t("settings.certificates.trusted")}
                </div>
              </div>
            </div>
          </>
        ) : (
          <div className="text-center py-8 text-muted-foreground/100 text-sm">{t("settings.certificates.no_cert")}</div>
        )}
        <div className="flex flex-wrap gap-3">
          <Input type="file" ref={certInputRef} className="hidden" accept=".crt,.pem,.cer" onChange={handleCertFileChange} />
          <Input type="file" ref={keyInputRef} className="hidden" accept=".key,.pem" onChange={handleKeyFileChange} />
          <Button onClick={() => { setPendingCertFile(null); setWaitingForKey(false); certInputRef.current?.click(); }} size="lg" disabled={uploading || saving} className="px-4 bg-primary/10 hover:bg-primary/20 text-primary text-sm font-medium transition-colors disabled:opacity-50">
            <Upload className="size-4" />{waitingForKey ? t("settings.certificates.choose_key") : t("settings.certificates.upload")}
          </Button>
          <Button onClick={() => setShowRegenDialog(true)} size="lg" disabled={uploading || saving} className="px-4 bg-accent hover:bg-accent/80 text-accent-foreground text-sm font-medium transition-colors disabled:opacity-50">
            <RefreshCw className="size-4" />{t("settings.certificates.regenerate")}
          </Button>
          <Button onClick={loadCertInfo} size="lg" disabled={loadingCert || saving} variant="outline" className="px-4 text-sm font-medium">
            <Globe className="size-4" />{t("settings.certificates.refresh")}
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
