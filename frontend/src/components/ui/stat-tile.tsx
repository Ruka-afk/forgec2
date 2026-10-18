
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
  icon,
  trend,
}: {
  label: React.ReactNode;
  value: React.ReactNode;
  sub?: React.ReactNode;
  tone?: Hue;
  centered?: boolean;
  labelBelow?: boolean;
  className?: string;
  icon?: React.ReactNode;
  trend?: React.ReactNode;
}) {
  const valueClass = tone ? hueStyles[resolveHue(tone)].text : "text-foreground";
  const labelClass = "text-xs font-semibold text-muted-foreground uppercase tracking-wider";
  const iconTone = tone ? hueStyles[resolveHue(tone)] : null;
  return (
    <div className={cn(centered && "text-center", "relative flex h-full flex-col", className)}>
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-1 flex-col">
          {/* Two-line label slot: stat grids put one- and two-word labels side
              by side, and without the reservation the values stair-step
              (measured on the dashboard: "BEACONS" one line, "QUEUED TASKS"
              two, values 35px apart). */}
          {!labelBelow && <div className={cn(labelClass, "mb-1 min-h-8 leading-4")}>{label}</div>}
          <div className={cn("text-2xl font-bold tabular-nums tracking-tight", valueClass)}>{value}</div>
          {labelBelow && <div className={cn(labelClass, "mt-1")}>{label}</div>}
          {sub && <div className="mt-1 text-xs text-muted-foreground">{sub}</div>}
        </div>
        {icon && <span className={cn("shrink-0 rounded-lg p-2.5 ring-1", iconTone ? `${iconTone.bg} ${iconTone.text} ring-border/50` : "bg-secondary text-muted-foreground ring-border/30")}>{icon}</span>}
      </div>
      {trend && <div className="mt-auto flex items-center gap-1.5 border-t border-border/40 pt-2 text-xs text-muted-foreground/80">{trend}</div>}
    </div>
  );
}
