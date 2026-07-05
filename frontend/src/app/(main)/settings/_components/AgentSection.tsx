import { AgentForm } from "./types";
import Toggle from "./Toggle";

export default function AgentSection({
  form, setForm, saving, inputCls, onSave,
}: {
  form: AgentForm;
  setForm: React.Dispatch<React.SetStateAction<AgentForm>>;
  saving: boolean;
  inputCls: string;
  onSave: (e: React.FormEvent) => void;
}) {
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-emerald-600 to-emerald-700 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-robot text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Implant Default Config</h2><p className="text-xs text-emerald-200">Default values for new implants</p></div>
        </div>
      </div>
      <div className="p-6">
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Heartbeat Interval (sec)</label>
              <input type="number" min={0} max={300} value={form.interval} onChange={(e) => setForm({ ...form, interval: Number(e.target.value) })} className={inputCls} />
              <p className="text-[10px] text-slate-400 dark:text-slate-500 mt-1">0 = real-time mode</p>
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Jitter (%)</label>
              <input type="number" min={0} max={100} value={form.jitter} onChange={(e) => setForm({ ...form, jitter: Number(e.target.value) })} className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Skip TLS Verification</label>
              <div className="h-11 flex items-center gap-3">
                <Toggle checked={form.skip_tls} onChange={(v) => setForm({ ...form, skip_tls: v })} />
                <span className="text-sm text-slate-600 dark:text-slate-400">{form.skip_tls ? "Enabled" : "Disabled"}</span>
              </div>
            </div>
          </div>
          <div>
            <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Default User-Agent</label>
            <input type="text" value={form.user_agent} onChange={(e) => setForm({ ...form, user_agent: e.target.value })} className={`${inputCls} font-mono`} />
          </div>
          <button type="submit" disabled={saving} className="h-11 px-6 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <i className="fa-solid fa-save mr-2"></i>Save Agent Config
          </button>
        </form>
      </div>
    </section>
  );
}
