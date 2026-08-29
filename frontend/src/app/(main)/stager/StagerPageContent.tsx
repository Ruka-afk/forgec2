"use client";
import { PageContainer } from "@/components/ui/page-container";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { POLL } from "@/lib/polling";
import { useTransientMessage } from "@/lib/hooks/useTransientMessage";
import { Banner } from "@/components/ui/banner";
import { downloadText } from "@/lib/download";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CircleCheck, Copy, Download, Trash2 } from "lucide-react";

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

export default function StagerPage({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();

  const { message, showMessage, clearMessage } = useTransientMessage();

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
  const { confirm, modal } = useConfirm();
  const [createdToken, setCreatedToken] = useState<{ token: string; stager_url: string; stage2_size: number; expires_at: string } | null>(null);

  const { data, loading, refresh: loadData } = useApiResource<{ tokens: StagerToken[]; listeners: Listener[] }>({
    fetcher: async () => {
      const res = await api.get<{ success: boolean; data: StagerToken[] }>("/stager/tokens");
      const lres = await api.get<unknown>(paths.listeners.list);
      const list = Array.isArray(lres) ? (lres as Listener[]) : (((lres as Record<string, unknown>)?.data as Listener[]) || ((lres as Record<string, unknown>)?.listeners as Listener[]) || []);
      return { tokens: res.success ? res.data : [], listeners: list };
    },
    toastThrottleMs: POLL.toastThrottle,
    errorMessage: t("stager.toast.create_failed"),
  });
  const tokens = data?.tokens ?? [];
  const listeners = data?.listeners ?? [];

  async function handleRegister() {
    if (!listenerId) { showMessage(t("stager.field_listener"), { tone: "warning" }); return; }
    setCreating(true); clearMessage(); setCreatedToken(null);
    try {
      const res = await api.postJson<{
        success: boolean; token: string; stager_url: string;
        stage2_size: number; expires_at: string; token_id: number;
      }>("/stager/register", {
        listener_id: parseInt(listenerId),
        arch, os, format,
        // Cleared input must not serialize NaN → null into an int field.
        ttl_minutes: Number.isFinite(parseInt(ttl)) ? parseInt(ttl) : 0,
        user_agent: userAgent,
        profile: profile,
        skip_tls_verify: skipTls,
        dns_domain: dnsDomain,
        dns_server: dnsServer,
      });
      if (res.token && res.stager_url) {
        setCreatedToken(res);
        showMessage(t("stager.toast.created"), { tone: "success" });
        loadData();
      }
    } catch (e: unknown) {
      showMessage(e instanceof Error ? e.message : t("stager.toast.create_failed"), { tone: "destructive" });
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id: number) {
    if (!(await confirm({ message: t("stager.delete_token") }))) return;
    try {
      await api.del(paths.stager.one(id));
      showMessage(t("stager.toast.deleted"), { tone: "success" });
      loadData();
    } catch { showMessage(t("stager.toast.delete_failed"), { tone: "destructive" }); }
  }

  // Purity-safe "now": Date.now() cannot run during render, so capture it in
  // an effect. Tokens load async anyway, so rows never render before this.
  const [expiryNowMs, setExpiryNowMs] = useState<number | null>(null);
  useEffect(() => { setExpiryNowMs(Date.now()); }, []);

  function isExpired(t: StagerToken) {
    if (expiryNowMs === null) return false;
    const d = new Date(t.expires_at).getTime();
    // Invalid/missing expires_at counts as expired: `Invalid < x` is false,
    // which used to keep malformed rows on the green "Active" badge forever.
    return Number.isNaN(d) || d <= expiryNowMs;
  }

  function downloadToken(token: string, filename: string) {
    downloadText(token, filename);
  }

  return (
    <PageContainer embedded={embedded} title={!embedded ? t("stager.title") : undefined} subtitle={!embedded ? t("stager.subtitle") : undefined}>

      {message && (
        <Banner tone={message.tone} className="mb-4 animate-fade-in" action={
          <Button variant="ghost" size="icon-sm" onClick={clearMessage} className="text-muted-foreground hover:text-foreground" aria-label={t("common.dismiss")}>
            &times;
          </Button>
        }>
          {message.text}
        </Banner>
      )}

      {createdToken && (
        <div className="p-4 bg-success/15 border border-success/30 rounded-lg space-y-2">
          <div className="text-sm font-semibold text-success">{t("stager.token_created")}</div>
          <div className="text-xs space-y-1">
            <div><span className="font-medium">{t("stager.stager_url")}</span> <code className="text-primary bg-secondary/80 px-1 rounded">{createdToken.stager_url}</code></div>
            <div><span className="font-medium">{t("stager.token_label")}</span>
              <code className="block mt-1 p-2 bg-secondary/80 dark:bg-muted/50 rounded text-(--fs-micro-sm) break-all font-mono">{createdToken.token}</code>
            </div>
            <div><span className="font-medium">{t("stager.stage2_size")}</span> {createdToken.stage2_size > 0 ? `${createdToken.stage2_size} bytes` : t("stager.stage2_lazy")}</div>
            <div><span className="font-medium">{t("stager.expires")}</span> {formatTime(createdToken.expires_at)}</div>
          </div>
          <div className="flex gap-2 mt-2">
            <CopyButton text={createdToken.token} className="mt-0">
              {(copied) => (
                <>
                  {copied ? <CircleCheck className="size-4" /> : <Copy className="size-4" />}
                  {t("stager.copy_token")}
                </>
              )}
            </CopyButton>
            <Button variant="outline" size="sm" onClick={() => downloadToken(createdToken.token, `stager_token_${Date.now()}.txt`)}>
              <Download className="size-4" />{t("stager.download")}
            </Button>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Card className="lg:col-span-1 p-(--card-spacing) space-y-3">
          <div className="text-sm font-semibold text-foreground">{t("stager.register_token")}</div>

          <div>
            <Label className="text-xs">{t("stager.field_listener")}</Label>
            <Select value={listenerId} onValueChange={(v) => setListenerId(v ?? "")}>
              <SelectTrigger className="w-full mt-1">
                <SelectValue placeholder={t("stager.select_listener_ph")} />
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
            <Input type="text" value={userAgent} onChange={e => setUserAgent(e.target.value)} placeholder={t("stager.empty_default")} className="w-full mt-1" />
          </div>

          <div>
            <Label className="text-xs">{t("stager.field_profile")}</Label>
            <Input type="text" value={profile} onChange={e => setProfile(e.target.value)} placeholder={t("stager.optional")} className="w-full mt-1" />
          </div>

          <div>
            <Label className="text-xs">{t("stager.field_dns_domain")}</Label>
            <Input type="text" value={dnsDomain} onChange={e => setDnsDomain(e.target.value)} placeholder={t("stager.optional")} className="w-full mt-1" />
          </div>

          <div>
            <Label className="text-xs">{t("stager.field_dns_server")}</Label>
            <Input type="text" value={dnsServer} onChange={e => setDnsServer(e.target.value)} placeholder={t("stager.optional")} className="w-full mt-1" />
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

        <Card className="lg:col-span-2 p-(--card-spacing)">
          <div className="text-sm font-semibold text-foreground mb-3">
            {t("stager.tokens_count", { count: tokens.length })}
          </div>

          {loading ? (
            <div className="text-sm text-muted-foreground"><Spinner size="xs" /> {t("common.loading")}</div>
          ) : tokens.length === 0 ? (
            <EmptyState title={t("stager.empty_tokens")} />
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
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
                          <Download className="size-4" />
                        </Button>
                        <Button variant="destructive" size="icon-xs" onClick={() => handleDelete(tk.id)}
                          title={t("stager.col_actions")} aria-label={t("stager.col_actions")}>
                          <Trash2 className="size-4" />
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
      {modal}
    </PageContainer>
  );
}
