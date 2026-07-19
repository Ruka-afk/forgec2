import { useState } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { ConfirmModal, Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Listener, ProfilePreset, SharedState } from "./types";
import ListenerModal from "./ListenerModal";
import { Lock, Plus, SlidersHorizontal, Trash2 } from "lucide-react";

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
  onProfileDeleted,
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
  onProfileDeleted?: (name: string) => void;
}) {
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [deleting, setDeleting] = useState(false);
  const currentProfile = profilePresets.find(p => p.name === shared.profile);
  const isBuiltin = !shared.profile || shared.profile === "default" || shared.profile === "";

  const handleDeleteProfile = () => {
    if (!shared.profile || isBuiltin) return;
    setCfm({msg: `Delete profile "${shared.profile}"? This cannot be undone.`, cb: async () => {
      setDeleting(true);
      try {
        await onProfileDeleted?.(shared.profile);
        changeProfile("default");
      } catch { toast.error("Failed to delete profile"); }
      setDeleting(false);
    }});
  };

  return (
    <Card className="p-4 sm:p-5 mb-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-5">
        <div className="flex items-center gap-x-3">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/40 rounded-xl flex items-center justify-center shrink-0">
            <SlidersHorizontal className="w-4 h-4" />
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground">Shared Settings</div>
            <div className="text-xs text-muted-foreground">Select listener and configure common parameters</div>
          </div>
        </div>
        <Button type="button" onClick={handleCreateListener} className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs rounded-xl font-medium flex items-center justify-center gap-x-1.5 shrink-0">
          <Plus className="w-4 h-4" /> Create Listener
        </Button>
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
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Listener *</span>
          <Select value={shared.listener_id} onValueChange={(val) => val != null && setShared(s => ({ ...s, listener_id: val }))}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="-- Select Listener --" />
            </SelectTrigger>
            <SelectContent>
              {listeners.map((l, i) => {
                const id = String(l.id ?? i);
                const name = l.name || "Unknown";
                const scheme = l.scheme || l.type || "http";
                const host = l.host || "";
                const port = l.port || "";
                return <SelectItem key={id} value={id}>{name} ({scheme}://{host}:{port})</SelectItem>;
              })}
            </SelectContent>
          </Select>
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">C2 URL (auto)</span>
          <Input aria-label="C2 URL" name="input-1" type="text" readOnly value={shared.c2_url} className="font-mono text-xs" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Malleable Profile</span>
          <div className="flex gap-1.5">
            <Select value={shared.profile || "default"} onValueChange={(val) => val != null && changeProfile(val)}>
              <SelectTrigger className="flex-1">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {profilePresets.map((p) => (
                  <SelectItem key={p.name} value={p.name}>{p.description || p.name}</SelectItem>
                ))}
                <SelectItem value="__import__">Import Custom Profile...</SelectItem>
              </SelectContent>
            </Select>
            {!isBuiltin && (
              <Button type="button" variant="destructive" size="icon" onClick={handleDeleteProfile} disabled={deleting} title="Delete profile" aria-label="Delete profile">
                {deleting ? <Spinner size="xs" /> : <Trash2 className="w-4 h-4" />}
              </Button>
            )}
          </div>
          {profileLocked && (
            <p className="mt-1.5 text-[11px] text-amber-600 dark:text-amber-400">
              <Lock className="w-4 h-4" />Profile locked
            </p>
          )}
          {currentProfile?.description && (
            <p className="mt-1 text-[10px] text-muted-foreground">{currentProfile.description}</p>
          )}
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Heartbeat (sec)</span>
          <Input aria-label="Sleep interval in seconds" name="input-3" type="number" min={1} max={86400} disabled={profileLocked} value={shared.interval} onChange={(e) => { const v = Math.max(1, Math.min(86400, Number(e.target.value) || 1)); setShared(s => ({ ...s, interval: String(v) })); }} />
          <p className="text-[10px] text-muted-foreground mt-1">1-86400 sec</p>
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Jitter (%)</span>
          <Input aria-label="Jitter percentage" name="input-4" type="number" min={0} max={100} disabled={profileLocked} value={shared.jitter} onChange={(e) => { const v = Math.max(0, Math.min(100, Number(e.target.value) || 0)); setShared(s => ({ ...s, jitter: String(v) })); }} />
        </div>
        <div className="col-span-2 lg:col-span-1">
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">User-Agent</span>
          <Input aria-label="User-Agent" name="input-5" type="text" disabled={profileLocked} value={shared.ua} onChange={(e) => setShared(s => ({ ...s, ua: e.target.value }))} className="text-xs font-mono" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">HTTP Proxy</span>
          <Input aria-label="http://proxy:8080" name="http-proxy-8080-6" type="text" placeholder="http://proxy:8080" value={shared.proxy} onChange={(e) => setShared(s => ({ ...s, proxy: e.target.value }))} className="text-xs font-mono" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Failover C2 (comma-sep)</span>
          <Input aria-label="http://backup1:8080,http://backup2:8080" name="http-backup1-8080-http-backup2-8080-7" type="text" placeholder="http://backup1:8080,http://backup2:8080" value={shared.failover} onChange={(e) => setShared(s => ({ ...s, failover: e.target.value }))} className="text-xs font-mono" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Crypto Key (32-byte Hex, opt)</span>
          <Input aria-label="Encryption key (64 hex characters)" name="a1b2c3d4-8" type="text" placeholder="a1b2c3d4..." value={shared.crypto_key} onChange={(e) => setShared(s => ({ ...s, crypto_key: e.target.value }))} className="text-xs font-mono" />
        </div>
      </div>
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Confirm" cancelText="Cancel" danger onConfirm={async () => { await cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </Card>
  );
}


