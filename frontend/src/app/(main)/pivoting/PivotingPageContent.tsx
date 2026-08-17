"use client";

import { useState, useMemo } from "react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { EmptyState } from "@/components/ui/empty-state";
import { PageContainer } from "@/components/ui/page-container";
import { Spinner } from "@/components/ui/spinner";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ArrowDown, ArrowLeftRight, ArrowRight, ArrowUp, Check, Info, Network, Play, PlusCircle, Radio, RotateCw, Route, Square, X } from "lucide-react";
import { formatBytes, formatCreated, formatUptime } from "./_components/types";
import { usePivotingData } from "./_components/usePivotingData";
import { api } from "@/lib/api";
import { toast } from "sonner";

export default function PivotingPageContent() {
  const { t } = useI18n();
  const {
    sessions,
    agents,
    loading,
    rportForwards,
    loadData,
    startRelay: startRelayApi,
    stopRelay: stopRelayApi,
    startLocalProxy: startLocalProxyApi,
    startRPort: startRPortApi,
    stopRPort: stopRPortApi,
    checkRPortStatus,
  } = usePivotingData();
  const [selectedAgent, setSelectedAgent] = useState("");
  const [relayPort, setRelayPort] = useState(1080);
  const [relayHost, setRelayHost] = useState("127.0.0.1");
  const [relayProtocol, setRelayProtocol] = useState<"socks" | "http">("socks");
  const [starting, setStarting] = useState(false);
  const [localStarting, setLocalStarting] = useState(false);
  const [localPort, setLocalPort] = useState(1080);
  const [localAuthEnabled, setLocalAuthEnabled] = useState(false);
  const [localUsername, setLocalUsername] = useState("");
  const [localPassword, setLocalPassword] = useState("");
  const [rportAgent, setRportAgent] = useState("");
  const [rportRemoteHost, setRportRemoteHost] = useState("127.0.0.1");
  const [rportRemotePort, setRportRemotePort] = useState(22);
  const [rportLocalPort, setRportLocalPort] = useState(8022);
  const [rportProtocol, setRportProtocol] = useState<"tcp" | "udp">("tcp");
  const [throughAgent, setThroughAgent] = useState("");
  const { confirm, modal } = useConfirm();

  const startRelay = async () => {
    setStarting(true);
    await startRelayApi(selectedAgent, relayPort, relayHost, relayProtocol);
    setStarting(false);
  };

  const startLocalProxy = async () => {
    setLocalStarting(true);
    await startLocalProxyApi(
      throughAgent,
      localPort,
      localAuthEnabled ? { username: localUsername, password: localPassword } : undefined,
    );
    setLocalStarting(false);
  };

  const stopRelay = async (agentId: string) => {
    if (!(await confirm({ message: t("pivoting.disconnect_socks") }))) return;
    await stopRelayApi(agentId);
  };

  const startRPort = async () => {
    await startRPortApi({
      rportAgent,
      remoteHost: rportRemoteHost,
      remotePort: rportRemotePort,
      localPort: rportLocalPort,
      protocol: rportProtocol,
    });
  };

  const stopRPort = async (id: string) => {
    await stopRPortApi(rportAgent, id);
  };

  const maxBytes = useMemo(
    () => sessions.reduce((m, s) => Math.max(m, s.bytes_in || 0, s.bytes_out || 0), 0),
    [sessions],
  );
  const maxRPortBytes = useMemo(
    () => rportForwards.reduce((m, r) => Math.max(m, r.bytes_in || 0, r.bytes_out || 0), 0),
    [rportForwards],
  );
  const tabOverallMax = Math.max(maxBytes, maxRPortBytes, 1);

  const activeSessions = useMemo(() => sessions.filter(s => s.active), [sessions]);
  const stoppedSessions = useMemo(() => sessions.filter(s => !s.active), [sessions]);

  return (
    <PageContainer title={t("pivoting.title")} icon={<Route className="w-4 h-4" />} subtitle={t("pivoting.subtitle")} contentClassName="space-y-6" actions={<>
        <Button variant="outline" onClick={loadData}>
          <RotateCw className="w-4 h-4" /> Refresh
        </Button>
      </>}>

      {throughAgent && (
        <div className="bg-info/8 border border-info/20 rounded-lg px-4 py-2.5 flex items-center gap-2 animate-fade-in">
          <Route className="w-4 h-4 text-info" />
          <span className="text-sm text-info">
            Traffic routing via agent: <strong>{throughAgent.substring(0, 12)}</strong>
          </span>
          <Button variant="ghost" size="icon" onClick={() => setThroughAgent("")} className="ml-auto text-info hover:text-primary" aria-label={t("common.clear")}>
            <X className="w-4 h-4" />
          </Button>
        </div>
      )}

      <Tabs defaultValue="relay">
        <TabsList className="mb-6">
          {[
            { key: "relay", label: "SOCKS Relay", Icon: Radio },
            { key: "local", label: "Local Proxy", Icon: Network },
            { key: "rport", label: "Reverse Port", Icon: ArrowLeftRight },
          ].map((tab) => (
            <TabsTrigger key={tab.key} value={tab.key} className="gap-2">
              <tab.Icon className="w-4 h-4" />
              <span>{tab.label}</span>
              {tab.key === "relay" && activeSessions.length > 0 && (
                <span className="w-2 h-2 bg-success rounded-full animate-pulse"></span>
              )}
            </TabsTrigger>
          ))}
        </TabsList>

      <TabsContent value="relay">
        <>
          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">
              <Radio className="w-4 h-4" />
              <span>{t("pivoting.sessions_active")}</span>
              <Badge variant="success">{activeSessions.length} running</Badge>
              {stoppedSessions.length > 0 && (
                <Badge variant="secondary">{stoppedSessions.length} stopped</Badge>
              )}
            </div>            {loading ? (
              <div className="text-muted-foreground text-sm py-8 text-center"><Spinner size="sm" /> {t("common.loading")}</div>
            ) : sessions.length === 0 ? (
              <EmptyState icon={Radio} title={t("pivoting.empty_relay_title")} message={t("pivoting.empty_relay_message")} />
            ) : (
              <div className="space-y-3">                {sessions.map((s, i) => (
                  <div key={s.id || i} className={`border rounded-lg p-4 transition-colors ${s.active ? "border-success/30 bg-success/10/50 dark:border-success/40 dark:bg-success/20" : "border-border opacity-60"}`}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-x-3">
                        <span className={`w-2.5 h-2.5 rounded-full ${s.active ? "bg-success animate-pulse" : "bg-muted-foreground"}`}></span>
                        <div>
                          <div className="font-medium text-sm text-foreground">{s.agent_id.substring(0, 12)}... {s.hostname && <span className="text-muted-foreground font-normal">({s.hostname})</span>}</div>
                          <div className="text-xs text-muted-foreground">Port {s.listen_port} {s.active ? "Running" : "Stopped"}  {formatCreated(s.created_at)}</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-x-3">
                        <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                          <Network className="w-4 h-4" />
                          {s.active_conn || 0} active / {s.conn_count || 0} total                        </div>
                        <Button
                          variant={throughAgent === s.agent_id ? "default" : "outline"}
                          size="sm"
                          onClick={() => {
                            setThroughAgent(s.agent_id);
                             toast.info(t("pivoting.toast.routing_via", { agent_id: s.agent_id.substring(0, 12) }));
                          }}
                        >
                          {throughAgent === s.agent_id ? <Check className="w-4 h-4" /> : <Route className="w-4 h-4" />}
                          {throughAgent === s.agent_id ? " Selected" : " Through Me"}
                        </Button>
                        {s.active && (
                          <Button variant="destructive" size="sm" onClick={() => stopRelay(s.agent_id)}>
                            <Square className="w-4 h-4" />
                          </Button>
                        )}
                      </div>                    </div>                    <div className="mb-1.5 flex justify-between text-xs text-muted-foreground">
                      <span className="flex items-center gap-1"><ArrowDown className="w-4 h-4" />{formatBytes(s.bytes_in || 0)} down</span>
                      <span className="flex items-center gap-1"><ArrowUp className="w-4 h-4" />{formatBytes(s.bytes_out || 0)} up</span>
                    </div>
                    <div className="flex h-2 bg-secondary rounded-full overflow-hidden">
                      <div className="bg-success transition-all" style={{ width: `${tabOverallMax > 0 ? ((s.bytes_in || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                      <div className="bg-info transition-all" style={{ width: `${tabOverallMax > 0 ? ((s.bytes_out || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                    </div>                  </div>
                ))}
              </div>            )}
          </Card>

          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">
              <PlusCircle className="w-4 h-4" /> Start New SOCKS Relay
            </div>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <div>
                <Label className="text-xs font-medium mb-1 block">{t("pivoting.target_agent")}</Label>
                <Select value={selectedAgent} onValueChange={(v) => setSelectedAgent(v ?? "")}>
                  <SelectTrigger className="w-full"><SelectValue placeholder="-- Select Agent --" /></SelectTrigger>
                  <SelectContent>
                    {agents.map(a => (
                      <SelectItem key={a.id} value={a.id}>{a.hostname} ({a.ip})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label className="text-xs font-medium mb-1 block">{t("pivoting.listen_host")}</Label>
                  <Input aria-label="127.0.0.1" name="input-1" type="text" value={relayHost} onChange={e => setRelayHost(e.target.value)} placeholder="127.0.0.1" />                </div>
                <div>                  <Label className="text-xs font-medium mb-1 block">{t("pivoting.listen_port")}</Label>
                  <Input aria-label={t("pivoting.socks_listen_port")} name="input-2" type="number" value={relayPort} onChange={e => setRelayPort(Number(e.target.value))} min={1} max={65535} />
                </div>
              </div>
            </div>            <div className="mt-4 grid grid-cols-1 lg:grid-cols-3 gap-4 items-end">
              <div>                <Label className="text-xs font-medium mb-1 block">Protocol</Label>                <Select value={relayProtocol} onValueChange={v => setRelayProtocol(v as "socks" | "http")}>
                  <SelectTrigger className="w-full"><SelectValue placeholder={t("pivoting.protocol_ph")} /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="socks">SOCKS5</SelectItem>
                    <SelectItem value="http">HTTP CONNECT</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button onClick={startRelay} size="lg" disabled={!selectedAgent || starting}
                className="lg:col-span-2 px-6 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium whitespace-nowrap transition-colors flex items-center justify-center gap-2">
                {starting ? <><Spinner size="xs" /> {t("pivoting.starting")}</> : <><Play className="w-4 h-4" /> {t("pivoting.start")} {relayProtocol.toUpperCase()} Relay</>}
              </Button>
            </div>
            <div className="mt-3 p-3 bg-muted rounded-lg text-xs text-muted-foreground flex items-start gap-1.5">
              <Info className="w-4 h-4" />
              <span>{t("pivoting.proxy_hint")}</span>
            </div>          </Card>
        </>
      </TabsContent>

      <TabsContent value="local">
        <>          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">
              <Network className="w-4 h-4" />
              <span>{t("pivoting.local_proxy")}</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">              <div>
                <Label className="text-xs font-medium mb-1 block">{t("pivoting.proxy_port")}</Label>
                <Input aria-label={t("pivoting.local_proxy_port")} name="input-4" type="number" value={localPort} onChange={e => setLocalPort(Number(e.target.value))} min={1} max={65535} />
              </div>
              <div>
                <Label className="text-xs font-medium mb-1 block">{t("pivoting.route_through")}</Label>
                <Select value={throughAgent} onValueChange={(v) => setThroughAgent(v ?? "")}>
                  <SelectTrigger className="w-full"><SelectValue placeholder="-- Direct (no pivot) --" /></SelectTrigger>
                  <SelectContent>
                    {sessions.filter(s => s.active).map(s => (
                      <SelectItem key={s.agent_id} value={s.agent_id}>{s.agent_id.substring(0, 12)}... (:{s.listen_port})</SelectItem>
                    ))}
                    {agents.map(a => (
                      <SelectItem key={a.id} value={a.id}>{a.hostname} ({a.ip})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>                <Label className="text-xs font-medium mb-1 block">{t("pivoting.auth_label")}</Label>
                <div className="flex items-center gap-3 mt-2">                  <Label className="flex items-center gap-2 cursor-pointer">
                    <Checkbox checked={localAuthEnabled} onCheckedChange={setLocalAuthEnabled} />                    <span className="text-sm text-muted-foreground">{t("pivoting.enable")}</span>
                  </Label>
                </div>
              </div>
            </div>            {localAuthEnabled && (
              <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>                  <Label className="text-xs font-medium mb-1 block">{t("pivoting.username_label")}</Label>
                  <Input aria-label={t("pivoting.operator_label")} name="input-7" type="text" value={localUsername} onChange={e => setLocalUsername(e.target.value)} placeholder="operator" />
                </div>                <div>
                  <Label className="text-xs font-medium mb-1 block">{t("pivoting.password")}</Label>
                  <Input aria-label={t("pivoting.password")} name="input-8" type="password" value={localPassword} onChange={e => setLocalPassword(e.target.value)} placeholder={t("pivoting.password")} />                </div>
              </div>
            )}
            <div className="mt-4 flex items-center gap-3">
              <Button onClick={startLocalProxy} disabled={localStarting} className="gap-2">
                <Play className="w-4 h-4" />
                {localStarting ? t("pivoting.starting") : t("pivoting.start_local_proxy")}
              </Button>
              <Button variant="outline" onClick={() => { setLocalPort(1080); setThroughAgent(""); setLocalAuthEnabled(false); setLocalUsername(""); setLocalPassword(""); }}>
                {t("pivoting.reset")}
              </Button>
            </div>
          </Card>          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-3">
              <Network className="w-4 h-4" /> {t("pivoting.direct_socks")}            </div>
            <p className="text-sm text-muted-foreground mb-4">
              {t("pivoting.direct_socks_desc")}
            </p>
            {agents.length > 0 ? (              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                {agents.map(a => (                  <Card key={a.id} className="rounded-2xl hover:border-warning/40 dark:hover:border-warning hover:shadow-sm transition-all">
                    <CardContent className="p-4">
                      <div className="flex justify-between items-center mb-1">
                        <div className="font-medium text-foreground">{a.hostname}</div>
                        <span className="text-xs text-muted-foreground">{a.ip}</span>
                      </div>
                      <div className="text-xs text-muted-foreground mb-3 font-mono">{a.id.substring(0, 12)}...</div>
                      <Button variant="secondary" size="sm" onClick={() => {
                        void api.post(paths.agents.socks(a.id), { port: localPort.toString() });
                        toast.success(t("pivoting.toast.direct_socks", { host: a.hostname, port: String(localPort) }));
                      }}
                        className="w-full text-xs px-3 py-2 transition-colors flex items-center justify-center gap-1.5">
                        <Play className="w-4 h-4" /> {t("pivoting.start_socks_on", { port: String(localPort) })}
                      </Button>
                    </CardContent>
                  </Card>
                ))}              </div>
            ) : (
              <div className="text-muted-foreground text-sm py-4 text-center">{t("pivoting.no_agents")}</div>            )}
          </Card>
        </>
      </TabsContent>

      <TabsContent value="rport">
        <>
          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">              <ArrowLeftRight className="w-4 h-4" />
              <span>{t("pivoting.active_reverse_forwards")}</span>
              <Badge variant="secondary">{rportForwards.filter(r => r.active).length} {t("pivoting.active")}</Badge>
            </div>
            {rportForwards.length === 0 ? (
              <EmptyState icon={ArrowLeftRight} title={t("pivoting.empty_rportfwd_title")} />
            ) : (
              <div className="space-y-3">
                {rportForwards.map(rf => (
                  <div key={rf.id} className={`rounded-lg p-4 transition-colors border ${rf.active ? "border-info/30 bg-info/10 dark:border-info/40 dark:bg-info/20" : "border-border opacity-60"}`}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-x-3">                        <span className={`w-2.5 h-2.5 rounded-full ${rf.active ? "bg-info animate-pulse" : "bg-muted-foreground"}`}></span>
                        <div>                          <div className="text-sm font-medium text-foreground flex items-center gap-2">                            <span className="font-mono">{rf.remote_host}:{rf.remote_port}</span>
                            <ArrowRight className="w-4 h-4" />
                            <span className="font-mono">localhost:{rf.local_port}</span>
                            <Badge variant="outline">{rf.protocol.toUpperCase()}</Badge>
                          </div>
                          <div className="text-xs text-muted-foreground mt-0.5">Agent: {rf.agent_id.substring(0, 12)}... {rf.active ? `Up: ${formatUptime(rf.uptime)}` : "Stopped"}{rf.error && <span className="text-destructive ml-2">{rf.error}</span>}</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => checkRPortStatus(rf.id)}
                        >
                          <Info className="w-4 h-4" /> Status
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => stopRPort(rf.id)}
                        >                          <Square className="w-4 h-4" /> Stop
                        </Button>
                      </div>                    </div>
                    <div className="mb-1.5 flex justify-between text-xs text-muted-foreground">
                      <span className="flex items-center gap-1"><ArrowDown className="w-4 h-4" />{formatBytes(rf.bytes_in || 0)}</span>
                      <span className="flex items-center gap-1"><ArrowUp className="w-4 h-4" />{formatBytes(rf.bytes_out || 0)}</span>                    </div>
                    <div className="flex h-2 bg-secondary rounded-full overflow-hidden">                      <div className="bg-success transition-all" style={{ width: `${tabOverallMax > 0 ? ((rf.bytes_in || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                      <div className="bg-info transition-all" style={{ width: `${tabOverallMax > 0 ? ((rf.bytes_out || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                    </div>
                  </div>
                ))}
              </div>            )}
          </Card>

          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">              <PlusCircle className="w-4 h-4" /> New Reverse Port Forward
            </div>            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
              <div>                <Label className="text-xs font-medium mb-1 block">{t("pivoting.agent")}</Label>
                <Select value={rportAgent} onValueChange={(v) => setRportAgent(v ?? "")}>
                  <SelectTrigger className="w-full"><SelectValue placeholder={t("pivoting.select_agent")} /></SelectTrigger>
                  <SelectContent>
                    {agents.map(a => (                    <SelectItem key={a.id} value={a.id}>{a.hostname} ({a.ip})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>              </div>
              <div>
                <Label className="text-xs font-medium mb-1 block">{t("pivoting.remote_host")}</Label>                <Input aria-label={t("pivoting.remote_host")} name="input-10" type="text" value={rportRemoteHost} onChange={e => setRportRemoteHost(e.target.value)} placeholder="127.0.0.1" />
              </div>              <div>                <Label className="text-xs font-medium mb-1 block">{t("pivoting.remote_port")}</Label>
                <Input aria-label={t("pivoting.remote_port")} name="input-11" type="number" value={rportRemotePort} onChange={e => setRportRemotePort(Number(e.target.value))} min={1} max={65535} />              </div>
              <div>
                <Label className="text-xs font-medium mb-1 block">{t("pivoting.local_forward_port")}</Label>                <Input aria-label={t("pivoting.local_forward_port")} name="input-12" type="number" value={rportLocalPort} onChange={e => setRportLocalPort(Number(e.target.value))} min={1} max={65535} />              </div>
            </div>            <div className="flex items-center gap-3">
              <div>
                <Label className="text-xs font-medium mb-1 block">{t("pivoting.protocol")}</Label>
                <Select value={rportProtocol} onValueChange={v => setRportProtocol(v as "tcp" | "udp")}>
                  <SelectTrigger className="w-full"><SelectValue placeholder={t("pivoting.protocol_ph")} /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="tcp">TCP</SelectItem>
                    <SelectItem value="udp">UDP</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button onClick={startRPort} size="lg" disabled={!rportAgent}
                className="px-6 bg-primary hover:bg-primary/90 text-primary-foreground disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium transition-colors flex items-center gap-2">
                <Play className="w-4 h-4" /> {t("pivoting.start_forward")}</Button>
              <Button variant="outline" size="lg"
                onClick={() => {
                  toast.info(t("pivoting.toast.refreshing_rport"));
                  void loadData();
                }}
                className="px-4 flex items-center gap-1.5"
              >
                <RotateCw className="w-4 h-4" /> {t("pivoting.refresh_status")}
              </Button>
            </div>
            <div className="mt-3 p-3 bg-primary/10 rounded-lg text-xs text-primary flex items-start gap-1.5">
              <Info className="w-4 h-4" />
              <span>Connect to localhost:{rportLocalPort} to reach {rportRemoteHost || "[target]"}:{rportRemotePort} via the agent. The agent establishes the outbound connection.</span>
            </div>
          </Card>        </>
      </TabsContent>
      </Tabs>
      {modal}
    </PageContainer>
  );
}

