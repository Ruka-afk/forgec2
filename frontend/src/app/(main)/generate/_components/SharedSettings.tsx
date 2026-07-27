import { useState } from "react";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { ConfirmModal, Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Listener, ProfilePreset, SharedState } from "./types";
import ListenerModal from "./ListenerModal";
import { Lock, Plus, SlidersHorizontal, Trash2 } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

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
  const { t } = useI18n();
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [deleting, setDeleting] = useState(false);
  const currentProfile = profilePresets.find(p => p.name === shared.profile);
  const isBuiltin = !shared.profile || shared.profile === "default" || shared.profile === "";

  const handleDeleteProfile = () => {
    if (!shared.profile || isBuiltin) return;
    setCfm({msg: t("generate.confirm_delete_profile", { name: shared.profile }), cb: async () => {
      setDeleting(true);
      try {
        await onProfileDeleted?.(shared.profile);
        changeProfile("default");
      } catch { toast.error(t("generate.toast.delete_profile_failed")); }
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
            <div className="text-sm font-semibold text-foreground">{t("generate.shared_title")}</div>
            <div className="text-xs text-muted-foreground">{t("generate.shared_subtitle")}</div>
          </div>
        </div>
        <Button type="button" onClick={handleCreateListener} className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs rounded-xl font-medium flex items-center justify-center gap-x-1.5 shrink-0">
          <Plus className="w-4 h-4" /> {t("generate.create_listener")}
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
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.listener_required")}</span>
          <Select value={shared.listener_id} onValueChange={(val) => val != null && setShared(s => ({ ...s, listener_id: val }))}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("generate.select_listener")} />
            </SelectTrigger>
            <SelectContent>
              {listeners.map((l, i) => {
                const id = String(l.id ?? i);
                const name = l.name || t("generate.unknown_listener");
                const scheme = l.scheme || l.type || "http";
                const host = l.host || "";
                const port = l.port || "";
                return <SelectItem key={id} value={id}>{name} ({scheme}://{host}:{port})</SelectItem>;
              })}
            </SelectContent>
          </Select>
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.c2_url_auto")}</span>
          <Input aria-label={t("generate.c2_url_auto")} name="input-1" type="text" readOnly value={shared.c2_url} className="font-mono text-xs" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.beacon_transport")}</span>
          <Select value={shared.beacon_transport || "http"} onValueChange={(val) => {
            if (val == null) return;
            const proto = ["tcp", "dns", "icmp"].includes(val) ? val : "http";
            setShared(s => ({ ...s, beacon_transport: val, protocol: proto }));
          }}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="http">HTTP(S)</SelectItem>
              <SelectItem value="wss">WebSocket (WSS)</SelectItem>
              <SelectItem value="grpc">gRPC</SelectItem>
              <SelectItem value="ssh">SSH</SelectItem>
              <SelectItem value="tcp">TCP</SelectItem>
              <SelectItem value="dns">DNS</SelectItem>
              <SelectItem value="icmp">ICMP</SelectItem>
              <SelectItem value="mtls">mTLS</SelectItem>
              <SelectItem value="h2c">HTTP/2 cleartext</SelectItem>
            </SelectContent>
          </Select>
          <p className="text-(--font-size-micro-sm) text-muted-foreground mt-1">{t("generate.beacon_transport_hint")}</p>
        </div>
        {["grpc", "ssh", "wss", "mtls", "h2c", "icmp"].includes(shared.beacon_transport || "") && (
          <div className="sm:col-span-2 lg:col-span-4 rounded-xl border border-warning/30 bg-warning/10 px-3 py-2 text-xs text-warning-foreground">
            <div className="font-semibold mb-0.5">{t("generate.transport_security_title")}</div>
            <div>
              {shared.beacon_transport === "grpc" && t("generate.transport_note_grpc")}
              {shared.beacon_transport === "ssh" && t("generate.transport_note_ssh")}
              {shared.beacon_transport === "wss" && t("generate.transport_note_wss")}
              {shared.beacon_transport === "mtls" && t("generate.transport_note_mtls")}
              {shared.beacon_transport === "h2c" && t("generate.transport_note_h2c")}
              {shared.beacon_transport === "icmp" && t("generate.transport_note_icmp")}
            </div>
          </div>
        )}
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.malleable_profile")}</span>
          <div className="flex gap-1.5">
            <Select value={shared.profile || "default"} onValueChange={(val) => val != null && changeProfile(val)}>
              <SelectTrigger className="flex-1">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {profilePresets.map((p) => (
                  <SelectItem key={p.name} value={p.name}>{p.description || p.name}</SelectItem>
                ))}
                <SelectItem value="__import__">{t("generate.import_profile")}</SelectItem>
              </SelectContent>
            </Select>
            {!isBuiltin && (
              <Tooltip>
                <TooltipTrigger render={<Button type="button" variant="destructive" size="icon" onClick={handleDeleteProfile} disabled={deleting} aria-label={t("generate.delete_profile")} />}>
                  {deleting ? <Spinner size="xs" /> : <Trash2 className="w-4 h-4" />}
                </TooltipTrigger>
                <TooltipContent>{t("generate.delete_profile")}</TooltipContent>
              </Tooltip>
            )}
          </div>
          {profileLocked && (
            <p className="mt-1.5 text-(--font-size-xs-sm) text-amber-600 dark:text-amber-400 flex items-center gap-1">
              <Lock className="w-4 h-4" />{t("generate.profile_locked")}
            </p>
          )}
          {currentProfile?.description && (
            <p className="mt-1 text-(--font-size-micro-sm) text-muted-foreground">{currentProfile.description}</p>
          )}
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.heartbeat_sec")}</span>
          <Input aria-label={t("generate.heartbeat_sec")} name="input-3" type="number" min={1} max={86400} disabled={profileLocked} value={shared.interval} onChange={(e) => { const v = Math.max(1, Math.min(86400, Number(e.target.value) || 1)); setShared(s => ({ ...s, interval: String(v) })); }} />
          <p className="text-(--font-size-micro-sm) text-muted-foreground mt-1">{t("generate.heartbeat_range")}</p>
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.jitter_pct")}</span>
          <Input aria-label={t("generate.jitter_pct")} name="input-4" type="number" min={0} max={100} disabled={profileLocked} value={shared.jitter} onChange={(e) => { const v = Math.max(0, Math.min(100, Number(e.target.value) || 0)); setShared(s => ({ ...s, jitter: String(v) })); }} />
        </div>
        {(shared.beacon_transport === "dns" || shared.protocol === "dns") && (
          <>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.doh_url")}</span>
              <Input aria-label={t("generate.doh_url")} name="dns-doh" type="text" value={shared.dns_doh_url} onChange={(e) => setShared(s => ({ ...s, dns_doh_url: e.target.value }))} placeholder="https://dns.google/dns-query" className="font-mono text-xs" />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.dot_addr")}</span>
              <Input aria-label={t("generate.dot_addr")} name="dns-dot" type="text" value={shared.dns_dot_addr} onChange={(e) => setShared(s => ({ ...s, dns_dot_addr: e.target.value }))} placeholder="1.1.1.1:853" className="font-mono text-xs" />
            </div>
          </>
        )}
        {shared.beacon_transport === "ssh" && (
          <>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.ssh_user")}</span>
              <Input aria-label={t("generate.ssh_user")} name="ssh-user" type="text" value={shared.ssh_user} onChange={(e) => setShared(s => ({ ...s, ssh_user: e.target.value }))} className="font-mono text-xs" />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.ssh_password")}</span>
              <Input aria-label={t("generate.ssh_password")} name="ssh-pass" type="password" value={shared.ssh_password} onChange={(e) => setShared(s => ({ ...s, ssh_password: e.target.value }))} className="font-mono text-xs" />
            </div>
            <div className="sm:col-span-2">
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.ssh_key")}</span>
              <Input aria-label={t("generate.ssh_key")} name="ssh-key" type="text" value={shared.ssh_key} onChange={(e) => setShared(s => ({ ...s, ssh_key: e.target.value }))} className="font-mono text-xs" />
            </div>
            <div className="sm:col-span-2">
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.ssh_host_key")}</span>
              <Input
                aria-label={t("generate.ssh_host_key")}
                name="ssh-host-key"
                type="text"
                value={shared.ssh_host_key || ""}
                onChange={(e) => setShared(s => ({ ...s, ssh_host_key: e.target.value }))}
                placeholder={t("generate.ssh_host_key_placeholder")}
                className="font-mono text-xs"
              />
              <p className="text-(--font-size-micro-sm) text-muted-foreground mt-1">{t("generate.ssh_host_key_hint")}</p>
            </div>
          </>
        )}
        <div className="col-span-2 lg:col-span-1">
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.user_agent")}</span>
          <Input aria-label={t("generate.user_agent")} name="input-5" type="text" disabled={profileLocked} value={shared.ua} onChange={(e) => setShared(s => ({ ...s, ua: e.target.value }))} className="text-xs font-mono" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.http_proxy")}</span>
          <Input aria-label={t("generate.http_proxy")} name="http-proxy-8080-6" type="text" placeholder="http://proxy:8080" value={shared.proxy} onChange={(e) => setShared(s => ({ ...s, proxy: e.target.value }))} className="text-xs font-mono" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.failover_c2")}</span>
          <Input aria-label={t("generate.failover_c2")} name="http-backup1-8080-http-backup2-8080-7" type="text" placeholder="http://backup1:8080,http://backup2:8080" value={shared.failover} onChange={(e) => setShared(s => ({ ...s, failover: e.target.value }))} className="text-xs font-mono" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("generate.crypto_key")}</span>
          <Input aria-label={t("generate.crypto_key")} name="a1b2c3d4-8" type="text" placeholder="a1b2c3d4..." value={shared.crypto_key} onChange={(e) => setShared(s => ({ ...s, crypto_key: e.target.value }))} className="text-xs font-mono" />
        </div>
      </div>
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.confirm")} cancelText={t("common.cancel")} danger onConfirm={async () => { await cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </Card>
  );
}


