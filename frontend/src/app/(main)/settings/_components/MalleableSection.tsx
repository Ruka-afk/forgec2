import { MalleableForm } from "./types";
import Toggle from "./Toggle";

export default function MalleableSection({
  form, setForm, saving, inputCls, textareaCls, onSave,
}: {
  form: MalleableForm;
  setForm: React.Dispatch<React.SetStateAction<MalleableForm>>;
  saving: boolean;
  inputCls: string;
  textareaCls: string;
  onSave: (e: React.FormEvent) => void;
}) {
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-violet-600 to-violet-800 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-shield text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Malleable C2 Profile</h2><p className="text-xs text-violet-200">Customize beacon traffic characteristics</p></div>
        </div>
      </div>
      <div className="p-6">
        <form onSubmit={onSave} className="space-y-4">
          <div className="flex items-center gap-3">
            <Toggle checked={form.enabled} onChange={(v) => setForm({ ...form, enabled: v })} />
            <span className="text-sm text-slate-600 dark:text-slate-400">{form.enabled ? "Enabled" : "Disabled"}</span>
            <span className="text-xs text-slate-400">Override default JSON response format</span>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">HTTP Status Code</label>
              <input type="number" min={100} max={599} value={form.status_code} onChange={(e) => setForm({ ...form, status_code: Number(e.target.value) })} className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Content-Type</label>
              <input type="text" placeholder="application/json" value={form.content_type} onChange={(e) => setForm({ ...form, content_type: e.target.value })} className={`${inputCls} font-mono`} />
            </div>
          </div>
          <div>
            <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Custom Headers (one per line)</label>
            <textarea rows={3} value={form.headers_text} onChange={(e) => setForm({ ...form, headers_text: e.target.value })} placeholder={"Server: nginx/1.24.0\nX-Powered-By: ASP.NET"} className={textareaCls}></textarea>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Prepend Content</label>
              <textarea rows={2} value={form.prepend} onChange={(e) => setForm({ ...form, prepend: e.target.value })} placeholder="<html><body><!--" className={textareaCls}></textarea>
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5">Append Content</label>
              <textarea rows={2} value={form.append} onChange={(e) => setForm({ ...form, append: e.target.value })} placeholder="--></body></html>" className={textareaCls}></textarea>
            </div>
          </div>
          <div className="p-3 bg-amber-50 dark:bg-amber-900/20 rounded-xl border border-amber-200 dark:border-amber-800 text-xs text-amber-700 dark:text-amber-400">
            <i className="fa-solid fa-triangle-exclamation mr-1"></i>
            Enabling profile requires compatible agents. Prepend/append is for traffic camouflage only.
          </div>
          <button type="submit" disabled={saving} className="h-11 px-6 bg-violet-600 hover:bg-violet-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <i className="fa-solid fa-save mr-2"></i>Save Profile
          </button>
        </form>
      </div>
    </section>
  );
}
