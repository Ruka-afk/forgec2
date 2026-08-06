"use client";

import { memo } from "react";

const STATUS_CONFIG: Record<string, { dot: string; bg: string; text: string }> = {
  online:    { dot: "bg-emerald-500", bg: "bg-emerald-500/10 dark:bg-emerald-500/10", text: "text-emerald-700 dark:text-emerald-400" },
  offline:   { dot: "bg-muted-foreground", bg: "bg-muted/50", text: "text-muted-foreground" },
  stale:     { dot: "bg-amber-500", bg: "bg-amber-500/10 dark:bg-amber-500/10", text: "text-amber-700 dark:text-amber-400" },
  locked:    { dot: "bg-red-500", bg: "bg-red-500/10 dark:bg-red-500/10", text: "text-red-700 dark:text-red-400" },
  completed: { dot: "bg-emerald-500", bg: "bg-emerald-500/10 dark:bg-emerald-500/10", text: "text-emerald-700 dark:text-emerald-400" },
  failed:    { dot: "bg-red-500", bg: "bg-red-500/10 dark:bg-red-500/10", text: "text-red-700 dark:text-red-400" },
  pending:   { dot: "bg-muted-foreground", bg: "bg-muted/50", text: "text-muted-foreground" },
  running:   { dot: "bg-blue-500", bg: "bg-blue-500/10 dark:bg-blue-500/10", text: "text-blue-700 dark:text-blue-400" },
  cancelled: { dot: "bg-muted-foreground", bg: "bg-muted/50", text: "text-muted-foreground" },
  pending_approval: { dot: "bg-amber-500", bg: "bg-amber-500/10 dark:bg-amber-500/10", text: "text-amber-700 dark:text-amber-400" },
};

export const StatusBadge = memo(function StatusBadge({
  status,
  pulse,
  ariaLabel,
}: {
  status: string;
  pulse?: boolean;
  /** Accessible name; defaults to the raw status text. */
  ariaLabel?: string;
}) {
  const cfg = STATUS_CONFIG[status] || STATUS_CONFIG.offline;
  return (
    <span
      role="status"
      aria-live={pulse ? "polite" : undefined}
      aria-label={ariaLabel ?? status}
      className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-(--fs-xs-sm) font-semibold tracking-wide ${cfg.bg} ${cfg.text}`}
    >
      <span aria-hidden="true" className={`w-1.5 h-1.5 rounded-full ${cfg.dot} ${pulse ? "animate-pulse-glow" : ""}`} />
      {status}
    </span>
  );
});
