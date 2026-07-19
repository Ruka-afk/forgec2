"use client";

import { useState, useEffect, useRef } from "react";
import { api } from "@/lib/api";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { toast } from "sonner";
import { Archive, Clock, Download, HardDrive, RefreshCw, Trash2, Upload } from "lucide-react";

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

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

export default function BackupSection() {
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [restoring, setRestoring] = useState<string | null>(null);
  const [confirmRestore, setConfirmRestore] = useState<BackupInfo | null>(null);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadBackups = async () => {
    setLoading(true);
    try {
      const d = await api.get("/settings/db/backups") as unknown as { data?: BackupInfo[] };
      setBackups(d.data ?? []);
    } catch {
      toast.error("Failed to load backups");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadBackups(); }, []);

  const handleCreateBackup = async () => {
    setCreating(true);
    try {
      await api.post("/settings/db/backup");
      toast.success("Backup created");
      await loadBackups();
    } catch {
      toast.error("Failed to create backup");
    } finally {
      setCreating(false);
    }
  };

  const handleRestoreFromServer = async (name: string) => {
    setRestoring(name);
    try {
      const d = await api.post("/settings/db/restore", { type: "file", name }) as unknown as { message?: string; restart?: boolean };
      toast.success(d.message ?? "Database restored");
      if (d.restart) {
        toast.info("Server is restarting...", { duration: 5000 });
        setTimeout(() => { window.location.reload(); }, 3000);
      }
    } catch {
      toast.error("Failed to restore database");
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
      const d = await api.postFormData("/settings/db/restore", fd) as unknown as { message?: string; restart?: boolean };
      toast.success(d.message ?? "Database restored");
      if (d.restart) {
        toast.info("Server is restarting...", { duration: 5000 });
        setTimeout(() => { window.location.reload(); }, 3000);
      }
    } catch {
      toast.error("Failed to restore from uploaded file");
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-amber-600 to-orange-600 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center">
            <Archive className="w-4 h-4 text-white" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-white">Backup &amp; Restore</h2>
            <p className="text-xs text-amber-100">Manage database backups and restore points</p>
          </div>
        </div>
      </div>

      <div className="p-4 sm:p-5 space-y-5">
        <div className="flex flex-wrap gap-3">
          <Button onClick={handleCreateBackup} disabled={creating} className="px-4 h-10 bg-primary/10 hover:bg-primary/20 text-primary rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            {creating ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Archive className="w-4 h-4" />}
            Create Backup
          </Button>
          <Button onClick={() => fileInputRef.current?.click()} disabled={uploading} className="px-4 h-10 bg-emerald-100 dark:bg-emerald-900/30 hover:bg-emerald-200 dark:hover:bg-emerald-800 text-emerald-700 dark:text-emerald-400 rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            {uploading ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4" />}
            Upload &amp; Restore
          </Button>
          <input ref={fileInputRef} type="file" accept=".db,.fbk" className="hidden" onChange={handleUploadRestore} />
          <Button onClick={loadBackups} disabled={loading} variant="ghost" className="px-3 h-10 rounded-xl text-sm text-muted-foreground">
            <RefreshCw className={`w-4 h-4 ${loading ? "animate-spin" : ""}`} />
          </Button>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-8 text-muted-foreground text-sm">
            <RefreshCw className="w-4 h-4 animate-spin mr-2" />Loading backups...
          </div>
        ) : backups.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 text-muted-foreground text-sm gap-2">
            <HardDrive className="w-8 h-8 opacity-30" />
            <p>No backups found. Create one to get started.</p>
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
                    const url = `/api/go/settings/db/backups/download?name=${encodeURIComponent(b.name)}`;
                    window.open(url, "_blank");
                  }} className="h-8 px-2 text-xs text-muted-foreground hover:text-foreground">
                    <Download className="w-3.5 h-3.5" />
                  </Button>
                  <Button variant="ghost" size="sm" disabled={restoring === b.name} onClick={() => setConfirmRestore(b)} className="h-8 px-3 text-xs text-amber-600 dark:text-amber-400 hover:text-amber-700 hover:bg-amber-50 dark:hover:bg-amber-900/20">
                    {restoring === b.name ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : "Restore"}
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
            <DialogTitle>Restore Database?</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            This will replace the current database with <span className="font-medium text-foreground">{confirmRestore?.name}</span>.
            The server will restart to apply changes. All unsaved state will be lost.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmRestore(null)}>Cancel</Button>
            <Button variant="destructive" onClick={() => { if (confirmRestore) handleRestoreFromServer(confirmRestore.name); }}>
              Restore &amp; Restart
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
