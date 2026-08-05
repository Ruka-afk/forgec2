"use client";

import { cn } from "@/lib/utils";

const SIZES = {
  sm: "h-1.5 w-1.5",
  md: "h-2 w-2",
  lg: "h-2.5 w-2.5",
} as const;

const TONES = {
  ok: "bg-emerald-500",
  warn: "bg-amber-500",
  crit: "bg-destructive",
  info: "bg-info",
  primary: "bg-primary",
  neutral: "bg-muted-foreground/50",
} as const;

/**
 * Shared LED — one source of truth for online / stale / critical dots used
 * in tables, status pills and entity headers.
 */
export function StatusDot({
  tone = "neutral",
  size = "md",
  pulse,
  className,
}: {
  tone?: keyof typeof TONES | string;
  size?: keyof typeof SIZES;
  pulse?: boolean;
  className?: string;
}) {
  const cls = TONES[tone as keyof typeof TONES] ?? TONES.neutral;
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-block shrink-0 rounded-full",
        SIZES[size],
        cls,
        pulse && "animate-pulse",
        className
      )}
    />
  );
}