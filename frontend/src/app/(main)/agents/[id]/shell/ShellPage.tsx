"use client";

import { useEffect, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import dynamic from "next/dynamic";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentList, type AgentSummary } from "@/lib/agents";
import { useI18n } from "@/lib/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { Spinner } from "@/components/ui/spinner";
import { PageContainer } from "@/components/ui/page-container";
const ShellTerminal = dynamic(() => import("@/components/ShellTerminal"), {
  ssr: false,
  loading: () => <div className="flex items-center justify-center h-96"><Spinner /></div>,
});

export default function AgentShellPage() {
  const { t } = useI18n();
  const router = useRouter();
  const { id: agentId } = useParams<{ id: string }>();
  const [osType, setOsType] = useState("windows");
  const [agents, setAgents] = useState<AgentSummary[]>([]);
  const [listError, setListError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    api.get(paths.agents.one(agentId), { signal: controller.signal })
      .then((data) => {
        if (controller.signal.aborted) return;
        const ag = (data.agent || {}) as Record<string, string>;
        const os = String(ag.os || "").toLowerCase();
        setOsType(os.includes("linux") || os.includes("darwin") ? "linux" : "windows");
      })
      .catch(() => { if (!controller.signal.aborted) toast.error(t("agents.load_failed")); });
    fetchAgentList().then(({ agents: list, error }) => {
      if (controller.signal.aborted) return;
      setAgents(list);
      setListError(error);
      if (error) toast.error(error);
    });
    return () => controller.abort();
  }, [agentId, t]);

  const handleAgentChange = (newId: string) => {
    if (newId) router.push(`/agents/${newId}/shell`);
  };

  return (
    <PageContainer className="h-full gap-3 px-4 py-3 sm:px-6">
      <div className="flex shrink-0 flex-col gap-2 sm:flex-row sm:items-center">
        <Select value={agentId} onValueChange={(v) => { if (v !== null) handleAgentChange(v); }}>
          <SelectTrigger className="w-full sm:min-w-[240px] sm:w-auto">
            <SelectValue placeholder={t("agents.shell_select_agent")} />
          </SelectTrigger>
          <SelectContent>
            {agents.map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {a.hostname} ({a.ip}) - {t(`agents.${a.status}_label`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {listError && (
          <p className="text-xs text-destructive" role="alert">
            {listError}
          </p>
        )}
        {!listError && agents.length === 0 && (
          <p className="text-xs text-muted-foreground">{t("agents.no_beacons")}</p>
        )}
      </div>
      {agentId && <div className="min-h-0 flex-1"><ShellTerminal agentId={agentId} osType={osType} /></div>}
    </PageContainer>
  );
}
