"use client";

import { useState } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ChevronDown, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useAppStore } from "@/lib/store";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";
import { bannerSurface } from "@/components/ui/banner";
import { StatusDot } from "@/components/ui/status-dot";

export default function AgentStatusBanner() {
  const [dismissedSig, setDismissedSig] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);
  const { t } = useI18n();
  const counts = useAppStore((s) => s.stats);

  if (!counts) return null;
  const stale = counts.stale_count || 0;
  const offline = counts.offline_count || 0;
  const online = counts.online_count || 0;
  if (stale === 0 && offline === 0) return null;

  const sig = `${offline}-${stale}`;
  if (dismissedSig === sig) return null;

  const hasOffline = offline > 0;
  const hasStale = stale > 0;

  return (
    <div className="px-4 sm:px-6 lg:px-8">
      <div className={bannerSurface("warning", "mt-3")} aria-live="polite">
        <Collapsible open={expanded} onOpenChange={setExpanded}>
        <div className="flex items-center gap-3 px-4 py-2.5">
          <span className="relative flex h-2.5 w-2.5 shrink-0">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-warning opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-warning"></span>
          </span>

          <div className="flex-1 flex items-center gap-2 flex-wrap text-sm">
            {hasOffline && (
              <Link href="/agents?status=offline" className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-destructive/10 text-destructive hover:bg-destructive/20 transition-colors font-medium text-xs">
                <StatusDot tone="destructive" size="xs" />
                {offline} {t("status_banner.offline")}
              </Link>
            )}
            {hasStale && (
              <Link href="/agents?status=stale" className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-warning/15 text-warning hover:bg-warning/25 transition-colors font-medium text-xs">
                <StatusDot tone="warning" size="xs" />
                {stale} {t("status_banner.stale")}
              </Link>
            )}
            <span className="text-warning text-xs">
              — {t("status_banner.commands_paused")}{online > 0 ? `, ${online} ${t("status_banner.online")}` : ""}
            </span>
          </div>

          <CollapsibleTrigger render={<Button variant="ghost" size="icon-xs" className="shrink-0 text-warning hover:text-warning/80" aria-label={t("status_banner.toggle_details")} />}>
            <ChevronDown className="w-3 h-3" />
          </CollapsibleTrigger>

          <Tooltip>
            <TooltipTrigger render={<Button variant="ghost" size="icon-xs" onClick={() => setDismissedSig(sig)} className="shrink-0 text-warning/80 hover:text-warning" aria-label={t("status_banner.dismiss")} />}>
              <X className="w-3 h-3" />
            </TooltipTrigger>
            <TooltipContent>{t("status_banner.dismiss_hint")}</TooltipContent>
          </Tooltip>
        </div>

        <CollapsibleContent>
          <div className="px-4 pb-3 pt-0 text-xs text-warning leading-relaxed border-t border-warning/20">
            <p className="mt-2">
              {t("status_banner.expanded_message")}
            </p>
            <div className="mt-2 flex items-center gap-2">
              <Link href="/agents?status=offline" className="text-warning hover:text-warning/80 underline underline-offset-2 transition-colors">
                {t("status_banner.view_offline")}
              </Link>
              <span className="text-warning/60">|</span>
              <Link href="/agents?status=stale" className="text-warning hover:text-warning/80 underline underline-offset-2 transition-colors">
                {t("status_banner.view_stale")}
              </Link>
            </div>
          </div>
        </CollapsibleContent>
        </Collapsible>
      </div>
    </div>
  );
}
