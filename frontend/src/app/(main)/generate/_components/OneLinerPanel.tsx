import { useState } from "react";
import { Listener, OneLinerForm, OneLinerType, OneLinerData } from "./types";
import { sanitizeHtml } from "@/lib/sanitize";

export default function OneLinerPanel({
  form, setForm, busy, result, onelinerData, listeners, getListenerInfo, onGenerate, onCopy,
}: {
  form: OneLinerForm;
  setForm: React.Dispatch<React.SetStateAction<OneLinerForm>>;
  busy: boolean;
  result: string;
  onelinerData?: OneLinerData;
  listeners: Listener[];
  getListenerInfo: (id: string) => { scheme: string; host: string; port: string | number; type: string; name: string } | null;
  onGenerate: () => void;
  onCopy: (text: string) => void;
}) {
  return (
    <div className="mt-8">
      <div className="flex items-center gap-x-3 mb-5">
        <div className="w-10 h-10 bg-rose-100 rounded-xl flex items-center justify-center">
          <i className="fa-solid fa-terminal text-rose-600"></i>
        </div>
        <div>
          <div className="text-sm font-semibold text-slate-900">One-Liner</div>
          <div className="text-xs text-slate-500">Generate 10+ one-liner commands with remote hosting</div>
        </div>
      </div>
      <div className="ui-card p-6 shadow-sm hover:shadow-md transition-shadow">
        <div className="space-y-4">
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Payload Type</label>
              <select value={form.payload_type} onChange={(e) => setForm({ ...form, payload_type: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm">
                <option value="exe">Windows EXE</option>
                <option value="ps1">PowerShell</option>
                <option value="linux">Linux ELF</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Listener</label>
              <select value={form.listener_id} onChange={(e) => {
                const info = getListenerInfo(e.target.value);
                let c2url = "";
                let protocol = "http";
                if (info) {
                  c2url = `${info.scheme}://${info.host}:${info.port}`;
                  if (info.scheme === "tcp" || info.scheme === "tls") protocol = "tcp";
                  else if (info.scheme === "dns" || info.type === "dns") protocol = "dns";
                }
                setForm({ ...form, listener_id: e.target.value, c2_url: c2url, protocol });
              }} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm">
                <option value="" disabled>-- Select --</option>
                {listeners.map((l, i) => {
                  const id = l.id || l.ID || String(i);
                  const name = l.name || l.Name || "Unknown";
                  const scheme = l.scheme || l.Scheme || l.type || l.Type || "http";
                  const host = l.host || l.Host || "";
                  const port = l.port || l.Port || "";
                  return <option key={id} value={id}>{name} ({scheme}://{host}:{port})</option>;
                })}
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Heartbeat (sec)</label>
              <input type="number" value={form.beacon_time} onChange={(e) => setForm({ ...form, beacon_time: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Jitter (%)</label>
              <input type="number" value={form.jitter} onChange={(e) => setForm({ ...form, jitter: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm" />
            </div>
          </div>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="flex items-center gap-x-2">
              <input type="checkbox" checked={form.skip_tls} onChange={(e) => setForm({ ...form, skip_tls: e.target.checked })} id="skip-tls-oneliner" className="w-4 h-4 accent-indigo-600" />
              <label htmlFor="skip-tls-oneliner" className="text-sm text-slate-700">Skip TLS</label>
            </div>
            <div className="flex items-center gap-x-2">
              <input type="checkbox" checked={form.persist} onChange={(e) => setForm({ ...form, persist: e.target.checked })} id="persist-oneliner" className="w-4 h-4 accent-indigo-600" />
              <label htmlFor="persist-oneliner" className="text-sm text-[var(--text-secondary)]">Persist</label>
            </div>
          </div>
          <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-rose-600 hover:bg-rose-700 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
            {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-bolt"></i> Generate One-Liners</>}
          </button>
        </div>
        {result === "success" && onelinerData ? (
          <div className="mt-4">
            <div className="text-xs text-emerald-600 mb-3 flex items-center gap-x-2">
              <i className="fa-solid fa-check-circle"></i>
              Download URL <code className="text-xs bg-slate-100 px-2 py-0.5 rounded">{onelinerData.download_url}</code> (valid 1hr)
            </div>
            <div className="space-y-2">
              {onelinerData.types.map((item: OneLinerType, idx: number) => (
                <div key={idx} className="border border-slate-200 rounded-2xl p-3 hover:border-rose-200 transition-colors">
                  <div className="flex items-center justify-between mb-1.5">
                    <div>
                      <span className="text-sm font-medium text-slate-800">{item.name}</span>
                      <span className="text-[10px] text-slate-400 ml-2">{item.desc}</span>
                    </div>
                    <button onClick={() => onCopy(item.command)} className="text-xs px-2.5 py-1 bg-slate-100 hover:bg-rose-100 rounded-xl text-slate-600">
                      <i className="fa-regular fa-copy mr-1"></i>Copy
                    </button>
                  </div>
                  <code className="block text-[11px] font-mono bg-slate-50 text-slate-700 p-2 rounded-xl whitespace-pre-wrap break-all leading-relaxed select-all">{item.command}</code>
                </div>
              ))}
            </div>
          </div>
        ) : result ? (
          <div className="mt-4" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />
        ) : null}
      </div>
    </div>
  );
}
