"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import Link from "next/link";
import dynamic from "next/dynamic";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentList, type AgentSummary } from "@/lib/agents";
import { useI18n } from "@/lib/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/status-indicator";
import { toast } from "sonner";
import { Spinner } from "@/components/ui/spinner";
import { ArrowLeft, ChevronRight } from "lucide-react";
import { timeAgo } from "@/lib/utils";
import { POLL } from "@/lib/polling";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { agentIdentityTitle, pickAgentField } from "@/lib/shell-ui";
import { agentDetailHref } from "../_components/agent-detail-utils";
import type { AgentStatus } from "@/types/agent";

const ShellTerminal = dynamic(() => import("@/components/ShellTerminal"), {
  ssr: false,
  loading: () => (
    <div className="flex h-full items-center justify-center bg-(--shell-terminal-bg)">
      <Spinner />
    </div>
  ),
});

export default function AgentShellPage() {
  const { t } = useI18n();
  const router = useRouter();
  const { id: agentId } = useParams<{ id: string }>();
  const [osType, setOsType] = useState("windows");
  const [hostname, setHostname] = useState("");
  const [username, setUsername] = useState("");
  const [ip, setIp] = useState("");
  const [lastSeen, setLastSeen] = useState("");
  const [status, setStatus] = useState<AgentStatus | undefined>();
  const [agents, setAgents] = useState<AgentSummary[]>([]);
  const [listError, setListError] = useState<string | null>(null);

  const loadAgent = useCallback(async (signal?: AbortSignal, quiet = false) => {
    try {
      const data = await api.get(paths.agents.one(agentId), { signal });
      if (signal?.aborted) return;
      const raw = data as { agent?: Record<string, unknown>; Agent?: Record<string, unknown> };
      const ag = raw.agent || raw.Agent || {};
      const os = pickAgentField(ag, "os", "OS").toLowerCase();
      setOsType(os.includes("linux") || os.includes("darwin") ? "linux" : "windows");
      setHostname(pickAgentField(ag, "hostname", "Hostname"));
      setUsername(pickAgentField(ag, "username", "Username"));
      setIp(pickAgentField(ag, "ip", "IP", "internal_ip"));
      setLastSeen(pickAgentField(ag, "last_seen", "LastSeen"));
      const st = pickAgentField(ag, "status", "Status");
      if (st === "online" || st === "stale" || st === "offline") setStatus(st);
    } catch {
      if (!signal?.aborted && !quiet) toast.error(t("agents.load_failed"));
    }
  }, [agentId, t]);

  useEffect(() => {
    const controller = new AbortController();
    void loadAgent(controller.signal);
    fetchAgentList().then(({ agents: list, error }) => {
      if (controller.signal.aborted) return;
      setAgents(list);
      setListError(error);
      if (error) toast.error(error);
    });
    return () => controller.abort();
  }, [loadAgent]);

  useVisibleInterval(() => { void loadAgent(undefined, true); }, POLL.shellStatus);

  const offline = Boolean(status && status !== "online");
  const title = agentIdentityTitle(hostname, username, agentId);

  return (
    <div className="flex h-full min-h-0 flex-col bg-(--shell-terminal-bg) text-slate-100">
      <h1 className="sr-only">{t("shell.title")}</h1>
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-white/10 px-3 sm:px-4">
        <Button
          variant="ghost"
          size="sm"
          render={<Link href={agentDetailHref(agentId)} />}
          className="gap-1.5 px-2 text-slate-200 hover:bg-white/10 hover:text-white"
        >
          <ArrowLeft className="size-4" />
          <span className="hidden sm:inline">{t("shell.back_to_session")}</span>
        </Button>
        <ChevronRight className="hidden size-3.5 text-slate-600 sm:block" />
        {status ? <StatusBadge status={status} pulse={status === "online"} /> : null}
        <div className="min-w-0 leading-tight">
          <div className="truncate text-sm font-semibold tracking-tight">{title}</div>
          <div className="truncate font-mono text-(--fs-micro-sm) text-slate-400">
            {[ip, osType].filter(Boolean).join(" · ")}
          </div>
        </div>
        <Select value={agentId} onValueChange={(v) => { if (v) router.push(`/agents/${v}/shell`); }}>
          <SelectTrigger
            className="ml-auto h-8 w-[min(18rem,40vw)] border-white/10 bg-white/5 text-xs text-slate-200"
            aria-label={t("shell.switch_session")}
          >
            <SelectValue placeholder={t("agents.shell_select_agent")} />
          </SelectTrigger>
          <SelectContent>
            {agents.map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {agentIdentityTitle(a.hostname, a.username, a.id)}
                {a.ip ? ` @ ${a.ip}` : ""} — {t(`agents.${a.status}_label`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </header>
      {offline && (
        <div className="flex shrink-0 items-start gap-3 border-b border-amber-400/20 bg-amber-500/15 px-4 py-2.5 text-amber-50" role="status">
          <div className="min-w-0">
            <p className="text-sm font-medium">{t("shell.offline_title", { status: t(`agents.${status}_label`) })}</p>
            <p className="text-xs text-amber-100/90">{t("shell.offline_body", { when: timeAgo(lastSeen, t) })}</p>
          </div>
        </div>
      )}
      {listError && (
        <p className="shrink-0 px-4 py-1.5 text-xs text-red-300" role="alert">{listError}</p>
      )}
      {agentId && (
        <ShellTerminal
          agentId={agentId}
          osType={osType}
          hostname={hostname}
          username={username}
          ip={ip}
          lastSeen={lastSeen}
          status={status}
          className="relative flex min-h-0 flex-1 flex-col overflow-hidden"
        />
      )}
    </div>
  );
}
