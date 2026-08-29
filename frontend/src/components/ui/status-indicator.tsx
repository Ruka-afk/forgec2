"use client";

import { memo } from "react";
import { cn } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { toneForStatus, toneStyles } from "@/lib/ui/statusStyles";
import type { AgentStatus, TaskStatus } from "@/lib/status";

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

const DOT_SIZES = {
  sm: "size-1.5",
  md: "size-2",
  lg: "size-2.5",
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
  const { t } = useI18n();
  const statusLabels: Record<string, string> = {
    online: t("status.online"),
    offline: t("status.offline"),
    stale: t("status.stale"),
    pending: t("status.pending"),
    running: t("status.running"),
    completed: t("status.completed"),
    failed: t("status.failed"),
    cancelled: t("status.cancelled"),
    pending_approval: t("status.pending_approval"),
    connected: t("status.connected"),
    disconnected: t("status.disconnected"),
    reconnecting: t("status.reconnecting"),
    active: t("status.active"),
    inactive: t("status.inactive"),
    locked: t("status.locked"),
    warning: t("status.warning"),
    error: t("status.error"),
    healthy: t("status.healthy"),
    unstable: t("status.unstable"),
    burned: t("status.burned"),
    unknown: t("status.unknown"),
  };
  const cfg = toneStyles[toneForStatus(status)];
  const displayLabel = label ?? statusLabels[status] ?? status;
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