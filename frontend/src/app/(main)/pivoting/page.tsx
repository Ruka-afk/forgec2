"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { EmptyState, ConfirmModal, PageHeader, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ArrowDown, ArrowLeftRight, ArrowRight, ArrowUp, Check, Info, Network, Play, PlusCircle, Radio, RotateCw, Route, Square, X } from "lucide-react";

interface RelaySession {
  id?: string;
  agent_id: string;
  hostname?: string;
  listen_port: number;
  active: boolean;
  bytes_in: number;
  bytes_out: number;
  active_conn: number;
  conn_count: number;
  created_at: string;
}

interface PivotAgent {
  id: string;
  hostname: string;
  ip: string;
  status: string;
}

interface RPortForwardStatus {
  id: string;
  agent_id: string;
  remote_host: string;
  remote_port: number;
  local_port: number;
  protocol: string;
  active: boolean;
  bytes_in: number;
  bytes_out: number;
  uptime: number;
  error?: string;
}


export default function PivotingPage() {
  const { t } = useI18n();
  const [sessions, setSessions] = useState<RelaySession[]>([]);
  const [agents, setAgents] = useState<PivotAgent[]>([]);
  const [loading, setLoading] = useState(true);
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
  const [rportForwards, setRportForwards] = useState<RPortForwardStatus[]>([]);
  const [throughAgent, setThroughAgent] = useState("");
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const loadData = useCallback(async () => {
    try {
      const [sessData, agentsData, rportData] = await Promise.all([
        api.get("/socks/sessions").catch(() => null),
        api.get("/agents?status=online").catch(() => null),
        api.get("/rportfwd/status").catch(() => null),
      ]);
      if (sessData) {
        setSessions((sessData.sessions || sessData.data || []) as RelaySession[]);
      }
      if (agentsData) {
        setAgents((agentsData.agents || []) as PivotAgent[]);
      }
      if (rportData) {
        setRportForwards((rportData.forwards || rportData.data || []) as RPortForwardStatus[]);
      }
    } catch {
      setSessions([]);
      setAgents([]);
      toast.error("Failed to load pivoting data");
    }    setLoading(false);
  }, []);

  useEffect(() => { loadData(); }, [loadData]);
  useVisibleInterval(loadData, 5000);

  const startRelay = async () => {
    if (!selectedAgent) return;
    setStarting(true);
    try {
      await api.post(`/agents/${selectedAgent}/socks_relay/start`, { agent_id: selectedAgent, port: relayPort.toString(), host: relayHost, protocol: relayProtocol });
      toast.success("SOCKS relay started");
    } catch {
      toast.error("Failed to start SOCKS relay");
    }
    setStarting(false);
    loadData();
  };

  const startLocalProxy = async () => {
    setLocalStarting(true);
    try {
      const body: Record<string, string> = {
        port: localPort.toString(),
        through_agent: throughAgent,
      };
      if (localAuthEnabled) {
        body.auth_enabled = "true";
        body.username = localUsername;
        body.password = localPassword;
      }
      await api.post(`/agents/${throughAgent}/socks`, body);
      toast.success(`Local proxy started on :${localPort}`);
    } catch {
      toast.error("Failed to start local proxy");
    }
    setLocalStarting(false);
  };

  const stopRelay = (agentId: string) => {
    setCfm({msg: t("pivoting.disconnect_socks"), cb: async () => {
      try {
        await api.post(`/agents/${agentId}/socks_relay/stop`, { agent_id: agentId });
        toast.success("SOCKS relay stopped");
      } catch {      toast.error("Failed to stop SOCKS relay");    }
      loadData();
    }});
  };  const startRPort = async () => {
    if (!rportAgent) return;
    try {
      await api.post(`/agents/${rportAgent}/rportfwd/start`, { agent_id: rportAgent, remote_host: rportRemoteHost, remote_port: rportRemotePort.toString(), local_port: rportLocalPort.toString(), protocol: rportProtocol });
      toast.success("Reverse port forward started");
      loadData();
    } catch (err) {
      toast.error(String(err));
    }
  };

  const stopRPort = async (id: string) => {
    try {
      await api.post(`/agents/${rportAgent}/rportfwd/stop`, { id });
      toast.success("Reverse port forward stopped");
    } catch {
      toast.error("Failed to stop RPort");    }
    loadData();
  };  const checkRPortStatus = async (id: string) => {
    try {
      const data = await api.get(`/agents/${id}/rportfwd/status`);
      toast.info(`RPort ${id}: ${data.active ? "Active" : "Inactive"}`);
    } catch { toast.error("Failed to check RPort status"); }
  };

  const formatBytes = (bytes: number) => {    if (!bytes || bytes === 0) return "0 B";    const k = 1024;    const sizes = ["B", "KB", "MB", "GB"];    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(1) + " " + sizes[i];  };

  const formatCreated = (d: string) => {
    if (!d) return "-";
    try {
      return new Date(d).toLocaleString();
    } catch {
      return d;
    }
  };

  const formatUptime = (seconds: number) => {
    if (!seconds) return "-";
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) return `${h}h ${m}m ${s}s`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
  };

  const maxBytes = Math.max(...sessions.map(s => Math.max(s.bytes_in || 0, s.bytes_out || 0)));
  const maxRPortBytes = Math.max(...rportForwards.map(r => Math.max(r.bytes_in || 0, r.bytes_out || 0)));
  const tabOverallMax = Math.max(maxBytes, maxRPortBytes, 1);

  const activeSessions = sessions.filter(s => s.active);
  const stoppedSessions = sessions.filter(s => !s.active);

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 space-y-6 animate-fade-slide-up">
      <PageHeader title={<><Route className="w-4 h-4" />Pivoting &amp; Tunnels</>} subtitle="SOCKS relays, local proxies, and reverse port forwarding through agent infrastructure">
        <Button variant="outline" onClick={loadData}>
          <RotateCw className="w-4 h-4" /> Refresh
        </Button>
      </PageHeader>

      {throughAgent && (
        <div className="bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800 rounded-xl px-4 py-2.5 flex items-center gap-2">
          <Route className="w-4 h-4" />
          <span className="text-sm text-indigo-700 dark:text-indigo-400">
            Traffic routing via agent: <strong>{throughAgent.substring(0, 12)}</strong>
          </span>
          <Button variant="ghost" size="icon" onClick={() => setThroughAgent("")} className="ml-auto text-indigo-500 hover:text-indigo-700 dark:hover:text-indigo-300" aria-label="Clear">
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
                <span className="w-2 h-2 bg-emerald-500 rounded-full animate-pulse"></span>
              )}
            </TabsTrigger>
          ))}
        </TabsList>

      <TabsContent value="relay">
        <>
          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">
              <Radio className="w-4 h-4" />
              <span>Active Sessions</span>
              <Badge variant="success">{activeSessions.length} running</Badge>
              {stoppedSessions.length > 0 && (
                <Badge variant="secondary">{stoppedSessions.length} stopped</Badge>
              )}
            </div>            {loading ? (
              <div className="text-muted-foreground text-sm py-8 text-center"><Spinner size="sm" /> Loading...</div>
            ) : sessions.length === 0 ? (
              <EmptyState icon={Radio} title="No relay sessions configured" message="Start a new relay below to begin pivoting" />
            ) : (
              <div className="space-y-3">                {sessions.map((s, i) => (
                  <div key={s.id || i} className={`border rounded-xl p-4 transition-colors ${s.active ? "border-emerald-200 bg-emerald-50/50 dark:border-emerald-800 dark:bg-emerald-900/20" : "border-border opacity-60"}`}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-x-3">
                        <span className={`w-2.5 h-2.5 rounded-full ${s.active ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground"}`}></span>
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
                            toast.info(`Routing via ${s.agent_id.substring(0, 12)}`);
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
                      <div className="bg-emerald-500 transition-all" style={{ width: `${tabOverallMax > 0 ? ((s.bytes_in || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                      <div className="bg-blue-500 transition-all" style={{ width: `${tabOverallMax > 0 ? ((s.bytes_out || 0) / tabOverallMax) * 100 : 0}%` }}></div>
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
                <Label className="text-xs font-medium mb-1 block">Target Agent</Label>
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
                  <Label className="text-xs font-medium mb-1 block">Listen Host</Label>
                  <Input aria-label="127.0.0.1" name="input-1" type="text" value={relayHost} onChange={e => setRelayHost(e.target.value)} placeholder="127.0.0.1" />                </div>
                <div>                  <Label className="text-xs font-medium mb-1 block">Listen Port</Label>
                  <Input aria-label="SOCKS listen port" name="input-2" type="number" value={relayPort} onChange={e => setRelayPort(Number(e.target.value))} min={1} max={65535} />
                </div>
              </div>
            </div>            <div className="mt-4 grid grid-cols-1 lg:grid-cols-3 gap-4 items-end">
              <div>                <Label className="text-xs font-medium mb-1 block">Protocol</Label>                <Select value={relayProtocol} onValueChange={v => setRelayProtocol(v as "socks" | "http")}>
                  <SelectTrigger className="w-full"><SelectValue placeholder="Protocol" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="socks">SOCKS5</SelectItem>
                    <SelectItem value="http">HTTP CONNECT</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button onClick={startRelay} disabled={!selectedAgent || starting}
                className="lg:col-span-2 h-11 px-6 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-xl text-sm font-medium whitespace-nowrap transition-colors flex items-center justify-center gap-2">
                {starting ? <><Spinner size="xs" /> Starting...</> : <><Play className="w-4 h-4" /> Start {relayProtocol.toUpperCase()} Relay</>}
              </Button>
            </div>
            <div className="mt-3 p-3 bg-muted rounded-xl text-xs text-muted-foreground flex items-start gap-1.5">
              <Info className="w-4 h-4" />
              <span>Agent connects outbound to this SOCKS server. Configure your proxychains to point to the listen address.</span>
            </div>          </Card>
        </>
      </TabsContent>

      <TabsContent value="local">
        <>          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">
              <Network className="w-4 h-4" />
              <span>Local Proxy Configuration</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">              <div>
                <Label className="text-xs font-medium mb-1 block">Proxy Port</Label>
                <Input aria-label="Local proxy port" name="input-4" type="number" value={localPort} onChange={e => setLocalPort(Number(e.target.value))} min={1} max={65535} />
              </div>
              <div>
                <Label className="text-xs font-medium mb-1 block">Route Through</Label>
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
              <div>                <Label className="text-xs font-medium mb-1 block">Authentication</Label>
                <div className="flex items-center gap-3 mt-2">                  <Label className="flex items-center gap-2 cursor-pointer">
                    <Checkbox checked={localAuthEnabled} onCheckedChange={setLocalAuthEnabled} />                    <span className="text-sm text-muted-foreground">Enable</span>
                  </Label>
                </div>
              </div>
            </div>            {localAuthEnabled && (
              <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>                  <Label className="text-xs font-medium mb-1 block">Username</Label>
                  <Input aria-label="operator" name="input-7" type="text" value={localUsername} onChange={e => setLocalUsername(e.target.value)} placeholder="operator" />
                </div>                <div>
                  <Label className="text-xs font-medium mb-1 block">Password</Label>
                  <Input aria-label="????????" name="input-8" type="password" value={localPassword} onChange={e => setLocalPassword(e.target.value)} placeholder="????????" />                </div>
              </div>
            )}
            <div className="mt-4 flex items-center gap-3">
              <Button onClick={startLocalProxy} disabled={localStarting} className="gap-2">
                <Play className="w-4 h-4" />
                {localStarting ? "Starting..." : "Start Local Proxy"}
              </Button>
              <Button variant="outline" onClick={() => { setLocalPort(1080); setThroughAgent(""); setLocalAuthEnabled(false); setLocalUsername(""); setLocalPassword(""); }}>
                Reset
              </Button>
            </div>
          </Card>          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-3">
              <Network className="w-4 h-4" /> Direct Agent SOCKS            </div>
            <p className="text-sm text-muted-foreground mb-4">
              Start a SOCKS listener directly on the agent host. Only usable when you can reach the agent IP.
            </p>
            {agents.length > 0 ? (              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                {agents.map(a => (                  <div key={a.id} className="border border-border rounded-xl p-4 hover:border-amber-300 dark:hover:border-amber-700 hover:shadow-sm transition-all">
                    <div className="flex justify-between items-center mb-1">
                      <div className="font-medium text-foreground">{a.hostname}</div>
                      <span className="text-xs text-muted-foreground">{a.ip}</span>
                    </div>
                    <div className="text-xs text-muted-foreground mb-3 font-mono">{a.id.substring(0, 12)}...</div>
                    <Button variant="secondary" size="sm" onClick={() => {
                      api.post(`/agents/${a.id}/socks`, { port: localPort.toString() });
                      toast.success(`Direct SOCKS on ${a.hostname}:${localPort}`);
                    }}
                      className="w-full text-xs px-3 py-2 transition-colors flex items-center justify-center gap-1.5">
                      <Play className="w-4 h-4" /> Start SOCKS on :{localPort}
                    </Button>
                  </div>
                ))}              </div>
            ) : (
              <div className="text-muted-foreground text-sm py-4 text-center">No agents online</div>            )}
          </Card>
        </>
      </TabsContent>

      <TabsContent value="rport">
        <>
          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">              <ArrowLeftRight className="w-4 h-4" />
              <span>Active Reverse Port Forwards</span>
              <Badge variant="secondary">{rportForwards.filter(r => r.active).length} active</Badge>
            </div>
            {rportForwards.length === 0 ? (
              <EmptyState icon={ArrowLeftRight} title="No reverse port forwards configured" />
            ) : (
              <div className="space-y-3">
                {rportForwards.map(rf => (
                  <div key={rf.id} className={`rounded-xl p-4 transition-colors border ${rf.active ? "border-blue-200 bg-blue-50/50 dark:border-blue-800 dark:bg-blue-900/20" : "border-border opacity-60"}`}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-x-3">                        <span className={`w-2.5 h-2.5 rounded-full ${rf.active ? "bg-blue-500 animate-pulse" : "bg-muted-foreground"}`}></span>
                        <div>                          <div className="text-sm font-medium text-foreground flex items-center gap-2">                            <span className="font-mono">{rf.remote_host}:{rf.remote_port}</span>
                            <ArrowRight className="w-4 h-4" />
                            <span className="font-mono">localhost:{rf.local_port}</span>
                            <Badge variant="outline">{rf.protocol.toUpperCase()}</Badge>
                          </div>
                          <div className="text-xs text-muted-foreground mt-0.5">Agent: {rf.agent_id.substring(0, 12)}... {rf.active ? `Up: ${formatUptime(rf.uptime)}` : "Stopped"}{rf.error && <span className="text-red-500 ml-2">{rf.error}</span>}</div>
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
                    <div className="flex h-2 bg-secondary rounded-full overflow-hidden">                      <div className="bg-emerald-500 transition-all" style={{ width: `${tabOverallMax > 0 ? ((rf.bytes_in || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                      <div className="bg-blue-500 transition-all" style={{ width: `${tabOverallMax > 0 ? ((rf.bytes_out || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                    </div>
                  </div>
                ))}
              </div>            )}
          </Card>

          <Card className="p-4 sm:p-5">
            <div className="font-semibold text-foreground flex items-center gap-x-2 mb-4">              <PlusCircle className="w-4 h-4" /> New Reverse Port Forward
            </div>            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
              <div>                <Label className="text-xs font-medium mb-1 block">Agent</Label>
                <Select value={rportAgent} onValueChange={(v) => setRportAgent(v ?? "")}>
                  <SelectTrigger className="w-full"><SelectValue placeholder="-- Select Agent --" /></SelectTrigger>
                  <SelectContent>
                    {agents.map(a => (                    <SelectItem key={a.id} value={a.id}>{a.hostname} ({a.ip})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>              </div>
              <div>
                <Label className="text-xs font-medium mb-1 block">Remote Host</Label>                <Input aria-label="127.0.0.1" name="input-10" type="text" value={rportRemoteHost} onChange={e => setRportRemoteHost(e.target.value)} placeholder="127.0.0.1" />
              </div>              <div>                <Label className="text-xs font-medium mb-1 block">Remote Port</Label>
                <Input aria-label="Remote port" name="input-11" type="number" value={rportRemotePort} onChange={e => setRportRemotePort(Number(e.target.value))} min={1} max={65535} />              </div>
              <div>
                <Label className="text-xs font-medium mb-1 block">Local Port</Label>                <Input aria-label="Local forward port" name="input-12" type="number" value={rportLocalPort} onChange={e => setRportLocalPort(Number(e.target.value))} min={1} max={65535} />              </div>
            </div>            <div className="flex items-center gap-3">
              <div>
                <Label className="text-xs font-medium mb-1 block">Protocol</Label>
                <Select value={rportProtocol} onValueChange={v => setRportProtocol(v as "tcp" | "udp")}>
                  <SelectTrigger className="w-full"><SelectValue placeholder="Protocol" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="tcp">TCP</SelectItem>
                    <SelectItem value="udp">UDP</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button onClick={startRPort} disabled={!rportAgent}
                className="h-11 px-6 bg-primary hover:bg-primary/90 text-primary-foreground disabled:opacity-50 disabled:cursor-not-allowed rounded-xl text-sm font-medium transition-colors flex items-center gap-2">
                <Play className="w-4 h-4" /> Start Forward</Button>
              <Button variant="outline"
                onClick={() => {
                  setRportForwards(prev => [...prev]);
                  toast.info("Refreshing RPort status...");
                  loadData();
                }}
                className="h-11 px-4 flex items-center gap-1.5"
              >
                <RotateCw className="w-4 h-4" /> Refresh Status
              </Button>
            </div>
            <div className="mt-3 p-3 bg-primary/10 rounded-xl text-xs text-primary flex items-start gap-1.5">
              <Info className="w-4 h-4" />
              <span>Connect to localhost:{rportLocalPort} to reach {rportRemoteHost || "[target]"}:{rportRemotePort} via the agent. The agent establishes the outbound connection.</span>
            </div>
          </Card>        </>
      </TabsContent>
      </Tabs>
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.disconnect")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>  );
}


