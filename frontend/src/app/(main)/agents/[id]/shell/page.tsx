"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import ShellTerminal from "@/components/ShellTerminal";
import { API_BASE } from "@/lib/constants";
import { fetchAgentList, type AgentSummary } from "@/lib/agents";

export default function AgentShellPage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter();
  const [id, setId] = useState("");
  const [osType, setOsType] = useState("windows");
  const [agents, setAgents] = useState<AgentSummary[]>([]);
  const [agentId, setAgentId] = useState("");

  useEffect(() => {
    params.then(({ id: agentId }) => {
      setId(agentId);
      setAgentId(agentId);
      fetch(`${API_BASE}?p=/agents/${agentId}&format=json`, { credentials: "include" })
        .then((r) => r.json())
        .then((data) => {
          const ag = data.Agent || data.agent || {};
          const os = String(ag.os || ag.OS || "").toLowerCase();
          setOsType(os.includes("linux") || os.includes("darwin") ? "linux" : "windows");
        })
        .catch((e) => console.error("Shell: failed to fetch agent", e));
    });
    fetchAgentList().then(setAgents);
  }, [params]);

  const handleAgentChange = (newId: string) => {
    if (newId) router.push(`/agents/${newId}/shell`);
  };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="mb-4 flex items-center gap-3">
        <select
          value={agentId}
          onChange={(e) => handleAgentChange(e.target.value)}
          className="bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 h-10 min-w-[240px]"
        >
          <option value="">Select agent...</option>
          {agents.map((a) => (
            <option key={a.id} value={a.id}>
              {a.hostname} ({a.ip}) - {a.status}
            </option>
          ))}
        </select>
      </div>
      <ShellTerminal agentId={id} osType={osType} />
    </div>
  );
}
