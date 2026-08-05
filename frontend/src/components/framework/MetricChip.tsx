"use client";

import { cn } from "@/lib/utils";
import { StatusDot } from "./StatusDot";

const TONES: Record<string, string> = {
  ok: "text-emerald-600 dark:text-emerald-400",
  warn: "text-amber-600 dark:text-amber-400",
  crit: "text-destructive",
  info: "text-info",
  default: "text-foreground",
  muted: "text-muted-foreground",
};

type Tone = keyof typeof TONES | string;

/**
 * Inline metric readout — mono value with an LED. Used in list pages,
 * detail headers and status strips.
 */
export function MetricChip({
  label,
  value,
  tone = "muted",
  dot,
  dotTone,
  pulse,
  className,
}: {
  label?: React.ReactNode;
  value: React.ReactNode;
  tone?: Tone;
  dot?: boolean;
  dotTone?: string;
  pulse?: boolean;
  className?: string;
}) {
  const toneCls = TONES[tone as string] ?? TONES.default;
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-md bg-secondary/50 dark:bg-secondary/30 px-2 py-1 ring-1 ring-border/60", className)}>
      {dot && <StatusDot tone={dotTone ?? tone} size="sm" pulse={pulse} />}
      {label && <span className="mono-eyebrow text-muted-foreground/60">{label}</span>}
      <span className={cn("mono-cell text-(--fs-compact) font-semibold", toneCls)}>{value}</span>
    </span>
  );
}