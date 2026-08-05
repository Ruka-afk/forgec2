"use client";

import { useState, useRef, memo } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { CopyButton, Spinner } from "@/components/UI";
import { timeAgo } from "@/lib/utils";
import type { AgentDetail, AgentDetailData, AgentStatus } from "@/types/agent";
import AgentHealthRing, { getHealthColor } from "./AgentHealthRing";
import { Calendar, Check, ChevronDown, Clipboard, Cpu, FileCode, FileText, GitBranch, Network, Radio, X } from "lucide-react";
import { Collapsible, CollapsibleTrigger } from "@/components/ui/collapsible";
import { useI18n } from "@/lib/i18n";
import { getRandomQuote } from "@/lib/easter-egg-quotes";

function InfoRow({ label, value, mono, title, copyValue, copyLabel, isSelect }: { label: string; value: string; mono?: boolean; title?: string; copyValue?: string; copyLabel?: string; isSelect?: boolean }) {
  const spanRef = useRef<HTMLSpanElement>(null);
  const handleClick = () => {
    if (isSelect && spanRef.current) {
      const range = document.createRange();
      range.selectNodeContents(spanRef.current);
      const sel = window.getSelection();
      if (sel) { sel.removeAllRanges(); sel.addRange(range); }
    }
  };
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleClick(); }
  };
  const isEmDash = value === "\u2014";
  const spanClasses = [
    "text-xs text-right",
    mono ? "font-mono" : "",
    "truncate max-w-[120px]",
    isEmDash ? "text-muted-foreground/70" : "text-foreground font-medium",
    isSelect ? "cursor-pointer select-all" : "",
  ].filter(Boolean).join(" ");
  return (
    <div className="flex items-center justify-between gap-2" title={title}>
      <span className="text-xs text-muted-foreground/70 shrink-0">{label}</span>
      <div className="flex items-center min-w-0">
        {isSelect ? (
          <span ref={spanRef} onClick={handleClick} tabIndex={0} role="button" onKeyDown={handleKeyDown} className={spanClasses}>{value}</span>
        ) : (
          <span className={spanClasses}>{value}</span>
        )}
        {copyValue && <CopyButton text={copyValue} label={copyLabel} />}
      </div>
    </div>
  );
}

let lastQuoteTime = 0;
function QuoteTooltip({ children }: { children: React.ReactNode }) {
  const [quote, setQuote] = useState<string | null>(null);

  const handleEnter = () => {
    const now = Date.now();
    if (now - lastQuoteTime < 30000 || Math.random() > 0.1) return;
    lastQuoteTime = now;
    setQuote(getRandomQuote());
  };

  return (
    <Tooltip>
      <TooltipTrigger
        render={<span onMouseEnter={handleEnter} onMouseLeave={() => setQuote(null)} />}
      >
        {children}
      </TooltipTrigger>
      {quote && (
        <TooltipContent side="top" sideOffset={8} className="max-w-[220px] text-xs">
          💬 {quote}
        </TooltipContent>
      )}
    </Tooltip>
  );
}

export interface AgentStatsGridProps {
  agent: Partial<AgentDetail>;
  data: AgentDetailData;
  healthScore: number;
  activityBuckets: number[];
  maxActivity: number;
  sleepValue: number;
  jitterValue: number;
  onSleepChange: (v: number) => void;
  onJitterChange: (v: number) => void;
  onApplySleep: () => void;
  sleepSaving: boolean;
  status: AgentStatus;
  childAgents: unknown[];
  childrenExpanded: boolean;
  onToggleChildren: () => void;
  onExportJSON: () => void;
  onExportMarkdown: () => void;
  onCopyAllInfo: () => void;
  killDate?: string;
  onSetKillDate: () => void;
  onClearKillDate: () => void;
}

export default memo(function AgentStatsGrid({
  agent, data, healthScore, activityBuckets, maxActivity,
  sleepValue, jitterValue, onSleepChange, onJitterChange,
  onApplySleep,   sleepSaving, status, childAgents, childrenExpanded,
  onToggleChildren, onExportJSON, onExportMarkdown, onCopyAllInfo,
  killDate, onSetKillDate, onClearKillDate,
}: AgentStatsGridProps) {
  const agentID = agent.id || "";
  const { t } = useI18n();
  const pid = agent.pid;
  const processName = agent.process_name || "";
  const version = agent.version || "\u2014";
  const username = agent.username || "\u2014";
  const ip = agent.ip || "\u2014";
  const publicIP = agent.public_ip || "";
  const domain = agent.domain || "";
  const country = agent.country || "";
  const city = agent.city || "";
  const latitude = agent.latitude;
  const longitude = agent.longitude;
  const interval = agent.current_interval ?? 0;
  const jitter = agent.current_jitter ?? 0;
  const uptime = data.uptime || "\u2014";
  const lastSeen = agent.last_seen || "";
  const activeWindow = agent.active_window || "";
  const parentID = agent.parent_id || "";
  const peerCount = Number(agent.peer_count ?? 0);
  const createdAt = agent.created_at || "";

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
      <Card className="p-4 gap-0 overflow-hidden border-border/70 bg-card/90 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30"><QuoteTooltip><div className="text-(--fs-micro-sm) font-semibold uppercase tracking-wider text-muted-foreground/70 mb-3"><Cpu className="w-3.5 h-3.5" />{t("agents.stats_system")}</div></QuoteTooltip><div className="space-y-2.5">
        <InfoRow label={t("agents.stats_agent_id")} value={agentID || "\u2014"} mono copyValue={agentID || undefined} copyLabel={t("agents.stats_agent_id")} isSelect />
        <InfoRow label={t("agents.stats_pid")} value={pid ? String(pid) : "\u2014"} />
        <InfoRow label={t("agents.stats_process")} value={processName || "\u2014"} />
        <InfoRow label={t("agents.stats_version")} value={version} />
        <InfoRow label={t("agents.stats_username")} value={username} />
      </div></Card>
      <Card className="p-4 gap-0 overflow-hidden border-border/70 bg-card/90 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30"><QuoteTooltip><div className="text-(--fs-micro-sm) font-semibold uppercase tracking-wider text-muted-foreground/70 mb-3"><Network className="w-3.5 h-3.5" />{t("agents.stats_network")}</div></QuoteTooltip><div className="space-y-2.5">
        <InfoRow label={t("agents.stats_local_ip")} value={ip} copyValue={ip !== "\u2014" ? ip : undefined} copyLabel={t("agents.stats_local_ip")} />
        <InfoRow label={t("agents.stats_public_ip")} value={publicIP || "\u2014"} copyValue={publicIP || undefined} copyLabel={t("agents.stats_public_ip")} />
        <InfoRow label={t("agents.stats_domain")} value={domain || "\u2014"} />
        <InfoRow label={t("agents.stats_location")} value={[country, city].filter(Boolean).join(", ") || "\u2014"} />
        <InfoRow label={t("agents.stats_listener")} value={agent.listener_id ? `#${agent.listener_id}` : "\u2014"} />
        {latitude != null && longitude != null && latitude !== 0 && longitude !== 0 && (
          <div className="mt-2 rounded-lg overflow-hidden border border-border">
            <img
              src={`https://staticmap.openstreetmap.de/staticmap.php?center=${latitude},${longitude}&zoom=10&size=300x150&markers=${latitude},${longitude},red-pushpin`}
              alt={`Map: ${latitude}, ${longitude}`}
              className="w-full h-auto"
              loading="lazy"
              onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
            />
          </div>
        )}
      </div></Card>
      <Card className="p-4 gap-0 overflow-hidden border-border/70 bg-card/90 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30"><QuoteTooltip><div className="text-(--fs-micro-sm) font-semibold uppercase tracking-wider text-muted-foreground/70 mb-3"><Radio className="w-3.5 h-3.5" />{t("agents.stats_beacon")}</div></QuoteTooltip><div className="space-y-2.5">
        <InfoRow label={t("agents.stats_sleep")} value={interval ? `${interval}s` : "\u2014"} />
        <InfoRow label={t("agents.stats_jitter")} value={jitter ? `${jitter}%` : "\u2014"} />
        <InfoRow label={t("agents.stats_uptime")} value={uptime} />
        <InfoRow label={t("agents.stats_last_seen")} value={lastSeen ? timeAgo(lastSeen) : "\u2014"} title={lastSeen ? new Date(lastSeen).toLocaleString() : undefined} />
        <InfoRow label={t("agents.stats_idle")} value={data.time_since_last_seen || "\u2014"} />
        <div className="pt-2 mt-2 border-t border-border">
          <div className="text-(--fs-micro-sm) text-muted-foreground/70 mb-1.5">{t("agents.stats_quick_adjust")}</div>
          <div className="flex items-center gap-1.5">
            <Input type="number" value={sleepValue} onChange={(e) => onSleepChange(Number(e.target.value))} min={0} max={86400}
              className="w-16 h-7 px-2 py-1 text-xs font-mono" placeholder={t("agents.sleep")} aria-label={t("agents.stats_sleep_seconds")} />
            <Input type="number" value={jitterValue} onChange={(e) => onJitterChange(Number(e.target.value))} min={0} max={100}
              className="w-14 h-7 px-2 py-1 text-xs font-mono" placeholder={t("agents.stats_jitter")} aria-label={t("agents.stats_jitter_percent")} />
            <Button size="xs" onClick={onApplySleep} disabled={sleepSaving || status !== "online"}
              className="gap-1">
              {sleepSaving ? <Spinner size="xs" color="white" /> : <Check className="w-4 h-4" />} {t("agents.apply")}
            </Button>
          </div>
        </div>
        <div className="pt-2 mt-2 border-t border-border">
          <div className="text-(--fs-micro-sm) text-muted-foreground/70 mb-1.5">{t("agents.stats_activity_24h")}</div>
          <div className="flex items-end gap-[2px] h-8">
            {activityBuckets.map((count, i) => {
              const pct = count > 0 ? Math.max(10, (count / maxActivity) * 100) : 0;
              return (
                <div key={i} className={`flex-1 rounded-sm transition-all ${count > 0 ? "bg-emerald-500" : "bg-secondary"}`}
                  style={{ height: count > 0 ? `${pct}%` : "2px" }}
                  title={`${Math.floor(i)}h: ${count} tasks`}></div>
              );
            })}
          </div>
        </div>
      </div></Card>
      <Card className="p-4 gap-0 overflow-hidden border-border/70 bg-card/90 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30"><QuoteTooltip><div className="text-(--fs-micro-sm) font-semibold uppercase tracking-wider text-muted-foreground/70 mb-3"><GitBranch className="w-3.5 h-3.5" />{t("agents.stats_activity")}</div></QuoteTooltip><div className="space-y-2.5">
        {activeWindow ? (<div className="flex items-center justify-between gap-2"><span className="text-xs text-muted-foreground/70 shrink-0">{t("agents.stats_window")}</span><span className="text-xs text-foreground font-medium truncate max-w-[120px]" title={activeWindow}>{activeWindow}</span></div>) : <InfoRow label={t("agents.stats_window")} value="\u2014" />}
        {parentID && <InfoRow label={t("agents.stats_parent")} value={parentID.substring(0, 8) + "..."} />}
        {peerCount > 0 && <InfoRow label={t("agents.stats_peers")} value={String(peerCount)} />}
        {childAgents.length > 0 ? (<Collapsible open={childrenExpanded} onOpenChange={onToggleChildren}><div className="flex items-center justify-between gap-2"><span className="text-xs text-muted-foreground/70 shrink-0">{t("agents.stats_children")}</span><CollapsibleTrigger><Button variant="ghost" size="xs" className="text-primary font-medium hover:underline">{childAgents.length} {t("agents.agents_count", { count: childAgents.length })} <ChevronDown className="w-2 h-2 ml-0.5" /></Button></CollapsibleTrigger></div></Collapsible>) : (!parentID && peerCount === 0 ? <InfoRow label={t("agents.stats_type")} value="Direct" /> : null)}
        <InfoRow label={t("agents.stats_created")} value={createdAt ? new Date(createdAt).toLocaleDateString() : "\u2014"} />
        <div className="pt-2 mt-2 border-t border-border">
          <div className="flex items-center justify-between gap-2">
            <span className="text-(--fs-micro-sm) text-muted-foreground/70">{t("agents.stats_kill_date")}</span>
            <div className="flex items-center gap-1.5">
              {killDate ? (
                <>
                  <span className="text-xs font-medium text-destructive">{new Date(killDate).toLocaleDateString()}</span>
                  <Button variant="ghost" size="icon-xs" onClick={onClearKillDate} aria-label={t("agents.detail_clear_kill_date")} className="text-muted-foreground hover:text-foreground">
                    <X className="w-3 h-3" />
                  </Button>
                </>
              ) : (
                <Button variant="ghost" size="xs" onClick={onSetKillDate} className="text-(--fs-micro-sm) text-primary hover:bg-transparent gap-1">
                  <Calendar className="w-3 h-3" /> {t("agents.set")}
                </Button>
              )}
            </div>
          </div>
        </div>
        <div className="pt-2 mt-2 border-t border-border">
          <div className="flex items-center justify-between gap-2">
            <span className="text-(--fs-micro-sm) text-muted-foreground/70">{t("agents.stats_health")}</span>
            <div className="flex items-center gap-2">
              <span className={`text-sm font-bold ${getHealthColor(healthScore)}`}>{healthScore}</span>
              <AgentHealthRing score={healthScore} size={32} />
            </div>
          </div>
        </div>
        <div className="flex items-center justify-between gap-2 pt-2 mt-2 border-t border-border">
          <span className="text-(--fs-micro-sm) text-muted-foreground/70">{t("agents.stats_export")}</span>
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="icon-xs" onClick={onExportJSON} aria-label={t("agents.stats_export_json")}>
              <FileCode className="w-4 h-4" />
            </Button>
            <Button variant="ghost" size="icon-xs" onClick={onExportMarkdown} aria-label={t("agents.stats_export_md")}>
              <FileText className="w-4 h-4" />
            </Button>
            <Button variant="ghost" size="icon-xs" onClick={onCopyAllInfo} aria-label={t("agents.stats_copy_all")}>
              <Clipboard className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </div></Card>
    </div>
  );
})
