"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";

interface LateralStats {
  online_agents?: number;
  OnlineAgents?: number;
  total_creds?: number;
  TotalCreds?: number;
  total_tasks?: number;
  TotalTasks?: number;
}

interface Credential {
  id?: string;
  username?: string;
  domain?: string;
  target?: string;
}

interface Agent {
  id?: string;
  ID?: string;
  hostname?: string;
  Hostname?: string;
  ip?: string;
  IP?: string;
}

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
  { key: "smb", label: "SMB", desc: "PsExec / Service Creation", icon: "fa-share-nodes" },
  { key: "winrm", label: "WinRM", desc: "PowerShell Remoting", icon: "fa-terminal" },
  { key: "wmi", label: "WMI", desc: "Process Creation", icon: "fa-gears" },
  { key: "ssh", label: "SSH", desc: "Linux/Unix Remote", icon: "fa-key" },
  { key: "pth", label: "Pass-the-Hash", desc: "NTLM Hash Auth", icon: "fa-ticket" },
];

export default function LateralPage() {
  const [stats, setStats] = useState<LateralStats>({});
  const [loading, setLoading] = useState(true);
  const [activeMethod, setActiveMethod] = useState("smb");
  const [form, setForm] = useState<FormData>(defaultForm);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [history, setHistory] = useState<MovementHistory[]>([]);
  const [submitting, setSubmitting] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const resp = await fetch(`${API_BASE}?p=/lateral&format=json`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      setStats(data);
    } catch (e) { console.error("Lateral: load stats failed", e); }
    try {
      const agentRes = await fetch(`${API_BASE}?p=/agents&format=json`);
      if (!agentRes.ok) throw new Error(`HTTP ${agentRes.status}`);
      const agentData = await agentRes.json();
      setAgents(agentData.Agents || agentData.agents || []);
    } catch { setAgents([]); }
    try {
      const credRes = await fetch(`${API_BASE}?p=/credentials&format=json`);
      if (!credRes.ok) throw new Error(`HTTP ${credRes.status}`);
      const credData = await credRes.json();
      setCredentials(credData.VaultEntries || credData.vault_entries || []);
    } catch { setCredentials([]); }
    try {
      const histRes = await fetch(`${API_BASE}?p=/api/v1/tasks?type=lateral&limit=50&format=json`);
      if (!histRes.ok) throw new Error(`HTTP ${histRes.status}`);
      const histData = await histRes.json();
      setHistory(histData.data || histData.Data || []);
    } catch { setHistory([]); }
    setLoading(false);
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadData()); }, [loadData]);

  const updateForm = (key: keyof FormData, value: string) => {
    setForm(prev => ({ ...prev, [key]: value }));
  };

  const handleSubmit = async () => {
    if (!form.source || !form.target) return;
    setSubmitting(true);
    try {
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

      await fetch(`${API_BASE}?p=/api/lateral/execute&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      loadData();
    } catch (e) { console.error("Lateral: execute command failed", e); }
    setSubmitting(false);
  };

  const getStatusBadge = (status: string) => {
    const s = status?.toLowerCase() ?? "";
    if (s === "success" || s === "completed") return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400";
    if (s === "failed") return "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400";
    if (s === "running") return "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400";
    return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400";
  };

  const renderMethodForm = () => {
    switch (activeMethod) {
      case "smb":
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-folder-tree mr-1.5 text-slate-400"></i>Share
                </label>
                <input type="text" placeholder="ADMIN$" className="ui-input font-mono" value={form.share} onChange={e => updateForm("share", e.target.value)} />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-id-card mr-1.5 text-slate-400"></i>Username
                </label>
                <input type="text" placeholder="DOMAIN\username" className="ui-input font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
              </div>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                <i className="fa-solid fa-lock mr-1.5 text-slate-400"></i>Password
              </label>
              <input type="password" placeholder="Password" className="ui-input font-mono" value={form.password} onChange={e => updateForm("password", e.target.value)} />
            </div>
          </>
        );
      case "winrm":
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-network-wired mr-1.5 text-slate-400"></i>Port
                </label>
                <input type="text" placeholder="5985" className="ui-input font-mono" value={form.port} onChange={e => updateForm("port", e.target.value)} />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-id-card mr-1.5 text-slate-400"></i>Username
                </label>
                <input type="text" placeholder="Administrator" className="ui-input font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
              </div>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                <i className="fa-solid fa-lock mr-1.5 text-slate-400"></i>Password
              </label>
              <input type="password" placeholder="Password" className="ui-input font-mono" value={form.password} onChange={e => updateForm("password", e.target.value)} />
            </div>
          </>
        );
      case "wmi":
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-code mr-1.5 text-slate-400"></i>Namespace
                </label>
                <input type="text" placeholder="root\cimv2" className="ui-input font-mono" value={form.namespace} onChange={e => updateForm("namespace", e.target.value)} />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-id-card mr-1.5 text-slate-400"></i>Username
                </label>
                <input type="text" placeholder="DOMAIN\username" className="ui-input font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
              </div>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                <i className="fa-solid fa-lock mr-1.5 text-slate-400"></i>Password
              </label>
              <input type="password" placeholder="Password" className="ui-input font-mono" value={form.password} onChange={e => updateForm("password", e.target.value)} />
            </div>
          </>
        );
      case "ssh":
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-network-wired mr-1.5 text-slate-400"></i>Port
                </label>
                <input type="text" placeholder="22" className="ui-input font-mono" value={form.port} onChange={e => updateForm("port", e.target.value)} />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-id-card mr-1.5 text-slate-400"></i>Username
                </label>
                <input type="text" placeholder="root" className="ui-input font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-lock mr-1.5 text-slate-400"></i>Password
                </label>
                <input type="password" placeholder="Password" className="ui-input font-mono" value={form.password} onChange={e => updateForm("password", e.target.value)} />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                  <i className="fa-solid fa-key mr-1.5 text-slate-400"></i>Key Path (optional)
                </label>
                <input type="text" placeholder="/path/to/key.pem" className="ui-input font-mono" value={form.key_path} onChange={e => updateForm("key_path", e.target.value)} />
              </div>
            </div>
          </>
        );
      case "pth":
        return (
          <>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                <i className="fa-solid fa-id-card mr-1.5 text-slate-400"></i>Username
              </label>
              <input type="text" placeholder="Administrator" className="ui-input font-mono" value={form.username} onChange={e => updateForm("username", e.target.value)} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                <i className="fa-solid fa-ticket mr-1.5 text-slate-400"></i>NTLM Hash
              </label>
              <input type="text" placeholder="aad3b435b51404eeaad3b435b51404ee:..." className="ui-input font-mono" value={form.hash} onChange={e => updateForm("hash", e.target.value)} />
            </div>
          </>
        );
      default:
        return null;
    }
  };

  if (loading)
    return (
      <div className="flex items-center justify-center h-64">
        <i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i>
      </div>
    );

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">横向移动</h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs sm:text-sm mt-1">SMB/WinRM/WMI/PsExec 远程执行 / Pass-the-Hash</p>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">在线 Implant</div>
              <div className="text-3xl font-bold mt-2 text-emerald-600">{stats.OnlineAgents || stats.online_agents || 0}</div>
              <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">可用于跳板</div>
            </div>
            <div className="w-12 h-12 bg-emerald-50 dark:bg-emerald-900/20 rounded-xl flex items-center justify-center">
              <i className="fa-solid fa-robot text-xl text-emerald-400"></i>
            </div>
          </div>
        </div>
        <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">可用凭据</div>
              <div className="text-3xl font-bold mt-2 text-amber-600">{stats.TotalCreds || stats.total_creds || 0}</div>
              <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">凭据保险库</div>
            </div>
            <div className="w-12 h-12 bg-amber-50 dark:bg-amber-900/20 rounded-xl flex items-center justify-center">
              <i className="fa-solid fa-key text-xl text-amber-400"></i>
            </div>
          </div>
        </div>
        <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">历史任务</div>
              <div className="text-3xl font-bold mt-2 text-blue-600">{stats.TotalTasks || stats.total_tasks || 0}</div>
              <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">横向移动记录</div>
            </div>
            <div className="w-12 h-12 bg-blue-50 dark:bg-blue-900/20 rounded-xl flex items-center justify-center">
              <i className="fa-solid fa-network-wired text-xl text-blue-400"></i>
            </div>
          </div>
        </div>
      </div>

      <div className="ui-card p-6 shadow-sm mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/20 rounded-xl flex items-center justify-center">
            <i className="fa-solid fa-arrows-split-up-and-left text-indigo-600 dark:text-indigo-400"></i>
          </div>
          <div>
            <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">新建横向移动任务</div>
            <div className="text-xs text-slate-500 dark:text-slate-400">选择 Implant、目标主机和执行方式</div>
          </div>
        </div>

        <div className="mb-6">
          <div className="flex gap-1 overflow-x-auto pb-1">
            {methods.map((m) => (
              <button key={m.key} onClick={() => setActiveMethod(m.key)}
                className={`flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-medium whitespace-nowrap transition-colors ${
                  activeMethod === m.key
                    ? "bg-indigo-50 text-indigo-600 border border-indigo-200 dark:bg-indigo-900/20 dark:text-indigo-400 dark:border-indigo-700"
                    : "text-slate-500 hover:text-slate-700 hover:bg-slate-50 dark:hover:bg-slate-700 dark:hover:text-slate-300 border border-transparent"
                }`}>
                <i className={`fa-solid ${m.icon}`}></i>
                {m.label}
              </button>
            ))}
          </div>
        </div>

        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                <i className="fa-solid fa-robot mr-1.5 text-slate-400"></i>源 Implant
              </label>
              <select className="ui-select" value={form.source} onChange={e => updateForm("source", e.target.value)}>
                <option value="">选择在线 Implant...</option>
                {agents.map(a => {
                  const aid = a.ID || a.id || "";
                  const host = a.Hostname || a.hostname || "";
                  return <option key={aid} value={aid}>{host}</option>;
                })}
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                <i className="fa-solid fa-crosshairs mr-1.5 text-slate-400"></i>目标主机
              </label>
              <input type="text" placeholder="IP 或主机名" className="ui-input font-mono" value={form.target} onChange={e => updateForm("target", e.target.value)} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
                <i className="fa-solid fa-route mr-1.5 text-slate-400"></i>跳板 Agent
              </label>
              <select className="ui-select" value={form.pivot} onChange={e => updateForm("pivot", e.target.value)}>
                <option value="">直接连接 (无跳板)</option>
                {agents.map(a => {
                  const aid = a.ID || a.id || "";
                  const host = a.Hostname || a.hostname || "";
                  return <option key={aid} value={aid}>通过 {host}</option>;
                })}
              </select>
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
              <i className="fa-solid fa-wallet mr-1.5 text-slate-400"></i>凭据            </label>
            <select className="ui-select" value={form.credential} onChange={e => updateForm("credential", e.target.value)}>
              <option value="">-- 手动输入凭据 --</option>
              {credentials.map(c => {
                const cid = c.id || "";
                const cuser = c.username || "";
                const cdomain = c.domain || "";
                const ctarget = c.target || "";
                return <option key={cid} value={cid}>{cdomain ? `${cdomain}\\` : ""}{cuser} ({ctarget})</option>;
              })}
            </select>
          </div>

          <div className="border-t border-[var(--border)] pt-4">
            <div className="text-xs font-semibold text-slate-600 dark:text-slate-300 mb-3">
              <i className={`fa-solid ${methods.find(m => m.key === activeMethod)?.icon} mr-1.5`}></i>
              {methods.find(m => m.key === activeMethod)?.label} 配置
            </div>
            {renderMethodForm()}
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">
              <i className="fa-solid fa-terminal mr-1.5 text-slate-400"></i>执行命令
            </label>
            <input type="text" placeholder="whoami /all | powershell -enc ..." className="ui-input font-mono" value={form.command} onChange={e => updateForm("command", e.target.value)} />
          </div>

          <div className="flex gap-3 pt-4 border-t border-[var(--border)]">
            <button onClick={handleSubmit} disabled={submitting || !form.source || !form.target}
              className="flex-1 h-11 ui-btn ui-btn-primary disabled:opacity-50 disabled:cursor-not-allowed">
              <i className={`fa-solid ${submitting ? "fa-circle-notch fa-spin" : "fa-rocket"}`}></i>
              <span>{submitting ? "执行中..." : "执行横向移动"}</span>
            </button>
          </div>
        </div>
      </div>

      <div className="ui-card shadow-sm overflow-hidden">
        <div className="flex items-center justify-between p-5 border-b border-[var(--border)]">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-blue-100 dark:bg-blue-900/20 rounded-lg flex items-center justify-center">
              <i className="fa-solid fa-clock-rotate-left text-blue-600 dark:text-blue-400 text-sm"></i>
            </div>
            <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">移动历史</span>
          </div>
            <span className="text-xs text-slate-500">{history.length} 条记录</span>
        </div>
        {history.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[var(--border)] bg-slate-50 dark:bg-slate-800/50">
                  <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300"></th>
                  <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">目标</th>
                  <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">方式</th>
                  <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">跳板</th>
                  <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">状态</th>
                  <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">时间</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                {history.map((h, i) => (
                  <tr key={h.id || i} className="hover:bg-slate-50 dark:hover:bg-slate-700/50">
                    <td className="py-3 px-4 font-mono text-[var(--text-secondary)]">{h.source || "-"}</td>
                    <td className="py-3 px-4 font-mono text-[var(--text-secondary)]">{h.target || "-"}</td>
                    <td className="py-3 px-4">
                      <span className="text-[10px] px-2 py-0.5 rounded-full bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">{h.method || "-"}</span>
                    </td>
                    <td className="py-3 px-4 text-slate-500 text-xs">{h.pivot || "直连"}</td>
                    <td className="py-3 px-4">
                      <span className={`text-[10px] px-2 py-0.5 rounded-full ${getStatusBadge(h.status || "")}`}>{h.status || "-"}</span>
                    </td>
                    <td className="py-3 px-4 text-xs text-slate-500">{h.created_at || "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="text-center py-12 text-slate-400 dark:text-slate-500">
            <i className="fa-solid fa-inbox text-3xl mb-2 text-slate-300 dark:text-slate-600"></i>
            <p>暂无横向移动记录</p>
          </div>
        )}
      </div>
    </div>
  );
}
