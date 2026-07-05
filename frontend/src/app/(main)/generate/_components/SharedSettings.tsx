import { Listener, ProfilePreset, SharedState } from "./types";
import ListenerModal from "./ListenerModal";

export default function SharedSettings({
  listeners,
  shared,
  profilePresets,
  profileLocked,
  showListenerModal,
  listenerForm,
  setShared,
  changeProfile,
  handleCreateListener,
  submitListener,
  setShowListenerModal,
  setListenerForm,
}: {
  listeners: Listener[];
  shared: SharedState;
  profilePresets: ProfilePreset[];
  profileLocked: boolean;
  showListenerModal: boolean;
  listenerForm: { name: string; ltype: string; host: string; port: string; proto: string };
  setShared: React.Dispatch<React.SetStateAction<SharedState>>;
  changeProfile: (profile: string) => void;
  handleCreateListener: () => void;
  submitListener: () => void;
  setShowListenerModal: React.Dispatch<React.SetStateAction<boolean>>;
  setListenerForm: React.Dispatch<React.SetStateAction<{ name: string; ltype: string; host: string; port: string; proto: string }>>;
}) {
  return (
    <div className="ui-card p-6 mb-6 shadow-sm">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-5">
        <div className="flex items-center gap-x-3">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/40 rounded-xl flex items-center justify-center shrink-0">
            <i className="fa-solid fa-sliders text-indigo-600 dark:text-indigo-400"></i>
          </div>
          <div>
            <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">Shared Settings</div>
            <div className="text-xs text-slate-500 dark:text-slate-400">Select listener and configure common parameters</div>
          </div>
        </div>
        <button type="button" onClick={handleCreateListener} className="px-4 h-9 bg-emerald-600 hover:bg-emerald-700 text-white text-xs rounded-xl font-medium flex items-center justify-center gap-x-1.5 shrink-0">
          <i className="fa-solid fa-plus"></i> Create Listener
        </button>
      </div>

      <ListenerModal
        show={showListenerModal}
        form={listenerForm}
        onChange={setListenerForm}
        onSubmit={submitListener}
        onClose={() => setShowListenerModal(false)}
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">Listener *</label>
          <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 rounded-xl px-3 h-10 text-sm cursor-pointer transition-all" value={shared.listener_id} onChange={(e) => setShared({ ...shared, listener_id: e.target.value })}>
            <option value="" disabled>-- Select Listener --</option>
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
          <label className="block text-xs font-semibold text-slate-600 mb-1.5">C2 URL (auto)</label>
          <input type="text" readOnly className="w-full bg-slate-100 dark:bg-slate-700 border border-slate-200 dark:border-slate-600 rounded-xl px-3 h-10 text-xs font-mono text-slate-500 dark:text-slate-300" value={shared.c2_url} />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Malleable Profile</label>
          <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 rounded-xl px-3 h-10 text-sm cursor-pointer transition-all" value={shared.profile || "default"} onChange={(e) => changeProfile(e.target.value)}>
            {profilePresets.map((p) => (
              <option key={p.name} value={p.name}>{p.description || p.name}</option>
            ))}
            <option value="__import__">Import Custom Profile...</option>
          </select>
          {profileLocked && (
            <p className="mt-1.5 text-[11px] text-amber-600 dark:text-amber-400">
              <i className="fa-solid fa-lock mr-1"></i>Profile locked
            </p>
          )}
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Heartbeat (sec)</label>
          <input type="number" min={0} max={300} disabled={profileLocked} className={`w-full border rounded-xl px-3 h-10 text-sm transition-all ${profileLocked ? "bg-slate-100 dark:bg-slate-600 opacity-60 cursor-not-allowed border-[var(--border)]" : "bg-slate-50 dark:bg-slate-700 border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"}`} value={shared.interval} onChange={(e) => setShared({ ...shared, interval: e.target.value })} />
          <p className="text-[10px] text-slate-400 dark:text-slate-500 mt-1">0 = real-time</p>
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Jitter (%)</label>
          <input type="number" disabled={profileLocked} className={`w-full border rounded-xl px-3 h-10 text-sm transition-all ${profileLocked ? "bg-slate-100 dark:bg-slate-600 opacity-60 cursor-not-allowed border-[var(--border)]" : "bg-slate-50 dark:bg-slate-700 border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"}`} value={shared.jitter} onChange={(e) => setShared({ ...shared, jitter: e.target.value })} />
        </div>
        <div className="col-span-2 lg:col-span-1">
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">User-Agent</label>
          <input type="text" disabled={profileLocked} className={`w-full border rounded-xl px-3 h-10 text-xs font-mono transition-all ${profileLocked ? "bg-slate-100 dark:bg-slate-600 opacity-60 cursor-not-allowed border-[var(--border)]" : "bg-slate-50 dark:bg-slate-700 border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"}`} value={shared.ua} onChange={(e) => setShared({ ...shared, ua: e.target.value })} />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">HTTP Proxy</label>
          <input type="text" placeholder="http://proxy:8080" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 rounded-xl px-3 h-10 text-xs font-mono transition-all" value={shared.proxy} onChange={(e) => setShared({ ...shared, proxy: e.target.value })} />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Failover C2 (comma-sep)</label>
          <input type="text" placeholder="http://backup1:8080,http://backup2:8080" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 rounded-xl px-3 h-10 text-xs font-mono transition-all" value={shared.failover} onChange={(e) => setShared({ ...shared, failover: e.target.value })} />
        </div>
        <div>
          <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Crypto Key (32-byte Hex, opt)</label>
          <input type="text" placeholder="a1b2c3d4..." className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 rounded-xl px-3 h-10 text-xs font-mono transition-all" value={shared.crypto_key} onChange={(e) => setShared({ ...shared, crypto_key: e.target.value })} />
        </div>
      </div>
    </div>
  );
}
