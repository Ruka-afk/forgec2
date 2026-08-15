"use client";

import { cn } from "@/lib/utils";
import { hueStyles, resolveHue, type Hue } from "@/lib/ui/statusStyles";

/**
 * Compact label-over-value stat tile used inside stat grids.
 * Renders no surface — wrap it in a Card (or use StatCard for animated
 * icon tiles).
 */
export function StatTile({
  label,
  value,
  sub,
  tone,
  centered = false,
  labelBelow = false,
  className,
}: {
  label: React.ReactNode;
  value: React.ReactNode;
  sub?: React.ReactNode;
  tone?: Hue;
  centered?: boolean;
  labelBelow?: boolean;
  className?: string;
}) {
  const valueClass = tone ? hueStyles[resolveHue(tone)].text : "text-foreground";
  const labelClass = "text-xs font-semibold text-muted-foreground uppercase tracking-wider";
  return (
    <div className={cn(centered && "text-center", className)}>
      {!labelBelow && <div className={cn(labelClass, "mb-1")}>{label}</div>}
      <div className={cn("text-2xl font-bold tabular-nums", valueClass)}>{value}</div>
      {labelBelow && <div className={cn(labelClass, "mt-1")}>{label}</div>}
      {sub && <div className="text-xs text-muted-foreground mt-1">{sub}</div>}
    </div>
  );
}
