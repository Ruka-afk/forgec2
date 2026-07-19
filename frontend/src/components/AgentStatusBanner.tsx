"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ChevronUp, ChevronDown, X } from "lucide-react";

interface NavStats {
  online_count?: number;
  stale_count?: number;
  offline_count?: number;
}

export default function AgentStatusBanner() {
  const [counts, setCounts] = useState<NavStats | null>(null);
  const [dismissedSig, setDismissedSig] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);

  const load = async () => {
    try {
      const data = await api.get<NavStats>("/api/v1/dashboard");
      setCounts(data);
    } catch { /* ignore */ }
  };

  useEffect(() => { load(); }, []);
  useVisibleInterval(load, 15000);

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
      <div className="mx-auto w-full mt-3 rounded-xl border border-amber-400/40 bg-amber-50 dark:bg-amber-900/20 overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-2.5">
          <span className="relative flex h-2.5 w-2.5 shrink-0">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-amber-500"></span>
          </span>

          <div className="flex-1 flex items-center gap-2 flex-wrap text-sm">
            {hasOffline && (
              <Link href="/agents?status=offline" className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-destructive/10 text-destructive hover:bg-destructive/20 transition-colors font-medium text-xs">
                <span className="w-1.5 h-1.5 rounded-full bg-destructive"></span>
                {offline} offline
              </Link>
            )}
            {hasStale && (
              <Link href="/agents?status=stale" className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 hover:bg-amber-200 dark:hover:bg-amber-900/50 transition-colors font-medium text-xs">
                <span className="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
                {stale} stale
              </Link>
            )}
            <span className="text-amber-700 dark:text-amber-300/70 text-xs">
              — interactive commands paused{online > 0 ? `, ${online} online` : ""}
            </span>
          </div>

          <Button
            variant="ghost"
            size="icon-xs"
            onClick={() => setExpanded((p) => !p)}
            className="shrink-0 text-amber-600 dark:text-amber-300/70 hover:text-amber-800 dark:hover:text-amber-200"
            title="Details"
            aria-label="Toggle details"
          >
            {expanded ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
          </Button>

          <Button
            variant="ghost"
            size="icon-xs"
            onClick={() => setDismissedSig(sig)}
            className="shrink-0 text-amber-500 dark:text-amber-300/60 hover:text-amber-700 dark:hover:text-amber-200"
            aria-label="Dismiss"
            title="Dismiss (reappears if counts change)"
          >
            <X className="w-3 h-3" />
          </Button>
        </div>

        {expanded && (
          <div className="px-4 pb-3 pt-0 text-xs text-amber-700 dark:text-amber-300/70 leading-relaxed border-t border-amber-300/30 dark:border-amber-500/10">
            <p className="mt-2">
              Shell, file browser, screenshot and other interactive commands will not return results until the agent beacons back. Ensure the agent executable is running on target hosts.
            </p>
            <div className="mt-2 flex items-center gap-2">
              <Link href="/agents?status=offline" className="text-amber-700 dark:text-amber-300/80 hover:text-amber-900 dark:hover:text-amber-200 underline underline-offset-2 transition-colors">
                View offline agents
              </Link>
              <span className="text-amber-400 dark:text-amber-500/30">|</span>
              <Link href="/agents?status=stale" className="text-amber-700 dark:text-amber-300/80 hover:text-amber-900 dark:hover:text-amber-200 underline underline-offset-2 transition-colors">
                View stale agents
              </Link>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
