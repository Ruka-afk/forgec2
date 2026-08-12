"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { ConfirmModal, PageHeader, Pagination } from "@/components/UI";
import { DataState } from "@/components/ui/data-state";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { BellOff, Check, CheckCheck, Trash2 } from "lucide-react";
import { NotificationRow } from "./_components/NotificationRow";

interface NotificationItem {
  id: number;
  type: string;
  title: string;
  message: string;
  agent_id: string;
  task_id: number;
  severity: string;
  read: boolean;
  created_at: string;
}

const NOTIF_TYPES = ["agent_online", "agent_offline", "task_completed", "task_failed"];

export default function NotificationsPage() {
  const { t } = useI18n();
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [typeFilter, setTypeFilter] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const [readFilter, setReadFilter] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [confirmClearAll, setConfirmClearAll] = useState(false);

  const loadNotifications = useCallback(() => {
    setLoading(true);
    setError(null);
    const params = new URLSearchParams({ page: String(page), pageSize: "50" });
    if (typeFilter) params.set("type", typeFilter);
    if (severityFilter) params.set("severity", severityFilter);
    if (readFilter) params.set("read", readFilter);
    api.get(paths.notifications.list(params.toString()))
      .then((data: { notifications?: NotificationItem[]; total?: number | string }) => {
        setNotifications(data.notifications || []);
        setTotal(Number(data.total) || 0);
      })
      .catch(() => {
        setNotifications([]);
        setTotal(0);
        setError(t("notifications.toast.load_failed"));
        toast.error(t("notifications.toast.load_failed"));
      })
      .finally(() => setLoading(false));
  }, [page, typeFilter, severityFilter, readFilter, t]);

  useEffect(() => { loadNotifications(); }, [loadNotifications]);

  const handleMarkRead = useCallback(async (id: number) => {
    try {
      await api.put(paths.notifications.markRead(id));
      loadNotifications();
    } catch {
      toast.error(t("notifications.toast.mark_read_failed"));
    }
  }, [loadNotifications, t]);

  const handleMarkAllRead = async () => {
    try {
      await api.put(paths.notifications.readAll);
      loadNotifications();
    } catch {
      toast.error(t("notifications.toast.mark_all_read_failed"));
    }
  };

  const handleDelete = useCallback(async (id: number) => {
    try {
      await api.del(paths.notifications.one(id));
      setSelectedIds((prev) => { const n = new Set(prev); n.delete(id); return n; });
      loadNotifications();
    } catch {
      toast.error(t("notifications.toast.delete_failed"));
    }
  }, [loadNotifications, t]);

  const handleClearAll = async () => {
    try {
      await api.del(paths.notifications.root);
      setSelectedIds(new Set());
      loadNotifications();
    } catch {
      toast.error(t("notifications.toast.clear_all_failed"));
    }
  };

  const handleBulkMarkRead = async () => {
    try {
      for (const id of selectedIds) {
        await api.put(paths.notifications.markRead(id));
      }
      setSelectedIds(new Set());
      loadNotifications();
    } catch {
      toast.error(t("notifications.toast.mark_read_failed"));
    }
  };

  const toggleSelect = useCallback((id: number) => {
    setSelectedIds((prev) => {
      const n = new Set(prev);
      if (n.has(id)) n.delete(id); else n.add(id);
      return n;
    });
  }, []);

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("notifications.title")} subtitle={`${total} ${t("notifications.total")}`}>
        <Button onClick={handleMarkAllRead} className="gap-2">
          <CheckCheck className="w-4 h-4" /> {t("notifications.mark_all_read")}
        </Button>
        {selectedIds.size > 0 && (
          <Button onClick={handleBulkMarkRead} className="gap-2">
            <Check className="w-4 h-4" /> {t("notifications.mark_n_read", { count: selectedIds.size })}
          </Button>
        )}
        <Button variant="destructive" onClick={() => setConfirmClearAll(true)} className="gap-2">
          <Trash2 className="w-4 h-4" /> {t("notifications.clear_all")}
        </Button>
      </PageHeader>

      <Card className="p-4 sm:p-5 mb-4">
        <div className="flex flex-wrap gap-3">
          <Select value={typeFilter || "all"} onValueChange={(v) => { setTypeFilter(v === "all" ? "" : v ?? ""); setPage(1); }}>
            <SelectTrigger className="w-full sm:w-48" aria-label={t("notifications.a11y_filter_type")}><SelectValue placeholder={t("notifications.all_types")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("notifications.all_types")}</SelectItem>
              {NOTIF_TYPES.map((tp) => <SelectItem key={tp} value={tp}>{tp.replace("_", " ")}</SelectItem>)}
            </SelectContent>
          </Select>
          <Select value={severityFilter || "all"} onValueChange={(v) => { setSeverityFilter(v === "all" ? "" : v ?? ""); setPage(1); }}>
            <SelectTrigger className="w-full sm:w-48" aria-label={t("notifications.a11y_filter_severity")}><SelectValue placeholder={t("notifications.all_severity")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("notifications.all_severity")}</SelectItem>
              <SelectItem value="success">{t("notifications.success")}</SelectItem>
              <SelectItem value="info">{t("notifications.info")}</SelectItem>
              <SelectItem value="warning">{t("notifications.warning")}</SelectItem>
              <SelectItem value="error">{t("notifications.error")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={readFilter || "all"} onValueChange={(v) => { setReadFilter(v === "all" ? "" : v ?? ""); setPage(1); }}>
            <SelectTrigger className="w-full sm:w-48" aria-label={t("notifications.a11y_filter_status")}><SelectValue placeholder={t("notifications.all_status")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("notifications.all_status")}</SelectItem>
              <SelectItem value="false">{t("notifications.unread")}</SelectItem>
              <SelectItem value="true">{t("notifications.read")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </Card>

      <Card className="overflow-hidden">
        <DataState loading={loading} error={error} onRetry={loadNotifications} empty={!loading && !error && notifications.length === 0} emptyIcon={BellOff} emptyTitle={t("notifications.empty")} emptyMessage={t("notifications.empty_hint")} loadingSkeleton={
          <div className="divide-y divide-border">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="flex items-start gap-3 px-4 sm:px-6 py-4">
                <Skeleton className="w-4 h-4 mt-1 shrink-0 rounded" />
                <Skeleton className="w-8 h-8 shrink-0 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-3/4" />
                  <Skeleton className="h-3 w-1/2" />
                  <div className="flex gap-3">
                    <Skeleton className="h-3 w-20" />
                    <Skeleton className="h-3 w-16" />
                  </div>
                </div>
                <Skeleton className="w-8 h-8 shrink-0 rounded" />
              </div>
            ))}
          </div>
        }>
          <div className="divide-y divide-border">
            {notifications.map((n) => (
              <NotificationRow
                key={n.id}
                item={n}
                isSelected={selectedIds.has(n.id)}
                onToggleSelect={toggleSelect}
                onMarkRead={handleMarkRead}
                onDelete={handleDelete}
                t={t}
              />
            ))}
          </div>
        </DataState>
      </Card>

      <Pagination page={page} pageSize={50} total={total} onPageChange={setPage} />
      <ConfirmModal
        open={confirmClearAll}
        title={t("notifications.clear_all_title")}
        message={t("notifications.clear_all_msg")}
        danger
        confirmText={t("notifications.clear_all_btn")}
        onConfirm={() => { handleClearAll(); setConfirmClearAll(false); }}
        onCancel={() => setConfirmClearAll(false)}
      />
    </div>
  );
}
