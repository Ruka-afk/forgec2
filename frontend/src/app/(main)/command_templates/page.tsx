"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal } from "@/components/UI";

interface Template {
  id?: string;
  ID?: string;
  name?: string;
  Name?: string;
  command?: string;
  Command?: string;
  description?: string;
  Description?: string;
  category?: string;
  Category?: string;
}

const categories = [
  { key: "recon", label: "信息收集", icon: "fa-search", emoji: "🔍" },
  { key: "privesc", label: "权限提升", icon: "fa-shield-halved", emoji: "🛡️" },
  { key: "lateral", label: "横向移动", icon: "fa-arrows-left-right", emoji: "↔️" },
  { key: "exfil", label: "数据外传", icon: "fa-file-export", emoji: "📤" },
  { key: "persist", label: "持久化", icon: "fa-hard-drive", emoji: "💾" },
];


export default function CommandTemplatesPage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [form, setForm] = useState({ name: "", category: "recon", command: "", description: "" });
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const loadTemplates = useCallback(async () => {
    try {
      const resp = await fetch(`${API_BASE}?p=/templates&format=json`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      setTemplates(data.templates || data.Templates || data || []);
    } catch {
      setTemplates([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadTemplates()); }, [loadTemplates]);

  const handleSave = async () => {
    try {
      await fetch(`${API_BASE}?p=/api/templates&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      setShowAdd(false);
      setForm({ name: "", category: "recon", command: "", description: "" });
      loadTemplates();
    } catch (e) { console.error("CommandTemplates: save template failed", e); }
  };

  const handleDelete = (id: string) => {
    setCfm({msg: "确认删除该模板？", cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/api/templates/${id}&format=json`, { method: "DELETE" });
        loadTemplates();
      } catch (e) { console.error("CommandTemplates: delete template failed", e); }
    }});
  };

  const grouped: Record<string, Template[]> = {};
  templates.forEach((t) => {
    const cat = t.category || t.Category || "other";
    if (!grouped[cat]) grouped[cat] = [];
    grouped[cat].push(t);
  });

  const getCatInfo = (cat: string) => categories.find((c) => c.key === cat) || { key: cat, label: cat, icon: "fa-file-lines", emoji: "📄" };

  if (loading)
    return (
      <div className="flex items-center justify-center h-64">
        <i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i>
      </div>
    );

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">命令模板</h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs sm:text-sm mt-1">预定义常用命令模板，一键执行</p>
        </div>
        <button onClick={() => setShowAdd(true)} className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm flex items-center gap-2 transition-colors">
          <i className="fa-solid fa-plus"></i>
          <span>新增模板</span>
        </button>
      </div>

      {templates.length === 0 ? (
        <div className="ui-card p-12 text-center">
          <i className="fa-solid fa-file-lines text-6xl text-slate-300 mb-4"></i>
          <h3 className="text-lg font-medium text-slate-700 mb-2">还没有模板</h3>
          <p className="text-slate-500 mb-4">添加第一个命令模板开始使用</p>
          <button onClick={() => setShowAdd(true)} className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm">
            <i className="fa-solid fa-plus mr-2"></i>新增模板
          </button>
        </div>
      ) : (
        Object.entries(grouped).map(([cat, temps]) => {
          const info = getCatInfo(cat);
          return (
            <div key={cat} className="mb-8">
              <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100 mb-4 flex items-center gap-2">
                <span>{info.emoji}</span>
                {info.label}
                <span className="text-sm font-normal text-slate-500">({temps.length} 个</span>
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {temps.map((t) => {
                  const id = t.id || t.ID || "";
                  const name = t.name || t.Name || "";
                  const cmd = t.command || t.Command || "";
                  const desc = t.description || t.Description || "";
                  return (
                    <div key={id} className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow group">
                      <div className="flex items-start justify-between mb-3">
                        <h3 className="font-semibold text-slate-900">{name}</h3>
                        <button onClick={() => handleDelete(id)} className="text-slate-400 hover:text-red-600 opacity-0 group-hover:opacity-100 transition-opacity">
                          <i className="fa-solid fa-trash"></i>
                        </button>
                      </div>
                      {desc && <p className="text-xs text-slate-500 mb-3">{desc}</p>}
                      <div className="bg-slate-50 border border-slate-200 rounded-xl p-3 mb-3">
                        <code className="text-xs font-mono text-slate-700 break-all">{cmd}</code>
                      </div>
                      <button className="w-full h-9 bg-indigo-50 hover:bg-indigo-100 text-indigo-700 rounded-xl text-xs font-medium transition-colors">
                        <i className="fa-solid fa-play mr-1"></i>使用此模板                      </button>
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })
      )}

      {showAdd && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowAdd(false)}></div>
          <div className="relative bg-white rounded-2xl shadow-2xl w-full max-w-2xl mx-4 overflow-hidden">
            <div className="bg-gradient-to-r from-indigo-500 to-purple-500 px-6 py-5">
              <div className="flex items-center gap-3">
                <div className="w-12 h-12 bg-white/20 rounded-full flex items-center justify-center">
                  <i className="fa-solid fa-file-circle-plus text-white text-xl"></i>
                </div>
                <div>
                  <h3 className="text-lg font-semibold text-white">新增命令模板</h3>
                  <p className="text-sm text-white/80">创建可重复使用的命令模板</p>
                </div>
              </div>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <label className="block text-sm font-medium text-[var(--text-secondary)] mb-2">模板名称</label>
                <input type="text" placeholder="枚举本地管理员" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-4 h-10 focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 transition-all" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </div>
              <div>
                <label className="block text-sm font-medium text-[var(--text-secondary)] mb-2">分类</label>
                <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-4 h-10 focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 transition-all cursor-pointer" value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })}>
                  <option value="recon">🔍 信息收集 (Recon)</option>
                  <option value="privesc">🛡️ 权限提升 (Privesc)</option>
                  <option value="lateral">↔️ 横向移动 (Lateral)</option>
                  <option value="exfil">📤 数据外传 (Exfil)</option>
                  <option value="persist">💾 持久化 (Persist)</option>
                  <option value="other">📁 其他 (Other)</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-[var(--text-secondary)] mb-2">命令</label>
                <textarea rows={4} placeholder="net localgroup administrators" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-4 py-3 focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 transition-all font-mono" value={form.command} onChange={(e) => setForm({ ...form, command: e.target.value })}></textarea>
              </div>
              <div>
                <label className="block text-sm font-medium text-[var(--text-secondary)] mb-2">描述（可选）</label>
                <input type="text" placeholder="枚举本地管理员权限" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-4 h-10 focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 transition-all" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
              </div>
            </div>
            <div className="px-6 pb-6 flex gap-3">
              <button onClick={handleSave} className="flex-1 h-11 bg-indigo-600 hover:bg-indigo-700 text-white font-medium rounded-xl transition-colors">
                <i className="fa-solid fa-save mr-2"></i>保存模板
              </button>
              <button onClick={() => setShowAdd(false)} className="px-6 h-11 bg-[var(--card-bg)] border border-[var(--border)] hover:bg-slate-50 text-slate-700 font-medium rounded-xl transition-colors">
                取消
              </button>
            </div>
          </div>
        </div>
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Delete" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
