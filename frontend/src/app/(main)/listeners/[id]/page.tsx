"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { API_BASE } from "@/lib/constants";

interface ListenerDetail {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Host?: string;
  host?: string;
  Port?: number | string;
  port?: number | string;
  Type?: string;
  type?: string;
  Scheme?: string;
  scheme?: string;
  Protocol?: string;
  protocol?: string;
  Enabled?: boolean | string;
  enabled?: boolean;
  Notes?: string;
  notes?: string;
  CreatedAt?: string;
}

interface Agent {
  ID?: string;
  id?: string;
  Hostname?: string;
  hostname?: string;
  IP?: string;
  ip?: string;
  OS?: string;
  os?: string;
  Arch?: string;
  arch?: string;
  LastSeen?: string;
  last_seen?: string;
  Status?: string;
  status?: string;
}

export default function ListenerDetailPage() {
  const params = useParams();
  const id = params?.id as string;
  const [listener, setListener] = useState<ListenerDetail | null>(null);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [stats, setStats] = useState({ total: 0, active: 0 });
  const [loading, setLoading] = useState(true);

  const loadDetail = useCallback(async () => {
    if (!id) return;
    try {
      const res = await fetch(`${API_BASE}?p=${encodeURIComponent(`/listeners/${id}`)}&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setListener(data.Listener || data.listener || data);
      const a: Agent[] = data.Agents || data.agents || [];
      setAgents(a);
      setStats({ total: a.length, active: a.filter(ag => (ag.Status || ag.status) === "online").length });
    } catch {
      setListener(null);
      setAgents([]);
      setStats({ total: 0, active: 0 });
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { Promise.resolve().then(() => loadDetail()); }, [loadDetail]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full"></div>
      </div>
    );
  }

  if (!listener) {
    return (
      <div className="text-center py-20 text-slate-400 dark:text-slate-500">
        <i className="fa-solid fa-plug text-4xl mb-3 opacity-30"></i>
        <p>Listener not found</p>
      </div>
    );
  }

  const name = listener.Name || listener.name || "Unknown";
  const scheme = listener.Scheme || listener.scheme || listener.Protocol || listener.protocol || listener.Type || listener.type || "http";
  const host = listener.Host || listener.host || "0.0.0.0";
  const port = listener.Port ?? listener.port ?? 8080;
  const isEnabled = listener.Enabled === true || listener.Enabled === "true" || listener.enabled === true;
  const notes = listener.notes || listener.Notes || "-";
  const createdAt = listener.CreatedAt || "-";

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex items-center gap-x-4 mb-6">
        <Link href="/listeners" className="text-slate-400 dark:text-slate-500 hover:text-slate-600 dark:text-slate-300">
          <i className="fa-solid fa-arrow-left text-xl"></i>
        </Link>
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">{name}</h1>
          <div className="text-slate-500 dark:text-slate-400 text-sm">{scheme}://{host}:{port}</div>
        </div>
        <div className="ml-auto">
          {isEnabled ? (
            <span className="px-3 py-1 bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 rounded-full text-sm">启用</span>
          ) : (
            <span className="px-3 py-1 bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-400 rounded-full text-sm">禁用</span>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-1">
          <div className="bg-[var(--card-bg)] border border-[var(--border)] rounded-3xl p-6">
            <h3 className="font-semibold mb-4 text-slate-900 dark:text-slate-100">监听器信息</h3>
            <div className="space-y-3 text-sm">
              <div><span className="text-slate-500 dark:text-slate-400">方案:</span> <span className="text-slate-900 dark:text-slate-100">{scheme}</span></div>
              <div><span className="text-slate-500 dark:text-slate-400">地址:</span> <span className="text-slate-900 dark:text-slate-100">{host}:{port}</span></div>
              <div><span className="text-slate-500 dark:text-slate-400">传输类型:</span> <span className="text-slate-900 dark:text-slate-100">{listener.Type || listener.type || scheme}</span></div>
              <div><span className="text-slate-500 dark:text-slate-400">备注:</span> <span className="text-slate-900 dark:text-slate-100">{notes}</span></div>
              <div><span className="text-slate-500 dark:text-slate-400">创建:</span> <span className="text-slate-900 dark:text-slate-100">{createdAt}</span></div>
            </div>
            <div className="mt-6 flex gap-2">
               <a href={`/generate?listener_id=${id}`} className="flex-1 text-center px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl text-sm">为此监听器生成 Implant</a>
            </div>
          </div>
        </div>

        <div className="lg:col-span-2">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-[var(--card-bg)] border border-[var(--border)] rounded-3xl p-5">
              <div className="text-xs text-slate-500 dark:text-slate-400">关联 Agent 总数</div>
              <div className="text-4xl font-semibold mt-2 text-slate-900 dark:text-slate-100">{stats.total}</div>
            </div>
            <div className="bg-[var(--card-bg)] border border-[var(--border)] rounded-3xl p-5">
              <div className="text-xs text-slate-500 dark:text-slate-400">当前活跃</div>
              <div className="text-4xl font-semibold mt-2 text-emerald-600">{stats.active}</div>
            </div>
            <div className="bg-[var(--card-bg)] border border-[var(--border)] rounded-3xl p-5">
              <div className="text-xs text-slate-500 dark:text-slate-400">负载均衡提示</div>
              <div className="text-sm mt-2 text-slate-600 dark:text-slate-300">可将多个监听器指向不同服务器或端口实现多路负载均衡。Agent 可随机或按策略连接不同监听器</div>
            </div>
          </div>
        </div>
      </div>

      <div className="mt-8">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100">使用此监听器的 Agent ({stats.total})</h2>
          <Link href="/agents" className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400">查看全部 Agent </Link>
        </div>

        <div className="bg-[var(--card-bg)] border border-[var(--border)] rounded-3xl overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-700/50">
              <tr>
                <th className="text-left py-3 px-6 font-medium text-slate-600 dark:text-slate-400">Agent</th>
                <th className="text-left py-3 px-4 font-medium text-slate-600 dark:text-slate-400">IP</th>
                <th className="text-left py-3 px-4 font-medium text-slate-600 dark:text-slate-400">系统</th>
                <th className="text-left py-3 px-4 font-medium text-slate-600 dark:text-slate-400">最后在线</th>
                <th className="text-center py-3 px-4 font-medium text-slate-600 dark:text-slate-400">状态</th>
                <th className="text-right py-3 px-6 font-medium text-slate-600 dark:text-slate-400">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
              {agents.length > 0 ? (
                agents.map((a, i) => {
                  const aid = a.ID || a.id || String(i);
                  const status = a.Status || a.status || "offline";
                  return (
                    <tr key={aid} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                      <td className="py-3 px-6 font-medium text-slate-900 dark:text-slate-100">{a.Hostname || a.hostname || "-"}</td>
                      <td className="py-3 px-4 font-mono text-xs text-slate-600 dark:text-slate-300">{a.IP || a.ip || "-"}</td>
                      <td className="py-3 px-4 text-xs text-slate-600 dark:text-slate-300">{a.OS || a.os || "-"}/{a.Arch || a.arch || "-"}</td>
                      <td className="py-3 px-4 text-xs text-slate-500 dark:text-slate-400">{a.LastSeen || a.last_seen || "-"}</td>
                      <td className="py-3 px-4 text-center">
                        {status === "online" ? (
                          <span className="px-2 py-0.5 text-xs bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 rounded">在线</span>
                        ) : (
                          <span className="px-2 py-0.5 text-xs bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-400 rounded">离线</span>
                        )}
                      </td>
                      <td className="py-3 px-6 text-right">
                        <a href={`/agents/${aid}`} className="text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 hover:underline text-sm">璇︽儏</a>
                      </td>
                    </tr>
                  );
                })
              ) : (
                <tr>
                  <td colSpan={6} className="py-10 text-center text-slate-400 dark:text-slate-500">暂无 Implant 使用此监听器</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
