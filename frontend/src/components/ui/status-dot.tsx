"use client";

import { memo } from "react";
import { cn } from "@/lib/utils";
import { toneForStatus, toneStyles, type Tone } from "@/lib/ui/statusStyles";

type StatusDotSize = "xs" | "sm" | "md" | "lg";

interface StatusDotProps {
  /** Explicit semantic tone. Takes precedence over `status`. */
  tone?: Tone;
  /** Status string resolved to a tone via `toneForStatus`. */
  status?: string;
  size?: StatusDotSize;
  pulse?: boolean;
  className?: string;
}

const DOT_SIZES: Record<StatusDotSize, string> = {
  xs: "size-1.5",
  sm: "size-2",
  md: "size-2.5",
  lg: "size-3",
};

/**
 * Centralized status dot. Replaces the many hand-rolled
 * `bg-success/warning/destructive rounded-full` spans scattered across the
 * app so status colors stay theme-aware and consistent.
 *
 * Pulse uses the tone-agnostic `animate-pulse` (opacity) rather than the
 * success-only `animate-pulse-glow` keyframe.
 */
export const StatusDot = memo(function StatusDot({
  tone,
  status,
  size = "sm",
  pulse = false,
  className,
}: StatusDotProps) {
  const resolvedTone = tone ?? (status ? toneForStatus(status) : "muted");
  const cfg = toneStyles[resolvedTone];
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-block rounded-full",
        DOT_SIZES[size],
        cfg.dot,
        pulse && "animate-pulse",
        className
      )}
    />
  );
});
