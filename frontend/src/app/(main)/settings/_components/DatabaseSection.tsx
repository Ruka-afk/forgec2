import { SettingsData } from "./types";

export default function DatabaseSection({
  data, saving, onVacuum, onBackup, onDownloadDB,
}: {
  data: SettingsData;
  saving: boolean;
  onVacuum: () => void;
  onBackup: () => void;
  onDownloadDB: () => void;
}) {
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-cyan-600 to-cyan-700 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-database text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Database</h2><p className="text-xs text-cyan-200">Statistics and management</p></div>
        </div>
      </div>
      <div className="p-6">
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3 mb-6">
          {[
            { label: "Size", value: data.DatabaseSize ? (data.DatabaseSize / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: "Agents", value: data.TotalAgents ?? 0 },
            { label: "Listeners", value: data.TotalListeners ?? data.total_listeners ?? 0 },
            { label: "Tasks", value: data.TotalTasks ?? data.total_tasks ?? 0 },
            { label: "Credentials", value: data.TotalCredentials ?? data.total_credentials ?? 0 },
            { label: "Audit Logs", value: data.TotalAudits ?? data.total_audits ?? 0 },
          ].map((stat) => (
            <div key={stat.label} className="bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4 border border-[var(--border)]">
              <div className="text-xs text-slate-500 dark:text-slate-400">{stat.label}</div>
              <div className="font-semibold text-sm text-slate-700 dark:text-slate-200 mt-1">{stat.value}</div>
            </div>
          ))}
        </div>
        <div className="flex flex-wrap gap-3">
          <button onClick={onVacuum} disabled={saving} className="px-4 h-10 bg-purple-100 dark:bg-purple-900/30 hover:bg-purple-200 dark:hover:bg-purple-800 text-purple-700 dark:text-purple-400 rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <i className="fa-solid fa-compress mr-1"></i>VACUUM
          </button>
          <button onClick={onBackup} disabled={saving} className="px-4 h-10 bg-blue-100 dark:bg-blue-900/30 hover:bg-blue-200 dark:hover:bg-blue-800 text-blue-700 dark:text-blue-400 rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <i className="fa-solid fa-copy mr-1"></i>Backup
          </button>
          <button onClick={onDownloadDB} className="px-4 h-10 bg-emerald-100 dark:bg-emerald-900/30 hover:bg-emerald-200 dark:hover:bg-emerald-800 text-emerald-700 dark:text-emerald-400 rounded-xl text-sm font-medium transition-colors">
            <i className="fa-solid fa-download mr-1"></i>Download
          </button>
        </div>
      </div>
    </section>
  );
}
