"use client";

import { memo } from "react";
import { cn } from "@/lib/utils";
import { toneForStatus, toneStyles } from "@/lib/ui/statusStyles";

type AgentStatus = "online" | "offline" | "stale";
type TaskStatus = "pending" | "running" | "completed" | "failed" | "cancelled";
type ConnectionStatus = "connected" | "disconnected" | "reconnecting";
type GenericStatus = "active" | "inactive" | "locked" | "warning" | "error";
type BreakerStatus = "healthy" | "unstable" | "burned" | "unknown";
type Status = AgentStatus | TaskStatus | ConnectionStatus | GenericStatus | BreakerStatus;

type StatusIndicatorVariant = "dot" | "dotOnly" | "badge";

interface StatusIndicatorProps {
  status: Status | string;
  variant?: StatusIndicatorVariant;
  pulse?: boolean;
  size?: "sm" | "md" | "lg";
  label?: string;
  /** Accessible name; defaults to the displayed label. */
  ariaLabel?: string;
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
  pending_approval: "Pending Approval",
  connected: "Connected",
  disconnected: "Disconnected",
  reconnecting: "Reconnecting",
  active: "Active",
  inactive: "Inactive",
  locked: "Locked",
  warning: "Warning",
  error: "Error",
  healthy: "Healthy",
  unstable: "Unstable",
  burned: "Burned",
  unknown: "Unknown",
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
  ariaLabel,
  className,
}: StatusIndicatorProps) {
  const cfg = toneStyles[toneForStatus(status)];
  const displayLabel = label ?? STATUS_LABELS[status] ?? status;
  const accessibleLabel = ariaLabel ?? displayLabel;
  const dotSize = DOT_SIZES[size];

  if (variant === "dotOnly") {
    return (
      <span
        role="img"
        aria-label={accessibleLabel}
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
        role="status"
        aria-live={pulse ? "polite" : undefined}
        aria-label={accessibleLabel}
        className={cn("inline-flex items-center gap-1.5", className)}
      >
        <span
          aria-hidden="true"
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
      role="status"
      aria-live={pulse ? "polite" : undefined}
      aria-label={accessibleLabel}
      className={cn(
        "inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold tracking-wide",
        cfg.bg,
        cfg.text,
        className
      )}
    >
      <span
        aria-hidden="true"
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

export const StatusBadge = StatusIndicator;
