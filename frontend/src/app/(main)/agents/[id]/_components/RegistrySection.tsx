"use client";

import { memo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { toast } from "sonner";
import { Database } from "lucide-react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import CollectCard from "./CollectCard";
import { useCollectTask } from "./useCollectTask";

interface RegistrySectionProps {
  agentId: string;
  online: boolean;
}

export default memo(function RegistrySection({ agentId, online }: RegistrySectionProps) {
  const { t } = useI18n();
  const { busy, result, collect } = useCollectTask(agentId);
  const [key, setKey] = useState("");
  const [valueName, setValueName] = useState("");
  const [valueData, setValueData] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);

  const run = async (kind: "get" | "set" | "delete") => {
    const k = key.trim();
    if (!k) {
      toast.error(t("agents.registry_key_required"));
      return;
    }
    if (kind === "get") {
      await collect("get", paths.agents.regGet(agentId), {
        body: { command: k },
        emptyText: t("agents.registry_empty"),
        errorText: t("agents.registry_failed"),
      });
    } else if (kind === "set") {
      if (!valueName.trim()) {
        toast.error(t("agents.registry_value_required"));
        return;
      }
      await collect("set", paths.agents.regSet(agentId), {
        body: { path: k, data: `${valueName.trim()}=${valueData}` },
        successText: t("agents.registry_set_ok"),
        errorText: t("agents.registry_failed"),
      });
    } else {
      await collect("delete", paths.agents.regDelete(agentId), {
        body: { command: k },
        successText: t("agents.registry_deleted"),
        errorText: t("agents.registry_failed"),
      });
    }
  };

  return (
    <>
      <CollectCard
        title={t("agents.registry_title")}
        icon={<Database className="size-3.5" />}
        emptyIcon={Database}
        emptyTitle={t("agents.registry_empty_title")}
        emptyHint={t("agents.registry_empty_hint")}
        result={result}
        resultLabel={key.trim() || undefined}
      >
        <div>
          <Label className="mb-1 block text-xs text-muted-foreground">{t("agents.registry_key")}</Label>
          <Input
            value={key}
            onChange={(e) => setKey(e.target.value)}
            placeholder={t("agents.registry_key_ph")}
            className="h-8 font-mono text-xs"
          />
        </div>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <div>
            <Label className="mb-1 block text-xs text-muted-foreground">{t("agents.registry_value")}</Label>
            <Input
              value={valueName}
              onChange={(e) => setValueName(e.target.value)}
              placeholder={t("agents.registry_value_ph")}
              className="h-8 font-mono text-xs"
            />
          </div>
          <div>
            <Label className="mb-1 block text-xs text-muted-foreground">{t("agents.registry_data")}</Label>
            <Input
              value={valueData}
              onChange={(e) => setValueData(e.target.value)}
              placeholder={t("agents.registry_data_ph")}
              className="h-8 font-mono text-xs"
            />
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" disabled={!online || busy !== null || !key.trim()} onClick={() => void run("get")}>
            {busy === "get" && <Spinner size="xs" />}
            {t("agents.registry_get")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={!online || busy !== null || !key.trim() || !valueName.trim()}
            onClick={() => void run("set")}
          >
            {busy === "set" && <Spinner size="xs" />}
            {t("agents.registry_set")}
          </Button>
          <Button size="sm" variant="destructive" disabled={!online || busy !== null || !key.trim()} onClick={() => setConfirmDelete(true)}>
            {t("agents.registry_delete")}
          </Button>
        </div>
      </CollectCard>
      <ConfirmModal
        open={confirmDelete}
        title={t("agents.registry_delete_title")}
        message={t("agents.registry_delete_confirm").replace("{key}", key.trim())}
        confirmText={t("agents.registry_delete")}
        danger
        onConfirm={() => {
          setConfirmDelete(false);
          void run("delete");
        }}
        onCancel={() => setConfirmDelete(false)}
      />
    </>
  );
});
