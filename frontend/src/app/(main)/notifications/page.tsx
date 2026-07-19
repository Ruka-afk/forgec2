"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { ConfirmModal, EmptyState, PageHeader, Pagination } from "@/components/UI";
import { formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { BellOff, Check, CheckCheck, CheckCircle, CircleAlert, Info, Trash2, XCircle } from "lucide-react";

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

const SEVERITY_VARIANT: Record<string, "success" | "destructive" | "warning" | "default"> = {
  success: "success",
  error: "destructive",
  warning: "warning",
  info: "default",
};

const SEVERITY_ICONS: Record<string, React.ReactNode> = {
  success: <CheckCircle className="w-4 h-4" />,
  error: <XCircle className="w-4 h-4" />,
  warning: <CircleAlert className="w-4 h-4" />,
  info: <Info className="w-4 h-4" />,
};

const NOTIF_TYPES = ["agent_online", "agent_offline", "task_completed", "task_failed"];

export default function NotificationsPage() {
  const { t } = useI18n();
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [typeFilter, setTypeFilter] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");
  const [readFilter, setReadFilter] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [confirmClearAll, setConfirmClearAll] = useState(false);

  const loadNotifications = useCallback(() => {
    setLoading(true);
    const params = new URLSearchParams({ page: String(page), pageSize: "50" });
    if (typeFilter) params.set("type", typeFilter);
    if (severityFilter) params.set("severity", severityFilter);
    if (readFilter) params.set("read", readFilter);
    api.get(`/notifications?${params}`)
      .then((data: { notifications?: NotificationItem[]; total?: number | string }) => {
        setNotifications(data.notifications || []);
        setTotal(Number(data.total) || 0);
      })
      .catch(() => { setNotifications([]); setTotal(0); })
      .finally(() => setLoading(false));
  }, [page, typeFilter, severityFilter, readFilter]);

  useEffect(() => { loadNotifications(); }, [loadNotifications]);

  const handleMarkRead = async (id: number) => {
    await api.put(`/notifications/${id}/read`);
    loadNotifications();
  };

  const handleMarkAllRead = async () => {
    await api.put("/notifications/read-all");
    loadNotifications();
  };

  const handleDelete = async (id: number) => {
    await api.del(`/notifications/${id}`);
    setSelectedIds((prev) => { const n = new Set(prev); n.delete(id); return n; });
    loadNotifications();
  };

  const handleClearAll = async () => {
    await api.del("/notifications");
    setSelectedIds(new Set());
    loadNotifications();
  };

  const handleBulkMarkRead = async () => {
    for (const id of selectedIds) {
      await api.put(`/notifications/${id}/read`);
    }
    setSelectedIds(new Set());
    loadNotifications();
  };

  const toggleSelect = (id: number) => {
    setSelectedIds((prev) => {
      const n = new Set(prev);
      if (n.has(id)) n.delete(id); else n.add(id);
      return n;
    });
  };

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
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
            <SelectTrigger className="w-full sm:w-48" aria-label="Filter by type"><SelectValue placeholder={t("notifications.all_types")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("notifications.all_types")}</SelectItem>
              {NOTIF_TYPES.map((tp) => <SelectItem key={tp} value={tp}>{tp.replace("_", " ")}</SelectItem>)}
            </SelectContent>
          </Select>
          <Select value={severityFilter || "all"} onValueChange={(v) => { setSeverityFilter(v === "all" ? "" : v ?? ""); setPage(1); }}>
            <SelectTrigger className="w-full sm:w-48" aria-label="Filter by severity"><SelectValue placeholder={t("notifications.all_severity")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("notifications.all_severity")}</SelectItem>
              <SelectItem value="success">{t("notifications.success")}</SelectItem>
              <SelectItem value="info">{t("notifications.info")}</SelectItem>
              <SelectItem value="warning">{t("notifications.warning")}</SelectItem>
              <SelectItem value="error">{t("notifications.error")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={readFilter || "all"} onValueChange={(v) => { setReadFilter(v === "all" ? "" : v ?? ""); setPage(1); }}>
            <SelectTrigger className="w-full sm:w-48" aria-label="Filter by status"><SelectValue placeholder={t("notifications.all_status")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("notifications.all_status")}</SelectItem>
              <SelectItem value="false">{t("notifications.unread")}</SelectItem>
              <SelectItem value="true">{t("notifications.read")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </Card>

      <Card className="overflow-hidden">
        {loading ? (
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
        ) : notifications.length > 0 ? (
          <div className="divide-y divide-border">
            {notifications.map((n) => (
              <div key={n.id} className={`flex items-start gap-3 px-4 sm:px-6 py-4 transition-colors ${n.read ? "" : "bg-indigo-50/50 dark:bg-indigo-900/10"}`}>
                <Checkbox
                  checked={selectedIds.has(n.id)}
                  onCheckedChange={() => toggleSelect(n.id)}
                  aria-label={`Select notification ${n.title}`}
                  className="mt-1 shrink-0"
                />
                <div className={`shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-xs ${SEVERITY_VARIANT[n.severity] === "success" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : SEVERITY_VARIANT[n.severity] === "destructive" ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400" : SEVERITY_VARIANT[n.severity] === "warning" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" : "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"}`}>
                  {SEVERITY_ICONS[n.severity] || <Info className="w-4 h-4" />}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="font-medium text-sm">{n.title || n.type}</span>
                    {!n.read && <span className="w-2 h-2 rounded-full bg-indigo-500 shrink-0"></span>}
                  </div>
                  <p className="text-sm text-muted-foreground truncate">{n.message}</p>
                  <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                    <span>{n.created_at ? formatTime(n.created_at) : "-"}</span>
                    {n.agent_id && <span>{t("notifications.agent_prefix")} {n.agent_id.substring(0, 8)}</span>}
                    <Badge variant={SEVERITY_VARIANT[n.severity] || "default"}>{n.severity}</Badge>
                    <span className="font-mono text-[10px]">{n.type}</span>
                  </div>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  {!n.read && (
                    <Button variant="ghost" size="sm" onClick={() => handleMarkRead(n.id)} className="w-8 h-8 p-0" title="Mark read" aria-label="Mark as read">
                      <Check className="w-4 h-4" />
                    </Button>
                  )}
                  <Button variant="ghost" size="sm" onClick={() => handleDelete(n.id)} className="w-8 h-8 p-0 text-muted-foreground hover:text-destructive" title="Delete" aria-label="Delete notification">
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="p-12 text-center text-muted-foreground">
            <EmptyState icon={BellOff} title={t("notifications.empty")} message={t("notifications.empty_hint")} />
          </div>
        )}
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
