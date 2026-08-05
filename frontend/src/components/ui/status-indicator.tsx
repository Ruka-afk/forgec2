"use client";

import { memo } from "react";
import { cn } from "@/lib/utils";

type AgentStatus = "online" | "offline" | "stale";
type TaskStatus = "pending" | "running" | "completed" | "failed" | "cancelled";
type ConnectionStatus = "connected" | "disconnected" | "reconnecting";
type GenericStatus = "active" | "inactive" | "locked" | "warning" | "error";
type Status = AgentStatus | TaskStatus | ConnectionStatus | GenericStatus;

type StatusIndicatorVariant = "dot" | "dotOnly" | "badge";

const STATUS_STYLES: Record<string, { dot: string; bg: string; text: string; ring?: string }> = {
  online:       { dot: "bg-emerald-500", bg: "bg-emerald-500/10", text: "text-emerald-700 dark:text-emerald-400", ring: "ring-emerald-500/30" },
  offline:      { dot: "bg-muted-foreground", bg: "bg-muted/50", text: "text-muted-foreground", ring: "ring-muted-foreground/30" },
  stale:        { dot: "bg-amber-500", bg: "bg-amber-500/10", text: "text-amber-700 dark:text-amber-400", ring: "ring-amber-500/30" },
  pending:      { dot: "bg-muted-foreground", bg: "bg-muted/50", text: "text-muted-foreground", ring: "ring-muted-foreground/30" },
  running:      { dot: "bg-blue-500", bg: "bg-blue-500/10", text: "text-blue-700 dark:text-blue-400", ring: "ring-blue-500/30" },
  completed:    { dot: "bg-emerald-500", bg: "bg-emerald-500/10", text: "text-emerald-700 dark:text-emerald-400", ring: "ring-emerald-500/30" },
  failed:       { dot: "bg-red-500", bg: "bg-red-500/10", text: "text-red-700 dark:text-red-400", ring: "ring-red-500/30" },
  cancelled:    { dot: "bg-muted-foreground", bg: "bg-muted/50", text: "text-muted-foreground", ring: "ring-muted-foreground/30" },
  connected:    { dot: "bg-emerald-500", bg: "bg-emerald-500/10", text: "text-emerald-700 dark:text-emerald-400", ring: "ring-emerald-500/30" },
  disconnected: { dot: "bg-red-500", bg: "bg-red-500/10", text: "text-red-700 dark:text-red-400", ring: "ring-red-500/30" },
  reconnecting: { dot: "bg-amber-500", bg: "bg-amber-500/10", text: "text-amber-700 dark:text-amber-400", ring: "ring-amber-500/30" },
  active:       { dot: "bg-emerald-500", bg: "bg-emerald-500/10", text: "text-emerald-700 dark:text-emerald-400", ring: "ring-emerald-500/30" },
  inactive:     { dot: "bg-muted-foreground", bg: "bg-muted/50", text: "text-muted-foreground", ring: "ring-muted-foreground/30" },
  locked:       { dot: "bg-red-500", bg: "bg-red-500/10", text: "text-red-700 dark:text-red-400", ring: "ring-red-500/30" },
  warning:      { dot: "bg-amber-500", bg: "bg-amber-500/10", text: "text-amber-700 dark:text-amber-400", ring: "ring-amber-500/30" },
  error:        { dot: "bg-red-500", bg: "bg-red-500/10", text: "text-red-700 dark:text-red-400", ring: "ring-red-500/30" },
};

interface StatusIndicatorProps {
  status: Status | string;
  variant?: StatusIndicatorVariant;
  pulse?: boolean;
  size?: "sm" | "md" | "lg";
  label?: string;
  className?: string;
}

const STATUS_LABELS: Record<string, string> = {
  online: "Online",
  offline: "Offline",
  stale: "Stale",
  pending: "Pending",
  running: "Running",
  completed: "Completed",
  failed: "Failed",
  cancelled: "Cancelled",
  connected: "Connected",
  disconnected: "Disconnected",
  reconnecting: "Reconnecting",
  active: "Active",
  inactive: "Inactive",
  locked: "Locked",
  warning: "Warning",
  error: "Error",
};

const DOT_SIZES = {
  sm: "w-1.5 h-1.5",
  md: "w-2 h-2",
  lg: "w-2.5 h-2.5",
};

export const StatusIndicator = memo(function StatusIndicator({
  status,
  variant = "badge",
  pulse = false,
  size = "md",
  label,
  className,
}: StatusIndicatorProps) {
  const cfg = STATUS_STYLES[status] || STATUS_STYLES.offline;
  const displayLabel = label ?? STATUS_LABELS[status] ?? status;
  const dotSize = DOT_SIZES[size];

  if (variant === "dotOnly") {
    return (
      <span
        role="img"
        aria-label={displayLabel}
        className={cn(
          "inline-block rounded-full",
          dotSize,
          cfg.dot,
          pulse && "animate-pulse-glow",
          className
        )}
      />
    );
  }

  if (variant === "dot") {
    return (
      <span
        className={cn("inline-flex items-center gap-1.5", className)}
      >
        <span
          className={cn(
            "inline-block rounded-full ring-2",
            dotSize,
            cfg.dot,
            cfg.ring,
            pulse && "animate-pulse-glow"
          )}
        />
        <span className={cn("text-xs font-medium", cfg.text)}>{displayLabel}</span>
      </span>
    );
  }

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold tracking-wide",
        cfg.bg,
        cfg.text,
        className
      )}
    >
      <span
        className={cn(
          "rounded-full",
          dotSize,
          cfg.dot,
          pulse && "animate-pulse-glow"
        )}
      />
      {displayLabel}
    </span>
  );
});
