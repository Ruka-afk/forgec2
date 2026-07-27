"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { downloadText } from "@/lib/download";
import { PageHeader, Pagination } from "@/components/UI";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Download, FileText, Filter } from "lucide-react";
import type { AuditLog } from "@/types/audit";

const ACTION_BADGES: Record<string, string> = {
  login: "success",
  create: "secondary",
  update: "warning",
  delete: "destructive",
  logout: "secondary",
  failed: "destructive",
};

const SEVERITY_BADGES: Record<string, string> = {
  info: "secondary",
  warning: "warning",
  error: "destructive",
  critical: "destructive",
};



export default function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [perPage] = useState(50);
  const [search, setSearch] = useState("");
  const [userFilter, setUserFilter] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [users, setUsers] = useState<{ username: string }[]>([]);
  const { t } = useI18n();

  useEffect(() => {
    const controller = new AbortController();
    api.get("/users", { signal: controller.signal })
      .then((data) => {
        const list = (data.users || data.data || []) as { username: string }[];
        setUsers(list);
      })
      .catch(() => {
        // Non-fatal: user filter can still use free text
      });
    return () => controller.abort();
  }, []);

  const loadLogs = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        page: String(page),
        pageSize: String(perPage),
      });
      if (search) params.set("search", search);
      if (userFilter) params.set("user", userFilter);
      if (actionFilter) params.set("action", actionFilter);
      const data = await api.get(`/audit/logs?${params}`, { signal });
      setLogs((data.data as AuditLog[]) || []);
      setTotal((data.total as number) || 0);
    } catch {
      setLogs([]);
      setTotal(0);
      toast.error(t("audit.toast.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [page, perPage, search, userFilter, actionFilter, t]);

  useEffect(() => {
    const controller = new AbortController();
    loadLogs(controller.signal);
    return () => controller.abort();
  }, [loadLogs]);

  const applyFilters = () => { setPage(1); };
  const resetFilters = () => {
    setSearch("");
    setUserFilter("");
    setActionFilter("");
    setPage(1);
  };

  const handleExport = () => {
    const csv = logs.filter(Boolean).map((l) => {
      const time = l.timestamp || "";
      const user = l.username || "";
      const ip = l.ip || "";
      const action = l.action || "";
      const resource = l.resource || "";
      const target = l.target || "";
      const status = l.status || "";
      const severity = l.severity || "";
      const details = (l.details || "").replace(/,/g, ";");
      return `${time},${user},${ip},${action},${resource},${target},${status},${severity},${details}`;
    }).join("\n");
    const header = "Timestamp,User,IP,Action,Resource,Target,Status,Severity,Details\n";
    downloadText(header + csv, "audit-logs.csv", "text/csv");
  };

  const getActionBadge = (action: string) => {
    const a = (action || "").toLowerCase();
    for (const [key, badge] of Object.entries(ACTION_BADGES)) {
      if (a.includes(key)) return badge;
    }
    return "secondary";
  };

  const getSeverityBadge = (severity: string) => {
    const s = (severity || "").toLowerCase();
    return SEVERITY_BADGES[s] || "secondary";
  };

  const getLogField = (log: AuditLog | null, field: keyof AuditLog | "severity") => {
    if (!log) return "";
    switch (field) {
      case "id": return String(log.id || "");
      case "timestamp": return String(log.timestamp || "");
      case "username": return String(log.username || "-");
      case "action": return String(log.action || "-");
      case "resource": return String(log.resource || "-");
      case "target": return String(log.target || "-");
      case "status": return String(log.status || "-");
      case "details": return String(log.details || "-");
      case "ip": return String(log.ip || "-");
      case "severity": return String(log.severity || "info");
      default: return "";
    }
  };

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("audit.title")} subtitle={t("audit.subtitle")}>
        <Button onClick={handleExport}>
          <Download className="w-4 h-4" />
          <span>{t("audit.export")} CSV</span>
        </Button>
      </PageHeader>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("audit.total_records")}</div>
              <div className="mt-2 text-2xl font-bold">{total}</div>
            </div>
            <div className="w-12 h-12 bg-indigo-50 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center">
              <FileText className="w-4 h-4" />
            </div>
          </div>
        </Card>
      </div>

      <Card className="p-4 mb-4">
        <div className="flex flex-wrap items-center gap-3">
          <Select value={actionFilter || "all"} onValueChange={(v) => setActionFilter(v === "all" || !v ? "" : v)}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder={t("audit.all_actions")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("audit.all_actions")}</SelectItem>
              <SelectItem value="login">{t("audit.login")}</SelectItem>
              <SelectItem value="logout">{t("audit.logout")}</SelectItem>
              <SelectItem value="create">{t("audit.create")}</SelectItem>
              <SelectItem value="update">{t("audit.update")}</SelectItem>
              <SelectItem value="delete">{t("audit.delete")}</SelectItem>
              <SelectItem value="failed">{t("audit.failed")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={userFilter || "all"} onValueChange={(v) => setUserFilter(v === "all" || !v ? "" : v)}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder={t("audit.all_users")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("audit.all_users")}</SelectItem>
              {users.map((u) => (
                <SelectItem key={u.username} value={u.username}>{u.username}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button onClick={applyFilters}>
            <Filter className="w-4 h-4" />
            <span>{t("audit.apply")}</span>
          </Button>
          <Button variant="outline" onClick={resetFilters}>
            {t("audit.reset")}
          </Button>
        </div>
      </Card>

      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="border-border">
                <TableHead className="text-left">{t("audit.time")}</TableHead>
                <TableHead className="text-left">{t("audit.user")}</TableHead>
                <TableHead className="text-left">IP</TableHead>
                <TableHead className="text-left">{t("audit.action")}</TableHead>
                <TableHead className="text-left">{t("audit.severity")}</TableHead>
                <TableHead className="text-left">{t("audit.resource")}</TableHead>
                <TableHead className="text-left">Target</TableHead>
                <TableHead className="text-left">{t("audit.status")}</TableHead>
                <TableHead className="text-left">{t("audit.details")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <TableRow key={i}>
                    {Array.from({ length: 9 }).map((_, j) => (
                      <TableCell key={j} className="py-3 px-4"><Skeleton className="h-4 w-full" /></TableCell>
                    ))}
                  </TableRow>
                ))
              ) : !loading && logs.length === 0 ? (
                <TableRow><TableCell colSpan={9} className="py-16 text-center">{t("audit.empty")}</TableCell></TableRow>
              ) : (
                logs.filter(Boolean).map((log, i) => {
                  const action = getLogField(log, "action");
                  const severity = getLogField(log, "severity");
                  return (
                    <TableRow key={getLogField(log, "id") || String(i)} onClick={() => setSelectedLog(log)} className="cursor-pointer"
                      tabIndex={0} role="button"
                      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setSelectedLog(log); } }}>
                      <TableCell className="text-xs font-mono whitespace-nowrap">{getLogField(log, "timestamp")}</TableCell>
                      <TableCell className="text-sm font-medium text-muted-foreground">{getLogField(log, "username")}</TableCell>
                      <TableCell className="text-xs font-mono">{getLogField(log, "ip")}</TableCell>
                      <TableCell>
                        <Badge variant={getActionBadge(action) as "success" | "secondary" | "warning" | "destructive"}>
                          {action}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={getSeverityBadge(severity) as "secondary" | "warning" | "destructive"}>
                          {severity}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs max-w-[200px] truncate">{getLogField(log, "resource")}</TableCell>
                      <TableCell className="text-xs font-mono">{getLogField(log, "target")}</TableCell>
                      <TableCell>
                        <Badge variant={(getLogField(log, "status") || "").toLowerCase().includes("fail") ? "destructive" : "success"}>
                          {getLogField(log, "status")}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs max-w-[300px] truncate">{getLogField(log, "details")}</TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </div>

        <Pagination page={page} pageSize={perPage} total={total} onPageChange={setPage} />
      </Card>

      <Dialog open={!!selectedLog} onOpenChange={() => setSelectedLog(null)}>
        <DialogContent className="sm:max-w-lg max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t("audit.detail_title")}</DialogTitle>
          </DialogHeader>
          {selectedLog && (<div className="space-y-4">
            {(["timestamp", "username", "ip", "action", "severity", "resource", "target", "status", "details"] as const).map(field => (
              <div key={field}>
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{field}</span>
                <p className={`text-sm mt-0.5 ${field === "action" || field === "severity" || field === "status" ? "" : "text-muted-foreground"}`}>
                  {field === "action" ? (
                    <Badge variant={getActionBadge(getLogField(selectedLog, field)) as "success" | "secondary" | "warning" | "destructive"}>
                      {selectedLog.action}
                    </Badge>
                  ) : field === "severity" ? (
                    <Badge variant={getSeverityBadge(getLogField(selectedLog, field)) as "secondary" | "warning" | "destructive"}>
                      {selectedLog.severity || "info"}
                    </Badge>
                  ) : field === "status" ? (
                    <Badge variant={(getLogField(selectedLog, field)).toLowerCase().includes("fail") ? "destructive" : "success"}>
                      {getLogField(selectedLog, field)}
                    </Badge>
                  ) : (
                    <span className="font-mono">{getLogField(selectedLog, field)}</span>
                  )}
                </p>
              </div>
            ))}
          </div>)}
        </DialogContent>
      </Dialog>
    </div>
  );
}
