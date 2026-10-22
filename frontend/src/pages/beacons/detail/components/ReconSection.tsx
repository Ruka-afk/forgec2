import { memo, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Radar } from "lucide-react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import CollectCard from "./CollectCard";
import { useCollectTask } from "./useCollectTask";

interface ReconSectionProps {
  agentId: string;
  online: boolean;
}

type ReconKind = "drives" | "services" | "users" | "netstat" | "av" | "portscan" | "session" | "usb" | "container";

const RECON_PATHS: Record<Exclude<ReconKind, "portscan">, (id: string) => string> = {
  drives: (id) => paths.agents.drives(id),
  services: (id) => paths.agents.services(id),
  users: (id) => paths.agents.users(id),
  netstat: (id) => paths.agents.netstat(id),
  av: (id) => paths.agents.av(id),
  session: (id) => paths.agents.sessionRecon(id),
  usb: (id) => paths.agents.usbEnum(id),
  container: (id) => paths.agents.containerDetect(id),
};

const RECON_LABEL_KEYS: Record<ReconKind, string> = {
  drives: "agents.recon_drives",
  services: "agents.recon_services",
  users: "agents.recon_users",
  netstat: "agents.recon_netstat",
  av: "agents.recon_av",
  portscan: "agents.recon_portscan",
  session: "agents.recon_session",
  usb: "agents.recon_usb",
  container: "agents.recon_container",
};

export default memo(function ReconSection({ agentId, online }: ReconSectionProps) {
  const { t } = useI18n();
  const { busy: active, result, collect } = useCollectTask(agentId);
  const [target, setTarget] = useState("127.0.0.1:80,443");
  const [source, setSource] = useState<ReconKind | null>(null);

  // The printed result belongs to one agent: a switch clears the label so it
  // cannot describe the previous agent's dump (useCollectTask drops the
  // dump itself). The scan target stays — it is operator input, not agent
  // state.
  useEffect(() => {
    setSource(null);
  }, [agentId]);

  const labelOf = (kind: ReconKind) => t(RECON_LABEL_KEYS[kind]);

  const run = async (kind: ReconKind) => {
    const out =
      kind === "portscan"
        ? await collect(kind, paths.agents.portscan(agentId), {
            body: target.trim() ? { target: target.trim() } : {},
            emptyText: t("agents.recon_empty"),
            errorKey: "agents.recon_failed",
          })
        : await collect(kind, RECON_PATHS[kind](agentId), {
            emptyText: t("agents.recon_empty"),
            errorKey: "agents.recon_failed",
          });
    if (out !== null) setSource(kind);
  };

  const kinds: ReconKind[] = ["drives", "services", "users", "netstat", "av", "portscan", "session", "usb", "container"];

  return (
    <CollectCard
      title={t("agents.recon_title")}
      icon={Radar}
      tone="primary"
      emptyIcon={Radar}
      emptyTitle={t("agents.recon_empty_title")}
      emptyHint={t("agents.recon_empty_hint")}
      result={result}
      resultLabel={source ? labelOf(source) : t("agents.recon_collecting")}
    >
      <div className="flex flex-wrap items-center gap-2">
        {kinds.map((kind) => (
          <Button
            key={kind}
            size="sm"
            variant={source === kind ? "secondary" : "outline"}
            disabled={!online || active !== null}
            onClick={() => void run(kind)}
          >
            {active === kind && <Spinner size="xs" />}
            {labelOf(kind)}
          </Button>
        ))}
      </div>
      <div className="max-w-md">
        <Label className="block mb-1.5">{t("agents.recon_target")}</Label>
        <Input
          value={target}
          onChange={(e) => setTarget(e.target.value)}
          placeholder="127.0.0.1:80,443"
          className="h-8 font-mono text-xs"
        />
      </div>
    </CollectCard>
  );
});
