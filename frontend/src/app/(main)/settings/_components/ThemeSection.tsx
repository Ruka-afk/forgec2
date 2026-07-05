export default function ThemeSection({ theme, onApplyTheme }: { theme: string; onApplyTheme: (t: string) => void }) {
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-violet-600 to-indigo-600 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-palette text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Theme</h2><p className="text-xs text-violet-200">Customize appearance</p></div>
        </div>
      </div>
      <div className="p-6">
        <div className="mb-6">
          <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4 flex items-center gap-2"><i className="fa-solid fa-moon text-slate-400"></i>Theme Mode</h3>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            {["light", "dark", "system"].map((mode) => (
              <button key={mode} onClick={() => onApplyTheme(mode)}
                className={`p-4 border rounded-xl transition-colors capitalize ${theme === mode ? "border-indigo-500 bg-indigo-50 dark:bg-indigo-900/30" : "bg-slate-50 dark:bg-slate-700 border-[var(--border)] hover:border-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/30"}`}>
                <div className={`text-2xl mb-2 ${mode === "light" ? "text-amber-500" : mode === "dark" ? "text-indigo-400" : "text-slate-500"}`}>
                  <i className={`fa-solid ${mode === "light" ? "fa-sun" : mode === "dark" ? "fa-moon" : "fa-desktop"}`}></i>
                </div>
                <div className="text-sm font-medium text-[var(--text-secondary)]">{mode}</div>
                <div className="text-xs text-slate-400 dark:text-slate-500 mt-1">{mode === "light" ? "Clean and bright" : mode === "dark" ? "Easy on the eyes" : "Match OS setting"}</div>
              </button>
            ))}
          </div>
        </div>
        <div className="bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4">
          <div className="flex items-center gap-3">
            <i className="fa-solid fa-info-circle text-slate-400"></i>
            <div className="text-xs text-slate-500 dark:text-slate-400">Theme settings are saved to localStorage.</div>
          </div>
        </div>
      </div>
    </section>
  );
}
