import { SettingsData, ServerForm } from "./types";
import Toggle from "./Toggle";

export default function ServerSection({
  data, form, setForm, saving, inputCls, onSave,
}: {
  data: SettingsData;
  form: ServerForm;
  setForm: React.Dispatch<React.SetStateAction<ServerForm>>;
  saving: boolean;
  inputCls: string;
  onSave: (e: React.FormEvent) => void;
}) {
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-indigo-600 to-indigo-700 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-server text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Server Configuration</h2><p className="text-xs text-indigo-200">Listen address, log level, transport settings</p></div>
        </div>
      </div>
      <div className="p-6">
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">HTTP Port</label>
              <input type="number" defaultValue={data.ServerPort ?? data.server_port} readOnly className={`${inputCls} bg-slate-100 dark:bg-slate-700 cursor-not-allowed text-slate-500`} />
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Listen Address</label>
              <input type="text" defaultValue={data.ServerAddress ?? data.server_address ?? ""} readOnly className={`${inputCls} bg-slate-100 dark:bg-slate-700 cursor-not-allowed text-slate-500 font-mono`} />
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Log Level</label>
              <select value={form.log_level} onChange={(e) => setForm({ ...form, log_level: e.target.value })} className={inputCls}>
                <option value="debug">Debug</option>
                <option value="info">Info</option>
                <option value="warn">Warning</option>
                <option value="error">Error</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">TLS</label>
              <div className="h-11 flex items-center">
                <span className={`inline-flex items-center px-3 py-1.5 text-xs font-medium rounded-xl ${data.TLSEnabled || data.tls_enabled ? "bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400" : "bg-slate-100 dark:bg-slate-600 text-slate-500 dark:text-slate-400"}`}>
                  <i className={`fa-solid ${data.TLSEnabled || data.tls_enabled ? "fa-lock" : "fa-unlock"} mr-1`}></i>
                  {data.TLSEnabled || data.tls_enabled ? "Enabled" : "Disabled"}
                </span>
              </div>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">TCP Transport</label>
              <div className="h-11 flex items-center gap-3">
                <Toggle checked={form.tcp_enabled} onChange={(v) => setForm({ ...form, tcp_enabled: v })} />
                <span className="text-sm text-slate-600 dark:text-slate-400">{form.tcp_enabled ? "Enabled" : "Disabled"}</span>
              </div>
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">TCP Address</label>
              <input type="text" placeholder="0.0.0.0:4444" value={form.tcp_addr} onChange={(e) => setForm({ ...form, tcp_addr: e.target.value })} className={`${inputCls} font-mono`} />
            </div>
          </div>
          <div className="border-t border-[var(--border)] pt-4 mt-2">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Offline Threshold (sec)</label>
                <input type="number" min={5} max={3600} value={form.offline_threshold} onChange={(e) => setForm({ ...form, offline_threshold: Number(e.target.value) })} className={inputCls} />
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Session Timeout (hours)</label>
                <input type="number" min={1} max={720} value={form.session_max_age} onChange={(e) => setForm({ ...form, session_max_age: Number(e.target.value) })} className={inputCls} />
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Cleanup Retention (days)</label>
                <input type="number" min={1} max={365} value={form.cleanup_retention} onChange={(e) => setForm({ ...form, cleanup_retention: Number(e.target.value) })} className={inputCls} />
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <button type="submit" disabled={saving} className="h-11 px-6 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
              <i className="fa-solid fa-save mr-2"></i>Save Server Config
            </button>
            <span className="text-xs text-slate-400 dark:text-slate-500"><i className="fa-solid fa-info-circle mr-1"></i>Some changes require restart</span>
          </div>
        </form>
      </div>
    </section>
  );
}
