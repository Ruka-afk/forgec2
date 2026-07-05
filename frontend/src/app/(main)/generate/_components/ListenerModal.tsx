interface ListenerForm {
  name: string;
  ltype: string;
  host: string;
  port: string;
  proto: string;
}

export default function ListenerModal({
  show,
  form,
  onChange,
  onSubmit,
  onClose,
}: {
  show: boolean;
  form: ListenerForm;
  onChange: (f: ListenerForm) => void;
  onSubmit: () => void;
  onClose: () => void;
}) {
  if (!show) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div className="bg-[var(--card-bg)] rounded-2xl p-6 w-full max-w-sm shadow-xl border border-[var(--border)] mx-4" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-base font-semibold text-slate-900 dark:text-slate-100 mb-4">New Listener</h3>
        <div className="space-y-3">
          <div>
            <label className="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">Name</label>
            <input type="text" value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })}
              className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 h-9 text-sm" />
          </div>
          <div>
            <label className="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">Type</label>
            <select value={form.ltype} onChange={(e) => onChange({ ...form, ltype: e.target.value })}
              className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 h-9 text-sm">
              <option value="http">http</option>
              <option value="tcp">tcp</option>
              <option value="dns">dns</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">Domain/IP</label>
            <input type="text" value={form.host} onChange={(e) => onChange({ ...form, host: e.target.value })}
              className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 h-9 text-sm" />
          </div>
          <div>
            <label className="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">Port</label>
            <input type="number" value={form.port} onChange={(e) => onChange({ ...form, port: e.target.value })}
              className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 h-9 text-sm" />
          </div>
          {form.ltype !== "dns" && (
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">Protocol</label>
              <input type="text" value={form.proto} onChange={(e) => onChange({ ...form, proto: e.target.value })}
                className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 h-9 text-sm" placeholder="http/https/tcp/tls" />
            </div>
          )}
        </div>
        <div className="flex items-center justify-end gap-2 mt-5">
          <button type="button" onClick={onClose} className="px-4 h-9 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-[var(--text-secondary)] text-xs font-medium rounded-xl transition-colors">Cancel</button>
          <button type="button" onClick={onSubmit} className="px-4 h-9 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-medium rounded-xl transition-colors">Confirm</button>
        </div>
      </div>
    </div>
  );
}
