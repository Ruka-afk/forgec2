const SUPPORTED_LANGS = [
  { code: "en", name: "English", native: "English", flag: "\uD83C\uDDFA\uD83C\uDDF8" },
  { code: "cn", name: "Chinese", native: "\u4E2D\u6587", flag: "\uD83C\uDDE8\uD83C\uDDF3" },
  { code: "jp", name: "Japanese", native: "\u65E5\u672C\u8A9E", flag: "\uD83C\uDDEF\uD83C\uDDF5" },
  { code: "ko", name: "Korean", native: "\uD55C\uAD6D\uC5B4", flag: "\uD83C\uDDF0\uD83C\uDDF7" },
  { code: "ar", name: "Arabic", native: "\u0627\u0644\u0639\u0631\u0628\u064A\u0629", flag: "\uD83C\uDDE6\uD83C\uDDEA" },
];

export default function LanguageSection({ language, onSetLanguage }: { language: string; onSetLanguage: (code: string) => void }) {
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-sky-500 to-cyan-600 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-language text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Language</h2><p className="text-xs text-sky-200">Select interface language</p></div>
        </div>
      </div>
      <div className="p-6">
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
          {SUPPORTED_LANGS.map((lang) => (
            <button key={lang.code} onClick={() => onSetLanguage(lang.code)}
              className={`p-4 border rounded-xl transition-colors ${language === lang.code ? "border-sky-500 bg-sky-50 dark:bg-sky-900/40" : "bg-slate-50 dark:bg-slate-700 border-[var(--border)] hover:border-sky-400 hover:bg-sky-50 dark:hover:bg-sky-900/30"}`}>
              <div className="text-2xl mb-2">{lang.flag}</div>
              <div className="text-sm font-medium text-[var(--text-secondary)]">{lang.native}</div>
              <div className="text-xs text-slate-400 dark:text-slate-500 mt-1">{lang.name}</div>
            </button>
          ))}
        </div>
        <div className="mt-6 bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4">
          <div className="flex items-center gap-3">
            <i className="fa-solid fa-info-circle text-slate-400"></i>
            <div className="text-xs text-slate-500 dark:text-slate-400">Language preference is saved in a cookie for 365 days.</div>
          </div>
        </div>
      </div>
    </section>
  );
}
