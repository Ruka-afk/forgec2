"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { downloadText } from "@/lib/download";
import { ConfirmModal, PageHeader, Spinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Copy, Download, Trash2 } from "lucide-react";

interface StagerToken {
  id: number;
  token: string;
  listener_id: number;
  arch: string;
  os: string;
  format: string;
  used: boolean;
  expires_at: string;
  created_by: string;
  created_at: string;
}

interface Listener {
  id: number;
  name: string;
  host: string;
  port: number;
  scheme: string;
}

export default function StagerPage() {
  const { t } = useI18n();

  const [tokens, setTokens] = useState<StagerToken[]>([]);
  const [listeners, setListeners] = useState<Listener[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");

  const [listenerId, setListenerId] = useState("");
  const [arch, setArch] = useState("amd64");
  const [os, setOs] = useState("windows");
  const [format, setFormat] = useState("exe");
  const [ttl, setTtl] = useState("60");
  const [userAgent, setUserAgent] = useState("");
  const [profile, setProfile] = useState("");
  const [dnsDomain, setDnsDomain] = useState("");
  const [dnsServer, setDnsServer] = useState("");
  const [skipTls, setSkipTls] = useState(false);
  const [creating, setCreating] = useState(false);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [createdToken, setCreatedToken] = useState<{ token: string; stager_url: string; stage2_size: number; expires_at: string } | null>(null);

  const fetchTokens = useCallback(async () => {
    try {
      const res = await api.get<{ success: boolean; data: StagerToken[] }>("/stager/tokens");
      if (res.success) setTokens(res.data);
    } catch { toast.error(t("stager.toast.create_failed")); }
  }, []);

  const fetchListeners = useCallback(async () => {
    try {
      const res = await api.get<Record<string, unknown>>("/api/listeners");
      setListeners(Array.isArray(res) ? res as unknown as Listener[] : ((res.data as Listener[]) || (res.listeners as Listener[]) || []));
    } catch { toast.error(t("stager.toast.create_failed")); }
  }, []);

  useEffect(() => {
    if (!message) return;
    const t = setTimeout(() => setMessage(""), 5000);
    return () => clearTimeout(t);
  }, [message]);

  useEffect(() => {
    Promise.all([fetchTokens(), fetchListeners()]).finally(() => setLoading(false));
  }, [fetchTokens, fetchListeners]);

  async function handleRegister() {
    if (!listenerId) { setMessage(t("stager.field_listener")); return; }
    setCreating(true); setMessage(""); setCreatedToken(null);
    try {
      const res = await api.postJson<{
        success: boolean; token: string; stager_url: string;
        stage2_size: number; expires_at: string; token_id: number;
      }>("/stager/register", {
        listener_id: parseInt(listenerId),
        arch, os, format,
        ttl_minutes: parseInt(ttl),
        user_agent: userAgent,
        profile: profile,
        skip_tls_verify: skipTls,
        dns_domain: dnsDomain,
        dns_server: dnsServer,
      });
      if (res.success) {
        setCreatedToken(res);
        setMessage(t("stager.toast.created"));
        fetchTokens();
      }
    } catch (e: unknown) {
      setMessage(e instanceof Error ? e.message : t("stager.toast.create_failed"));
    } finally {
      setCreating(false);
    }
  }

  function handleDelete(id: number) {
    setCfm({msg: t("stager.delete_token"), cb: async () => {
      try {
        await api.del(`/stager/${id}`);
        setMessage(t("stager.toast.deleted"));
        fetchTokens();
      } catch { setMessage(t("stager.toast.delete_failed")); }
    }});
  }

  function isExpired(t: StagerToken) {
    return new Date(t.expires_at) < new Date();
  }

  function downloadToken(token: string, filename: string) {
    downloadText(token, filename);
  }

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("stager.title")} subtitle={t("stager.subtitle")} />

      {message && (
        <div className="px-4 py-2 bg-indigo-50 dark:bg-indigo-900/30 border border-indigo-200 dark:border-indigo-800 rounded-xl text-sm text-indigo-700 dark:text-indigo-300">
          {message}
          <Button variant="ghost" size="icon-sm" onClick={() => setMessage("")} className="ml-2 text-indigo-400 hover:text-indigo-600" aria-label="Dismiss">&times;</Button>
        </div>
      )}

      {createdToken && (
        <div className="p-4 bg-emerald-50 dark:bg-emerald-900/20 border border-emerald-200 dark:border-emerald-800 rounded-xl space-y-2">
          <div className="text-sm font-semibold text-emerald-700 dark:text-emerald-300">{t("stager.token_created")}</div>
          <div className="text-xs space-y-1">
            <div><span className="font-medium">{t("stager.stager_url")}</span> <code className="text-indigo-600 dark:text-indigo-400 bg-secondary/80 px-1 rounded">{createdToken.stager_url}</code></div>
            <div><span className="font-medium">{t("stager.token_label")}</span>
              <code className="block mt-1 p-2 bg-secondary/80 dark:bg-muted/50 rounded text-[10px] break-all font-mono">{createdToken.token}</code>
            </div>
            <div><span className="font-medium">{t("stager.stage2_size")}</span> {createdToken.stage2_size} bytes</div>
            <div><span className="font-medium">{t("stager.expires")}</span> {formatTime(createdToken.expires_at)}</div>
          </div>
          <div className="flex gap-2 mt-2">
            <Button variant="secondary" size="sm" onClick={() => { navigator.clipboard.writeText(createdToken.token); toast.success(t("stager.token_copied")); }}>
              <Copy className="w-4 h-4" />{t("stager.copy_token")}
            </Button>
            <Button variant="outline" size="sm" onClick={() => downloadToken(createdToken.token, `stager_token_${Date.now()}.txt`)}>
              <Download className="w-4 h-4" />{t("stager.download")}
            </Button>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Card className="lg:col-span-1 p-4 sm:p-5 space-y-3">
          <div className="text-sm font-semibold text-foreground">{t("stager.register_token")}</div>

          <div>
            <Label className="text-xs">{t("stager.field_listener")}</Label>
            <Select value={listenerId} onValueChange={(v) => setListenerId(v ?? "")}>
              <SelectTrigger className="w-full mt-1">
                <SelectValue placeholder="Select listener..." />
              </SelectTrigger>
              <SelectContent>
                {listeners.map(l => (
                  <SelectItem key={l.id} value={String(l.id)}>
                    {l.name} ({l.scheme}://{l.host}:{l.port})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid grid-cols-2 gap-2">
            <div>
              <Label className="text-xs">{t("stager.field_os")}</Label>
              <Select value={os} onValueChange={(v) => v && setOs(v)}>
                <SelectTrigger className="w-full mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="windows">{t("stager.os_windows")}</SelectItem>
                  <SelectItem value="linux">{t("stager.os_linux")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="text-xs">{t("stager.field_arch")}</Label>
              <Select value={arch} onValueChange={(v) => v && setArch(v)}>
                <SelectTrigger className="w-full mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="amd64">amd64</SelectItem>
                  <SelectItem value="386">386</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-2">
            <div>
              <Label className="text-xs">{t("stager.field_format")}</Label>
              <Select value={format} onValueChange={(v) => v && setFormat(v)}>
                <SelectTrigger className="w-full mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="exe">{t("stager.format_exe")}</SelectItem>
                  <SelectItem value="dll">{t("stager.format_dll")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="text-xs">{t("stager.field_ttl")}</Label>
              <Input type="number" value={ttl} onChange={e => setTtl(e.target.value)} className="w-full mt-1" />
            </div>
          </div>

          <div>
            <Label className="text-xs">{t("stager.field_ua")}</Label>
            <Input type="text" value={userAgent} onChange={e => setUserAgent(e.target.value)} placeholder="leave empty for default" className="w-full mt-1" />
          </div>

          <div>
            <Label className="text-xs">{t("stager.field_profile")}</Label>
            <Input type="text" value={profile} onChange={e => setProfile(e.target.value)} placeholder="optional" className="w-full mt-1" />
          </div>

          <div>
            <Label className="text-xs">{t("stager.field_dns_domain")}</Label>
            <Input type="text" value={dnsDomain} onChange={e => setDnsDomain(e.target.value)} placeholder="optional" className="w-full mt-1" />
          </div>

          <div>
            <Label className="text-xs">{t("stager.field_dns_server")}</Label>
            <Input type="text" value={dnsServer} onChange={e => setDnsServer(e.target.value)} placeholder="optional" className="w-full mt-1" />
          </div>

          <Label className="flex items-center gap-2 text-xs text-muted-foreground">
            <Checkbox checked={skipTls} onCheckedChange={(checked) => setSkipTls(checked === true)} />
            {t("stager.field_skip_tls")}
          </Label>

          <Button onClick={handleRegister} disabled={creating || !listenerId}
            className="w-full h-11">
            {creating ? t("stager.generating") : t("stager.register_dialog")}
          </Button>
        </Card>

        <Card className="lg:col-span-2 p-4 sm:p-5">
          <div className="text-sm font-semibold text-foreground mb-3">
            {t("stager.tokens_count", { count: tokens.length })}
          </div>

          {loading ? (
            <div className="text-sm text-muted-foreground"><Spinner size="xs" /> Loading...</div>
          ) : tokens.length === 0 ? (
            <div className="text-sm text-muted-foreground italic">{t("stager.empty_tokens")}</div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow className="text-xs uppercase tracking-wider text-muted-foreground font-semibold border-b border-border">
                    <TableHead className="text-left py-3 px-4 sm:py-3.5">{t("stager.col_id")}</TableHead>
                    <TableHead className="text-left py-3 px-4 sm:py-3.5">{t("stager.col_os")}</TableHead>
                    <TableHead className="text-left py-3 px-4 sm:py-3.5">{t("stager.col_arch")}</TableHead>
                    <TableHead className="text-left py-3 px-4 sm:py-3.5">{t("stager.col_format")}</TableHead>
                    <TableHead className="text-left py-3 px-4 sm:py-3.5">{t("stager.col_status")}</TableHead>
                    <TableHead className="text-left py-3 px-4 sm:py-3.5">{t("stager.col_expires")}</TableHead>
                    <TableHead className="text-left py-3 px-4 sm:py-3.5">{t("stager.col_created_by")}</TableHead>
                    <TableHead className="text-right py-3 px-4 sm:py-3.5">{t("stager.col_actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tokens.map(tk => (
                    <TableRow key={tk.id}>
                      <TableCell className="py-3 px-4 sm:py-3.5 font-mono truncate max-w-[200px]">{tk.id}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5">{tk.os}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5">{tk.arch}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5">{tk.format}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5">
                        {tk.used ? (
                          <Badge variant="warning">{t("stager.status_used")}</Badge>
                        ) : isExpired(tk) ? (
                          <Badge variant="destructive">{t("stager.status_expired")}</Badge>
                        ) : (
                          <Badge variant="success">{t("stager.status_active")}</Badge>
                        )}
                      </TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5 text-muted-foreground">
                        {formatTime(tk.expires_at)}
                      </TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5">{tk.created_by || "-"}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5 text-right">
                        <Button variant="ghost" size="icon-xs" onClick={() => downloadToken(tk.token, `stager_token_${tk.id}.txt`)}
                          title={t("stager.download")} aria-label={t("stager.download")}>
                          <Download className="w-4 h-4" />
                        </Button>
                        <Button variant="destructive" size="icon-xs" onClick={() => handleDelete(tk.id)}
                          title={t("stager.col_actions")} aria-label={t("stager.col_actions")}>
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </Card>
      </div>
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.confirm")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
