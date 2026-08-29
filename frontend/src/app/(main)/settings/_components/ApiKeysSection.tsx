"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { timeAgo, formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Banner } from "@/components/ui/banner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from "@/components/ui/dialog";
import { CopyButton } from "@/components/ui/copy-button";
import { KeyRound, Plus, RotateCw, Trash2 } from "lucide-react";

interface ApiKeyEntry {
  id: number;
  name: string;
  prefix: string;
  last_used?: string;
  expires_at?: string;
  active: boolean;
  created_at: string;
}

type ApiKeyRow = ApiKeyEntry & { expired: boolean };

interface CreatedKey {
  id?: number;
  name?: string;
  key: string;
}

export default function ApiKeysSection() {
  const { t } = useI18n();
  const { confirm, modal } = useConfirm();
  const [keys, setKeys] = useState<ApiKeyRow[]>([]);
  const [loading, setLoading] = useState(true);

  // create dialog
  const [createOpen, setCreateOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [newExpiresDays, setNewExpiresDays] = useState("");
  const [creating, setCreating] = useState(false);

  // one-time plaintext display
  const [createdKey, setCreatedKey] = useState<CreatedKey | null>(null);

  const load = useCallback(async () => {
    try {
      // api.get auto-unwraps {success,data} — the value IS the array.
      const d = await api.get<ApiKeyEntry[] | { data?: ApiKeyEntry[] }>(paths.settings.apiKeys);
      const list = Array.isArray(d) ? d : d.data || [];
      // Precompute expiry against a single load-time timestamp so the render
      // loop stays pure.
      const now = Date.now();
      setKeys(list.map((k) => ({
        ...k,
        expired: !!k.expires_at && new Date(k.expires_at).getTime() < now,
      })));
    } catch {
      setKeys([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const handleCreate = async () => {
    if (!newName.trim()) { toast.error(t("settings.apikeys.toast_name_required")); return; }
    setCreating(true);
    try {
      const body: Record<string, string> = { name: newName.trim() };
      if (newExpiresDays) {
        const days = Number(newExpiresDays);
        if (!Number.isFinite(days) || days <= 0) {
          toast.error(t("settings.apikeys.toast_bad_expiry"));
          setCreating(false);
          return;
        }
        body.expires_at = new Date(Date.now() + days * 86_400_000).toISOString();
      }
      // postJson unwraps {success,data}: the payload (with the show-once
      // key) arrives directly — checking d.data?.key here meant the one-time
      // secret was never displayed anywhere.
      const d = await api.postJson<CreatedKey | { data?: CreatedKey }>(paths.settings.apiKeys, body);
      setCreateOpen(false);
      setNewName("");
      setNewExpiresDays("");
      const created = (d as CreatedKey).key ? (d as CreatedKey) : (d as { data?: CreatedKey }).data;
      if (created?.key) {
        setCreatedKey(created);
      }
      void load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("settings.apikeys.toast_create_failed"));
    } finally {
      setCreating(false);
    }
  };

  const handleRevoke = async (k: ApiKeyEntry) => {
    if (!(await confirm({ message: t("settings.apikeys.confirm_revoke", { name: k.name }) }))) return;
    try {
      await api.del(paths.settings.apiKey(k.id));
      toast.success(t("settings.apikeys.toast_revoked"));
      void load();
    } catch {
      toast.error(t("settings.apikeys.toast_revoke_failed"));
    }
  };

  const handleRotate = async (k: ApiKeyEntry) => {
    if (!(await confirm({ message: t("settings.apikeys.confirm_rotate", { name: k.name }) }))) return;
    try {
      const d = await api.postJson<CreatedKey | { data?: CreatedKey }>(paths.settings.apiKeyRotate(k.id), {});
      const rotated = (d as CreatedKey).key ? (d as CreatedKey) : (d as { data?: CreatedKey }).data;
      if (rotated?.key) setCreatedKey(rotated);
      toast.success(t("settings.apikeys.toast_rotated"));
      void load();
    } catch {
      toast.error(t("settings.apikeys.toast_rotate_failed"));
    }
  };

  return (
    <Card className="overflow-hidden">
      {modal}
      <CardHeaderRow icon={KeyRound} tone="info" accent={false}
        title={t("settings.apikeys.title")}
        description={t("settings.apikeys.subtitle")}
        action={
          <Button size="sm" onClick={() => setCreateOpen(true)} className="gap-1.5">
            <Plus className="size-4" /> {t("settings.apikeys.create")}
          </Button>
        }
      />
      <div className="p-(--card-spacing)">
        {loading ? (
          <div className="py-8 text-center"><Spinner /></div>
        ) : keys.length === 0 ? (
          <p className="text-xs text-muted-foreground text-center py-6">{t("settings.apikeys.empty")}</p>
        ) : (
          <div className="divide-y divide-border">
            {keys.map((k) => {
              const expired = k.expired;
              return (
                <div key={k.id} className="py-3 flex items-center gap-3 flex-wrap">
                  <div className="flex-1 min-w-[180px]">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-foreground">{k.name}</span>
                      {!k.active ? (
                        <Badge variant="secondary" className="text-(--fs-micro)">{t("settings.apikeys.revoked")}</Badge>
                      ) : expired ? (
                        <Badge variant="warning" className="text-(--fs-micro)">{t("settings.apikeys.expired")}</Badge>
                      ) : (
                        <Badge variant="success" className="text-(--fs-micro)">{t("settings.apikeys.active")}</Badge>
                      )}
                    </div>
                    <div className="text-xs text-muted-foreground font-mono mt-0.5">{k.prefix}…</div>
                  </div>
                  <div className="text-xs text-muted-foreground min-w-[120px]">
                    {t("settings.apikeys.last_used")}: {k.last_used ? timeAgo(k.last_used, t) : "—"}
                  </div>
                  <div className="text-xs text-muted-foreground min-w-[110px]">
                    {k.expires_at ? `${t("settings.apikeys.expires")}: ${formatTime(k.expires_at)}` : t("settings.apikeys.never_expires")}
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <Button variant="ghost" size="icon-sm" onClick={() => void handleRotate(k)} disabled={!k.active}
                      title={t("settings.apikeys.rotate")} aria-label={t("settings.apikeys.rotate")}
                      className="text-muted-foreground hover:text-primary">
                      <RotateCw className="size-4" />
                    </Button>
                    <Button variant="ghost" size="icon-sm" onClick={() => void handleRevoke(k)} disabled={!k.active}
                      title={t("common.delete")} aria-label={t("common.delete")}
                      className="text-muted-foreground hover:text-destructive">
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader><DialogTitle>{t("settings.apikeys.create_title")}</DialogTitle></DialogHeader>
          <div className="space-y-3">
            <div>
              <Label>{t("settings.apikeys.name_label")}</Label>
              <Input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="ci-pipeline" className="mt-1" />
            </div>
            <div>
              <Label>{t("settings.apikeys.expiry_label")}</Label>
              <Input type="number" min={1} value={newExpiresDays} onChange={(e) => setNewExpiresDays(e.target.value)}
                placeholder={t("settings.apikeys.expiry_ph")} className="mt-1" />
            </div>
            <p className="text-xs text-muted-foreground">{t("settings.apikeys.auth_hint")}</p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>{t("common.cancel")}</Button>
            <Button onClick={handleCreate} disabled={creating} className="gap-1.5">
              {creating ? <Spinner size="xs" /> : null}{t("settings.apikeys.create_btn")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* One-time plaintext display */}
      <Dialog open={!!createdKey} onOpenChange={(open) => { if (!open) setCreatedKey(null); }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>{t("settings.apikeys.created_title")}</DialogTitle></DialogHeader>
          <Banner tone="warning" className="text-xs">{t("settings.apikeys.created_warning")}</Banner>
          <div className="flex items-center gap-2">
            <code className="flex-1 text-xs font-mono bg-muted rounded-lg px-3 py-2 break-all select-all">{createdKey?.key}</code>
            <CopyButton text={createdKey?.key || ""} />
          </div>
          <p className="text-xs text-muted-foreground">{t("settings.apikeys.usage_hint")}</p>
          <DialogFooter>
            <Button onClick={() => setCreatedKey(null)}>{t("settings.apikeys.done")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
