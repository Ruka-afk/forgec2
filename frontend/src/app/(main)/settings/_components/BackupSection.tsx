"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Archive, Clock, Download, HardDrive, RefreshCw, Upload } from "lucide-react";
import { EmptyState, Spinner } from "@/components/UI";
import { formatTime } from "@/lib/utils";

interface BackupInfo {
  name: string;
  size: number;
  mod_time: string;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

export default function BackupSection() {
  const { t } = useI18n();
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [restoring, setRestoring] = useState<string | null>(null);
  const [confirmRestore, setConfirmRestore] = useState<BackupInfo | null>(null);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadBackups = useCallback(async () => {
    setLoading(true);
    try {
      const d = await api.get<{ data?: BackupInfo[] }>(paths.settings.dbBackups);
      setBackups(d.data ?? []);
    } catch {
      toast.error(t("settings.toast.load_backups_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void loadBackups(); }, [loadBackups]);

  const handleCreateBackup = async () => {
    setCreating(true);
    try {
      await api.post(paths.settings.dbBackup);
      toast.success(t("settings.toast.backup_created"));
      await loadBackups();
    } catch {
      toast.error(t("settings.toast.create_backup_failed"));
    } finally {
      setCreating(false);
    }
  };

  const handleRestoreFromServer = async (name: string) => {
    setRestoring(name);
    try {
      const d = await api.post<{ message?: string; restart?: boolean }>(paths.settings.dbRestore, { type: "file", name });
      toast.success(d.message ?? t("settings.toast.db_restored"));
      if (d.restart) {
        toast.info(t("settings.toast.server_restarting"), { duration: 5000 });
        setTimeout(() => { window.location.reload(); }, 3000);
      }
    } catch {
      toast.error(t("settings.toast.restore_db_failed"));
    } finally {
      setRestoring(null);
      setConfirmRestore(null);
    }
  };

  const handleUploadRestore = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const fd = new FormData();
      fd.append("type", "upload");
      fd.append("file", file);
      const d = await api.postFormData<{ message?: string; restart?: boolean }>(paths.settings.dbRestore, fd);
      toast.success(d.message ?? t("settings.toast.db_restored"));
      if (d.restart) {
        toast.info(t("settings.toast.server_restarting"), { duration: 5000 });
        setTimeout(() => { window.location.reload(); }, 3000);
      }
    } catch {
      toast.error(t("settings.toast.restore_upload_failed"));
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  return (
    <Card className="overflow-hidden">
      <div className="bg-amber-500/10 border-b border-amber-500/20 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center">
            <Archive className="w-4 h-4 text-amber-600 dark:text-amber-400" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-foreground">{t("settings.backup.title")}</h2>
            <p className="text-xs text-amber-100">{t("settings.backup.subtitle")}</p>
          </div>
        </div>
      </div>

      <div className="p-4 sm:p-5 space-y-5">
        <div className="flex flex-wrap gap-3">
          <Button onClick={handleCreateBackup} disabled={creating} className="px-4 h-10 bg-primary/10 hover:bg-primary/20 text-primary rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            {creating ? <Spinner size="xs" /> : <Archive className="w-4 h-4" />}
            {t("settings.backup.create")}
          </Button>
          <Button onClick={() => fileInputRef.current?.click()} disabled={uploading} className="px-4 h-10 bg-emerald-100 dark:bg-emerald-900/30 hover:bg-emerald-200 dark:hover:bg-emerald-800 text-emerald-700 dark:text-emerald-400 rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            {uploading ? <Spinner size="xs" /> : <Upload className="w-4 h-4" />}
            {t("settings.backup.upload_restore")}
          </Button>
          <input ref={fileInputRef} type="file" accept=".db,.fbk" className="" onChange={handleUploadRestore} />
          <Button onClick={loadBackups} disabled={loading} variant="ghost" className="px-3 h-10 rounded-xl text-sm text-muted-foreground">
            {loading ? <Spinner size="xs" /> : <RefreshCw className="w-4 h-4" />}
          </Button>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
            <Spinner size="xs" className="mr-2" />{t("settings.backup.loading")}
          </div>
        ) : backups.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 text-muted-foreground text-sm gap-2">
            <EmptyState icon={HardDrive} title={t("settings.backup.empty_title")} message={t("settings.backup.empty_message")} />
          </div>
        ) : (
          <div className="space-y-2">
            {backups.map((b) => (
              <div key={b.name} className="flex items-center justify-between bg-muted rounded-xl px-4 py-3 border border-border hover:border-primary/20 transition-colors">
                <div className="flex items-center gap-3 min-w-0">
                  <Archive className="w-4 h-4 text-muted-foreground shrink-0" />
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-foreground truncate">{b.name}</div>
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                      <Clock className="w-3 h-3" />
                      <span>{formatTime(b.mod_time)}</span>
                      <span>·</span>
                      <span>{formatSize(b.size)}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Button variant="ghost" size="sm" onClick={() => {
                    window.open(paths.settings.dbBackupsDownload(b.name), "_blank");
                  }} className="h-8 px-2 text-xs text-muted-foreground hover:text-foreground">
                    <Download className="w-3.5 h-3.5" />
                  </Button>
                  <Button variant="ghost" size="sm" disabled={restoring === b.name} onClick={() => setConfirmRestore(b)} className="h-8 px-3 text-xs text-amber-600 dark:text-amber-400 hover:text-amber-700 hover:bg-amber-50 dark:hover:bg-amber-900/20">
                    {restoring === b.name ? <Spinner size="xs" /> : t("settings.backup.restore")}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <Dialog open={!!confirmRestore} onOpenChange={(open) => { if (!open) setConfirmRestore(null); }}>
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>{t("settings.backup.restore_title")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("settings.backup.restore_message").replace("{name}", confirmRestore?.name ?? "")}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmRestore(null)}>{t("common.cancel")}</Button>
            <Button variant="destructive" onClick={() => { if (confirmRestore) handleRestoreFromServer(confirmRestore.name); }}>
              Restore &amp; Restart
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
