
import { useCallback, useEffect, useMemo, useRef, useState, lazy, Suspense } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Link } from "react-router-dom";

import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentList, type AgentSummary } from "@/lib/agents";
import { useI18n } from "@/lib/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/status-indicator";
import { toast } from "sonner";
import { Spinner } from "@/components/ui/spinner";
import { ArrowLeft, ChevronRight, Plus, X } from "lucide-react";
import { cn, timeAgo } from "@/lib/utils";
import { POLL } from "@/lib/polling";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { agentIdentityTitle, pickAgentField } from "@/lib/shell-ui";
import { agentDetailHref } from "../components/agent-detail-utils";
import type { AgentStatus } from "@/types/agent";
import ErrorBoundary from "@/components/ErrorBoundary";

const ShellTerminal = lazy(() => import("@/components/ShellTerminal")); // xterm is heavy; spinner fallback at usage below

/** One open console per agent; cap limits concurrent xterm instances. */
const MAX_SHELL_TABS = 8;

interface ShellTab {
  agentId: string;
}

interface AgentShellMeta {
  osType: string;
  hostname: string;
  username: string;
  ip: string;
  lastSeen: string;
  status?: AgentStatus;
}

function guessOs(os?: string): string {
  const v = (os || "").toLowerCase();
  return v.includes("linux") || v.includes("darwin") ? "linux" : "windows";
}

function metaFromSummary(a: AgentSummary): AgentShellMeta {
  return {
    osType: guessOs(a.os),
    hostname: a.hostname,
    username: a.username,
    ip: a.ip,
    lastSeen: a.last_seen,
    status: a.status,
  };
}

export default function AgentShellPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { id } = useParams();
  const routeAgentId = id ?? "";
  const [tabs, setTabs] = useState<ShellTab[]>(() => (routeAgentId ? [{ agentId: routeAgentId }] : []));
  const [activeAgentId, setActiveAgentId] = useState(routeAgentId);
  const [metaByAgent, setMetaByAgent] = useState<Record<string, AgentShellMeta>>({});
  const [agents, setAgents] = useState<AgentSummary[]>([]);
  const [listError, setListError] = useState<string | null>(null);
  const loadedRef = useRef<Set<string>>(new Set());
  const selectTriggerRef = useRef<HTMLButtonElement | null>(null);

  // Keep route ↔ active tab in sync (deep link or browser back).
  useEffect(() => {
    if (!routeAgentId) return;
    setTabs((prev) => (prev.some((tb) => tb.agentId === routeAgentId) ? prev : [...prev, { agentId: routeAgentId }]));
    setActiveAgentId(routeAgentId);
  }, [routeAgentId]);

  const loadMeta = useCallback(async (id: string, signal?: AbortSignal, quiet = false) => {
    if (!id) return;
    try {
      const data = await api.get(paths.agents.one(id), { signal });
      if (signal?.aborted) return;
      const raw = data as { agent?: Record<string, unknown>; Agent?: Record<string, unknown> };
      const ag = raw.agent || raw.Agent || {};
      const st = pickAgentField(ag, "status", "Status");
      const next: AgentShellMeta = {
        osType: guessOs(pickAgentField(ag, "os", "OS")),
        hostname: pickAgentField(ag, "hostname", "Hostname"),
        username: pickAgentField(ag, "username", "Username"),
        ip: pickAgentField(ag, "ip", "IP", "internal_ip"),
        lastSeen: pickAgentField(ag, "last_seen", "LastSeen"),
        status: st === "online" || st === "stale" || st === "offline" ? st : undefined,
      };
      setMetaByAgent((prev) => ({ ...prev, [id]: next }));
    } catch {
      if (!signal?.aborted && !quiet) toast.error(t("agents.load_failed"));
    }
  }, [t]);

  useEffect(() => {
    const controller = new AbortController();
    fetchAgentList().then(({ agents: list, error }) => {
      if (controller.signal.aborted) return;
      setAgents(list);
      setListError(error);
      if (error) toast.error(error);
      setMetaByAgent((prev) => {
        const merged = { ...prev };
        for (const a of list) {
          if (!merged[a.id]) merged[a.id] = metaFromSummary(a);
        }
        return merged;
      });
    });
    return () => controller.abort();
  }, []);

  // Fetch fresh meta for tabs not yet loaded from the detail endpoint.
  useEffect(() => {
    const controller = new AbortController();
    for (const tab of tabs) {
      if (tab.agentId && !loadedRef.current.has(tab.agentId)) {
        loadedRef.current.add(tab.agentId);
        void loadMeta(tab.agentId, controller.signal, true).catch(() => {
          loadedRef.current.delete(tab.agentId);
        });
      }
    }
    return () => controller.abort();
  }, [tabs, loadMeta]);

  const activeMeta = metaByAgent[activeAgentId];
  const offline = Boolean(activeMeta?.status && activeMeta.status !== "online");
  // Offline agents won't come back in 5s; back off to 15s to cut idle load.
  useVisibleInterval(() => {
    if (activeAgentId) {
      loadedRef.current.delete(activeAgentId);
      void loadMeta(activeAgentId, undefined, true);
    }
  }, offline ? 15_000 : POLL.shellStatus);

  const openAgentTab = useCallback((nextId: string) => {
    if (!nextId) return;
    const existing = tabs.find((tb) => tb.agentId === nextId);
    if (existing) {
      setActiveAgentId(nextId);
      if (nextId !== routeAgentId) navigate(`/agents/${nextId}/shell`, { replace: true });
      return;
    }
    if (tabs.length >= MAX_SHELL_TABS) {
      toast.error(t("shell.tab_limit", { max: MAX_SHELL_TABS }));
      return;
    }
    setTabs((prev) => [...prev, { agentId: nextId }]);
    setActiveAgentId(nextId);
    navigate(`/agents/${nextId}/shell`);
  }, [tabs, routeAgentId, navigate, t]);

  const closeTab = useCallback((nextId: string) => {
    setTabs((prev) => {
      const next = prev.filter((tb) => tb.agentId !== nextId);
      if (nextId === activeAgentId) {
        const fallback = next[next.length - 1];
        if (fallback) {
          setActiveAgentId(fallback.agentId);
          navigate(`/agents/${fallback.agentId}/shell`, { replace: true });
        } else {
          navigate("/agents");
        }
      }
      return next;
    });
  }, [activeAgentId, navigate]);

  const listMeta = useMemo(() => {
    const map: Record<string, AgentShellMeta> = {};
    for (const a of agents) map[a.id] = metaFromSummary(a);
    return map;
  }, [agents]);

  const resolveMeta = useCallback(
    (agentId: string): AgentShellMeta =>
      metaByAgent[agentId] || listMeta[agentId] || {
        osType: "windows",
        hostname: "",
        username: "",
        ip: "",
        lastSeen: "",
        status: undefined,
      },
    [metaByAgent, listMeta],
  );

  const activeMetaResolved = activeAgentId ? resolveMeta(activeAgentId) : undefined;
  const title = activeAgentId
    ? agentIdentityTitle(activeMetaResolved?.hostname, activeMetaResolved?.username, activeAgentId)
    : t("shell.title");
  const activeOffline = Boolean(activeMetaResolved?.status && activeMetaResolved.status !== "online");

  return (
    <div className="flex h-full min-h-0 flex-col bg-(--shell-terminal-bg) text-slate-100">
      <h1 className="sr-only">{t("shell.title")}</h1>
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-white/10 px-3 sm:px-4">
        <Button
          variant="ghost"
          size="sm"
          render={<Link to={agentDetailHref(activeAgentId || routeAgentId)} />}
          className="gap-1.5 px-2 text-slate-200 hover:bg-white/10 hover:text-white"
        >
          <ArrowLeft className="size-4" />
          <span className="hidden sm:inline">{t("shell.back_to_session")}</span>
        </Button>
        <ChevronRight className="hidden size-3.5 text-slate-600 sm:block" />
        {activeMetaResolved?.status ? <StatusBadge status={activeMetaResolved.status} pulse={activeMetaResolved.status === "online"} /> : null}
        <div className="min-w-0 leading-tight">
          <div className="truncate text-sm font-semibold tracking-tight">{title}</div>
          <div className="truncate font-mono text-(--fs-micro-sm) text-slate-400">
            {[activeMetaResolved?.ip, activeMetaResolved?.osType].filter(Boolean).join(" · ")}
          </div>
        </div>
        <Select value={activeAgentId} onValueChange={(v) => { if (v) openAgentTab(v); }}>
          <SelectTrigger
            ref={selectTriggerRef}
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

      {tabs.length > 0 && (
        <div
          className="flex shrink-0 items-center gap-1 overflow-x-auto border-b border-white/10 bg-black/20 px-2 py-1"
          role="tablist"
          aria-label={t("shell.tabs_label")}
        >
          {tabs.map((tab) => {
            const meta = resolveMeta(tab.agentId);
            const selected = tab.agentId === activeAgentId;
            const label = agentIdentityTitle(meta.hostname, meta.username, tab.agentId);
            return (
              <div
                key={tab.agentId}
                role="tab"
                aria-selected={selected}
                tabIndex={0}
                className={cn(
                  "group flex h-7 shrink-0 items-center gap-1 rounded-md border px-2 text-(--fs-micro-sm) outline-none",
                  selected
                    ? "border-sky-500/40 bg-sky-500/15 text-slate-100"
                    : "border-white/5 bg-white/5 text-slate-400 hover:bg-white/10 hover:text-slate-200",
                )}
                onClick={() => openAgentTab(tab.agentId)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    openAgentTab(tab.agentId);
                  }
                }}
              >
                <span className="max-w-[10rem] truncate font-mono" title={label}>{label}</span>
                {tabs.length > 1 && (
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    aria-label={t("shell.close_tab")}
                    className="size-5 text-slate-500 hover:bg-white/10 hover:text-slate-200"
                    onClick={(e) => {
                      e.stopPropagation();
                      closeTab(tab.agentId);
                    }}
                  >
                    <X className="size-3" />
                  </Button>
                )}
              </div>
            );
          })}
          <Button
            variant="ghost"
            size="icon-xs"
            aria-label={t("shell.new_tab")}
            title={t("shell.new_tab")}
            className="size-6 shrink-0 text-slate-400 hover:bg-white/10 hover:text-white"
            disabled={tabs.length >= MAX_SHELL_TABS}
            onClick={() => selectTriggerRef.current?.click()}
          >
            <Plus className="size-3.5" />
          </Button>
        </div>
      )}

      {activeOffline && activeMetaResolved && (
        <div className="flex shrink-0 items-start gap-3 border-b border-amber-400/20 bg-amber-500/15 px-4 py-2.5 text-amber-50" role="status">
          <div className="min-w-0">
            <p className="text-sm font-medium">{t("shell.offline_title", { status: t(`agents.${activeMetaResolved.status}_label`) })}</p>
            <p className="text-xs text-amber-100/90">{t("shell.offline_body", { when: timeAgo(activeMetaResolved.lastSeen, t) })}</p>
          </div>
        </div>
      )}
      {listError && (
        <p className="shrink-0 px-4 py-1.5 text-xs text-red-300" role="alert">{listError}</p>
      )}
      <div className="relative flex min-h-0 flex-1 flex-col">
        {tabs.map((tab) => {
          const meta = resolveMeta(tab.agentId);
          const selected = tab.agentId === activeAgentId;
          return (
            <div
              key={tab.agentId}
              className={cn(
                "min-h-0 flex-1 flex-col overflow-hidden",
                selected ? "flex" : "hidden",
              )}
            >
              <ErrorBoundary resetKey={tab.agentId}>
                <Suspense fallback={(
                  <div className="flex h-full items-center justify-center bg-(--shell-terminal-bg)">
                    <Spinner />
                  </div>
                )}>
                  <ShellTerminal
                    agentId={tab.agentId}
                    osType={meta.osType}
                    hostname={meta.hostname}
                    username={meta.username}
                    ip={meta.ip}
                    lastSeen={meta.lastSeen}
                    status={meta.status}
                    className="relative flex min-h-0 flex-1 flex-col overflow-hidden"
                  />
                </Suspense>
              </ErrorBoundary>
            </div>
          );
        })}
        {tabs.length === 0 && (
          <div className="flex flex-1 items-center justify-center text-sm text-slate-500">
            {t("shell.session_hint")}
          </div>
        )}
      </div>
    </div>
  );
}
