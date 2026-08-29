"use client";
import { PageContainer } from "@/components/ui/page-container";

import { useState, useEffect, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { POLL } from "@/lib/polling";
import { toast } from "sonner";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { Pagination } from "@/components/ui/pagination";
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

export default function NotificationsPage({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  const [page, setPage] = useState(1);
  const [typeFilter, setTypeFilter] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const [readFilter, setReadFilter] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [confirmClearAll, setConfirmClearAll] = useState(false);

  const paramsRef = useRef({ page, typeFilter, severityFilter, readFilter });
  paramsRef.current = { page, typeFilter, severityFilter, readFilter };

  const { data, loading, error, refresh: loadNotifications } = useApiResource<{ notifications: NotificationItem[]; total: number }>({
    fetcher: async () => {
      const p = paramsRef.current;
      const params = new URLSearchParams({ page: String(p.page), pageSize: "50" });
      if (p.typeFilter) params.set("type", p.typeFilter);
      if (p.severityFilter) params.set("severity", p.severityFilter);
      if (p.readFilter) params.set("read", p.readFilter);
      const res = await api.get(paths.notifications.list(params.toString()));
      return {
        notifications: (res.notifications || []) as NotificationItem[],
        total: Number(res.total) || 0,
      };
    },
    errorMessage: t("notifications.toast.load_failed"),
    toastThrottleMs: POLL.toastThrottleAlerts,
    retainOnError: false,
  });
  const notifications = data?.notifications ?? [];
  const total = data?.total ?? 0;

  const skipFirstRef = useRef(true);
  useEffect(() => {
    if (skipFirstRef.current) { skipFirstRef.current = false; return; }
    loadNotifications();
  }, [page, typeFilter, severityFilter, readFilter, loadNotifications]);

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
    <PageContainer embedded={embedded} title={!embedded ? t("notifications.title") : undefined} subtitle={!embedded ? `${total} ${t("notifications.total")}` : undefined} actions={<>
        <Button onClick={handleMarkAllRead} className="gap-2">
          <CheckCheck className="size-4" /> {t("notifications.mark_all_read")}
        </Button>
        {selectedIds.size > 0 && (
          <Button onClick={handleBulkMarkRead} className="gap-2">
            <Check className="size-4" /> {t("notifications.mark_n_read", { count: selectedIds.size })}
          </Button>
        )}
        <Button variant="destructive" onClick={() => setConfirmClearAll(true)} className="gap-2">
          <Trash2 className="size-4" /> {t("notifications.clear_all")}
        </Button>
      </>}>
      {embedded && (
        <div className="mb-4 flex flex-wrap justify-end gap-2">
          <Button onClick={handleMarkAllRead} size="sm" className="gap-2">
            <CheckCheck className="size-4" /> {t("notifications.mark_all_read")}
          </Button>
          <Button variant="destructive" size="sm" onClick={() => setConfirmClearAll(true)} className="gap-2">
            <Trash2 className="size-4" /> {t("notifications.clear_all")}
          </Button>
        </div>
      )}

      <Card className="p-(--card-spacing) mb-4">
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
                <Skeleton className="size-4 mt-1 shrink-0 rounded" />
                <Skeleton className="size-8 shrink-0 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-3/4" />
                  <Skeleton className="h-3 w-1/2" />
                  <div className="flex gap-3">
                    <Skeleton className="h-3 w-20" />
                    <Skeleton className="h-3 w-16" />
                  </div>
                </div>
                <Skeleton className="size-8 shrink-0 rounded" />
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
    </PageContainer>
  );
}
