"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal } from "@/components/UI";

interface PrivescAgent {
  id?: string;
  ID?: string;
  hostname?: string;
  Hostname?: string;
  ip?: string;
  IP?: string;
  os?: string;
  OS?: string;
}

interface PrivescHistory {
  id?: string;
  ID?: string;
  agent_id?: string;
  AgentID?: string;
  check_type?: string;
  CheckType?: string;
  status?: string;
  Status?: string;
  result?: string;
  Result?: string;
  created_at?: string;
  CreatedAt?: string;
  findings_count?: number;
  FindingsCount?: number;
}

interface PrivescFinding {
  id?: string;
  title?: string;
  severity?: string;
  cve_id?: string;
  description?: string;
  exploit_command?: string;
  recommendation?: string;
}

const CHECK_TYPES = [
  { value: "all", icon: "🔍", label: "全部", desc: "执行所有权限检查" },
  { value: "printnightmare", icon: "🖨️", label: "PrintNightmare", desc: "Windows 打印服务漏洞" },
  { value: "elevate", icon: "⬆️", label: "Elevate", desc: "权限提升" },
  { value: "uac_bypass", icon: "🛡️", label: "UAC Bypass", desc: "用户账户控制绕过" },
  { value: "amsi_bypass", icon: "🛡️", label: "AMSI Bypass", desc: "反恶意软件扫描绕过" },
  { value: "etw_bypass", icon: "🔇", label: "ETW Bypass", desc: "事件跟踪绕过" },
  { value: "cvescan", icon: "🔬", label: "CVE Scan", desc: "已知 CVE 漏洞扫描" },
  { value: "binary_abuse", icon: "⚠️", label: "Binary Abuse", desc: "可执行文件滥用" },
  { value: "service_exploit", icon: "⚙️", label: "Service Exploit", desc: "服务配置漏洞" },
  { value: "token_abuse", icon: "🎭", label: "Token Abuse", desc: "令牌操作滥用" },
  { value: "kernel_exploit", icon: "💻", label: "Kernel Exploit", desc: "系统内核漏洞" },
  { value: "password_finder", icon: "🔑", label: "Password Finder", desc: "凭据文件搜索" },
];

function severityBadge(severity: string) {
  switch (severity) {
    case "critical": return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30";
    case "high": return "bg-orange-500/10 text-orange-600 dark:text-orange-400 border-orange-500/30";
    case "medium": return "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400 border-yellow-500/30";
    case "low": return "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30";
    default: return "bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/30";
  }
}

function severityIcon(severity: string) {
  switch (severity) {
    case "critical": return "fa-circle-exclamation text-red-500";
    case "high": return "fa-triangle-exclamation text-orange-500";
    case "medium": return "fa-circle-exclamation text-yellow-500";
    case "low": return "fa-circle-info text-blue-500";
    default: return "fa-circle-question text-slate-400";
  }
}

export default function PrivescPage() {
  const [agents, setAgents] = useState<PrivescAgent[]>([]);
  const [history, setHistory] = useState<PrivescHistory[]>([]);
  const [findings, setFindings] = useState<PrivescFinding[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [checkType, setCheckType] = useState("all");
  const [running, setRunning] = useState(false);
  const [expandedFinding, setExpandedFinding] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("all");
  const [toastMsg, setToastMsg] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => { if (toastMsg) { const t = setTimeout(() => setToastMsg(null), 3000); return () => clearTimeout(t); } }, [toastMsg]);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const loadData = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/privesc&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setAgents(data.agents || data.Agents || []);
      setHistory(data.history || data.History || []);
      setFindings(data.findings || data.Findings || []);
    } catch {
      setAgents([]);
      setHistory([]);
      setFindings([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    Promise.resolve().then(() => {
      loadData();
      intervalRef.current = setInterval(loadData, 10000);
    });
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [loadData]);

  const handleRun = async () => {
    if (!selectedAgent) {
      setToastMsg("请选择目标 Agent");
      return;
    }
    setRunning(true);
    try {
      await fetch(`${API_BASE}?p=/privesc/run&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ agent_id: selectedAgent, check_type: checkType }),
      });
      setTimeout(loadData, 2000);
    } catch (e) { console.error("Privesc: start check failed", e); }
    setRunning(false);
  };

  const handleViewHistory = async (historyId: string) => {
    try {
      const res = await fetch(`${API_BASE}?p=/api/privesc/history/${historyId}&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setFindings(data.findings || data.Findings || data.tasks || data.Tasks || []);
    } catch (e) { console.error("Privesc: load history failed", e); }
  };

  const handleProcessResult = async (historyId: string) => {
    try {
      await fetch(`${API_BASE}?p=/api/privesc/result&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ history_id: historyId }),
      });
      loadData();
    } catch (e) { console.error("Privesc: process result failed", e); }
  };

  const handleExecuteExploit = (finding: PrivescFinding) => {
    setCfm({msg: `确定要执行以下提权命令？\n\n${finding.title || "未知"}`, cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/privesc/execute&format=json`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ agent_id: selectedAgent, check_type: checkType, exploit_command: finding.exploit_command }),
        });
      } catch (e) { console.error("Privesc: execute exploit failed", e); }
    }});
  };

  const handleExportJSON = () => {
    const blob = new Blob([JSON.stringify(findings, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `privesc_findings_${new Date().toISOString().split("T")[0]}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleExportCSV = () => {
    const headers = ["Title", "Severity", "CVE", "Description", "Recommendation"];
    const rows = findings.map((f) => [
      f.title || "", f.severity || "", f.cve_id || "", (f.description || "").replace(/,/g, ";"), (f.recommendation || "").replace(/,/g, ";"),
    ]);
    const csv = [headers, ...rows].map((r) => r.map((c) => `"${c}"`).join(",")).join("\n");
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `privesc_findings_${new Date().toISOString().split("T")[0]}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const formatTime = (t: string) => {
    if (!t) return "-";
    try { return new Date(t).toLocaleString(); } catch { return t; }
  };

  const totalChecks = history.length;
  const criticalCount = findings.filter((f) => f.severity === "critical").length;
  const highCount = findings.filter((f) => f.severity === "high").length;
  const mediumCount = findings.filter((f) => f.severity === "medium").length;
  const lowCount = findings.filter((f) => f.severity === "low").length;

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">
            <i className="fa-solid fa-shield-virus text-indigo-500 mr-2"></i>权限提升扫描
          </h1>
          <p className="text-xs sm:text-sm text-slate-500 dark:text-slate-400 mt-1">发现系统潜在漏洞并获得最高权限</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={handleExportJSON} className="px-3 h-9 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-xl text-xs font-medium flex items-center gap-1.5 transition-colors">
            <i className="fa-solid fa-file-code"></i> JSON
          </button>
          <button onClick={handleExportCSV} className="px-3 h-9 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-xl text-xs font-medium flex items-center gap-1.5 transition-colors">
            <i className="fa-solid fa-file-csv"></i> CSV
          </button>
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-5 gap-4 mb-6">
        <div className="ui-card p-4 shadow-sm">
          <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">总检查数</div>
          <div className="text-2xl font-bold mt-1 text-indigo-600 dark:text-indigo-400">{totalChecks}</div>
        </div>
        <div className="ui-card p-4 shadow-sm">
          <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">严重</div>
          <div className="text-2xl font-bold mt-1 text-red-600 dark:text-red-400">{criticalCount}</div>
        </div>
        <div className="ui-card p-4 shadow-sm">
          <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">高危</div>
          <div className="text-2xl font-bold mt-1 text-orange-600 dark:text-orange-400">{highCount}</div>
        </div>
        <div className="ui-card p-4 shadow-sm">
          <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">中危</div>
          <div className="text-2xl font-bold mt-1 text-yellow-600 dark:text-yellow-400">{mediumCount}</div>
        </div>
        <div className="ui-card p-4 shadow-sm">
          <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">低危</div>
          <div className="text-2xl font-bold mt-1 text-blue-600 dark:text-blue-400">{lowCount}</div>
        </div>
      </div>

      <div className="ui-card p-6 shadow-sm mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center">
            <i className="fa-solid fa-shield-virus text-indigo-600 dark:text-indigo-400"></i>
          </div>
          <div>
            <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">新建权限提升任务</div>
            <div className="text-xs text-slate-500 dark:text-slate-400">选择 Agent 执行权限提升任务</div>
          </div>
        </div>

        <div className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-[var(--text-secondary)] mb-2">
              <i className="fa-solid fa-robot mr-1.5 text-slate-400"></i>目标 Agent
            </label>
            <select value={selectedAgent} onChange={(e) => setSelectedAgent(e.target.value)}
              className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                <option value="">选择目标 Implant...</option>
              {agents.map((a) => {
                const id = a.id || a.ID || "";
                const hostname = a.hostname || a.Hostname || "";
                const ip = a.ip || a.IP || "";
                const os = a.os || a.OS || "";
                return <option key={id} value={id}>{hostname} ({ip}) - {os}</option>;
              })}
            </select>
          </div>

          <div>
            <label className="block text-sm font-semibold text-slate-900 dark:text-slate-100 mb-3">检查类型</label>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {CHECK_TYPES.map((ct) => (
                <label key={ct.value} className={`flex items-center gap-3 p-3 rounded-xl cursor-pointer transition-colors ${checkType === ct.value ? "border-2 border-indigo-500 bg-indigo-50 dark:bg-indigo-900/20" : "border border-[var(--border)] hover:bg-slate-50 dark:hover:bg-slate-700/50"}`}>
                  <input type="radio" name="check_type" value={ct.value} checked={checkType === ct.value} onChange={() => setCheckType(ct.value)} className="w-4 h-4 accent-indigo-600 shrink-0" />
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{ct.icon} {ct.label}</div>
                    <div className="text-xs text-slate-500 dark:text-slate-400 truncate">{ct.desc}</div>
                  </div>
                </label>
              ))}
            </div>
          </div>

          <div className="flex gap-3 pt-3 border-t border-[var(--border)]">
            <button onClick={handleRun} disabled={running} className="flex-1 h-11 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-sm font-medium rounded-xl transition-colors flex items-center justify-center">
              {running ? <><i className="fa-solid fa-spinner fa-spin mr-2"></i>执行中...</> : <><i className="fa-solid fa-play mr-2"></i>执行权限提升</>}
            </button>
          </div>
        </div>
      </div>

      <div className="ui-card mb-6 overflow-hidden">
        <div className="px-6 py-4 border-b border-[var(--border)] flex items-center justify-between flex-wrap gap-3">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">安全发现 <span className="text-sm font-normal text-slate-500 ml-2">{findings.length} </span></h2>
          <div className="flex items-center gap-2">
            {["all", "critical", "high", "medium", "low"].map((s) => (
              <button key={s} onClick={() => setStatusFilter(s)}
                className={`px-2.5 h-7 rounded-lg text-[10px] font-medium transition-colors ${statusFilter === s ? "bg-indigo-600 text-white" : "bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-600"}`}>
                {s === "all" ? "全部" : s.charAt(0).toUpperCase() + s.slice(1)}
              </button>
            ))}
          </div>
        </div>
        <div className="divide-y divide-slate-100 dark:divide-slate-700 max-h-[400px] overflow-y-auto">
          {findings.length === 0 ? (
            <div className="py-12 text-center text-slate-400">
              <i className="fa-solid fa-shield-check text-3xl mb-2"></i>
              <p className="text-sm">暂无安全发现</p>
              <p className="text-xs mt-1 text-slate-500">执行权限检查以识别潜在漏洞</p>
            </div>
          ) : findings.filter((f) => statusFilter === "all" || f.severity === statusFilter).map((f, i) => {
            const fid = f.id || String(i);
            const isExpanded = expandedFinding === fid;
            return (
              <div key={fid} className="p-4 hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-3 flex-1">
                    <i className={`fa-solid ${severityIcon(f.severity || "low")} mt-1`}></i>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-medium text-slate-900 dark:text-slate-100">{f.title || "-"}</span>
                        <span className={`px-2 py-0.5 text-[10px] rounded-full border font-medium ${severityBadge(f.severity || "low")}`}>
                          {(f.severity || "unknown").toUpperCase()}
                        </span>
                        {f.cve_id && <span className="px-2 py-0.5 text-[10px] rounded-md bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 font-mono">{f.cve_id}</span>}
                      </div>
                      {isExpanded && (
                        <div className="mt-3 space-y-2">
                          {f.description && <p className="text-sm text-slate-600 dark:text-slate-400">{f.description}</p>}
                          {f.recommendation && (
                            <p className="text-sm text-indigo-600 dark:text-indigo-400">
                              <i className="fa-solid fa-lightbulb mr-1"></i>建议: {f.recommendation}
                            </p>
                          )}
                          {f.exploit_command && (
                            <div className="flex items-center gap-2">
                              <code className="text-xs font-mono bg-slate-900 dark:bg-black text-emerald-400 px-3 py-1.5 rounded-lg flex-1 overflow-x-auto">{f.exploit_command}</code>
                              <button onClick={() => handleExecuteExploit(f)} className="px-3 h-8 bg-red-600 hover:bg-red-700 text-white rounded-lg text-xs font-medium flex items-center gap-1.5 transition-colors shrink-0">
                                <i className="fa-solid fa-bolt"></i> 执行
                              </button>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                  <button onClick={() => setExpandedFinding(isExpanded ? null : fid)} className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors">
                    <i className={`fa-solid ${isExpanded ? "fa-chevron-up" : "fa-chevron-down"}`}></i>
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div className="ui-card overflow-hidden">
        <div className="px-6 py-4 border-b border-[var(--border)]">
          <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">历史记录 <span className="text-sm font-normal text-slate-500 ml-2">{history.length} </span></h2>
        </div>
        {history.length === 0 ? (
          <div className="py-12 text-center text-slate-400">
            <i className="fa-solid fa-clock-rotate-left text-3xl mb-2"></i>
            <p className="text-sm">暂无权限提升记录</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 dark:bg-slate-700/50 border-b border-[var(--border)]">
                <tr className="text-xs text-slate-500 dark:text-slate-400">
                  <th className="text-left py-3 px-4 font-medium">时间</th>
                  <th className="text-left py-3 px-4 font-medium">Agent</th>
                  <th className="text-left py-3 px-4 font-medium">检查类型</th>
                  <th className="text-left py-3 px-4 font-medium">状态</th>
                  <th className="text-left py-3 px-4 font-medium">结果</th>
                  <th className="text-left py-3 px-4 font-medium">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                {history.map((item, i) => {
                  const hid = item.id || item.ID || String(i);
                  return (
                    <tr key={hid} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                      <td className="py-3 px-4 text-xs font-mono text-slate-500">{formatTime(item.created_at || item.CreatedAt || "")}</td>
                      <td className="py-3 px-4 text-xs font-mono text-slate-600 dark:text-slate-300">{(item.agent_id || item.AgentID || "").substring(0, 8)}</td>
                      <td className="py-3 px-4 text-xs text-[var(--text-secondary)]">{item.check_type || item.CheckType || "-"}</td>
                      <td className="py-3 px-4">
                        <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${(item.status || item.Status) === "completed" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : (item.status || item.Status) === "running" ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400" : "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-400"}`}>
                          {item.status || item.Status || "-"}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-xs text-indigo-600 dark:text-indigo-400 font-medium">{item.findings_count ?? item.FindingsCount ?? 0}</td>
                      <td className="py-3 px-4">
                        <div className="flex items-center gap-1">
                          <button onClick={() => handleViewHistory(hid)} className="p-1.5 text-slate-400 hover:text-indigo-600 dark:hover:text-indigo-400 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors" title="查看结果">
                            <i className="fa-solid fa-eye text-xs"></i>
                          </button>
                          {(item.status || item.Status) === "completed" && (
                            <button onClick={() => handleProcessResult(hid)} className="p-1.5 text-slate-400 hover:text-emerald-600 dark:hover:text-emerald-400 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors" title="处理结果">
                              <i className="fa-solid fa-cog text-xs"></i>
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {toastMsg && (
        <div className="fixed bottom-6 right-6 z-50 bg-slate-900 dark:bg-slate-700 text-white text-sm px-5 py-3 rounded-xl shadow-xl max-w-xs">
          {toastMsg}
        </div>
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Execute" cancelText="Cancel" onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
