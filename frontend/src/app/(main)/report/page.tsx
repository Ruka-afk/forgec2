"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";

interface ReportStats {
  total_agents?: number;
  TotalAgents?: number;
  online_agents?: number;
  OnlineAgents?: number;
  total_tasks?: number;
  TotalTasks?: number;
  success_tasks?: number;
  SuccessTasks?: number;
  failed_tasks?: number;
  FailedTasks?: number;
  total_creds?: number;
  TotalCreds?: number;
  total_audits?: number;
  TotalAudits?: number;
  total_listeners?: number;
  TotalListeners?: number;
  total_findings?: number;
  TotalFindings?: number;
  critical_findings?: number;
  CriticalFindings?: number;
  high_findings?: number;
  HighFindings?: number;
  medium_findings?: number;
  MediumFindings?: number;
}

interface AgentRow {
  id?: string;
  hostname?: string;
  ip?: string;
  os?: string;
  last_seen?: string;
  status?: string;
}

interface TaskStatRow {
  type?: string;
  total?: number;
  success?: number;
  failed?: number;
  success_rate?: number;
}

interface CredRow {
  type?: string;
  count?: number;
  source?: string;
}

interface ListenerRow {
  id?: string;
  name?: string;
  protocol?: string;
  status?: string;
  agent_count?: number;
  traffic?: string;
}

interface FindingRow {
  id?: string;
  title?: string;
  severity?: string;
  cve_id?: string;
  description?: string;
  recommendation?: string;
}

interface ReportHistoryRow {
  id?: string;
  template?: string;
  format?: string;
  created_at?: string;
  sections?: string[];
  size?: string;
}

const SECTIONS = [
  { key: "overview", label: "概览", icon: "fa-chart-pie" },
  { key: "agents", label: "Implant", icon: "fa-robot" },
  { key: "tasks", label: "任务", icon: "fa-list-check" },
  { key: "credentials", label: "凭据", icon: "fa-key" },
  { key: "network", label: "网络", icon: "fa-network-wired" },
  { key: "recommendations", label: "建议", icon: "fa-lightbulb" },
];

const TEMPLATES = [
  { value: "full", label: "完整报告", desc: "包含所有章节的详细报告" },
  { value: "executive", label: "执行摘要", desc: "高层级摘要和关键发现" },
  { value: "technical", label: "技术专版", desc: "深入详细的技术分析" },
];

const DATE_PRESETS = [
  { value: "7d", label: "最近 7 天" },
  { value: "30d", label: "最近 30 天" },
  { value: "90d", label: "最近 90 天" },
  { value: "custom", label: "自定义" },
];

export default function ReportPage() {
  const [activeSection, setActiveSection] = useState("overview");
  const [stats, setStats] = useState<ReportStats>({});
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [datePreset, setDatePreset] = useState("30d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [template, setTemplate] = useState("full");
  const [agents, setAgents] = useState<AgentRow[]>([]);
  const [taskStats, setTaskStats] = useState<TaskStatRow[]>([]);
  const [creds, setCreds] = useState<CredRow[]>([]);
  const [listeners, setListeners] = useState<ListenerRow[]>([]);
  const [findings, setFindings] = useState<FindingRow[]>([]);
  const [history, setHistory] = useState<ReportHistoryRow[]>([]);
  const [expandedFinding, setExpandedFinding] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const computeDateRange = useCallback(() => {
    if (datePreset === "custom") {
      return { start: customStart, end: customEnd };
    }
    const days = parseInt(datePreset);
    const end = new Date();
    const start = new Date();
    start.setDate(start.getDate() - days);
    return {
      start: start.toISOString().split("T")[0],
      end: end.toISOString().split("T")[0],
    };
  }, [datePreset, customStart, customEnd]);

  const loadOverview = useCallback(async () => {
    try {
      const resp = await fetch(`${API_BASE}?p=/report?format=json`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      setStats(data);
    } catch {
    }
  }, []);

  const loadPreview = useCallback(async () => {
    try {
      const { start, end } = computeDateRange();
      const params = new URLSearchParams({ format: "json" });
      if (start) params.set("start", start);
      if (end) params.set("end", end);

      const agentsRes = await fetch(`${API_BASE}?p=/api/report/agents&${params}`);
      if (!agentsRes.ok) throw new Error(`HTTP ${agentsRes.status}`);
      const agentsData = await agentsRes.json();
      setAgents(agentsData.agents || agentsData.Agents || []);

      const tasksRes = await fetch(`${API_BASE}?p=/api/report/tasks&${params}`);
      if (!tasksRes.ok) throw new Error(`HTTP ${tasksRes.status}`);
      const tasksData = await tasksRes.json();
      setTaskStats(tasksData.stats || tasksData.Stats || []);

      const credsRes = await fetch(`${API_BASE}?p=/api/report/credentials&${params}`);
      if (!credsRes.ok) throw new Error(`HTTP ${credsRes.status}`);
      const credsData = await credsRes.json();
      setCreds(credsData.credentials || credsData.Credentials || []);

      const netRes = await fetch(`${API_BASE}?p=/api/report/network&${params}`);
      if (!netRes.ok) throw new Error(`HTTP ${netRes.status}`);
      const netData = await netRes.json();
      setListeners(netData.listeners || netData.Listeners || []);

      const findRes = await fetch(`${API_BASE}?p=/api/report/findings&${params}`);
      if (!findRes.ok) throw new Error(`HTTP ${findRes.status}`);
      const findData = await findRes.json();
      setFindings(findData.findings || findData.Findings || []);
    } catch {
    }
  }, [computeDateRange]);

  const loadHistory = useCallback(async () => {
    try {
      const resp = await fetch(`${API_BASE}?p=/api/report/history?format=json`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      setHistory(data.reports || data.Reports || []);
    } catch {
    }
  }, []);

  const loadAll = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadOverview(), loadPreview(), loadHistory()]);
    setLoading(false);
  }, [loadOverview, loadPreview, loadHistory]);

  useEffect(() => {
    Promise.resolve().then(() => {
      loadAll();
      intervalRef.current = setInterval(loadOverview, 30000);
    });
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [loadAll, loadOverview]);

  useEffect(() => {
    Promise.resolve().then(() => {
      if (!loading) loadPreview();
    });
  }, [datePreset, customStart, customEnd, loading, loadPreview]);

  const handleGenerate = async () => {
    setGenerating(true);
    try {
      const { start, end } = computeDateRange();
      let sections: string[];
      if (template === "technical") {
        sections = ["agents", "tasks", "credentials", "network", "recommendations"];
      } else if (template === "executive") {
        sections = ["overview", "recommendations"];
      } else {
        sections = SECTIONS.map((s) => s.key);
      }
      const res = await fetch(`${API_BASE}?p=/api/report/generate&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ start_date: start, end_date: end, template, sections, format: "html" }),
      });
      if (res.ok) {
        await loadHistory();
      } else {
      }
    } catch {
    } finally {
      setGenerating(false);
    }
  };

  const handleExportPDF = () => {
    const { start, end } = computeDateRange();
    const params = new URLSearchParams({ format: "json" });
    if (start) params.set("start", start);
    if (end) params.set("end", end);
    params.set("template", template);
    window.open(`${API_BASE}?p=/api/report/export/pdf&${params}`, "_blank");
  };

  const handleDeleteReport = async (id: string) => {
    try {
      await fetch(`${API_BASE}?p=/api/report/${id}&format=json`, { method: "DELETE" });
      loadHistory();
    } catch {
    }
  };

  const severityColor = (s: string) => {
    if (s === "critical") return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20";
    if (s === "high") return "bg-orange-500/10 text-orange-600 dark:text-orange-400 border-orange-500/20";
    if (s === "medium") return "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400 border-yellow-500/20";
    if (s === "low") return "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20";
    return "bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/20";
  };

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto mb-20 md:mb-0">
        <div className="flex items-center justify-center h-64">
          <i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">
            <i className="fa-solid fa-file-lines text-indigo-500 mr-2"></i>报告生成器          </h1>
          <p className="text-xs sm:text-sm text-slate-500 dark:text-slate-400 mt-1">生成操作审计报告，查看数据的趋势和历史记录</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={handleExportPDF} className="px-4 h-10 bg-red-600 hover:bg-red-700 text-white rounded-xl text-sm font-medium flex items-center gap-x-2 transition-colors">
            <i className="fa-solid fa-file-pdf"></i>导出 PDF
          </button>
          <button onClick={handleGenerate} disabled={generating} className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white rounded-xl text-sm font-medium flex items-center gap-x-2 transition-colors">
            {generating ? <i className="fa-solid fa-spinner fa-spin"></i> : <i className="fa-solid fa-wand-magic-sparkles"></i>}
            {generating ? "生成中..." : "生成报告"}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <StatCard icon="fa-robot" color="indigo" label="Implant 总数" value={stats.TotalAgents || stats.total_agents || 0} sub={`${stats.OnlineAgents || stats.online_agents || 0} 在线`} subColor="text-emerald-600" />
        <StatCard icon="fa-list-check" color="emerald" label="任务执行" value={stats.TotalTasks || stats.total_tasks || 0} sub={`${stats.SuccessTasks || stats.success_tasks || 0} 成功 / ${stats.FailedTasks || stats.failed_tasks || 0} 失败`} subColor="text-slate-500" />
        <StatCard icon="fa-key" color="amber" label="凭据获取" value={stats.TotalCreds || stats.total_creds || 0} sub="已收集" subColor="text-slate-500" />
        <StatCard icon="fa-bug" color="red" label="安全发现" value={stats.TotalFindings || stats.total_findings || 0} sub={`严重: ${stats.CriticalFindings || stats.critical_findings || 0} | 高 ${stats.HighFindings || stats.high_findings || 0}`} subColor="text-red-500" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6 mb-6">
        <div className="lg:col-span-1">
          <div className="ui-card p-2">
            {SECTIONS.map((s) => (
              <button key={s.key} onClick={() => setActiveSection(s.key)}
                className={`w-full flex items-center gap-3 px-4 py-3 rounded-xl text-left text-sm font-medium transition-colors ${activeSection === s.key ? "bg-indigo-50 dark:bg-indigo-900/20 text-indigo-700 dark:text-indigo-300" : "text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-700/50"}`}>
                <i className={`fa-solid ${s.icon} w-5 text-center`}></i>
                {s.label}
              </button>
            ))}
          </div>
        </div>

        <div className="lg:col-span-3">
          {activeSection === "overview" && (
            <div className="ui-card p-6 shadow-sm">
              <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100 mb-6">报告设置</h2>
              <div className="space-y-6">
                <div>
                  <label className="block text-sm font-semibold text-slate-900 dark:text-slate-100 mb-3">报告模板</label>
                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    {TEMPLATES.map((t) => (
                      <label key={t.value} className={`flex flex-col items-start p-4 rounded-xl cursor-pointer transition-colors ${template === t.value ? "border-2 border-indigo-500 bg-indigo-50 dark:bg-indigo-900/20" : "border border-[var(--border)] hover:bg-slate-50 dark:hover:bg-slate-700/50"}`}>
                        <input type="radio" name="template" value={t.value} checked={template === t.value} onChange={() => setTemplate(t.value)} className="w-4 h-4 accent-indigo-600 mb-2" />
                        <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{t.label}</div>
                        <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">{t.desc}</div>
                      </label>
                    ))}
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900 dark:text-slate-100 mb-3">日期范围</label>
                  <div className="flex flex-wrap gap-2 mb-3">
                    {DATE_PRESETS.map((p) => (
                      <button key={p.value} onClick={() => setDatePreset(p.value)}
                        className={`px-3 h-9 rounded-xl text-xs font-medium transition-colors ${datePreset === p.value ? "bg-indigo-600 text-white" : "bg-slate-100 dark:bg-slate-700 text-[var(--text-secondary)] hover:bg-slate-200 dark:hover:bg-slate-600"}`}>
                        {p.label}
                      </button>
                    ))}
                  </div>
                  {datePreset === "custom" && (
                    <div className="grid grid-cols-2 gap-3">
                      <input type="date" className="ui-input" value={customStart} onChange={(e) => setCustomStart(e.target.value)} placeholder="开始日期" />
                      <input type="date" className="ui-input" value={customEnd} onChange={(e) => setCustomEnd(e.target.value)} placeholder="结束日期" />
                    </div>
                  )}
                </div>

                <div className="border-t border-[var(--border)] pt-4">
                  <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-3">报告历史</h3>
                  {history.length === 0 ? (
                    <div className="text-center py-6 text-slate-400">
                      <i className="fa-solid fa-inbox text-2xl mb-2 block"></i>
                      <p className="text-sm">暂无历史报告</p>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {history.map((r, i) => {
                        const id = r.id || String(i);
                        return (
                          <div key={id} className="flex items-center justify-between p-3 bg-slate-50 dark:bg-[var(--card-bg)] rounded-xl hover:bg-slate-100 dark:hover:bg-slate-600/50 transition-colors">
                            <div className="flex items-center gap-3">
                              <i className="fa-solid fa-file-lines text-indigo-500"></i>
                              <div>
                                <div className="text-sm font-medium text-[var(--text-secondary)]">{r.template || "未知模板"} - {r.format?.toUpperCase() || "HTML"}</div>
                                <div className="text-xs text-slate-500">{r.created_at || "-"} {r.size ? `· ${r.size}` : ""}</div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <a href={`${API_BASE}?p=/report/${id}/download&format=html`} download className="p-2 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 rounded-lg transition-colors">
                                <i className="fa-solid fa-download"></i>
                              </a>
                              <button onClick={() => handleDeleteReport(id)} className="p-2 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/30 rounded-lg transition-colors">
                                <i className="fa-solid fa-trash"></i>
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {activeSection === "agents" && (
            <div className="ui-card overflow-hidden">
              <div className="px-6 py-4 border-b border-[var(--border)]">
                <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Implant 详情 <span className="text-sm font-normal text-slate-500 ml-2">共{agents.length} 台</span></h2>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="bg-slate-50 dark:bg-[var(--card-bg)] border-b border-[var(--border)]">
                    <tr className="text-xs text-slate-500 dark:text-slate-400">
                      <th className="text-left py-3 px-4 font-medium">主机名</th>
                      <th className="text-left py-3 px-4 font-medium">IP 地址</th>
                      <th className="text-left py-3 px-4 font-medium">操作系统</th>
                      <th className="text-left py-3 px-4 font-medium">最后在线</th>
                      <th className="text-left py-3 px-4 font-medium">状态</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                    {agents.length === 0 ? (
                      <tr><td colSpan={5} className="py-12 text-center text-slate-400"><i className="fa-solid fa-inbox text-3xl mb-2 block"></i>暂无数据</td></tr>
                    ) : agents.map((a, i) => (
                      <tr key={a.id || i} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                        <td className="py-3 px-4 text-slate-900 dark:text-slate-100 font-medium">{a.hostname || "-"}</td>
                        <td className="py-3 px-4 font-mono text-xs text-slate-600 dark:text-slate-300">{a.ip || "-"}</td>
                        <td className="py-3 px-4 text-slate-600 dark:text-slate-300">{a.os || "-"}</td>
                        <td className="py-3 px-4 text-xs text-slate-500">{a.last_seen || "-"}</td>
                        <td className="py-3 px-4">
                          <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${a.status === "online" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : "bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400"}`}>
                            <span className={`w-1.5 h-1.5 rounded-full ${a.status === "online" ? "bg-emerald-500" : "bg-slate-400"}`}></span>
                            {a.status || "unknown"}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {activeSection === "tasks" && (
            <div className="ui-card overflow-hidden">
              <div className="px-6 py-4 border-b border-[var(--border)]">
                <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">任务统计 <span className="text-sm font-normal text-slate-500 ml-2">按类型汇总</span></h2>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="bg-slate-50 dark:bg-[var(--card-bg)] border-b border-[var(--border)]">
                    <tr className="text-xs text-slate-500 dark:text-slate-400">
                      <th className="text-left py-3 px-4 font-medium">任务类型</th>
                      <th className="text-left py-3 px-4 font-medium">总数</th>
                      <th className="text-left py-3 px-4 font-medium">成功</th>
                      <th className="text-left py-3 px-4 font-medium">失败</th>
                      <th className="text-left py-3 px-4 font-medium">成功率</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                    {taskStats.length === 0 ? (
                      <tr><td colSpan={5} className="py-12 text-center text-slate-400"><i className="fa-solid fa-inbox text-3xl mb-2 block"></i>暂无数据</td></tr>
                    ) : taskStats.map((t, i) => (
                      <tr key={i} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                        <td className="py-3 px-4 text-slate-900 dark:text-slate-100 font-medium">{t.type || "-"}</td>
                        <td className="py-3 px-4 text-slate-600 dark:text-slate-300">{t.total ?? 0}</td>
                        <td className="py-3 px-4 text-emerald-600 dark:text-emerald-400">{t.success ?? 0}</td>
                        <td className="py-3 px-4 text-red-600 dark:text-red-400">{t.failed ?? 0}</td>
                        <td className="py-3 px-4">
                          <div className="flex items-center gap-2">
                            <div className="flex-1 h-2 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden max-w-[100px]">
                              <div className="h-full bg-indigo-500 rounded-full" style={{ width: `${t.success_rate ?? Math.round(((t.success ?? 0) / (t.total || 1)) * 100)}%` }}></div>
                            </div>
                            <span className="text-xs text-slate-600 dark:text-slate-300 tabular-nums">{t.success_rate ?? Math.round(((t.success ?? 0) / (t.total || 1)) * 100)}%</span>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {activeSection === "credentials" && (
            <div className="ui-card overflow-hidden">
              <div className="px-6 py-4 border-b border-[var(--border)]">
                <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">凭据汇总<span className="text-sm font-normal text-slate-500 ml-2">共{creds.reduce((s, c) => s + (c.count ?? 0), 0)} 条</span></h2>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="bg-slate-50 dark:bg-[var(--card-bg)] border-b border-[var(--border)]">
                    <tr className="text-xs text-slate-500 dark:text-slate-400">
                      <th className="text-left py-3 px-4 font-medium">凭据类型</th>
                      <th className="text-left py-3 px-4 font-medium">数量</th>
                      <th className="text-left py-3 px-4 font-medium">来源</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                    {creds.length === 0 ? (
                      <tr><td colSpan={3} className="py-12 text-center text-slate-400"><i className="fa-solid fa-inbox text-3xl mb-2 block"></i>暂无数据</td></tr>
                    ) : creds.map((c, i) => (
                      <tr key={i} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                        <td className="py-3 px-4 text-slate-900 dark:text-slate-100 font-medium">{c.type || "-"}</td>
                        <td className="py-3 px-4 text-indigo-600 dark:text-indigo-400 font-semibold">{c.count ?? 0}</td>
                        <td className="py-3 px-4 text-slate-500 dark:text-slate-400">{c.source || "-"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {activeSection === "network" && (
            <div className="ui-card overflow-hidden">
              <div className="px-6 py-4 border-b border-[var(--border)]">
                <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">网络情况 <span className="text-sm font-normal text-slate-500 ml-2">监听器状态概览</span></h2>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="bg-slate-50 dark:bg-[var(--card-bg)] border-b border-[var(--border)]">
                    <tr className="text-xs text-slate-500 dark:text-slate-400">
                      <th className="text-left py-3 px-4 font-medium">监听器</th>
                      <th className="text-left py-3 px-4 font-medium">协议</th>
                      <th className="text-left py-3 px-4 font-medium">状态</th>
                      <th className="text-left py-3 px-4 font-medium">Implant 数量</th>
                      <th className="text-left py-3 px-4 font-medium">流量</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                    {listeners.length === 0 ? (
                      <tr><td colSpan={5} className="py-12 text-center text-slate-400"><i className="fa-solid fa-inbox text-3xl mb-2 block"></i>暂无数据</td></tr>
                    ) : listeners.map((l, i) => (
                      <tr key={l.id || i} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                        <td className="py-3 px-4 text-slate-900 dark:text-slate-100 font-medium">{l.name || "-"}</td>
                        <td className="py-3 px-4"><span className="px-2 py-0.5 text-xs rounded-md bg-slate-100 dark:bg-slate-700 text-[var(--text-secondary)]">{l.protocol || "-"}</span></td>
                        <td className="py-3 px-4">
                          <span className={`inline-flex items-center gap-1 text-xs font-medium ${l.status === "active" ? "text-emerald-600 dark:text-emerald-400" : "text-slate-500"}`}>
                            <span className={`w-1.5 h-1.5 rounded-full ${l.status === "active" ? "bg-emerald-500" : "bg-slate-400"}`}></span>
                            {l.status || "-"}
                          </span>
                        </td>
                        <td className="py-3 px-4 text-slate-600 dark:text-slate-300">{l.agent_count ?? 0}</td>
                        <td className="py-3 px-4 text-slate-600 dark:text-slate-300">{l.traffic || "-"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {activeSection === "recommendations" && (
            <div className="ui-card overflow-hidden">
              <div className="px-6 py-4 border-b border-[var(--border)]">
                <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">安全发现与建议<span className="text-sm font-normal text-slate-500 ml-2">共{findings.length} 条</span></h2>
              </div>
              <div className="divide-y divide-slate-100 dark:divide-slate-700">
                {findings.length === 0 ? (
                  <div className="py-12 text-center text-slate-400">
                    <i className="fa-solid fa-shield-check text-3xl mb-2 block"></i>
                    <p className="text-sm">暂无安全发现</p>
                  </div>
                ) : findings.map((f, i) => {
                  const id = f.id || String(i);
                  return (
                    <div key={id} className="p-4 hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                      <div className="flex items-start justify-between gap-3">
                        <div className="flex items-start gap-3 flex-1">
                          <div className="mt-0.5">
                            <i className={`fa-solid ${f.severity === "critical" ? "fa-circle-exclamation text-red-500" : f.severity === "high" ? "fa-triangle-exclamation text-orange-500" : f.severity === "medium" ? "fa-circle-exclamation text-yellow-500" : "fa-circle-info text-blue-500"}`}></i>
                          </div>
                          <div className="flex-1">
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className="text-sm font-medium text-slate-900 dark:text-slate-100">{f.title || "-"}</span>
                              <span className={`px-2 py-0.5 text-[10px] rounded-full border font-medium ${severityColor(f.severity || "low")}`}>
                                {(f.severity || "unknown").toUpperCase()}
                              </span>
                              {f.cve_id && <span className="px-2 py-0.5 text-[10px] rounded-md bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 font-mono">{f.cve_id}</span>}
                            </div>
                            {expandedFinding === id && (
                              <div className="mt-3 space-y-2">
                                <p className="text-sm text-slate-600 dark:text-slate-400">{f.description || ""}</p>
                                {f.recommendation && (
                                  <p className="text-sm text-indigo-600 dark:text-indigo-400">
                                    <i className="fa-solid fa-lightbulb mr-1"></i>建议: {f.recommendation}
                                  </p>
                                )}
                              </div>
                            )}
                          </div>
                        </div>
                        <button onClick={() => setExpandedFinding(expandedFinding === id ? null : id)} className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors">
                          <i className={`fa-solid ${expandedFinding === id ? "fa-chevron-up" : "fa-chevron-down"}`}></i>
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function StatCard({ icon, color, label, value, sub, subColor }: {
  icon: string; color: string; label: string; value: number; sub: string; subColor: string;
}) {
  const colorMap: Record<string, string> = {
    indigo: "bg-indigo-50 dark:bg-indigo-900/20",
    emerald: "bg-emerald-50 dark:bg-emerald-900/20",
    amber: "bg-amber-50 dark:bg-amber-900/20",
    red: "bg-red-50 dark:bg-red-900/20",
    blue: "bg-blue-50 dark:bg-blue-900/20",
  };
  const textColor: Record<string, string> = {
    indigo: "text-indigo-600 dark:text-indigo-400",
    emerald: "text-emerald-600 dark:text-emerald-400",
    amber: "text-amber-600 dark:text-amber-400",
    red: "text-red-600 dark:text-red-400",
    blue: "text-blue-600 dark:text-blue-400",
  };
  const iconColor: Record<string, string> = {
    indigo: "text-indigo-400",
    emerald: "text-emerald-400",
    amber: "text-amber-400",
    red: "text-red-400",
    blue: "text-blue-400",
  };
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">{label}</div>
          <div className={`text-3xl font-bold mt-2 ${textColor[color] || "text-slate-900 dark:text-slate-100"}`}>{value}</div>
          <div className={`text-xs mt-1 font-medium ${subColor}`}>{sub}</div>
        </div>
        <div className={`w-12 h-12 ${colorMap[color] || "bg-slate-50"} rounded-xl flex items-center justify-center`}>
          <i className={`fa-solid ${icon} text-xl ${iconColor[color] || "text-slate-400"}`}></i>
        </div>
      </div>
    </div>
  );
}
