"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { downloadText } from "@/lib/download";
import { PageContainer } from "@/components/ui/page-container";
import { Pagination } from "@/components/ui/pagination";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { DataState } from "@/components/ui/data-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Download, FileText, Filter, Terminal } from "lucide-react";
import type { AuditLog } from "@/types/audit";
import { useInteractStore } from "@/lib/interact-store";
import { auditSessionId, normalizeAuditLogs } from "./_components/audit-log";

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
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [perPage] = useState(50);
  const [userFilter, setUserFilter] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [users, setUsers] = useState<{ username: string }[]>([]);
  const { t } = useI18n();

  useEffect(() => {
    const controller = new AbortController();
    api.get(paths.users.list, { signal: controller.signal })
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
    setError(null);
    try {
      const params = new URLSearchParams({
        page: String(page),
        pageSize: String(perPage),
      });
      if (userFilter) params.set("user", userFilter);
      if (actionFilter) params.set("action", actionFilter);
      const data = await api.get(paths.audit.logs(params.toString()), { signal });
      const payload = (data && typeof data === "object") ? data as Record<string, unknown> : {};
      setLogs(normalizeAuditLogs(payload.logs));
      setTotal(Number(payload.total) || 0);
    } catch (e) {
      if ((e as Error).name === "AbortError") return;
      setLogs([]);
      setTotal(0);
      const msg = t("audit.toast.load_failed");
      setError(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }, [page, perPage, userFilter, actionFilter, t]);

  useEffect(() => {
    const controller = new AbortController();
    loadLogs(controller.signal);
    return () => controller.abort();
  }, [loadLogs]);

  const applyFilters = () => { setPage(1); };
  const resetFilters = () => {
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

  const openSession = (agentId: string) => {
    useInteractStore.getState().open(agentId);
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
      case "agent_id": return String(log.agent_id || "");
      default: return "";
    }
  };

  return (
    <PageContainer title={t("audit.title")} subtitle={t("audit.subtitle")} actions={<>
        <Button onClick={handleExport}>
          <Download className="w-4 h-4" />
          <span>{t("audit.export")} CSV</span>
        </Button>
      </>}>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t("audit.total_records")}</div>
              <div className="mt-2 text-2xl font-bold">{total}</div>
            </div>
            <div className="w-12 h-12 bg-primary/10 dark:bg-primary/20 rounded-xl flex items-center justify-center">
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
        <DataState
          loading={loading}
          error={error}
          onRetry={() => loadLogs()}
          empty={!loading && !error && logs.length === 0}
          emptyTitle={t("audit.empty")}
          loadingSkeleton={
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
                    <TableHead className="text-left">{t("audit.col_target")}</TableHead>
                    <TableHead className="text-left">{t("audit.status")}</TableHead>
                    <TableHead className="text-left">{t("audit.details")}</TableHead>
                    <TableHead className="text-right">{t("audit.interact")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {Array.from({ length: 5 }).map((_, i) => (
                    <TableRow key={i}>
                      {Array.from({ length: 10 }).map((_, j) => (
                        <TableCell key={j} className="py-3 px-4"><Skeleton className="h-4 w-full" /></TableCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          }
        >
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
                <TableHead className="text-left">{t("audit.col_target")}</TableHead>
                <TableHead className="text-left">{t("audit.status")}</TableHead>
                <TableHead className="text-left">{t("audit.details")}</TableHead>
                <TableHead className="text-right">{t("audit.interact")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.filter(Boolean).map((log, i) => {
                const action = getLogField(log, "action");
                const severity = getLogField(log, "severity");
                const sessionId = auditSessionId(log);
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
                    <TableCell className="text-right">
                      {sessionId ? (
                        <Button
                          type="button"
                          variant="ghost"
                          size="xs"
                          className="gap-1"
                          onClick={(e) => {
                            e.stopPropagation();
                            openSession(sessionId);
                          }}
                        >
                          <Terminal aria-hidden="true" className="w-3.5 h-3.5" />
                          {t("audit.interact")}
                        </Button>
                      ) : null}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
        </DataState>

        <Pagination page={page} pageSize={perPage} total={total} onPageChange={setPage} />
      </Card>

      <Dialog open={!!selectedLog} onOpenChange={() => setSelectedLog(null)}>
        <DialogContent className="sm:max-w-lg max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t("audit.detail_title")}</DialogTitle>
          </DialogHeader>
          {selectedLog && (<div className="space-y-4">
            {auditSessionId(selectedLog) && (
              <Button
                type="button"
                className="gap-1"
                onClick={() => {
                  openSession(auditSessionId(selectedLog));
                  setSelectedLog(null);
                }}
              >
                <Terminal aria-hidden="true" className="w-4 h-4" />
                {t("audit.interact")}
              </Button>
            )}
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
    </PageContainer>
  );
}
