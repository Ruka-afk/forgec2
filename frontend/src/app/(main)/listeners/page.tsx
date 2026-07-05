"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { PageHeader, SearchInput, Pagination, ConfirmModal } from "@/components/UI";

interface Listener {
  id?: string;
  ID?: string;
  name?: string;
  Name?: string;
  type?: string;
  Type?: string;
  Scheme?: string;
  scheme?: string;
  Protocol?: string;
  protocol?: string;
  host?: string;
  Host?: string;
  port?: number | string;
  Port?: number | string;
  enabled?: boolean;
  Enabled?: boolean;
  notes?: string;
  Notes?: string;
}


export default function ListenersPage() {
  const [listeners, setListeners] = useState<Listener[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ name: "", type: "http", host: "0.0.0.0", port: "8443", protocol: "http" });

  const handleTypeChange = (type: string) => {
    setCreateForm(prev => ({ ...prev, type, protocol: type === "https" ? "https" : type }));
  };
  const [creating, setCreating] = useState(false);
  const [editingListener, setEditingListener] = useState<Listener | null>(null);
  const [showEdit, setShowEdit] = useState(false);
  const [editForm, setEditForm] = useState({ name: "", type: "http", host: "0.0.0.0", port: "", protocol: "http", notes: "" });
  const handleEditTypeChange = (type: string) => {
    setEditForm(prev => ({ ...prev, type, protocol: type === "https" ? "https" : type }));
  };
  const [actionMsg, setActionMsg] = useState("");
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [agentCountMap, setAgentCountMap] = useState<Record<string, number>>({});

  const loadListeners = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/api/listeners&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setListeners(data.data || data.listeners || data.Listeners || []);
    } catch {
      setListeners([]);
    }
    setLoading(false);
  }, []);

  const handleCreate = async () => {
    if (!createForm.name || !createForm.host || !createForm.port) {
      setActionMsg("Name, host and port are required");
      setTimeout(() => setActionMsg(""), 3000);
      return;
    }
    setCreating(true);
    try {
      const body = new URLSearchParams();
      body.append("name", createForm.name);
      body.append("type", createForm.type);
      body.append("host", createForm.host);
      body.append("port", createForm.port);
      body.append("protocol", createForm.protocol);
      const res = await fetch(`${API_BASE}?p=/api/listeners`, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: body.toString() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      if (data.success) {
        setShowCreate(false);
        setCreateForm({ name: "", type: "http", host: "0.0.0.0", port: "8443", protocol: "http" });
        loadListeners();
        setActionMsg("Listener created successfully");
        setTimeout(() => setActionMsg(""), 2500);
      } else {
        setActionMsg("Failed: " + (data.error || "Unknown error"));
        setTimeout(() => setActionMsg(""), 4000);
      }
    } catch (e) {
      console.error("Listeners: create failed", e);
      setActionMsg("Failed to create listener");
      setTimeout(() => setActionMsg(""), 4000);
    }
    setCreating(false);
  };

  const handleCopy = async (address: string) => {
    try {
      await navigator.clipboard.writeText(address);
      setActionMsg("复制地址成功");
      setTimeout(() => setActionMsg(""), 2000);
    } catch (e) { console.error("Listeners: copy address failed", e); }
  };

  const handleEdit = (listener: Listener) => {
    setEditingListener(listener);
    setEditForm({
      name: listener.name || listener.Name || "",
      type: listener.type || listener.Type || "http",
      host: listener.host || listener.Host || "",
      port: String(listener.port ?? listener.Port ?? ""),
      protocol: listener.Scheme || listener.scheme || listener.Protocol || listener.protocol || listener.type || listener.Type || "http",
      notes: listener.notes || listener.Notes || "",
    });
    setShowEdit(true);
  };

  const handleEditSave = async () => {
    const id = editingListener?.id || editingListener?.ID || "";
    if (!id || !editForm.name || !editForm.host || !editForm.port) {
      setActionMsg("Name, host and port are required");
      setTimeout(() => setActionMsg(""), 3000);
      return;
    }
    try {
      const body = JSON.stringify({
        name: editForm.name,
        type: editForm.type,
        host: editForm.host,
        port: parseInt(editForm.port) || 0,
        protocol: editForm.protocol,
        notes: editForm.notes,
      });
      await fetch(`${API_BASE}?p=/api/listeners/${id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body });
      setShowEdit(false);
      setEditingListener(null);
      loadListeners();
    } catch (e) { console.error("Listeners: update listener failed", e); }
  };

  const handleToggle = async (listener: Listener) => {
    const id = listener.id || listener.ID || "";
    if (!id) return;
    const enabled = !(listener.enabled === true || listener.Enabled === true);
    try {
      await fetch(`${API_BASE}?p=/api/listeners/${id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ enabled }) });
      loadListeners();
    } catch (e) { console.error("Listeners: toggle listener failed", e); }
  };

  const handleDelete = (listener: Listener) => {
    const id = listener.id || listener.ID || "";
    if (!id) return;
    setCfm({msg: "确认删除监听器？", cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/api/listeners/${id}`, { method: "DELETE" });
        loadListeners();
      } catch (e) { console.error("Listeners: delete listener failed", e); }
    }});
  };

  useEffect(() => {
    Promise.resolve().then(() => {
      loadListeners();
      fetch(`${API_BASE}?p=/agents&page=1&pageSize=500&format=json`, { credentials: "include" })
        .then((r) => r.json())
        .then((d) => {
          const agents = d.Agents || d.agents || d.Beacons || [];
          const map: Record<string, number> = {};
          (agents as { listener_id?: number; ListenerID?: number }[]).forEach((a) => {
            const lid = String(a.listener_id ?? a.ListenerID ?? "");
            if (lid && lid !== "0") map[lid] = (map[lid] || 0) + 1;
          });
          setAgentCountMap(map);
        })
        .catch(() => setAgentCountMap({}));
    });
  }, [loadListeners]);

  const total = listeners.length;
  const enabledCount = listeners.filter(l => l.enabled === true || l.Enabled === true).length;
  const httpCount = listeners.filter(l => (l.type || l.Type) === "http").length;
  const tcpCount = listeners.filter(l => (l.type || l.Type) === "tcp").length;
  const dnsCount = listeners.filter(l => (l.type || l.Type) === "dns").length;

  const filtered = listeners.filter(l => {
    const name = (l.name || l.Name || "").toLowerCase();
    const host = (l.host || l.Host || "").toLowerCase();
    if (search && !name.includes(search.toLowerCase()) && !host.includes(search.toLowerCase())) return false;
    const type = l.type || l.Type || "";
    if (typeFilter && type !== typeFilter) return false;
    const enabled = l.enabled === true || l.Enabled === true;
    if (statusFilter === "enabled" && !enabled) return false;
    if (statusFilter === "disabled" && enabled) return false;
    return true;
  });

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title="Listeners" subtitle="配置 C2 监听器来接收来自 Implant 的连接，生成 Implant 时选择目标监听器">
        <button onClick={() => setShowCreate(true)} className="px-4 sm:px-5 h-11 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl sm:rounded-2xl flex items-center justify-center gap-x-2 text-sm font-medium w-full sm:w-auto">
          <i className="fa-solid fa-plus"></i>
          <span>Create Listener</span>
        </button>
      </PageHeader>

      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3 sm:gap-4 mb-4 sm:mb-6">
        <div className="ui-card sm:rounded-3xl p-4 sm:p-5">
          <div className="text-[10px] sm:text-xs text-slate-500 dark:text-slate-400">总计</div>
          <div className="text-2xl sm:text-3xl font-semibold mt-1 text-slate-900 dark:text-slate-100">{loading ? "..." : total}</div>
        </div>
        <div className="ui-card sm:rounded-3xl p-4 sm:p-5">
          <div className="text-[10px] sm:text-xs text-slate-500 dark:text-slate-400">运行中</div>
          <div className="text-2xl sm:text-3xl font-semibold mt-1 text-emerald-600">{loading ? "..." : enabledCount}</div>
        </div>
        <div className="ui-card sm:rounded-3xl p-4 sm:p-5">
          <div className="text-[10px] sm:text-xs text-slate-500 dark:text-slate-400">HTTP</div>
          <div className="text-2xl sm:text-3xl font-semibold mt-1 text-slate-900 dark:text-slate-100">{loading ? "..." : httpCount}</div>
        </div>
        <div className="ui-card sm:rounded-3xl p-4 sm:p-5">
          <div className="text-[10px] sm:text-xs text-slate-500 dark:text-slate-400">TCP</div>
          <div className="text-2xl sm:text-3xl font-semibold mt-1 text-slate-900 dark:text-slate-100">{loading ? "..." : tcpCount}</div>
        </div>
        <div className="ui-card sm:rounded-3xl p-4 sm:p-5 col-span-2 sm:col-span-1">
          <div className="text-[10px] sm:text-xs text-slate-500 dark:text-slate-400">DNS</div>
          <div className="text-2xl sm:text-3xl font-semibold mt-1 text-purple-600">{loading ? "..." : dnsCount}</div>
        </div>
      </div>

      <div className="flex flex-col sm:flex-row gap-3 mb-4">
        <SearchInput value={search} onChange={setSearch} placeholder="搜索名称或 Host..." className="flex-1" />
        <div className="flex gap-2">
          <select value={typeFilter} onChange={e => setTypeFilter(e.target.value)}
            className="flex-1 h-11 px-3 ui-card text-sm dark:text-slate-100">
            <option value="">全部类型</option>
            <option value="http">HTTP</option>
            <option value="https">HTTPS</option>
            <option value="tcp">TCP</option>
            <option value="dns">DNS</option>
            <option value="smb">SMB</option>
            <option value="icmp">ICMP</option>
          </select>
          <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)}
            className="flex-1 h-11 px-3 ui-card text-sm dark:text-slate-100">
            <option value="">全部状态</option>
            <option value="enabled">已启用</option>
            <option value="disabled">已禁用</option>
          </select>
        </div>
      </div>

      <div className="ui-card sm:rounded-3xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-700/50 border-b border-[var(--border)]">
              <tr>
                <th className="text-left py-3 px-4 sm:py-4 sm:px-6 font-medium text-slate-600 dark:text-slate-400 min-w-[120px]">名称</th>
                <th className="text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-slate-600 dark:text-slate-400 min-w-[80px]">类型</th>
                <th className="text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-slate-600 dark:text-slate-400 min-w-[160px]">地址</th>
                <th className="text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-slate-600 dark:text-slate-400 min-w-[60px]">Agents</th>
                <th className="text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-slate-600 dark:text-slate-400 min-w-[80px]">状态</th>
                <th className="text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-slate-600 dark:text-slate-400 min-w-[120px]">备注</th>
                <th className="text-right py-3 px-4 sm:py-4 sm:px-6 font-medium text-slate-600 dark:text-slate-400 min-w-[200px]">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
              {loading ? (
                [1, 2, 3, 4, 5].map(i => (
                  <tr key={i}>
                    <td className="py-3 px-4 sm:py-4 sm:px-6"><div className="h-4 bg-slate-200 dark:bg-slate-700 rounded animate-pulse w-24"></div></td>
                    <td className="py-3 px-3 sm:py-4 sm:px-4"><div className="h-4 bg-slate-200 dark:bg-slate-700 rounded animate-pulse w-14"></div></td>
                    <td className="py-3 px-3 sm:py-4 sm:px-4"><div className="h-4 bg-slate-200 dark:bg-slate-700 rounded animate-pulse w-32"></div></td>
                    <td className="py-3 px-3 sm:py-4 sm:px-4"><div className="h-4 bg-slate-200 dark:bg-slate-700 rounded animate-pulse w-16"></div></td>
                    <td className="py-3 px-3 sm:py-4 sm:px-4"><div className="h-4 bg-slate-200 dark:bg-slate-700 rounded animate-pulse w-20"></div></td>
                    <td className="py-3 px-4 sm:py-4 sm:px-6 text-right"><div className="flex items-center justify-end gap-1 sm:gap-2"><div className="h-8 bg-slate-200 dark:bg-slate-700 rounded-lg animate-pulse w-20"></div><div className="h-8 bg-slate-200 dark:bg-slate-700 rounded-lg animate-pulse w-20"></div><div className="h-8 bg-slate-200 dark:bg-slate-700 rounded-lg animate-pulse w-20"></div><div className="h-8 bg-slate-200 dark:bg-slate-700 rounded-lg animate-pulse w-20"></div></div></td>
                  </tr>
                ))
              ) : filtered.length > 0 ? (
                filtered.map(l => {
                  const id = l.id || l.ID || "";
                  const name = l.name || l.Name || "";
                  const type = l.type || l.Type || "";
                  const scheme = l.Scheme || l.scheme || l.Protocol || l.protocol || type;
                  const host = l.host || l.Host || "";
                  const port = l.port ?? l.Port ?? "";
                  const enabled = l.enabled === true || l.Enabled === true;
                  const notes = l.notes || l.Notes || "-";
                  return (
                    <tr key={id} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                      <td className="py-3 px-4 sm:py-4 sm:px-6 font-medium"><a href={`/listeners/${id}`} className="text-indigo-600 hover:underline">{name}</a></td>
                      <td className="py-3 px-3 sm:py-4 sm:px-4">
                        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium ${type === "http" ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400" : "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"}`}>{type}</span>
                      </td>
                      <td className="py-3 px-3 sm:py-4 sm:px-4 font-mono text-xs text-slate-600 dark:text-slate-300">{scheme}://{host}:{port}</td>
                      <td className="py-3 px-3 sm:py-4 sm:px-4 text-center">
                        <span className="text-xs font-mono text-slate-600 dark:text-slate-300">{agentCountMap[id] ?? 0}</span>
                      </td>
                      <td className="py-3 px-3 sm:py-4 sm:px-4">
                        {enabled ? (
                          <span className="inline-flex items-center px-2 py-0.5 text-[11px] bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 rounded-full">
                            <span className="w-1.5 h-1.5 bg-emerald-500 rounded-full mr-1"></span> 运行中
                          </span>
                        ) : (
                          <span className="inline-flex items-center px-2 py-0.5 text-[11px] bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-400 rounded-full">停止</span>
                        )}
                      </td>
                      <td className="py-3 px-3 sm:py-4 sm:px-4 text-xs text-slate-500 dark:text-slate-400 max-w-[150px] truncate">{notes}</td>
                       <td className="py-3 px-4 sm:py-4 sm:px-6 text-right">
                         <div className="flex items-center justify-end gap-1 sm:gap-2">
                           <button onClick={() => handleCopy(`${scheme}://${host}:${port}`)} className="w-9 h-9 sm:px-3 sm:py-1 sm:w-auto sm:h-auto text-xs bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg sm:rounded-xl flex items-center justify-center gap-x-1 text-slate-600 dark:text-slate-300">
                             <i className="fa-solid fa-copy text-sm sm:text-xs"></i>
                              <span className="hidden sm:inline">复制</span>
                            </button>
                            <button onClick={() => handleEdit(l)} className="w-9 h-9 sm:px-3 sm:py-1 sm:w-auto sm:h-auto text-xs bg-indigo-100 dark:bg-indigo-900/40 hover:bg-indigo-200 dark:hover:bg-indigo-900/60 text-indigo-700 dark:text-indigo-400 rounded-lg sm:rounded-xl flex items-center justify-center gap-x-1">
                              <i className="fa-solid fa-edit text-sm sm:text-xs"></i>
                              <span className="hidden sm:inline">编辑</span>
                           </button>
                           <button onClick={() => handleToggle(l)} className={`w-9 h-9 sm:px-3 sm:py-1 sm:w-auto sm:h-auto text-xs ${enabled ? "bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-400 hover:bg-amber-200 dark:hover:bg-amber-900/60" : "bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-400 hover:bg-emerald-200 dark:hover:bg-emerald-900/60"} rounded-lg sm:rounded-xl flex items-center justify-center gap-x-1`}>
                             <i className="fa-solid fa-power-off text-sm sm:text-xs"></i>
                               <span className="hidden sm:inline">{enabled ? "停止" : "启动"}</span>
                           </button>
                           <button onClick={() => handleDelete(l)} className="w-9 h-9 sm:px-3 sm:py-1 sm:w-auto sm:h-auto text-xs bg-red-100 dark:bg-red-900/40 hover:bg-red-200 dark:hover:bg-red-900/60 text-red-700 dark:text-red-400 rounded-lg sm:rounded-xl flex items-center justify-center gap-x-1">
                             <i className="fa-solid fa-trash text-sm sm:text-xs"></i>
                              <span className="hidden sm:inline">删除</span>
                           </button>
                         </div>
                       </td>
                    </tr>
                  );
                })
              ) : (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-slate-400 dark:text-slate-500">
                    <i className="fa-solid fa-plug text-4xl mb-3 opacity-30"></i><br />
                    暂无匹配的监听器记录                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <div className="sm:hidden px-4 py-2 text-center text-xs text-slate-400 dark:text-slate-500 border-t border-slate-100 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50">
          <i className="fa-solid fa-arrows-left-right mr-1"></i> 滑动查看更多
        </div>
      </div>

      <div className="mt-4 sm:mt-6 p-3 sm:p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-100 dark:border-blue-800 rounded-2xl sm:rounded-3xl text-xs text-blue-700 dark:text-blue-400">
        <i className="fa-solid fa-info-circle mr-1"></i>
        <strong>Tip:</strong> Select a listener when generating an implant to connect agents to the corresponding address.
      </div>

      {actionMsg && (
        <div className="fixed top-4 right-4 z-[60] px-4 py-2 bg-emerald-600 text-white text-sm rounded-xl shadow-lg">
          {actionMsg}
        </div>
      )}

      {showCreate && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-[var(--card-bg)] rounded-2xl w-full max-w-md p-6 shadow-xl">
            <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100 mb-4">Create Listener</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-slate-500 mb-1">Name</label>
                <input value={createForm.name} onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm" placeholder="my-listener" />
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Type</label>
                <select value={createForm.type} onChange={(e) => handleTypeChange(e.target.value)}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm">
                  <option value="http">HTTP</option>
                  <option value="https">HTTPS</option>
                  <option value="tcp">TCP</option>
                  <option value="dns">DNS</option>
                  <option value="smb">SMB</option>
                  <option value="icmp">ICMP</option>
                </select>
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Host</label>
                <input value={createForm.host} onChange={(e) => setCreateForm({ ...createForm, host: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Port</label>
                <input value={createForm.port} onChange={(e) => setCreateForm({ ...createForm, port: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm" placeholder="8443" />
              </div>
            </div>
            <div className="flex gap-2 mt-5">
              <button onClick={() => setShowCreate(false)} className="flex-1 h-10 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg text-sm text-slate-600 dark:text-slate-300">Cancel</button>
              <button onClick={handleCreate} disabled={creating || !createForm.name || !createForm.port}
                className="flex-1 h-10 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 rounded-lg text-sm text-white font-medium">
                {creating ? "Creating..." : "Create"}
              </button>
            </div>
          </div>
        </div>
      )}

      {showEdit && editingListener && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-[var(--card-bg)] rounded-2xl w-full max-w-md p-6 shadow-xl">
            <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100 mb-4">Edit Listener</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-xs text-slate-500 mb-1">Name</label>
                <input value={editForm.name} onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Type</label>
                <select value={editForm.type} onChange={(e) => handleEditTypeChange(e.target.value)}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm">
                  <option value="http">HTTP</option>
                  <option value="https">HTTPS</option>
                  <option value="tcp">TCP</option>
                  <option value="dns">DNS</option>
                  <option value="smb">SMB</option>
                  <option value="icmp">ICMP</option>
                </select>
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Host</label>
                <input value={editForm.host} onChange={(e) => setEditForm({ ...editForm, host: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Port</label>
                <input value={editForm.port} onChange={(e) => setEditForm({ ...editForm, port: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Notes</label>
                <input value={editForm.notes} onChange={(e) => setEditForm({ ...editForm, notes: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 h-10 text-sm" />
              </div>
            </div>
            <div className="flex gap-2 mt-5">
              <button onClick={() => { setShowEdit(false); setEditingListener(null); }} className="flex-1 h-10 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg text-sm text-slate-600 dark:text-slate-300">Cancel</button>
              <button onClick={handleEditSave} className="flex-1 h-10 bg-indigo-600 hover:bg-indigo-700 rounded-lg text-sm text-white font-medium">Save</button>
            </div>
          </div>
        </div>
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Delete" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
