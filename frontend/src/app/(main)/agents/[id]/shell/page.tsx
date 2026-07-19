"use client";

import { useEffect, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import Link from "next/link";
import dynamic from "next/dynamic";
import { api } from "@/lib/api";
import { fetchAgentList, type AgentSummary } from "@/lib/agents";
import { useI18n } from "@/lib/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { ChevronRight } from "lucide-react";
import { Spinner } from "@/components/UI";

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

  useEffect(() => {
    api.get(`/agents/${agentId}`)
      .then((data) => {
        const ag = (data.agent || {}) as Record<string, string>;
        const os = String(ag.os || "").toLowerCase();
        setOsType(os.includes("linux") || os.includes("darwin") ? "linux" : "windows");
      })
      .catch(() => { toast.error(t("agents.load_failed")); });
    fetchAgentList().then(setAgents);
  }, [agentId, t]);

  const handleAgentChange = (newId: string) => {
    if (newId) router.push(`/agents/${newId}/shell`);
  };

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <div className="flex items-center gap-2 text-xs text-muted-foreground mb-4">
        <Link href="/agents" className="hover:text-foreground transition-colors">{t("agents.header_agents")}</Link>
        <ChevronRight className="w-4 h-4" />
        <Link href={`/agents/${agentId}`} className="hover:text-foreground transition-colors">{agentId.slice(0, 8)}</Link>
        <ChevronRight className="w-4 h-4" />
        <span className="text-foreground">{t("agents.shell_title")}</span>
      </div>
      <div className="mb-4 flex items-center gap-3">
        <Select value={agentId} onValueChange={(v) => { if (v !== null) handleAgentChange(v); }}>
          <SelectTrigger className="min-w-[240px]">
            <SelectValue placeholder={t("agents.shell_select_agent")} />
          </SelectTrigger>
          <SelectContent>
            {agents.map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {a.hostname} ({a.ip}) - {a.status}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {agentId && <ShellTerminal agentId={agentId} osType={osType} />}
    </div>
  );
}
