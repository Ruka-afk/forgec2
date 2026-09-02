"use client";

import Link from "next/link";
import { ArrowLeft, Maximize2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { StatusBadge } from "@/components/ui/status-indicator";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { AgentStatus } from "@/types/agent";
import { agentDetailHref } from "./agent-detail-utils";

interface AgentDetailChromeProps {
  agentId: string;
  hostname?: string;
  status?: AgentStatus;
  embedded?: boolean;
  onClose?: () => void;
}

/** Shared session bar for the full /agents/:id page and the list drawer. */
export default function AgentDetailChrome({
  agentId,
  hostname,
  status,
  embedded,
  onClose,
}: AgentDetailChromeProps) {
  const { t } = useI18n();
  const href = agentDetailHref(agentId);
  return (
    <div
      className={cn(
        "flex h-11 shrink-0 items-center gap-2 border-b border-border/70 bg-background/95 px-3 sm:px-4",
        embedded && "sticky top-0 z-40 backdrop-blur-md",
      )}
    >
      {embedded && onClose ? (
        <Button variant="ghost" size="icon-xs" onClick={onClose} aria-label={t("common.close")}>
          <X className="size-4" />
        </Button>
      ) : (
        <Button variant="ghost" size="sm" render={<Link href="/agents" />} className="gap-1.5 px-2">
          <ArrowLeft className="size-4" />
          <span className="hidden sm:inline">{t("agents.detail_back_to_agents")}</span>
        </Button>
      )}
      <div className="min-w-0 flex-1 truncate text-sm font-medium text-foreground">
        {hostname || t("agents.detail_session")}
      </div>
      {status ? <StatusBadge status={status} pulse={status === "online"} /> : null}
      <CopyButton text={typeof window === "undefined" ? href : `${window.location.origin}${href}`} label={t("agents.detail_copy_link")} size="icon-xs" />
      {embedded ? (
        <Button variant="outline" size="sm" render={<Link href={href} />} className="gap-1.5">
          <Maximize2 className="size-3.5" />
          <span className="hidden sm:inline">{t("agents.detail_open_full_page")}</span>
        </Button>
      ) : null}
    </div>
  );
}
