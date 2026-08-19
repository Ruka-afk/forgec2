"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { useMutation } from "@/lib/hooks/useMutation";
import { toast } from "sonner";
import { PageContainer } from "@/components/ui/page-container";
import { PageSpinner, Spinner } from "@/components/ui/spinner";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Banner } from "@/components/ui/banner";
import { IconBadge } from "@/components/ui/icon-badge";
import { StatCard } from "@/components/ui/animated-stat-card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Bot, Code, Crosshair, FolderTree, GitBranch, History, IdCard, Inbox, Key, Lock, Network, Rocket, Route, Share2, Terminal, Ticket, Wallet } from "lucide-react";

interface LateralStats {
  online_agents?: number;
  total_creds?: number;
  total_tasks?: number;
}

interface Credential {
  id?: string;
  username?: string;
  domain?: string;
  target?: string;
}

import type { Agent } from "@/types/agent";

interface MovementHistory {
  id?: string;
  source?: string;
  target?: string;
  method?: string;
  status?: string;
  output?: string;
  created_at?: string;
  pivot?: string;
}

interface FormData {
  source: string;
  target: string;
  pivot: string;
  username: string;
  password: string;
  hash: string;
  port: string;
  share: string;
  namespace: string;
  command: string;
  key_path: string;
  credential: string;
}

const defaultForm: FormData = {
  source: "",
  target: "",
  pivot: "",
  username: "",
  password: "",
  hash: "",
  port: "",
  share: "ADMIN$",
  namespace: "root\\cimv2",
  command: "whoami /all",
  key_path: "",
  credential: "",
};

const methods = [
  { key: "smb", label: "SMB", desc: "PsExec / Service Creation", icon: <Share2 className="w-4 h-4" /> },
  { key: "winrm", label: "WinRM", desc: "PowerShell Remoting", icon: <Terminal className="w-4 h-4" /> },
  { key: "wmi", label: "WMI", desc: "Process Creation", icon: <Network className="w-4 h-4" /> },
  { key: "ssh", label: "SSH", desc: "Linux/Unix Remote", icon: <Key className="w-4 h-4" /> },
  { key: "pth", label: "Pass-the-Hash", desc: "NTLM Hash Auth", icon: <Ticket className="w-4 h-4" /> },
];

export default function LateralPageContent() {
  const [activeMethod, setActiveMethod] = useState("smb");
  const [form, setForm] = useState<FormData>(defaultForm);
  const { t } = useI18n();

  const { mutate: runLateral, isPending: submitting } = useMutation({
    fn: async () => {
      const payload: Record<string, string> = {
        source: form.source,
        target: form.target,
        method: activeMethod,
      };
      if (form.pivot) payload.pivot = form.pivot;
      if (form.credential) payload.credential = form.credential;
      if (form.username) payload.username = form.username;
      if (form.command) payload.command = form.command;

      if (activeMethod === "smb") {
        if (form.password) payload.password = form.password;
        payload.share = form.share;
      } else if (activeMethod === "winrm") {
        if (form.password) payload.password = form.password;
        payload.port = form.port || "5985";
      } else if (activeMethod === "wmi") {
        if (form.password) payload.password = form.password;
        payload.namespace = form.namespace;
      } else if (activeMethod === "ssh") {
        if (form.password) payload.password = form.password;
        if (form.key_path) payload.key_path = form.key_path;
        payload.port = form.port || "22";
      } else if (activeMethod === "pth") {
        if (form.hash) payload.hash = form.hash;
      }

      await api.postJson(paths.lateral.execute, payload);
    },
    onSuccess: () => loadData(),
    onError: () => toast.error(t("lateral.toast.execute_failed")),
  });

  const { data, loading, refresh: loadData } = useApiResource<{
    stats: LateralStats;
    agents: Agent[];
    credentials: Credential[];
    history: MovementHistory[];
  }>({
    fetcher: async () => {
      let failed = 0;
      const [data, agentData, credData, histData] = await Promise.all([
        api.get(paths.lateral.historyAll).catch(() => { failed++; return { lateral: [] }; }),
        api.get(paths.agents.list()).catch(() => { failed++; return { agents: [] }; }),
        api.get(paths.credentials.list()).catch(() => { failed++; return { vault_entries: [] }; }),
        api.get(paths.tasks.list("type=lateral&pageSize=50")).catch(() => { failed++; return { tasks: [] }; }),
      ]);
      if (failed > 0) toast.error(t("lateral.toast.load_failed"));
      return {
        stats: data as LateralStats,
        agents: (agentData.agents || []) as Agent[],
        credentials: (credData.vault_entries || []) as Credential[],
        history: (histData.tasks || []) as MovementHistory[],
      };
    },
    toastThrottleMs: 0,
    errorMessage: t("lateral.toast.load_failed"),
  });
  const stats = data?.stats ?? {};
  const agents = data?.agents ?? [];
  const credentials = data?.credentials ?? [];
  const history = data?.history ?? [];

  const updateForm = (key: keyof FormData, value: string) => {
    setForm(prev => ({ ...prev, [key]: value }));
  };

  const handleSubmit = async () => {
    if (!form.source || !form.target) return;
    await runLateral();
  };

  const getStatusBadge = (status: string) => {
    const s = status?.toLowerCase() ?? "";
    const variant = s === "success" || s === "completed" ? "success" : s === "failed" ? "destructive" : s === "running" ? "default" : "warning";
    return variant;
  };

  const renderMethodForm = () => {
    switch (activeMethod) {
      case "smb":
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <FolderTree className="w-4 h-4" />{t("lateral.a11y_share")}
                </span>
                <Input aria-label={t("lateral.a11y_share")} name="admin-0" type="text" placeholder="ADMIN$" className="font-mono" value={form.share} onChange={e => updateForm("share", e.target.value)} />
              </div>
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <IdCard className="w-4 h-4" />{t("lateral.a11y_username")}
                </span>
                <Input aria-label={t("lateral.a11y_username")} name="domain-username-1" type="text" placeholder="DOMAIN\username" className="font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
              </div>
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                <Lock className="w-4 h-4" />{t("lateral.a11y_password")}
              </span>
              <Input aria-label={t("lateral.a11y_password")} name="password-2" type="password" placeholder={t("lateral.a11y_password")} className="font-mono" value={form.password} onChange={e => updateForm("password", e.target.value)} />
            </div>
          </>
        );
      case "winrm":
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <Network className="w-4 h-4" />{t("lateral.a11y_port")}
                </span>
                <Input aria-label={t("lateral.a11y_port")} name="5985-3" type="text" placeholder="5985" className="font-mono" value={form.port} onChange={e => updateForm("port", e.target.value)} />
              </div>
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <IdCard className="w-4 h-4" />{t("lateral.a11y_username")}
                </span>
                <Input aria-label={t("lateral.a11y_username")} name="administrator-4" type="text" placeholder="Administrator" className="font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
              </div>
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                <Lock className="w-4 h-4" />{t("lateral.a11y_password")}
              </span>
              <Input aria-label={t("lateral.a11y_password")} name="password-5" type="password" placeholder={t("lateral.a11y_password")} className="font-mono" value={form.password} onChange={e => updateForm("password", e.target.value)} />
            </div>
          </>
        );
      case "wmi":
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <Code className="w-4 h-4" />{t("lateral.a11y_namespace")}
                </span>
                <Input aria-label={t("lateral.a11y_namespace")} name="root-cimv2-6" type="text" placeholder="root\cimv2" className="font-mono" value={form.namespace} onChange={e => updateForm("namespace", e.target.value)} />
              </div>
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <IdCard className="w-4 h-4" />{t("lateral.a11y_username")}
                </span>
                <Input aria-label={t("lateral.a11y_username")} name="domain-username-7" type="text" placeholder="DOMAIN\username" className="font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
              </div>
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                <Lock className="w-4 h-4" />{t("lateral.a11y_password")}
              </span>
              <Input aria-label={t("lateral.a11y_password")} name="password-8" type="password" placeholder={t("lateral.a11y_password")} className="font-mono" value={form.password} onChange={e => updateForm("password", e.target.value)} />
            </div>
          </>
        );
      case "ssh":
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <Network className="w-4 h-4" />{t("lateral.a11y_port")}
                </span>
                <Input aria-label={t("lateral.a11y_port")} name="22-9" type="text" placeholder="22" className="font-mono" value={form.port} onChange={e => updateForm("port", e.target.value)} />
              </div>
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <IdCard className="w-4 h-4" />{t("lateral.a11y_username")}
                </span>
                <Input aria-label={t("lateral.a11y_username")} name="root-10" type="text" placeholder="root" className="font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <Lock className="w-4 h-4" />{t("lateral.a11y_password")}
                </span>
                <Input aria-label={t("lateral.a11y_password")} name="password-11" type="password" placeholder={t("lateral.a11y_password")} className="font-mono" value={form.password} onChange={e => updateForm("password", e.target.value)} />
              </div>
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                  <Key className="w-4 h-4" />{t("lateral.a11y_key_path")}
                </span>
                <Input aria-label={t("lateral.a11y_key_path")} name="path-to-key-pem-12" type="text" placeholder="/path/to/key.pem" className="font-mono" value={form.key_path} onChange={e => updateForm("key_path", e.target.value)} />
              </div>
            </div>
          </>
        );
      case "pth":
        return (
          <>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                <IdCard className="w-4 h-4" />{t("lateral.a11y_username")}
              </span>
              <Input aria-label={t("lateral.a11y_username")} name="administrator-13" type="text" placeholder="Administrator" className="font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                <Ticket className="w-4 h-4" />{t("lateral.a11y_hash")}
              </span>
              <Input aria-label={t("lateral.a11y_hash")} name="aad3b435b51404eeaad3b435b51404ee-14" type="text" placeholder="aad3b435b51404eeaad3b435b51404ee:..." className="font-mono" value={form.hash} onChange={e => updateForm("hash", e.target.value)} />
            </div>
          </>
        );
      default:
        return null;
    }
  };

  if (loading)
    return <PageContainer title={t("lateral.title")} subtitle={`SMB/WinRM/WMI/PsExec ${t("lateral.subtitle_exec")} / Pass-the-Hash`}><PageSpinner /></PageContainer>;

  return (
    <PageContainer title={t("lateral.title")} subtitle={`SMB/WinRM/WMI/PsExec ${t("lateral.subtitle_exec")} / Pass-the-Hash`} contentClassName="space-y-6">

      <Banner tone="warning" className="items-start">
        <div className="font-semibold">{t("lateral.honesty_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("lateral.honesty_desc")}</div>
      </Banner>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <StatCard label={t("lateral.online_implant")} value={stats.online_agents || 0} color="success" icon={<Bot className="w-4 h-4" />} sub={t("lateral.pivot_available")} />
        <StatCard label={t("lateral.available_creds")} value={stats.total_creds || 0} color="warning" icon={<Key className="w-4 h-4" />} sub={t("lateral.cred_vault")} />
        <StatCard label={t("lateral.history_tasks")} value={stats.total_tasks || 0} color="primary" icon={<Network className="w-4 h-4" />} sub={t("lateral.lateral_records")} />
      </div>

      <Card className=" px-4">
        <div className="flex items-center gap-x-3 mb-5">
          <IconBadge icon={GitBranch} color="primary" size="xl" className="dark:bg-primary/15" />
          <div>
            <div className="text-sm font-semibold text-foreground">{t("lateral.new_task")}</div>
            <div className="text-xs text-muted-foreground">{t("lateral.select_target_method")}</div>
          </div>
        </div>

        <div className="mb-6">
          <div className="flex gap-1 overflow-x-auto pb-1">
            {methods.map((m) => (
              <Button key={m.key} onClick={() => setActiveMethod(m.key)} variant="outline"
                className={`flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium whitespace-nowrap transition-colors ${
                  activeMethod === m.key
                    ? "bg-primary/10 text-primary border-primary/30 dark:bg-primary/20  dark:border-primary/40"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted dark:hover:text-muted-foreground border-transparent"
                }`}>
                {m.icon}
                {m.label}
              </Button>
            ))}
          </div>
        </div>

        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                <Bot className="w-4 h-4" />{t("lateral.source_implant")}
              </span>
              <Select value={form.source || undefined} onValueChange={v => updateForm("source", v ?? "")}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("lateral.select_online_implant")} />
                </SelectTrigger>
                <SelectContent>
                  {agents.map(a => {
                    const aid = a.id || "";
                    const host = a.hostname || "";
                    return <SelectItem key={aid} value={aid}>{host}</SelectItem>;
                  })}
                </SelectContent>
              </Select>
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                <Crosshair className="w-4 h-4" />{t("lateral.target_host")}
              </span>
              <Input aria-label={t("lateral.ip_or_hostname")} name="ip-16" type="text" placeholder={t("lateral.ip_or_hostname")} className="font-mono" value={form.target} onChange={e => updateForm("target", e.target.value)} />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
                <Route className="w-4 h-4" />{t("lateral.pivot_agent")}
              </span>
              <Select value={form.pivot || undefined} onValueChange={v => updateForm("pivot", v ?? "")}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("lateral.direct_connect")} />
                </SelectTrigger>
                <SelectContent>
                  {agents.map(a => {
                    const aid = a.id || "";
                    const host = a.hostname || "";
                    return <SelectItem key={aid} value={aid}>{t("lateral.via")} {host}</SelectItem>;
                  })}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
              <Wallet className="w-4 h-4" />{t("lateral.credential")}            </span>
            <Select value={form.credential || undefined} onValueChange={v => updateForm("credential", v ?? "")}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("lateral.manual_cred")} />
              </SelectTrigger>
              <SelectContent>
                {credentials.map(c => {
                  const cid = c.id || "";
                  const cuser = c.username || "";
                  const cdomain = c.domain || "";
                  const ctarget = c.target || "";
                  return <SelectItem key={cid} value={cid}>{cdomain ? `${cdomain}\\` : ""}{cuser} ({ctarget})</SelectItem>;
                })}
              </SelectContent>
            </Select>
          </div>

          <div className="border-t border-border pt-4">
            <div className="text-xs font-semibold text-muted-foreground mb-3">
              {(() => { const m = methods.find(m => m.key === activeMethod); return m ? <span className="mr-1.5">{m.icon}</span> : null; })()}
              {methods.find(m => m.key === activeMethod)?.label} {t("lateral.config")}
            </div>
            {renderMethodForm()}
          </div>

          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">
              <Terminal className="w-4 h-4" />{t("lateral.exec_command")}
            </span>
            <Input aria-label={t("lateral.a11y_command")} name="whoami-all-powershell-enc-19" type="text" placeholder="whoami /all | powershell -enc ..." className="font-mono" value={form.command} onChange={e => updateForm("command", e.target.value)} />
          </div>

          <div className="flex gap-3 pt-4 border-t border-border">
            <Button onClick={handleSubmit} size="lg" disabled={submitting || !form.source || !form.target}
              className="flex-1 disabled:opacity-50 disabled:cursor-not-allowed">
              {submitting ? <Spinner size="xs" /> : <Rocket className="w-4 h-4" />}
              <span>{submitting ? t("lateral.executing") : t("lateral.execute_lateral")}</span>
            </Button>
          </div>
        </div>
      </Card>

      <Card className=" overflow-hidden">
        <div className="flex items-center justify-between p-5 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-primary/10 rounded-lg flex items-center justify-center">
              <History className="w-4 h-4" />
            </div>
            <span className="text-sm font-semibold text-foreground">{t("lateral.move_history")}</span>
          </div>
          <span className="text-xs text-muted-foreground">{history.length} {t("lateral.records")}</span>
        </div>
        {history.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow className="bg-muted">
                <TableHead className="font-semibold"></TableHead>
                <TableHead className="font-semibold">{t("lateral.target")}</TableHead>
                <TableHead className="font-semibold">{t("lateral.method")}</TableHead>
                <TableHead className="font-semibold">{t("lateral.pivot")}</TableHead>
                <TableHead className="font-semibold">{t("lateral.status")}</TableHead>
                <TableHead className="font-semibold">{t("lateral.time")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {history.map((h, i) => (
                <TableRow key={h.id || i}>
                  <TableCell className="font-mono text-muted-foreground">{h.source || "-"}</TableCell>
                  <TableCell className="font-mono text-muted-foreground">{h.target || "-"}</TableCell>
                  <TableCell>
                    <Badge variant="outline" className="text-(--fs-micro-sm)">{h.method || "-"}</Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs">{h.pivot || t("lateral.direct")}</TableCell>
                  <TableCell>
                    <Badge variant={getStatusBadge(h.status || "")}>{h.status || "-"}</Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{h.created_at || "-"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <div className="text-center py-12 text-muted-foreground">
            <Inbox className="w-4 h-4" />
            <p>{t("lateral.empty")}</p>
          </div>
        )}
      </Card>
    </PageContainer>
  );
}

