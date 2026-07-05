import { PurgeDays } from "./types";

export default function MaintenanceSection({
  purgeDays, setPurgeDays, saving, onPurge, onPurgeScreenshots,
}: {
  purgeDays: PurgeDays;
  setPurgeDays: React.Dispatch<React.SetStateAction<PurgeDays>>;
  saving: boolean;
  onPurge: (type: string) => void;
  onPurgeScreenshots: () => void;
}) {
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-orange-600 to-orange-700 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-broom text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Data Maintenance</h2><p className="text-xs text-orange-200">Clean old data, free up space</p></div>
        </div>
      </div>
      <div className="p-6 space-y-4">
        {[
          { label: "Purge Old Tasks", desc: "Delete completed/failed tasks older than specified days", type: "tasks" },
          { label: "Purge Audit Logs", desc: "Delete audit logs older than specified days", type: "audit" },
        ].map((item) => (
          <div key={item.type} className="p-4 bg-slate-50 dark:bg-slate-700/50 rounded-xl border border-[var(--border)] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <div className="text-sm font-medium text-[var(--text-secondary)]">{item.label}</div>
              <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">{item.desc}</div>
            </div>
            <div className="flex items-center gap-2">
              <select value={purgeDays[item.type as keyof PurgeDays]} onChange={(e) => setPurgeDays({ ...purgeDays, [item.type]: e.target.value })} className="bg-[var(--card-bg)] border border-[var(--border)] text-xs rounded-xl px-3 h-8 text-[var(--text-secondary)]">
                <option value="7">7 days</option>
                <option value="14">14 days</option>
                <option value="30">30 days</option>
                <option value="60">60 days</option>
                <option value="90">90 days</option>
              </select>
              <button onClick={() => onPurge(item.type)} disabled={saving} className="px-4 h-8 bg-red-100 hover:bg-red-200 text-red-700 rounded-xl text-xs font-medium transition-colors disabled:opacity-50">
                <i className="fa-solid fa-trash mr-1"></i>Purge
              </button>
            </div>
          </div>
        ))}
        <div className="p-4 bg-slate-50 dark:bg-slate-700/50 rounded-xl border border-[var(--border)] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div className="text-sm font-medium text-[var(--text-secondary)]">Purge Screenshots</div>
            <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Delete all stored screenshots</div>
          </div>
          <button onClick={onPurgeScreenshots} disabled={saving} className="px-4 h-8 bg-red-100 hover:bg-red-200 text-red-700 rounded-xl text-xs font-medium transition-colors disabled:opacity-50">
            <i className="fa-solid fa-trash mr-1"></i>Purge
          </button>
        </div>
      </div>
    </section>
  );
}
