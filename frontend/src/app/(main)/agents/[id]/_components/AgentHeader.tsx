"use client";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { CopyButton, StatusBadge, Spinner } from "@/components/UI";
import type { AgentDetail } from "@/types/agent";
import { Activity, Apple, ArrowLeft, Camera, ChevronRight, Clipboard, Crown, Database, FolderOpen, History, Key, Keyboard, Link as LinkIcon, ListChecks, MapPin, Monitor, MoreHorizontal, Shield, Skull, SlidersHorizontal, Terminal, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useI18n } from "@/lib/i18n";

function getOSIcon(os: string): React.ReactNode {
  const cls = "w-7 h-7 text-indigo-600 dark:text-indigo-400";
  switch (os.toLowerCase()) {
    case "windows": return <Monitor className={cls} />;
    case "linux": return <Terminal className={cls} />;
    case "darwin": case "macos": return <Apple className={cls} />;
    default: return <Monitor className={cls} />;
  }
}

export interface AgentHeaderProps {
  agent: AgentDetail;
  agentId: string;
  moreOpen: boolean;
  setMoreOpen: (v: boolean) => void;
  agentAge: string;
  status: string;
  actionLoading: string | null;
  onQuickAction: (action: string, label: string) => void;
  credCount: number | null;
  onKill: () => void;
  onUninstall: () => void;
}

export default function AgentHeader({
  agent, agentId, moreOpen, setMoreOpen, agentAge, status,
  actionLoading, onQuickAction, credCount, onKill, onUninstall,
}: AgentHeaderProps) {
  const { t } = useI18n();
  const hostname = agent.hostname || "\u2014";
  const ip = agent.ip || "\u2014";
  const publicIP = agent.public_ip || "";
  const os = agent.os || "\u2014";
  const arch = agent.arch || "\u2014";
  const username = agent.username || "\u2014";
  const integrity = agent.integrity || "";
  const elevated = Boolean(agent.elevated);
  const domain = agent.domain || "";
  const country = agent.country || "";
  const city = agent.city || "";

  const quickActions = [
    { action: "screenshot", label: t("agents.header_screenshot"), icon: <Camera className="w-5 h-5" /> },
    { action: "ps", label: t("agents.header_processes"), icon: <ListChecks className="w-5 h-5" /> },
    { action: "hashdump", label: t("agents.header_hashdump"), icon: <Database className="w-5 h-5" /> },
    { action: "creds_dump", label: t("agents.header_creds_dump"), icon: <Database className="w-5 h-5" />, badge: credCount },
    { action: "clipboard_get", label: t("agents.header_clipboard"), icon: <Clipboard className="w-5 h-5" /> },
    { action: "privesc_check", label: t("agents.header_privesc"), icon: <Shield className="w-5 h-5" /> },
    { action: "keylogger_start", label: t("agents.header_key_start"), icon: <Terminal className="w-5 h-5" /> },
    { action: "keylogger_stop", label: t("agents.header_key_stop"), icon: <Keyboard className="w-5 h-5" /> },
    { action: "keylogger_dump", label: t("agents.header_key_dump"), icon: <Database className="w-5 h-5" /> },
  ];

  return (
    <>
      <div className="flex items-center gap-2 mb-4">
        <Link href="/agents" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
          <ArrowLeft className="w-4 h-4" /> {t("agents.header_agents")}
        </Link>
        <ChevronRight className="w-4 h-4" />
        <span className="text-sm text-foreground font-medium">{hostname}</span>
        {agentAge && <span className="text-[10px] text-muted-foreground/70 ml-1">(alive {agentAge})</span>}
      </div>

      <Card className="p-4 sm:p-5 mb-4 gap-0">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className="w-14 h-14 rounded-xl bg-indigo-100 dark:bg-indigo-900/30 flex items-center justify-center shrink-0">
              {getOSIcon(os)}
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <h1 className="text-xl font-bold text-foreground truncate">{hostname}</h1>
                <StatusBadge status={status} pulse={status === "online"} />
                {integrity && (<Badge variant={integrity.toLowerCase() === "system" || integrity.toLowerCase() === "high" ? "destructive" : integrity.toLowerCase() === "medium" ? "secondary" : "outline"} className="text-[10px] uppercase">{integrity}</Badge>)}
                {elevated && (<Badge variant="warning" className="text-[10px]"><Crown className="w-4 h-4" />{t("agents.header_elevated")}</Badge>)}
              </div>
              <div className="text-sm text-muted-foreground mt-1 flex items-center flex-wrap">
                <span>{ip}</span><CopyButton text={ip} label="IP" />
                {publicIP && <><span className="mx-1.5">&middot;</span><span>{publicIP}</span><CopyButton text={publicIP} label="Public IP" /></>}
                <span className="mx-1.5">&middot;</span><span>{username}</span>
                <span className="mx-1.5">&middot;</span><span>{os} {arch}</span>
                {domain && <><span className="mx-1.5">&middot;</span><span>{domain}</span></>}
              </div>
              {country && (<p className="text-xs text-muted-foreground/70 mt-0.5"><MapPin className="w-4 h-4" />{[country, city].filter(Boolean).join(", ")}</p>)}
            </div>
          </div>
          <div className="flex items-center gap-1.5 flex-wrap shrink-0">
            <Link href={`/agents/${agentId}/shell`} className="px-3 h-8 bg-card border border-border rounded-xl text-[11px] font-medium text-foreground hover:bg-muted transition-colors flex items-center gap-1.5"><Terminal className="w-4 h-4" /> {t("agents.shell_title")} <kbd className="text-[9px] opacity-50 ml-1">S</kbd></Link>
            <Link href={`/agents/${agentId}/files`} className="px-3 h-8 bg-card border border-border rounded-xl text-[11px] font-medium text-foreground hover:bg-muted transition-colors flex items-center gap-1.5"><FolderOpen className="w-4 h-4" /> {t("agents.files_title")} <kbd className="text-[9px] opacity-50 ml-1">F</kbd></Link>
            <Link href={`/agents/${agentId}/screen`} className="px-3 h-8 bg-card border border-border rounded-xl text-[11px] font-medium text-foreground hover:bg-muted transition-colors flex items-center gap-1.5"><Monitor className="w-4 h-4" /> {t("agents.screen_title")} <kbd className="text-[9px] opacity-50 ml-1">D</kbd></Link>
            <Link href={`/tasks?agent_id=${agentId}`} className="px-3 h-8 bg-card border border-border rounded-xl text-[11px] font-medium text-foreground hover:bg-muted transition-colors flex items-center gap-1.5"><ListChecks className="w-4 h-4" /> {t("agents.detail_task_breakdown")}</Link>
            <div className="relative" data-more-menu>
              <Button variant="secondary" size="sm" onClick={(e) => { e.stopPropagation(); setMoreOpen(!moreOpen); }} className="gap-1.5"><MoreHorizontal className="w-4 h-4" /> {t("agents.header_more")}</Button>
              {moreOpen && (<div className="absolute right-0 top-full mt-1 w-48 bg-card border border-border rounded-xl shadow-lg py-1 z-[50]">
                <Link href={`/agents/${agentId}/token`} className="flex items-center gap-2.5 px-3 py-2 text-xs text-foreground hover:bg-muted transition-colors"><Key className="w-4 h-4" /> {t("agents.token_title")}</Link>
                <Link href={`/agents/${agentId}/persistence`} className="flex items-center gap-2.5 px-3 py-2 text-xs text-foreground hover:bg-muted transition-colors"><LinkIcon className="w-4 h-4" /> {t("agents.persistence_title")}</Link>
                <Link href={`/agents/${agentId}/remote-desktop`} className="flex items-center gap-2.5 px-3 py-2 text-xs text-foreground hover:bg-muted transition-colors"><Monitor className="w-4 h-4" /> {t("agents.rdp_title")}</Link>
                <Link href={`/agents/${agentId}/recording`} className="flex items-center gap-2.5 px-3 py-2 text-xs text-foreground hover:bg-muted transition-colors"><History className="w-4 h-4" /> {t("agents.recording_title")}</Link>
                <Link href={`/agents/${agentId}/config`} className="flex items-center gap-2.5 px-3 py-2 text-xs text-foreground hover:bg-muted transition-colors"><SlidersHorizontal className="w-4 h-4" /> {t("agents.config_hot_config")}</Link>
                <Link href={`/agents/${agentId}/traffic`} className="flex items-center gap-2.5 px-3 py-2 text-xs text-foreground hover:bg-muted transition-colors"><Activity className="w-4 h-4" /> {t("agents.traffic_title")}</Link>
                <div className="border-t border-border my-1"></div>
                <Button variant="ghost" size="sm" onClick={onKill} className="justify-start gap-2.5 px-3 py-2 text-xs text-destructive hover:bg-destructive/10 w-full"><Skull className="w-4 h-4" /> {t("agents.kill_agent")}</Button>
                <Button variant="ghost" size="sm" onClick={onUninstall} className="justify-start gap-2.5 px-3 py-2 text-xs text-destructive hover:bg-destructive/10 w-full"><Trash2 className="w-4 h-4" /> {t("agents.uninstall")}</Button>
              </div>)}
            </div>
          </div>
        </div>
      </Card>

      <div className="grid grid-cols-3 sm:grid-cols-4 lg:grid-cols-6 gap-2 mb-4">
        {quickActions.map((item) => (
          <Card key={item.action} onClick={() => onQuickAction(item.action, item.label)} title={item.label} className={`p-3 flex flex-col items-center gap-1.5 hover:bg-muted transition-colors cursor-pointer ${status !== "online" || actionLoading !== null ? "opacity-40 pointer-events-none" : ""}`}>
            {actionLoading === item.action ? (<Spinner size="sm" />) : item.icon}
            <span className="text-[10px] font-medium text-foreground leading-tight text-center">{item.label}</span>
            {"badge" in item && item.badge != null && (item.badge as number) > 0 && (
              <Badge className="absolute top-1 right-1 min-w-[16px] h-4 px-1 text-[9px]">{(item.badge as number) > 99 ? "99+" : (item.badge as number)}</Badge>
            )}
          </Card>
        ))}
      </div>
    </>
  );
}
