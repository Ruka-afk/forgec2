"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { api, getCsrfToken } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadFromResponse } from "@/lib/download";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { useAgentList } from "@/lib/hooks/useAgentList";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { formatTime } from "@/lib/utils";
import { EmptyState } from "@/components/ui/empty-state";
import { PageContainer } from "@/components/ui/page-container";
import { IconBadge } from "@/components/ui/icon-badge";
import { Pagination } from "@/components/ui/pagination";
import { Spinner } from "@/components/ui/spinner";
import { DataState } from "@/components/ui/data-state";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CheckCircle, CircleAlert, Download, PawPrint, Play, RefreshCw, Table2, Trash2, Upload } from "lucide-react";

const PAGE_SIZE = 10;


interface BHResult {
  ID?: number;
  id?: number;
  AgentID?: string;
  agent_id?: string;
  AgentName?: string;
  agent_name?: string;
  Method?: string;
  method?: string;
  Users?: number;
  users?: number;
  Computers?: number;
  computers?: number;
  Groups?: number;
  groups?: number;
  DAs?: number;
  das?: number;
  SPNs?: number;
  spns?: number;
  CreatedAt?: string;
  created_at?: string;
}

export default function BloodHoundPage() {
  const { t } = useI18n();
  const { agents } = useAgentList();
  const [results, setResults] = useState<BHResult[]>([]);
  const [binaryStatus, setBinaryStatus] = useState<{ uploaded: boolean; filename: string }>({ uploaded: false, filename: "" });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [method, setMethod] = useState("DCOnly");
  const [collecting, setCollecting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [page, setPage] = useState(1);
  const { confirm, modal } = useConfirm();

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      let failed = 0;
      const [resultsRes, statusRes] = await Promise.all([
        api.get(paths.bloodhound.list).catch(() => { failed++; return null; }),
        api.get<{ uploaded: boolean; filename: string }>("/bloodhound/status").catch(() => { failed++; return null; }),
      ]);
      if (resultsRes) setResults((resultsRes.data || []) as BHResult[]);
      if (statusRes) setBinaryStatus(statusRes);
      if (failed === 2) {
        setError(t("bloodhound.toast.load_failed"));
        toast.error(t("bloodhound.toast.load_failed"));
      }
    } catch {
      setError(t("bloodhound.toast.load_failed"));
      toast.error(t("bloodhound.toast.load_failed"));
    }
    setLoading(false);
  }, [t]);

  useEffect(() => { loadData(); }, [loadData]);

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const form = new FormData();
      form.append("file", file);
      await api.postFormData(paths.bloodhound.upload, form);
      toast.success(t("bloodhound.toast.sharp_hound_uploaded"));
      loadData();
    } catch { toast.error(t("bloodhound.toast.upload_sharp_hound_failed")); }
    setUploading(false);
  };

  const handleCollect = async () => {
    if (!selectedAgent) return;
    setCollecting(true);
    try {
      await api.post(paths.bloodhound.collect, { agent_id: selectedAgent, method });
      toast.success(t("bloodhound.toast.collection_started"));
      loadData();
    } catch { toast.error(t("bloodhound.toast.start_collection_failed")); }
    setCollecting(false);
  };

  const handleDownload = async (id: number) => {
    try {
      const res = await fetch(`${API_BASE}/bloodhound/${id}/download`, { credentials: "include", headers: { "X-CSRF-Token": getCsrfToken() } });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      await downloadFromResponse(res, `bloodhound-${id}.zip`);
    } catch { toast.error(t("bloodhound.toast.download_report_failed")); }
  };

  const handleDelete = async (id: number) => {
    if (!(await confirm({ message: t("bloodhound.delete_report") }))) return;
    try {
      await api.del(paths.bloodhound.one(id));
      loadData();
    } catch { toast.error(t("bloodhound.toast.delete_report_failed")); }
  };

  const getVal = (obj: BHResult, keys: (keyof BHResult)[]) => {
    for (const k of keys) {
      const v = obj[k];
      if (v !== undefined && v !== null) return v;
    }
    return undefined;
  };

  const pageCount = Math.max(1, Math.ceil(results.length / PAGE_SIZE));
  const currentPage = Math.min(page, pageCount);
  const paginatedResults = results.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);

  return (
    <PageContainer title={t("bloodhound.title")} subtitle={t("bloodhound.subtitle")} contentClassName="space-y-6">

      <Card className="px-4 sm:px-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
        <div className="flex items-center gap-x-3 mb-5">
          <IconBadge icon={binaryStatus.uploaded ? CheckCircle : CircleAlert} color={binaryStatus.uploaded ? "success" : "warning"} size="lg" />
          <div>
            <div className="text-sm font-semibold text-foreground">{t("bloodhound.sharp_hound_status")}</div>
            <div className="text-xs text-muted-foreground">
              {binaryStatus.uploaded
                ? `Uploaded ${binaryStatus.filename}`
                : "SharpHound.exe not uploaded"}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Label className="relative cursor-pointer">
            <Input aria-label={t("bloodhound.upload_exe")} name="input-0" type="file" accept=".exe" onChange={handleUpload} className="sr-only" />
            <span className="inline-flex shrink-0 items-center justify-center rounded-lg bg-primary px-2.5 py-1.5 text-sm font-medium text-primary-foreground transition-all hover:bg-primary/80 disabled:pointer-events-none disabled:opacity-50 gap-1.5 cursor-pointer">
              {uploading ? <Spinner size="xs" /> : <Upload className="w-4 h-4" />}
              <span>{uploading ? t("bloodhound.uploading") : t("bloodhound.upload_btn")}</span>
            </span>
          </Label>
          {binaryStatus.uploaded && (
            <span className="text-xs text-success">
              <CheckCircle className="w-4 h-4" />{binaryStatus.filename}
            </span>
          )}
        </div>
      </Card>

      <Card className="px-4 sm:px-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
        <div className="flex items-center gap-x-3 mb-5">
          <IconBadge icon={PawPrint} color="primary" size="xl" />
          <div>
            <div className="text-sm font-semibold text-foreground">{t("bloodhound.new_collection_task")}</div>
            <div className="text-xs text-muted-foreground">{t("bloodhound.new_collection_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-4">
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bloodhound.target_agent")}</span>
            <Select value={selectedAgent || "placeholder"} onValueChange={(v) => setSelectedAgent(v === "placeholder" || v === null ? "" : v)}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="-- Select Agent --" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="placeholder">{t("bloodhound.select_agent")}</SelectItem>
                {agents.map(a => {
                  const id = a.id || "";
                  const hostname = a.hostname || "";
                  const ip = a.ip || "";
                  return <SelectItem key={id} value={id}>{hostname} ({ip})</SelectItem>;
                })}
              </SelectContent>
            </Select>
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bloodhound.collection_method")}</span>
            <Select value={method} onValueChange={(v) => { if (v) setMethod(v); }}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="DCOnly">DCOnly</SelectItem>
                <SelectItem value="All">{t("bloodhound.collect_all")}</SelectItem>
                <SelectItem value="Session">Session</SelectItem>
                <SelectItem value="LDAP">LDAP</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-end">
            <Button onClick={handleCollect} disabled={collecting || !selectedAgent}
              className="bg-primary hover:bg-primary/90 text-primary-foreground disabled:opacity-50 disabled:cursor-not-allowed">
              {collecting ? <Spinner size="xs" /> : <Play className="w-4 h-4" />}
              <span>{collecting ? "Collecting..." : "Start Collection"}</span>
            </Button>
          </div>
        </div>
      </Card>

      <Card className="px-0 mb-0 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
        <div className="flex items-center justify-between px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-secondary rounded-lg flex items-center justify-center">
              <Table2 className="w-4 h-4" />
            </div>
            <h2 className="text-sm font-semibold text-foreground">{t("bloodhound.collection_results")}</h2>
            <span className="text-xs text-muted-foreground">({results.length})</span>
          </div>
          <Button variant="ghost" size="sm" onClick={() => loadData()}>
            <RefreshCw className="w-4 h-4" />Refresh
          </Button>
        </div>
        <DataState loading={loading} error={error} onRetry={loadData} empty={!loading && !error && results.length === 0} emptyIcon={PawPrint} emptyTitle={t("bloodhound.empty_title")} emptyMessage={t("bloodhound.empty_message")}>
        <div>
          {results.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">ID</TableHead>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">{t("bloodhound.col_agent")}</TableHead>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">{t("bloodhound.col_method")}</TableHead>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">{t("bloodhound.col_users")}</TableHead>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">{t("bloodhound.col_computers")}</TableHead>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">{t("bloodhound.col_groups")}</TableHead>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">DA</TableHead>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">{t("bloodhound.col_spn")}</TableHead>
                  <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5">{t("bloodhound.col_time")}</TableHead>
                  <TableHead className="text-right py-3 px-4 sm:py-3.5 sm:px-5">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="divide-y divide-border">
                {paginatedResults.map((r, i) => {
                  const id = (getVal(r, ["ID", "id"]) as number) ?? i;
                  const agent = (getVal(r, ["AgentName", "agent_name"]) as string) ?? (getVal(r, ["AgentID", "agent_id"]) as string) ?? "-";
                  const methodVal = (getVal(r, ["Method", "method"]) as string) ?? "-";
                  const users = (getVal(r, ["Users", "users"]) as number) ?? 0;
                  const computers = (getVal(r, ["Computers", "computers"]) as number) ?? 0;
                  const groups = (getVal(r, ["Groups", "groups"]) as number) ?? 0;
                  const das = (getVal(r, ["DAs", "das"]) as number) ?? 0;
                  const spns = (getVal(r, ["SPNs", "spns"]) as number) ?? 0;
                  const time = (getVal(r, ["CreatedAt", "created_at"]) as string) ?? "";
                  return (
                    <TableRow key={id || i} className="hover:bg-muted">
                      <TableCell className="py-3 px-4 font-mono text-xs text-muted-foreground">{id}</TableCell>
                      <TableCell className="py-3 px-4 text-muted-foreground font-medium">{agent}</TableCell>
                      <TableCell className="py-3 px-4">
                        <Badge variant="outline" className="text-(--fs-micro-sm)">{methodVal}</Badge>
                      </TableCell>
                       <TableCell className="py-3 px-4 font-mono text-primary">{users}</TableCell>
                       <TableCell className="py-3 px-4 font-mono text-primary">{computers}</TableCell>
                       <TableCell className="py-3 px-4 font-mono text-primary">{groups}</TableCell>
                       <TableCell className="py-3 px-4 font-mono text-chart-5 font-semibold">{das}</TableCell>
                       <TableCell className="py-3 px-4 font-mono text-primary">{spns}</TableCell>
                      <TableCell className="py-3 px-4 text-xs text-muted-foreground">{formatTime(time)}</TableCell>
                      <TableCell className="py-3 px-4">
                        <div className="flex items-center gap-2">
                          <Button variant="ghost" size="icon-xs" onClick={() => handleDownload(id)}
                            className="bg-success/15 hover:bg-success/25 text-success rounded-lg transition-colors"
                            aria-label={t("bloodhound.download_report")}>
                            <Download className="w-4 h-4" />
                          </Button>
                          <Button variant="ghost" size="icon-xs" onClick={() => handleDelete(id)}
                            className="bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-lg transition-colors"
                            aria-label={t("bloodhound.a11y_delete_report")}>
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          ) : (
            <div className="text-center py-12 text-muted-foreground">
              <EmptyState icon={PawPrint} title={t("bloodhound.empty_title")} message={t("bloodhound.empty_message")} />
            </div>
          )}
        </div>
        </DataState>
        <Pagination page={currentPage} pageSize={PAGE_SIZE} total={results.length} onPageChange={setPage} />
      </Card>
      {modal}
    </PageContainer>
  );
}

