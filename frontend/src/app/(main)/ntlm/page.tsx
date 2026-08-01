"use client";

import { useEffect, useState, useCallback } from "react";
import { EmptyState, PageHeader, Spinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ArrowLeftRight, BarChart2, Database, Megaphone, Play, RefreshCw, Square, Zap } from "lucide-react";

import type { Agent } from "@/types/agent";

type TabType = "coerce" | "relay" | "status";

export default function NtlmPage() {
  const { t } = useI18n();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [target, setTarget] = useState("");
  const [listenAddr, setListenAddr] = useState("0.0.0.0:8080");
  const [coerceType, setCoerceType] = useState("printerbug");
  const [relayListener, setRelayListener] = useState("0.0.0.0:8445");
  const [relayTarget, setRelayTarget] = useState("");
  const [relayFlags, setRelayFlags] = useState("");
  const [loading, setLoading] = useState(false);
  const [agentsError, setAgentsError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabType>("coerce");
  const [relayStatus, setRelayStatus] = useState<{ active: Record<string, unknown>[]; count: number; running: boolean }>({ active: [], count: 0, running: false });

  const loadAgents = useCallback(async () => {
    setAgentsError(null);
    try {
      const data = await api.get(`/agents`);
      setAgents((data.agents || data || []) as Agent[]);
    } catch {
      setAgentsError(t("ntlm.toast.load_agents_failed"));
      toast.error(t("ntlm.toast.load_agents_failed"));
    }
  }, [t]);

  useEffect(() => { loadAgents(); }, [loadAgents]);

  const loadRelayStatus = useCallback(async () => {
    try {
      const data = await api.get<{ active: Record<string, unknown>[]; count: number; running: boolean }>("/ntlm/relay_status");
      setRelayStatus(data);
    } catch { toast.error(t("ntlm.toast.relay_status_failed")); }
  }, [t]);

  useEffect(() => {
    if (activeTab === "status") loadRelayStatus();
  }, [activeTab, loadRelayStatus]);

  const getAgentId = (a: Agent) => a.id || "";
  const getHostname = (a: Agent) => a.hostname || "";
  const getIP = (a: Agent) => a.ip || "";

  const handleCoerce = async () => {
    if (!selectedAgent || !target) {
      toast.error(t("ntlm.toast.select_agent_target"));
      return;
    }
    setLoading(true);
    try {
      const body = new URLSearchParams({ target, listen_addr: listenAddr });
      const data = await api.post(`/agents/${selectedAgent}/coerce/${coerceType}`, Object.fromEntries(body));
      if (data.success) toast.success(t("ntlm.toast.coerce_dispatched", { task_id: String(data.task_id) }));
      else toast.error((data.error as string) || t("ntlm.toast.coerce_failed"));
    } catch (e) { toast.error(String(e)); }
    setLoading(false);
  };

  const handleRelayStart = async () => {
    if (!selectedAgent || !relayTarget) {
      toast.error(t("ntlm.toast.select_agent_relay"));
      return;
    }
    setLoading(true);
    try {
      const body = new URLSearchParams({ target: relayTarget, listener: relayListener, flags: relayFlags });
      const data = await api.post(`/agents/${selectedAgent}/relay/start`, Object.fromEntries(body));
      if (data.success) toast.success(t("ntlm.toast.relay_started", { task_id: String(data.task_id) }));
      else toast.error((data.error as string) || t("ntlm.toast.start_failed"));
    } catch (e) { toast.error(String(e)); }
    setLoading(false);
  };

  const handleRelayStop = async () => {
    if (!selectedAgent) {
      toast.error(t("ntlm.toast.select_agent_first"));
      return;
    }
    setLoading(true);
    try {
      const data = await api.post(`/agents/${selectedAgent}/relay/stop`, {});
      if (data.success) toast.success(t("ntlm.toast.relay_stopped"));
      else toast.error((data.error as string) || t("ntlm.toast.stop_failed"));
    } catch (e) { toast.error(String(e)); }
    setLoading(false);
  };

  const tabs: { key: TabType; label: string; icon: React.ReactNode }[] = [
    { key: "coerce", label: t("ntlm.tab_coerce"), icon: <Megaphone className="w-4 h-4" /> },
    { key: "relay", label: t("ntlm.tab_relay"), icon: <ArrowLeftRight className="w-4 h-4" /> },
    { key: "status", label: t("ntlm.tab_status"), icon: <BarChart2 className="w-4 h-4" /> },
  ];

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={<><Zap className="w-4 h-4" />{t("ntlm.title")}</>} subtitle={t("ntlm.subtitle")} />

      <Card className="p-3 mb-4 border-warning/40 bg-warning/10 text-sm text-warning-foreground">
        <div className="font-semibold">{t("ntlm.experimental_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("ntlm.experimental_desc")}</div>
      </Card>

      {agentsError && (
        <div className="mb-4 px-4 py-3 rounded-xl bg-destructive/10 border border-destructive/30 text-sm text-destructive flex items-center justify-between">
          <span>{agentsError}</span>
          <Button variant="ghost" size="sm" onClick={() => setAgentsError(null)} aria-label="Dismiss">&times;</Button>
        </div>
      )}

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as TabType)}>
        <TabsList className="mb-6">
          {tabs.map((tab) => (
            <TabsTrigger key={tab.key} value={tab.key} className="gap-2">
              {tab.icon}
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>

      {/* Agent selector (common) */}
      <Card className="p-4 sm:p-5 mb-6">
        <Label className="text-xs mb-1.5">{t("ntlm.target_agent")}</Label>
        <Select value={selectedAgent || "none"} onValueChange={(v) => setSelectedAgent(v === "none" ? "" : v ?? "")}>
          <SelectTrigger className="max-w-md">
            <SelectValue placeholder={t("ntlm.select_agent_placeholder")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">{t("ntlm.select_agent_placeholder")}</SelectItem>
            {agents.map(a => {
              const id = getAgentId(a);
              return <SelectItem key={id} value={id}>{getHostname(a)} ({getIP(a)})</SelectItem>;
            })}
          </SelectContent>
        </Select>
      </Card>

      {/* Coercion Tab */}
      <TabsContent value="coerce">
<Card className="p-4 sm:p-5">
          <div className="flex items-center gap-x-3 mb-5">
            <div className="w-10 h-10 bg-amber-100 dark:bg-amber-900/30 rounded-xl flex items-center justify-center">
              <Megaphone className="w-4 h-4" />
            </div>
            <div>
              <div className="text-sm font-semibold text-foreground">{t("ntlm.coerce_title")}</div>
              <div className="text-xs text-muted-foreground">{t("ntlm.coerce_desc")}</div>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
            <div>
              <Label className="text-xs mb-1.5">{t("ntlm.coerce_technique")}</Label>
              <Select value={coerceType} onValueChange={(v) => { if (v) setCoerceType(v); }}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="printerbug">PrinterBug (MS-RPRN)</SelectItem>
                  <SelectItem value="petitpotam">PetitPotam (MS-EFSR)</SelectItem>
                  <SelectItem value="dfs">DFSCoerce (MS-DFSNM)</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="text-xs mb-1.5">{t("ntlm.target_ip")}</Label>
              <Input
                value={target}
                onChange={(e) => setTarget(e.target.value)}
                placeholder={t("ntlm.target_ph")}
              />
            </div>
            <div>
              <Label className="text-xs mb-1.5">{t("ntlm.callback_listener")}</Label>
              <Input
                value={listenAddr}
                onChange={(e) => setListenAddr(e.target.value)}
                placeholder="0.0.0.0:8080"
              />
            </div>
          </div>

          <Button
            onClick={handleCoerce}
            disabled={loading || !selectedAgent || !target}
            className="bg-amber-500 hover:bg-amber-600 text-white disabled:opacity-50"
          >
            {loading ? <><Spinner size="sm" className="mr-2" />{t("ntlm.dispatching")}</> : <><Play className="w-4 h-4" />{t("ntlm.execute_coercion")}</>}
          </Button>
        </Card>
      </TabsContent>

      {/* Relay Tab */}
      <TabsContent value="relay">
        <div className="space-y-6">
<Card className="p-4 sm:p-5">
            <div className="flex items-center gap-x-3 mb-5">
              <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center">
                <ArrowLeftRight className="w-4 h-4" />
              </div>
              <div>
                <div className="text-sm font-semibold text-foreground">{t("ntlm.relay_title")}</div>
                <div className="text-xs text-muted-foreground">{t("ntlm.relay_desc")}</div>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
              <div>
                <Label className="text-xs mb-1.5">{t("ntlm.relay_target")}</Label>
                <Input
                  value={relayTarget}
                  onChange={(e) => setRelayTarget(e.target.value)}
                  placeholder={t("ntlm.relay_target_ph")}
                />
              </div>
              <div>
                <Label className="text-xs mb-1.5">{t("ntlm.relay_listener")}</Label>
                <Input
                  value={relayListener}
                  onChange={(e) => setRelayListener(e.target.value)}
                  placeholder="0.0.0.0:8445"
                />
              </div>
              <div>
                <Label className="text-xs mb-1.5">{t("ntlm.flags")}</Label>
                <Input
                  value={relayFlags}
                  onChange={(e) => setRelayFlags(e.target.value)}
                  placeholder="e.g. --remove-mic --auth"
                />
              </div>
            </div>

            <div className="flex gap-3">
              <Button
                onClick={handleRelayStart}
                disabled={loading || !selectedAgent || !relayTarget}
                className="disabled:opacity-50"
              >
                {loading ? <><Spinner size="sm" className="mr-2" />{t("ntlm.starting")}</> : <><Play className="w-4 h-4" />{t("ntlm.start_relay")}</>}
              </Button>
              <Button
                variant="destructive"
                onClick={handleRelayStop}
                disabled={loading || !selectedAgent}
              >
                <Square className="w-4 h-4" />{t("ntlm.stop_relay")}
              </Button>
            </div>
          </Card>
        </div>
      </TabsContent>

      {/* Status Tab */}
      <TabsContent value="status">
<Card className="p-4 sm:p-5">
          <div className="flex items-center gap-x-3 mb-5">
            <div className="w-10 h-10 bg-emerald-100 dark:bg-emerald-900/30 rounded-xl flex items-center justify-center">
              <BarChart2 className="w-4 h-4" />
            </div>
            <div>
              <div className="text-sm font-semibold text-foreground">{t("ntlm.relay_sessions")}</div>
              <div className="text-xs text-muted-foreground">{t("ntlm.relay_sessions_desc")}</div>
            </div>
          </div>

          <div className="mb-4 flex items-center gap-x-3">
            <span className={`w-3 h-3 rounded-full ${relayStatus.running ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground"}`}></span>
            <span className="text-sm font-medium text-foreground">
              {relayStatus.running ? `${relayStatus.count} ${t("ntlm.sessions_active")}` : t("ntlm.sessions_inactive")}
            </span>
              <Button variant="outline" size="sm" onClick={loadRelayStatus} className="ml-auto">
              <RefreshCw className="w-4 h-4 mr-1" />{t("ntlm.refresh")}
            </Button>
          </div>

          {relayStatus.active.length === 0 ? (
            <EmptyState icon={Database} title={t("ntlm.no_sessions")} message={t("ntlm.no_sessions_hint")} />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("ntlm.col_session_id")}</TableHead>
                  <TableHead>{t("ntlm.col_target")}</TableHead>
                  <TableHead>{t("ntlm.col_listener")}</TableHead>
                  <TableHead>{t("ntlm.col_started")}</TableHead>
                  <TableHead className="text-right">{t("ntlm.col_hashes")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {relayStatus.active.map((session: Record<string, unknown>, idx: number) => (
                  <TableRow key={String(session.id || idx)}>
                    <TableCell className="font-mono text-xs">{String(session.id || "-")}</TableCell>
                    <TableCell>{String(session.target || "-")}</TableCell>
                    <TableCell className="font-mono text-xs">{String(session.listener || "-")}</TableCell>
                    <TableCell className="text-xs">{String(session.started_at || "-")}</TableCell>
                    <TableCell className="text-right font-mono">{String(session.hashes_captured ?? 0)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Card>
      </TabsContent>
      </Tabs>
    </div>
  );
}


