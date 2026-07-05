"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";

interface ScanAgent {
  id?: string;
  ID?: string;
  hostname?: string;
  Hostname?: string;
  ip?: string;
  IP?: string;
}

interface ScanResult {
  IP?: string;
  ip?: string;
  Port?: number;
  port?: number;
  Protocol?: string;
  protocol?: string;
  Status?: string;
  status?: string;
  Service?: string;
  service?: string;
  Version?: string;
  version?: string;
  Banner?: string;
  banner?: string;
}

interface ActiveScan {
  ID?: string;
  id?: string;
  Agent?: string;
  agent?: string;
  Target?: string;
  target?: string;
  Type?: string;
  type?: string;
  Progress?: number;
  progress?: number;
  Status?: string;
  status?: string;
  StartedAt?: string;
  started_at?: string;
}

interface ScanHistory {
  ID?: string;
  id?: string;
  Target?: string;
  target?: string;
  Type?: string;
  type?: string;
  Ports?: number;
  ports?: number;
  Results?: number;
  results?: number;
  Status?: string;
  status?: string;
  CreatedAt?: string;
  created_at?: string;
}

interface ScannerData {
  agents?: ScanAgent[];
  results?: ScanResult[];
  active_scans?: ActiveScan[];
  history?: ScanHistory[];
}

export default function ScannerPage() {
  const [data, setData] = useState<ScannerData | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [targetAddr, setTargetAddr] = useState("");
  const [scanType, setScanType] = useState("tcp_connect");
  const [portMode, setPortMode] = useState("top");
  const [customPorts, setCustomPorts] = useState("");
  const [showCustomRange, setShowCustomRange] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [activeTab, setActiveTab] = useState<"results" | "active" | "history">("results");

  const loadData = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/scanner&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const result = await res.json();
      setData({
        agents: result.Agents || result.agents || [],
        results: result.Results || result.results || [],
        active_scans: result.ActiveScans || result.active_scans || [],
        history: result.History || result.history || [],
      });
    } catch {
      setData({ agents: [], results: [], active_scans: [], history: [] });
    }
    setLoading(false);
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadData()); }, [loadData]);

  useEffect(() => {
    Promise.resolve().then(() => setShowCustomRange(portMode === "custom"));
  }, [portMode]);

  useEffect(() => {
    Promise.resolve().then(() => {
      if (data?.active_scans && data.active_scans.some(s => (s.Status || s.status) === "running")) {
        const timer = setInterval(loadData, 3000);
        return () => clearInterval(timer);
      }
    });
  }, [data?.active_scans, loadData]);

  const handleStartScan = async () => {
    if (!selectedAgent || !targetAddr) return;
    setScanning(true);
    try {
      const body = new URLSearchParams();
      body.append("agent_id", selectedAgent);
      body.append("target", targetAddr);
      body.append("scan_type", scanType);
      if (portMode === "custom" && customPorts) {
        body.append("port_range", customPorts);
      } else {
        body.append("top_ports", "1000");
      }
      await fetch(`${API_BASE}?p=/api/scan&format=json`, {
        method: "POST",
        body,
      });
      setActiveTab("active");
      loadData();
    } catch (e) { console.error("Scanner: start scan failed", e); }
    setScanning(false);
  };

  const handleExport = (format: "csv" | "json") => {
    const results = data?.results || [];
    if (results.length === 0) return;
    let content: string;
    let filename: string;
    let mimeType: string;
    if (format === "csv") {
      const header = "IP,Port,Protocol,State,Service,Version,Banner";
      const rows = results.map(r => {
        const ip = r.IP ?? r.ip ?? "";
        const port = r.Port ?? r.port ?? "";
        const proto = r.Protocol ?? r.protocol ?? "";
        const state = r.Status ?? r.status ?? "";
        const svc = r.Service ?? r.service ?? "";
        const ver = r.Version ?? r.version ?? "";
        const banner = (r.Banner ?? r.banner ?? "").replace(/,/g, " ");
        return `${ip},${port},${proto},${state},${svc},${ver},${banner}`;
      });
      content = [header, ...rows].join("\n");
      filename = "scan_results.csv";
      mimeType = "text/csv";
    } else {
      content = JSON.stringify(results, null, 2);
      filename = "scan_results.json";
      mimeType = "application/json";
    }
    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  };

  const getStatusColor = (status: string) => {
    const s = status?.toLowerCase() ?? "";
    if (s === "open") return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400";
    if (s === "closed") return "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400";
    if (s === "filtered") return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400";
    return "bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-300";
  };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="mb-6">
        <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">网络扫描器</h1>
        <p className="text-slate-500 dark:text-slate-400 text-xs sm:text-sm mt-1">通过Agent进行网络扫描、端口识别、漏洞探测</p>
      </div>

      <div className="ui-card p-6 mb-6 shadow-sm hover:shadow-md transition-shadow">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-blue-100 dark:bg-blue-900/30 rounded-xl flex items-center justify-center">
            <i className="fa-solid fa-radar text-blue-600 dark:text-blue-400"></i>
          </div>
          <div>
            <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">新建扫描任务</div>
            <div className="text-xs text-slate-500 dark:text-slate-400">选择Agent创建扫描任务</div>
          </div>
        </div>

        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">目标 Agent</label>
              <select value={selectedAgent} onChange={e => setSelectedAgent(e.target.value)}
                className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                <option value="">-- 选择 Agent --</option>
                {data?.agents?.map(a => {
                  const id = a.id || a.ID || "";
                  const hostname = a.hostname || a.Hostname || "";
                  const ip = a.ip || a.IP || "";
                  return <option key={id} value={id}>{hostname} ({ip})</option>;
                })}
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">目标地址</label>
              <input type="text" value={targetAddr} onChange={e => setTargetAddr(e.target.value)} placeholder="192.168.1.1 或 10.0.0.0/24"
                className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 font-mono focus:outline-none focus:border-indigo-500 dark:text-slate-100" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">扫描类型</label>
              <select value={scanType} onChange={e => setScanType(e.target.value)}
                className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                <option value="tcp_connect">TCP Connect</option>
                <option value="tcp_syn">TCP SYN (Stealth)</option>
                <option value="udp">UDP</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">端口范围</label>
              <select value={portMode} onChange={e => setPortMode(e.target.value)}
                className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                <option value="top">Top 1000 端口</option>
                <option value="top100">Top 100 端口</option>
                <option value="custom">自定义范围</option>
              </select>
            </div>
          </div>

          {showCustomRange && (
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">端口范围</label>
              <input type="text" value={customPorts} onChange={e => setCustomPorts(e.target.value)} placeholder="1-1000,8080,8443"
                className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 font-mono focus:outline-none focus:border-indigo-500 dark:text-slate-100" />
              <p className="text-xs text-slate-500 mt-1">支持格式: 1-1000,8080,8443</p>
            </div>
          )}

          <div className="flex items-center gap-3 pt-2">
            <button onClick={handleStartScan} disabled={scanning || !selectedAgent || !targetAddr}
              className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-xl text-sm font-medium transition-colors">
              <i className={`fa-solid ${scanning ? "fa-circle-notch fa-spin" : "fa-play"} mr-1`}></i>
              <span>{scanning ? "扫描中..." : "开始扫描"}</span>
            </button>
            <span className="text-xs text-slate-500 dark:text-slate-400">
              <i className="fa-solid fa-circle-info mr-1"></i>
              扫描将在后台执行，完成后可查看结果            </span>
          </div>
        </div>
      </div>

      {(data?.active_scans?.length ?? 0) > 0 && (
        <div className="ui-card p-6 mb-6 shadow-sm hover:shadow-md transition-shadow">
          <div className="flex items-center gap-x-3 mb-4">
            <div className="w-10 h-10 bg-orange-100 dark:bg-orange-900/30 rounded-xl flex items-center justify-center">
              <i className="fa-solid fa-spinner fa-spin text-orange-600 dark:text-orange-400"></i>
            </div>
            <div>
              <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">活跃扫描</div>
              <div className="text-xs text-slate-500 dark:text-slate-400">{data?.active_scans?.length || 0} 个扫描任务正在执行</div>
            </div>
          </div>
          <div className="space-y-3">
            {data?.active_scans?.map((scan, i) => {
              const scanId = scan.ID || scan.id || String(i);
              const target = scan.Target || scan.target || "";
              const progress = scan.Progress ?? scan.progress ?? 0;
              const status = scan.Status || scan.status || "running";
              const type = scan.Type || scan.type || "";
              const agent = scan.Agent || scan.agent || "";
              return (
                <div key={scanId} className="bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <i className="fa-solid fa-crosshairs text-slate-400 text-xs"></i>
                      <span className="text-sm font-medium text-slate-900 dark:text-slate-100 font-mono">{target}</span>
                      <span className="text-[10px] px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">{type}</span>
                    </div>
                    <span className="text-xs text-slate-500 dark:text-slate-400">{status} via {agent}</span>
                  </div>
                  <div className="w-full h-2 bg-slate-200 dark:bg-slate-600 rounded-full overflow-hidden">
                    <div className="h-full bg-indigo-500 rounded-full transition-all duration-500" style={{ width: `${progress}%` }}></div>
                  </div>
                  <div className="text-xs text-slate-500 dark:text-slate-400 mt-1 text-right">{progress}%</div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <div className="ui-card hover:shadow-md transition-shadow">
        <div className="flex items-center justify-between p-4 border-b border-[var(--border)]">
          <div className="flex gap-1">
            {(["results", "active", "history"] as const).map(tab => (
              <button key={tab} onClick={() => setActiveTab(tab)}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
                  activeTab === tab
                    ? "bg-indigo-50 text-indigo-600 dark:bg-indigo-900/20 dark:text-indigo-400"
                    : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
                }`}>
                {tab === "results" ? "扫描结果" : tab === "active" ? "活跃扫描" : "扫描历史"}
              </button>
            ))}
          </div>
          {activeTab === "results" && (
            <div className="flex items-center gap-2">
              <button onClick={() => handleExport("csv")}
                className="px-3 py-1.5 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-lg text-xs transition-colors">
                <i className="fa-solid fa-file-csv mr-1"></i>CSV
              </button>
              <button onClick={() => handleExport("json")}
                className="px-3 py-1.5 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-lg text-xs transition-colors">
                <i className="fa-solid fa-file-code mr-1"></i>JSON
              </button>
            </div>
          )}
        </div>

        <div className="p-4">
          {activeTab === "results" && (
            loading ? (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead><tr className="border-b border-[var(--border)]">
                    {["IP", "端口", "协议", "状态", "服务", "版本", "Banner"].map(h => (
                      <th key={h} className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">{h}</th>
                    ))}
                  </tr></thead>
                  <tbody>{[1,2,3].map(i => (
                    <tr key={i}>{[1,2,3,4,5,6,7].map(j => (
                      <td key={j} className="py-3 px-4"><div className="h-3 bg-slate-200 dark:bg-slate-700 rounded animate-pulse w-16"></div></td>
                    ))}</tr>
                  ))}</tbody>
                </table>
              </div>
            ) : data?.results && data.results.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-[var(--border)]">
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">IP</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">端口</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">协议</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">状态</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">服务</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">版本</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">Banner</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                    {data.results.map((r, i) => (
                      <tr key={i} className="hover:bg-slate-50 dark:hover:bg-slate-700/50">
                        <td className="py-3 px-4 font-mono text-[var(--text-secondary)]">{r.IP ?? r.ip ?? "-"}</td>
                        <td className="py-3 px-4 font-mono font-medium text-blue-600 dark:text-blue-400">{r.Port ?? r.port ?? "-"}</td>
                        <td className="py-3 px-4 text-slate-600 dark:text-slate-300">{r.Protocol ?? r.protocol ?? "-"}</td>
                        <td className="py-3 px-4"><span className={`text-[10px] px-2 py-0.5 rounded-full ${getStatusColor(r.Status ?? r.status ?? "open")}`}>{r.Status ?? r.status ?? "open"}</span></td>
                        <td className="py-3 px-4 text-[var(--text-secondary)]">{r.Service ?? r.service ?? "-"}</td>
                        <td className="py-3 px-4 text-slate-500 text-xs">{r.Version ?? r.version ?? "-"}</td>
                        <td className="py-3 px-4 text-xs text-slate-500 max-w-xs truncate">{r.Banner ?? r.banner ?? "-"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="text-center py-8 text-slate-400 dark:text-slate-500">
                <i className="fa-solid fa-inbox text-3xl mb-2 text-slate-300 dark:text-slate-600"></i>
                <p>暂无扫描结果</p>
              </div>
            )
          )}

          {activeTab === "active" && (
            data?.active_scans && data.active_scans.length > 0 ? (
              <div className="space-y-3">
                {data.active_scans.map((scan, i) => {
                  const scanId = scan.ID || scan.id || String(i);
                  const progress = scan.Progress ?? scan.progress ?? 0;
                  return (
                    <div key={scanId} className="bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4">
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-sm font-medium font-mono text-slate-900 dark:text-slate-100">{scan.Target || scan.target}</span>
                        <span className="text-xs text-slate-500">{progress}%</span>
                      </div>
                      <div className="w-full h-2 bg-slate-200 dark:bg-slate-600 rounded-full overflow-hidden">
                        <div className="h-full bg-indigo-500 rounded-full transition-all duration-500" style={{ width: `${progress}%` }}></div>
                      </div>
                      <div className="flex items-center justify-between mt-2">
                        <span className="text-[10px] px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">{scan.Type || scan.type}</span>
                        <span className="text-xs text-slate-500">{scan.StartedAt || scan.started_at || ""}</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="text-center py-8 text-slate-400 dark:text-slate-500">
                <i className="fa-solid fa-inbox text-3xl mb-2 text-slate-300 dark:text-slate-600"></i>
                <p>暂无活跃扫描任务</p>
              </div>
            )
          )}

          {activeTab === "history" && (
            data?.history && data.history.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-[var(--border)]">
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">目标</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">类型</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">端口数</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">结果</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">状态</th>
                      <th className="text-left py-3 px-4 font-semibold text-slate-600 dark:text-slate-300">时间</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                    {data.history.map((h, i) => (
                      <tr key={h.ID || h.id || i} className="hover:bg-slate-50 dark:hover:bg-slate-700/50">
                        <td className="py-3 px-4 font-mono text-[var(--text-secondary)]">{h.Target ?? h.target}</td>
                        <td className="py-3 px-4 text-slate-600 dark:text-slate-300">{h.Type ?? h.type}</td>
                        <td className="py-3 px-4 text-slate-600 dark:text-slate-300">{h.Ports ?? h.ports ?? 0}</td>
                        <td className="py-3 px-4 font-medium text-blue-600 dark:text-blue-400">{h.Results ?? h.results ?? 0}</td>
                        <td className="py-3 px-4">
                          <span className={`text-[10px] px-2 py-0.5 rounded-full ${
                            (h.Status ?? h.status ?? "") === "completed"
                              ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                              : "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
                          }`}>{h.Status ?? h.status}</span>
                        </td>
                        <td className="py-3 px-4 text-xs text-slate-500">{h.CreatedAt ?? h.created_at ?? "-"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="text-center py-8 text-slate-400 dark:text-slate-500">
                <i className="fa-solid fa-clock-rotate-left text-3xl mb-2 text-slate-300 dark:text-slate-600"></i>
                <p>暂无扫描历史</p>
              </div>
            )
          )}
        </div>
      </div>
    </div>
  );
}
