"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal } from "@/components/UI";

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

interface RPortForward {
  id?: string;
  agent_id: string;
  remote_host: string;
  remote_port: number;
  local_port: number;
  protocol: string;
  active: boolean;
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
  const [sessions, setSessions] = useState<RelaySession[]>([]);
  const [agents, setAgents] = useState<PivotAgent[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<"relay" | "local" | "rport">("relay");
  const [selectedAgent, setSelectedAgent] = useState("");
  const [relayPort, setRelayPort] = useState(1080);
  const [relayHost, setRelayHost] = useState("127.0.0.1");
  const [relayProtocol, setRelayProtocol] = useState<"socks" | "http">("socks");
  const [starting, setStarting] = useState(false);
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
  const [toast, setToast] = useState<{ text: string; type: string } | null>(null);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const showToast = useCallback((text: string, type: string = "info") => {
    setToast({ text, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const loadData = useCallback(async () => {
    try {
      const [sessRes, agentsRes, rportRes] = await Promise.all([
        fetch(`${API_BASE}?p=/socks/sessions&format=json`),
        fetch(`${API_BASE}?p=/agents?status=online&format=json`),
        fetch(`${API_BASE}?p=/rportfwd/status&format=json`).catch(() => null),
      ]);
      if (sessRes.ok) {
        const sessData = await sessRes.json();
        setSessions(sessData.sessions || sessData.data || []);
      }
      if (agentsRes.ok) {
        const agentsData = await agentsRes.json();
        setAgents(agentsData.Agents || agentsData.agents || []);
      }
      if (rportRes && rportRes.ok) {
        const rportData = await rportRes.json();
        setRportForwards(rportData.forwards || rportData.data || []);
      }
    } catch {
      setSessions([]);
      setAgents([]);
    }    setLoading(false);
  }, []);

  useEffect(() => {
    Promise.resolve().then(() => {
      loadData();
      intervalRef.current = setInterval(loadData, 5000);
    });
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [loadData]);

  const startRelay = async () => {
    if (!selectedAgent) return;
    setStarting(true);
    try {
      const body = new URLSearchParams();
      body.append("agent_id", selectedAgent);
      body.append("port", relayPort.toString());
      body.append("host", relayHost);
      body.append("protocol", relayProtocol);      await fetch(`${API_BASE}?p=/agents/${selectedAgent}/socks_relay/start`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      showToast("SOCKS relay started", "success");
    } catch {
      showToast("Failed to start SOCKS relay", "error");
    }
    setStarting(false);
    loadData();
  };

  const stopRelay = (agentId: string) => {
    setCfm({msg: "Disconnect current SOCKS relay? Active connections will be dropped.", cb: async () => {
      try {
        const body = new URLSearchParams();
        body.append("agent_id", agentId);
        await fetch(`${API_BASE}?p=/agents/${agentId}/socks_relay/stop`, {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },        body: body.toString(),
        });
        showToast("SOCKS relay stopped", "success");
      } catch {      showToast("Failed to stop SOCKS relay", "error");    }
      loadData();
    }});
  };  const startRPort = async () => {
    if (!rportAgent) return;
    try {
      const body = new URLSearchParams();
      body.append("agent_id", rportAgent);
      body.append("remote_host", rportRemoteHost);
      body.append("remote_port", rportRemotePort.toString());
      body.append("local_port", rportLocalPort.toString());
      body.append("protocol", rportProtocol);      const res = await fetch(`${API_BASE}?p=/agents/${rportAgent}/rportfwd/start`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      if (res.ok) {
        showToast("Reverse port forward started", "success");
        loadData();
      } else {        showToast(`Failed: ${res.status}`, "error");      }
    } catch (err) {
      showToast(String(err), "error");
    }
  };

  const stopRPort = async (id: string) => {
    try {      const body = new URLSearchParams();
      body.append("id", id);      await fetch(`${API_BASE}?p=/agents/${rportAgent}/rportfwd/stop`, {        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      showToast("Reverse port forward stopped", "success");
    } catch {
      showToast("Failed to stop RPort", "error");    }
    loadData();
  };  const checkRPortStatus = async (id: string) => {
    try {
      const res = await fetch(`${API_BASE}?p=/rportfwd/status/${id}&format=json`);
      if (res.ok) {        const data = await res.json();        showToast(`RPort ${id}: ${data.active ? "Active" : "Inactive"}`, "info");
      }    } catch (e) { console.error("Pivoting: check rport status failed", e); }
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
    <div className="max-w-7xl mx-auto space-y-6 mb-20 md:mb-0">
      {toast && (
        <div className={`fixed top-4 right-4 z-50 px-4 py-3 rounded-xl text-sm font-medium shadow-lg ${          toast.type === "success" ? "bg-emerald-600 text-white" :
          toast.type === "error" ? "bg-red-600 text-white" :          "bg-blue-600 text-white"        }`}>          {toast.text}
        </div>
      )}      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">            <i className="fa-solid fa-arrows-turn-to-dots text-indigo-500 mr-2"></i>Pivoting & Tunnels
          </h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs mt-1">SOCKS relays, local proxies, and reverse port forwarding through agent infrastructure</p>
        </div>        <div className="flex items-center gap-2">
          <button onClick={loadData} className="flex items-center gap-x-2 px-4 py-2 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 text-[var(--text-secondary)] rounded-xl text-sm transition-colors">
            <i className="fa-solid fa-rotate"></i> Refresh
          </button>
        </div>
      </div>

      {throughAgent && (
        <div className="bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800 rounded-xl px-4 py-2.5 flex items-center gap-2">
          <i className="fa-solid fa-route text-indigo-500"></i>          <span className="text-sm text-indigo-700 dark:text-indigo-400">
            Traffic routing via agent: <strong>{throughAgent.substring(0, 12)}</strong>
          </span>
          <button onClick={() => setThroughAgent("")} className="ml-auto text-indigo-500 hover:text-indigo-700 dark:hover:text-indigo-300">            <i className="fa-solid fa-xmark"></i>
          </button>
        </div>
      )}

      <div className="flex border-b border-[var(--border)]">        {([["relay", "SOCKS Relay", "fa-tower-broadcast"], ["local", "Local Proxy", "fa-sitemap"], ["rport", "Reverse Port", "fa-arrow-right-arrow-left"]] as const).map(([key, label, icon]) => (
          <button key={key} onClick={() => setActiveTab(key)}
            className={`flex items-center gap-x-2 px-5 py-3 text-sm font-medium border-b-2 transition-colors ${activeTab === key ? "border-indigo-500 text-indigo-600 dark:text-indigo-400" : "border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-300"}`}>
            <i className={`fa-solid ${icon}`}></i>
            <span>{label}</span>
            {key === "relay" && activeSessions.length > 0 && (
              <span className="w-2 h-2 bg-emerald-500 rounded-full animate-pulse"></span>
            )}
          </button>
        ))}      </div>

      {activeTab === "relay" && (
        <>
          <div className="ui-card p-6 shadow-sm">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2 mb-4">
              <i className="fa-solid fa-tower-broadcast text-emerald-500"></i>
              <span>Active Sessions</span>
              <span className="ml-2 px-2.5 py-0.5 bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400 text-xs font-semibold rounded-lg">{activeSessions.length} running</span>
              {stoppedSessions.length > 0 && (
                <span className="px-2.5 py-0.5 bg-slate-100 dark:bg-slate-700 text-slate-500 dark:text-slate-400 text-xs rounded-lg">{stoppedSessions.length} stopped</span>
              )}
            </div>            {loading ? (
              <div className="text-slate-400 dark:text-slate-500 text-sm py-8 text-center">Loading...</div>
            ) : sessions.length === 0 ? (
              <div className="text-slate-400 dark:text-slate-500 text-sm py-8 text-center">
                <i className="fa-solid fa-tower-broadcast text-3xl mb-2 text-slate-300 dark:text-slate-600"></i>
                <p>No relay sessions configured</p>
                <p className="text-xs mt-1 text-slate-400 dark:text-slate-500">Start a new relay below to begin pivoting</p>              </div>
            ) : (
              <div className="space-y-3">                {sessions.map((s, i) => (
                  <div key={s.id || i} className={`border rounded-2xl p-4 transition-colors ${s.active ? "border-emerald-200 bg-emerald-50/50 dark:border-emerald-800 dark:bg-emerald-900/20" : "border-[var(--border)] opacity-60"}`}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-x-3">
                        <span className={`w-2.5 h-2.5 rounded-full ${s.active ? "bg-emerald-500 animate-pulse" : "bg-slate-300 dark:bg-slate-600"}`}></span>
                        <div>
                          <div className="font-medium text-sm text-slate-900 dark:text-slate-100">{s.agent_id.substring(0, 12)}... {s.hostname && <span className="text-slate-400 dark:text-slate-500 font-normal">({s.hostname})</span>}</div>
                          <div className="text-xs text-slate-500 dark:text-slate-400">Port {s.listen_port} {s.active ? "Running" : "Stopped"}  {formatCreated(s.created_at)}</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-x-3">
                        <div className="text-xs text-slate-500 dark:text-slate-400 flex items-center gap-1.5">
                          <i className="fa-solid fa-network-wired"></i>
                          {s.active_conn || 0} active / {s.conn_count || 0} total                        </div>
                        <button
                          onClick={() => {                            setThroughAgent(s.agent_id);
                            showToast(`Routing via ${s.agent_id.substring(0, 12)}`, "info");
                          }}
                          className={`px-3 py-1.5 text-xs rounded-xl transition-colors ${                            throughAgent === s.agent_id                              ? "bg-indigo-600 text-white"
                              : "bg-slate-100 dark:bg-slate-700 text-[var(--text-secondary)] hover:bg-indigo-50 dark:hover:bg-indigo-900/20"                          }`}
                        >                          <i className={`fa-solid ${throughAgent === s.agent_id ? "fa-check" : "fa-route"}`}></i>                          {throughAgent === s.agent_id ? " Selected" : " Through Me"}
                        </button>
                        {s.active && (
                          <button onClick={() => stopRelay(s.agent_id)} className="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white rounded-xl text-xs transition-colors">
                            <i className="fa-solid fa-stop"></i>                          </button>
                        )}
                      </div>                    </div>                    <div className="mb-1.5 flex justify-between text-xs text-slate-500">
                      <span className="flex items-center gap-1"><i className="fa-solid fa-arrow-down text-emerald-500"></i>{formatBytes(s.bytes_in || 0)} down</span>
                      <span className="flex items-center gap-1"><i className="fa-solid fa-arrow-up text-blue-500"></i>{formatBytes(s.bytes_out || 0)} up</span>
                    </div>
                    <div className="flex h-2 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
                      <div className="bg-emerald-500 transition-all" style={{ width: `${tabOverallMax > 0 ? ((s.bytes_in || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                      <div className="bg-blue-500 transition-all" style={{ width: `${tabOverallMax > 0 ? ((s.bytes_out || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                    </div>                  </div>
                ))}
              </div>            )}
          </div>

          <div className="ui-card p-6 shadow-sm">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2 mb-4">
              <i className="fa-solid fa-plus-circle text-indigo-500"></i> Start New SOCKS Relay
            </div>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Target Agent</label>
                <select value={selectedAgent} onChange={e => setSelectedAgent(e.target.value)}                  className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                  <option value="">-- Select Agent --</option>
                  {agents.map(a => (
                    <option key={a.id} value={a.id}>{a.hostname} ({a.ip})</option>
                  ))}
                </select>
              </div>              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Listen Host</label>
                  <input type="text" value={relayHost} onChange={e => setRelayHost(e.target.value)} placeholder="127.0.0.1"
                    className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100" />                </div>
                <div>                  <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Listen Port</label>
                  <input type="number" value={relayPort} onChange={e => setRelayPort(Number(e.target.value))} min={1} max={65535}
                    className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100" />
                </div>
              </div>
            </div>            <div className="mt-4 grid grid-cols-1 lg:grid-cols-3 gap-4 items-end">
              <div>                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Protocol</label>                <select value={relayProtocol} onChange={e => setRelayProtocol(e.target.value as "socks" | "http")}
                  className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                  <option value="socks">SOCKS5</option>
                  <option value="http">HTTP CONNECT</option>
                </select>
              </div>
              <button onClick={startRelay} disabled={!selectedAgent || starting}
                className="lg:col-span-2 h-[42px] px-6 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-xl text-sm font-medium whitespace-nowrap transition-colors flex items-center justify-center gap-2">
                {starting ? <><i className="fa-solid fa-spinner fa-spin"></i> Starting...</> : <><i className="fa-solid fa-play"></i> Start {relayProtocol.toUpperCase()} Relay</>}
              </button>
            </div>
            <div className="mt-3 p-3 bg-slate-50 dark:bg-slate-700/50 rounded-xl text-xs text-slate-500 dark:text-slate-400 flex items-start gap-1.5">
              <i className="fa-solid fa-info-circle text-slate-400 mt-0.5"></i>              <span>Agent connects outbound to this SOCKS server. Configure your proxychains to point to the listen address.</span>
            </div>          </div>
        </>
      )}

      {activeTab === "local" && (
        <>          <div className="ui-card p-6 shadow-sm">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2 mb-4">
              <i className="fa-solid fa-sitemap text-amber-500"></i>
              <span>Local Proxy Configuration</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Proxy Port</label>
                <input type="number" value={localPort} onChange={e => setLocalPort(Number(e.target.value))} min={1} max={65535}
                  className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-amber-500 dark:text-slate-100" />
              </div>
              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Route Through</label>
                <select value={throughAgent} onChange={e => setThroughAgent(e.target.value)}
                  className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-amber-500 dark:text-slate-100">
                  <option value="">-- Direct (no pivot) --</option>
                  {sessions.filter(s => s.active).map(s => (
                    <option key={s.agent_id} value={s.agent_id}>{s.agent_id.substring(0, 12)}... (:{s.listen_port})</option>
                  ))}
                  {agents.map(a => (
                    <option key={a.id} value={a.id}>{a.hostname} ({a.ip})</option>
                  ))}                </select>
              </div>
              <div>                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Authentication</label>
                <div className="flex items-center gap-3 mt-2">                  <label className="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" checked={localAuthEnabled} onChange={e => setLocalAuthEnabled(e.target.checked)} className="rounded border-slate-300 text-amber-600 focus:ring-amber-500" />                    <span className="text-sm text-[var(--text-secondary)]">Enable</span>
                  </label>
                </div>
              </div>
            </div>            {localAuthEnabled && (
              <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>                  <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Username</label>
                  <input type="text" value={localUsername} onChange={e => setLocalUsername(e.target.value)} placeholder="operator"
                    className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-amber-500 dark:text-slate-100" />
                </div>                <div>
                  <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Password</label>
                  <input type="password" value={localPassword} onChange={e => setLocalPassword(e.target.value)} placeholder="????????"
                    className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-amber-500 dark:text-slate-100" />                </div>
              </div>
            )}
          </div>          <div className="ui-card p-6 shadow-sm">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2 mb-3">
              <i className="fa-solid fa-network-wired text-amber-500"></i> Direct Agent SOCKS            </div>
            <p className="text-sm text-slate-500 dark:text-slate-400 mb-4">
              Start a SOCKS listener directly on the agent host. Only usable when you can reach the agent IP.
            </p>
            {agents.length > 0 ? (              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                {agents.map(a => (                  <div key={a.id} className="border border-[var(--border)] rounded-2xl p-4 hover:border-amber-300 dark:hover:border-amber-700 hover:shadow-sm transition-all">
                    <div className="flex justify-between items-center mb-1">
                      <div className="font-medium text-slate-900 dark:text-slate-100">{a.hostname}</div>
                      <span className="text-xs text-slate-400 dark:text-slate-500">{a.ip}</span>
                    </div>
                    <div className="text-xs text-slate-500 dark:text-slate-400 mb-3 font-mono">{a.id.substring(0, 12)}...</div>
                    <button onClick={() => {
                      const body = new URLSearchParams();
                      body.append("port", localPort.toString());
                      fetch(`${API_BASE}?p=/agents/${a.id}/socks`, {
                        method: "POST",
                        headers: { "Content-Type": "application/x-www-form-urlencoded" },
                        body: body.toString(),                      });
                      showToast(`Direct SOCKS on ${a.hostname}:${localPort}`, "success");
                    }}
                      className="w-full text-xs px-3 py-2 bg-amber-100 hover:bg-amber-200 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 rounded-xl transition-colors flex items-center justify-center gap-1.5">
                      <i className="fa-solid fa-play"></i> Start SOCKS on :{localPort}
                    </button>
                  </div>
                ))}              </div>
            ) : (
              <div className="text-slate-400 dark:text-slate-500 text-sm py-4 text-center">No agents online</div>            )}
          </div>
        </>
      )}

      {activeTab === "rport" && (
        <>
          <div className="ui-card p-6 shadow-sm">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2 mb-4">              <i className="fa-solid fa-arrow-right-arrow-left text-blue-500"></i>              <span>Active Reverse Port Forwards</span>
              <span className="ml-2 px-2.5 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 text-xs font-semibold rounded-lg">{rportForwards.filter(r => r.active).length} active</span>
            </div>
            {rportForwards.length === 0 ? (
              <div className="text-slate-400 dark:text-slate-500 text-sm py-8 text-center">
                <i className="fa-solid fa-arrow-right-arrow-left text-3xl mb-2 text-slate-300 dark:text-slate-600"></i>                <p>No reverse port forwards configured</p>
              </div>
            ) : (
              <div className="space-y-3">
                {rportForwards.map(rf => (
                  <div key={rf.id} className={`rounded-2xl p-4 transition-colors border ${rf.active ? "border-blue-200 bg-blue-50/50 dark:border-blue-800 dark:bg-blue-900/20" : "border-[var(--border)] opacity-60"}`}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-x-3">                        <span className={`w-2.5 h-2.5 rounded-full ${rf.active ? "bg-blue-500 animate-pulse" : "bg-slate-300 dark:bg-slate-600"}`}></span>
                        <div>                          <div className="text-sm font-medium text-slate-900 dark:text-slate-100 flex items-center gap-2">                            <span className="font-mono">{rf.remote_host}:{rf.remote_port}</span>
                            <i className="fa-solid fa-arrow-right text-slate-400 text-xs"></i>
                            <span className="font-mono">localhost:{rf.local_port}</span>
                            <span className="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-400 text-xs rounded">{rf.protocol.toUpperCase()}</span>
                          </div>
                          <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Agent: {rf.agent_id.substring(0, 12)}... {rf.active ? `Up: ${formatUptime(rf.uptime)}` : "Stopped"}{rf.error && <span className="text-red-500 ml-2">{rf.error}</span>}</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <button                          onClick={() => checkRPortStatus(rf.id)}
                          className="px-3 py-1.5 bg-slate-100 dark:bg-slate-700 text-[var(--text-secondary)] rounded-xl text-xs transition-colors flex items-center gap-1"
                        >
                          <i className="fa-solid fa-circle-info"></i> Status
                        </button>
                        <button
                          onClick={() => stopRPort(rf.id)}
                          className="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white rounded-xl text-xs transition-colors flex items-center gap-1"
                        >                          <i className="fa-solid fa-stop"></i> Stop
                        </button>
                      </div>                    </div>
                    <div className="mb-1.5 flex justify-between text-xs text-slate-500">
                      <span className="flex items-center gap-1"><i className="fa-solid fa-arrow-down text-emerald-500"></i>{formatBytes(rf.bytes_in || 0)}</span>
                      <span className="flex items-center gap-1"><i className="fa-solid fa-arrow-up text-blue-500"></i>{formatBytes(rf.bytes_out || 0)}</span>                    </div>
                    <div className="flex h-2 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">                      <div className="bg-emerald-500 transition-all" style={{ width: `${tabOverallMax > 0 ? ((rf.bytes_in || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                      <div className="bg-blue-500 transition-all" style={{ width: `${tabOverallMax > 0 ? ((rf.bytes_out || 0) / tabOverallMax) * 100 : 0}%` }}></div>
                    </div>
                  </div>
                ))}
              </div>            )}
          </div>

          <div className="ui-card p-6 shadow-sm">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2 mb-4">              <i className="fa-solid fa-plus-circle text-blue-500"></i> New Reverse Port Forward
            </div>            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
              <div>                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Agent</label>
                <select value={rportAgent} onChange={e => setRportAgent(e.target.value)}
                  className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500 dark:text-slate-100">
                  <option value="">-- Select Agent --</option>
                  {agents.map(a => (                    <option key={a.id} value={a.id}>{a.hostname} ({a.ip})</option>
                  ))}
                </select>              </div>
              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Remote Host</label>                <input type="text" value={rportRemoteHost} onChange={e => setRportRemoteHost(e.target.value)} placeholder="127.0.0.1"
                  className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500 dark:text-slate-100" />
              </div>              <div>                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Remote Port</label>
                <input type="number" value={rportRemotePort} onChange={e => setRportRemotePort(Number(e.target.value))} min={1} max={65535}                  className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500 dark:text-slate-100" />              </div>
              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Local Port</label>                <input type="number" value={rportLocalPort} onChange={e => setRportLocalPort(Number(e.target.value))} min={1} max={65535}
                  className="w-full ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500 dark:text-slate-100" />              </div>
            </div>            <div className="flex items-center gap-3">
              <div>
                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1 block">Protocol</label>
                <select value={rportProtocol} onChange={e => setRportProtocol(e.target.value as "tcp" | "udp")}
                  className="ui-card px-3 py-2.5 text-sm focus:outline-none focus:border-blue-500 dark:text-slate-100">
                  <option value="tcp">TCP</option>
                  <option value="udp">UDP</option>                </select>
              </div>
              <button onClick={startRPort} disabled={!rportAgent}
                className="h-[42px] px-6 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-xl text-sm font-medium transition-colors flex items-center gap-2">
                <i className="fa-solid fa-play"></i> Start Forward</button>
              <button                onClick={() => {                  setRportForwards(prev => [...prev]);
                  showToast("Refreshing RPort status...", "info");
                  loadData();
                }}
                className="h-[42px] px-4 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-xl text-sm transition-colors flex items-center gap-1.5"
              >
                <i className="fa-solid fa-rotate"></i> Refresh Status
              </button>
            </div>
            <div className="mt-3 p-3 bg-blue-50 dark:bg-blue-900/20 rounded-xl text-xs text-blue-700 dark:text-blue-400 flex items-start gap-1.5">
              <i className="fa-solid fa-info-circle text-blue-500 mt-0.5"></i>              <span>Connect to localhost:{rportLocalPort} to reach {rportRemoteHost || "[target]"}:{rportRemotePort} via the agent. The agent establishes the outbound connection.</span>
            </div>
          </div>        </>
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Disconnect" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>  );
}