"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";

interface TranslationEntry {
  key: string;
  value: string;
  defaultValue?: string;
}

interface TranslationStat {
  total: number;
  completed: number;
  percentage: number;
}

interface MissingKey {
  key: string;
  missingLangs: string[];
}

const LANGUAGES = [
  { code: "zh-CN", name: "简体中文", native: "简体中文", flag: "🇨🇳" },
  { code: "en", name: "English", native: "English", flag: "🇬🇧" },
  { code: "ja", name: "日本語", native: "日本語", flag: "🇯🇵" },
  { code: "ko", name: "한국어", native: "한국어", flag: "🇰🇷" },
];

export default function TranslationsPage() {
  const [activeLang, setActiveLang] = useState("zh-CN");
  const [translations, setTranslations] = useState<TranslationEntry[]>([]);
  const [editValues, setEditValues] = useState<Record<string, string>>({});
  const [stats, setStats] = useState<Record<string, TranslationStat>>({});
  const [missingKeys, setMissingKeys] = useState<MissingKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [saving, setSaving] = useState<Record<string, boolean>>({});
  const [showAddModal, setShowAddModal] = useState(false);
  const [newKey, setNewKey] = useState("");
  const [newValues, setNewValues] = useState<Record<string, string>>({});
  const [showOnlyMissing, setShowOnlyMissing] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadStats = useCallback(async () => {
    try {
      const resp = await fetch(`${API_BASE}?p=/api/translations/stats&format=json`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      if (data.stats) setStats(data.stats);
    } catch (e) { console.error("Translations: load stats failed", e); }
  }, []);

  const loadTranslations = useCallback(async (lang: string) => {
    setLoading(true);
    try {
      const resp = await fetch(`${API_BASE}?p=/api/translations&lang=${lang}&format=json`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      const entries: TranslationEntry[] = data.translations ? Object.entries(data.translations).map(([key, value]) => ({
        key,
        value: value as string,
      })) : data.entries || data.Entries || [];
      setTranslations(entries);
      const edits: Record<string, string> = {};
      entries.forEach((e) => { edits[e.key] = e.value; });
      setEditValues(edits);
    } catch {
      setTranslations([]);
      setEditValues({});
    } finally {
      setLoading(false);
    }
  }, []);

  const loadMissingKeys = useCallback(async () => {
    try {
      const resp = await fetch(`${API_BASE}?p=/api/translations/check&format=json`);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const data = await resp.json();
      setMissingKeys(data.missing_translations || data.missing || data.Missing || []);
    } catch { setMissingKeys([]); }
  }, []);

  const loadAll = useCallback(async () => {
    await Promise.all([loadStats(), loadMissingKeys()]);
    await loadTranslations(activeLang);
  }, [loadStats, loadMissingKeys, loadTranslations, activeLang]);

  useEffect(() => { Promise.resolve().then(() => loadAll()); }, [loadAll]);

  useEffect(() => {
    Promise.resolve().then(() => {
      intervalRef.current = setInterval(loadStats, 30000);
    });
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, [loadStats]);

  useEffect(() => { Promise.resolve().then(() => loadTranslations(activeLang)); }, [activeLang, loadTranslations]);

  const handleSave = async (key: string) => {
    setSaving((s) => ({ ...s, [key]: true }));
    try {
      await fetch(`${API_BASE}?p=/api/translations/save&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key, value: editValues[key] || "", lang: activeLang }),
      });
    } catch (e) { console.error("Translations: save translation failed", e); }
    setSaving((s) => ({ ...s, [key]: false }));
  };

  const handleAddKey = async () => {
    if (!newKey.trim()) return;
    try {
      await fetch(`${API_BASE}?p=/api/translations/add&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: newKey, values: newValues }),
      });
      setShowAddModal(false);
      setNewKey("");
      setNewValues({});
      loadTranslations(activeLang);
      loadStats();
      loadMissingKeys();
    } catch (e) { console.error("Translations: add key failed", e); }
  };

  const handleExportJSON = () => {
    const obj: Record<string, string> = {};
    translations.forEach((t) => { obj[t.key] = editValues[t.key] || t.value; });
    const blob = new Blob([JSON.stringify(obj, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `translations_${activeLang}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleExportCSV = () => {
    const headers = ["Key", "Value"];
    const rows = translations.map((t) => [t.key, editValues[t.key] || t.value]);
    const csv = [headers, ...rows].map((r) => r.map((c) => `"${(c || "").replace(/"/g, '""')}"`).join(",")).join("\n");
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `translations_${activeLang}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const filtered = translations.filter((t) => {
    if (!search && !showOnlyMissing) return true;
    const term = search.toLowerCase();
    const matchesSearch = !search || t.key.toLowerCase().includes(term) || t.value.toLowerCase().includes(term);
    const isMissing = missingKeys.some((m) => m.key === t.key && m.missingLangs.includes(activeLang));
    if (showOnlyMissing) return isMissing && matchesSearch;
    return matchesSearch;
  });

  const currentStats = stats[activeLang] || { total: translations.length, completed: 0, percentage: 0 };
  const missingCount = missingKeys.filter((m) => m.missingLangs.includes(activeLang)).length;

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">
            <i className="fa-solid fa-language text-indigo-500 mr-2"></i>翻译管理
          </h1>
          <p className="text-xs sm:text-sm text-slate-500 dark:text-slate-400 mt-1">管理并编辑 ForgeC2 多语言翻译</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => setShowAddModal(true)} className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm font-medium flex items-center gap-x-2 transition-colors">
            <i className="fa-solid fa-plus"></i> 新增          </button>
          <button onClick={handleExportJSON} className="px-3 h-10 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-xl text-xs font-medium flex items-center gap-1.5 transition-colors">
            <i className="fa-solid fa-file-code"></i> JSON
          </button>
          <button onClick={handleExportCSV} className="px-3 h-10 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-xl text-xs font-medium flex items-center gap-1.5 transition-colors">
            <i className="fa-solid fa-file-csv"></i> CSV
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        {LANGUAGES.map((lang) => {
          const s = stats[lang.code] || { total: 0, completed: 0, percentage: 0 };
          const missing = missingKeys.filter((m) => m.missingLangs.includes(lang.code)).length;
          return (
            <button key={lang.code} onClick={() => setActiveLang(lang.code)}
              className={`bg-[var(--card-bg)] border rounded-2xl p-5 shadow-sm hover:shadow-md transition-all text-left ${activeLang === lang.code ? "border-indigo-500 ring-1 ring-indigo-500/20" : "border-[var(--border)]"}`}>
              <div className="flex items-center justify-between mb-3">
                <span className="text-2xl">{lang.flag}</span>
                {missing > 0 && <span className="px-2 py-0.5 text-[10px] rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 font-medium">{missing} 缺失</span>}
              </div>
              <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">{lang.native}</div>
              <div className="text-xs text-slate-500 dark:text-slate-400">{lang.name}</div>
              <div className="mt-3">
                <div className="flex items-center justify-between text-xs mb-1">
                  <span className="text-slate-500">{s.completed}/{s.total}</span>
                  <span className={`font-semibold ${s.percentage >= 90 ? "text-emerald-600 dark:text-emerald-400" : s.percentage >= 50 ? "text-amber-600 dark:text-amber-400" : "text-red-600 dark:text-red-400"}`}>{s.percentage ?? 0}%</span>
                </div>
                <div className="h-1.5 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
                  <div className={`h-full rounded-full transition-all ${s.percentage >= 90 ? "bg-emerald-500" : s.percentage >= 50 ? "bg-amber-500" : "bg-red-500"}`} style={{ width: `${s.percentage ?? 0}%` }}></div>
                </div>
              </div>
            </button>
          );
        })}
      </div>

      {missingCount > 0 && (
        <div className="bg-amber-50 dark:bg-amber-900/10 border border-amber-200 dark:border-amber-800/30 rounded-2xl p-4 mb-6">
          <div className="flex items-center gap-2">
            <i className="fa-solid fa-triangle-exclamation text-amber-500"></i>
            <span className="text-sm font-medium text-amber-800 dark:text-amber-300">当前语言缺失 {missingCount} 个翻译项</span>
            <button onClick={() => setShowOnlyMissing(!showOnlyMissing)} className={`ml-auto px-3 h-7 rounded-lg text-xs font-medium transition-colors ${showOnlyMissing ? "bg-amber-600 text-white" : "bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300 hover:bg-amber-200"}`}>
              {showOnlyMissing ? "显示全部" : "仅看缺失"}
            </button>
          </div>
          {showOnlyMissing && (
            <div className="mt-3 flex flex-wrap gap-1.5">
              {missingKeys.filter((m) => m.missingLangs.includes(activeLang)).slice(0, 15).map((m) => (
                <span key={m.key} className="px-2 py-0.5 text-[10px] font-mono rounded-md bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-300">{m.key}</span>
              ))}
              {missingCount > 15 && <span className="text-xs text-amber-600 dark:text-amber-400 ml-1">+{missingCount - 15} more</span>}
            </div>
          )}
        </div>
      )}

      <div className="ui-card overflow-hidden">
        <div className="p-4 border-b border-[var(--border)] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">翻译条目</h2>
            <span className="text-xs text-slate-500">{filtered.length} </span>
          </div>
          <div className="relative">
            <i className="fa-solid fa-search absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-sm"></i>
            <input type="text" placeholder="输入翻译..." value={search} onChange={(e) => setSearch(e.target.value)}
              className="pl-9 pr-4 py-2 bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl text-sm focus:outline-none focus:border-indigo-500 w-full sm:w-64" />
          </div>
        </div>
        <div className="max-h-[500px] overflow-y-auto divide-y divide-slate-100 dark:divide-slate-700">
          {loading ? (
            <div className="p-8 text-center text-slate-500">
              <i className="fa-solid fa-spinner fa-spin text-xl mb-2"></i>
              <p className="text-sm">加载翻译...</p>
            </div>
          ) : filtered.length === 0 ? (
            <div className="p-12 text-center text-slate-400">
              <i className="fa-solid fa-language text-3xl mb-2"></i>
              <p className="text-sm">暂无翻译条目</p>
            </div>
          ) : (
            filtered.map((entry) => {
              const isMissing = missingKeys.some((m) => m.key === entry.key && m.missingLangs.includes(activeLang));
              const isEdited = editValues[entry.key] !== entry.value && editValues[entry.key] !== undefined;
              const isSaving = saving[entry.key];
              return (
                <div key={entry.key} className={`px-4 py-3 transition-colors hover:bg-slate-50 dark:hover:bg-slate-700/50 ${isMissing ? "bg-amber-50/50 dark:bg-amber-900/5" : ""}`}>
                  <div className="flex items-start gap-4">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1.5">
                        <span className="text-sm font-mono text-indigo-600 dark:text-indigo-400 break-all">{entry.key}</span>
                        {isMissing && <span className="px-1.5 py-0.5 text-[9px] rounded bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 font-medium shrink-0">缺失</span>}
                        {isEdited && <span className="px-1.5 py-0.5 text-[9px] rounded bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 font-medium shrink-0">已修改</span>}
                      </div>
                      <div className="flex items-start gap-3">
                        <input type="text" value={editValues[entry.key] ?? ""} onChange={(e) => setEditValues({ ...editValues, [entry.key]: e.target.value })}
                          className="flex-1 text-sm bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-lg px-3 py-2 focus:outline-none focus:border-indigo-500 dark:text-slate-100 placeholder-slate-400" placeholder="输入翻译..." />
                        <button onClick={() => handleSave(entry.key)} disabled={isSaving || !isEdited}
                          className="px-3 h-9 bg-indigo-600 hover:bg-indigo-700 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-lg text-xs font-medium flex items-center gap-1.5 transition-colors shrink-0">
                          {isSaving ? <i className="fa-solid fa-spinner fa-spin"></i> : <i className="fa-solid fa-check"></i>}
                          {isSaving ? "保存中" : "保存"}
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              );
            })
          )}
        </div>
        <div className="px-4 py-3 border-t border-[var(--border)] bg-slate-50 dark:bg-slate-700/30 flex items-center justify-between text-xs text-slate-500">
          <span>共计 {currentStats.completed}/{currentStats.total} ({currentStats.percentage ?? Math.round(translations.length > 0 ? (translations.filter((t) => t.value).length / translations.length) * 100 : 0)}%)</span>
          <span>缺失: {missingMissingCount(activeLang, missingKeys)}</span>
        </div>
      </div>

      {showAddModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" onClick={() => setShowAddModal(false)}>
          <div className="bg-[var(--card-bg)] rounded-2xl shadow-xl w-full max-w-lg p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-5">
              <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">添加翻译</h3>
              <button onClick={() => setShowAddModal(false)} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded-lg transition-colors">
                <i className="fa-solid fa-times"></i>
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-[var(--text-secondary)] mb-1.5">键名</label>
                <input type="text" value={newKey} onChange={(e) => setNewKey(e.target.value)} placeholder="示例: page.section.title"
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100 font-mono" />
              </div>
              {LANGUAGES.map((lang) => (
                <div key={lang.code}>
                  <label className="block text-sm font-medium text-[var(--text-secondary)] mb-1.5">
                    {lang.flag} {lang.native}
                  </label>
                  <input type="text" value={newValues[lang.code] || ""} onChange={(e) => setNewValues({ ...newValues, [lang.code]: e.target.value })}
                    placeholder={`输入 ${lang.native} 翻译`}
                    className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 py-2.5 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100" />
                </div>
              ))}
              <div className="flex gap-3 pt-3">
                <button onClick={handleAddKey} className="flex-1 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm font-medium transition-colors">
                  <i className="fa-solid fa-plus mr-1.5"></i>添加
                </button>
                <button onClick={() => setShowAddModal(false)} className="px-6 h-10 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] rounded-xl text-sm font-medium transition-colors">
                  取消
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function missingMissingCount(lang: string, missingKeys: MissingKey[]): number {
  return missingKeys.filter((m) => m.missingLangs.includes(lang)).length;
}
