"use client";

import { memo, useEffect, useRef, useState } from "react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";

const COLOR_MAP: Record<string, { text: string; bg: string; glow: string }> = {
  indigo:  { text: "text-primary", bg: "bg-primary/10", glow: "shadow-primary/10" },
  emerald: { text: "text-emerald-600 dark:text-emerald-400", bg: "bg-emerald-500/10", glow: "shadow-emerald-500/10" },
  amber:   { text: "text-amber-600 dark:text-amber-400", bg: "bg-amber-500/10", glow: "shadow-amber-500/10" },
  red:     { text: "text-destructive", bg: "bg-destructive/10", glow: "shadow-destructive/10" },
  blue:    { text: "text-blue-600 dark:text-blue-400", bg: "bg-blue-500/10", glow: "shadow-blue-500/10" },
  purple:  { text: "text-purple-600 dark:text-purple-400", bg: "bg-purple-500/10", glow: "shadow-purple-500/10" },
  cyan:    { text: "text-cyan-600 dark:text-cyan-400", bg: "bg-cyan-500/10", glow: "shadow-cyan-500/10" },
};

export const StatCard = memo(function StatCard({
  label,
  value,
  color = "indigo",
  sub,
  subColor,
  icon,
  dot,
  dotTone,
  className,
  style,
}: {
  label: string;
  value: number | string;
  color?: string;
  sub?: string;
  subColor?: string;
  icon?: React.ReactNode;
  dot?: boolean;
  dotTone?: string;
  className?: string;
  style?: React.CSSProperties;
}) {
  const [displayValue, setDisplayValue] = useState<number | string>(typeof value === "number" ? 0 : value);
  const animatedRef = useRef<number>(0);

  useEffect(() => {
    if (typeof value !== "number") {
      setDisplayValue(value);
      return;
    }
    const target = value;
    const duration = 600;
    const start = performance.now();
    const startVal = animatedRef.current;

    const animate = (now: number) => {
      const elapsed = now - start;
      const progress = Math.min(elapsed / duration, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      const current = Math.round(startVal + (target - startVal) * eased);
      animatedRef.current = current;
      setDisplayValue(current);
      if (progress < 1) requestAnimationFrame(animate);
    };
    requestAnimationFrame(animate);
  }, [value]);

  const colors = COLOR_MAP[color] || COLOR_MAP.indigo;

  return (
    <Card
      style={style}
      className={cn(
        "p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30",
        className
      )}
    >
      <div className="flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <p className="mono-eyebrow text-muted-foreground/70">{label}</p>
          <p className="mt-1.5 text-[1.625rem] font-bold leading-none text-foreground font-mono font-variant-numeric tabular-nums" aria-live="polite">{displayValue}</p>
          <div className="flex items-center gap-1.5 mt-2 min-h-4">
            {dot && (
              <span className={`w-1.5 h-1.5 rounded-full ${dotTone === "ok" ? "bg-emerald-500" : dotTone === "warn" ? "bg-amber-500" : dotTone === "crit" ? "bg-destructive" : "bg-info"} animate-pulse`} />
            )}
            {sub && <p className={cn("text-xs", subColor || colors.text)}>{sub}</p>}
          </div>
        </div>
        {icon && (
          <div
            className={cn(
              "w-9 h-9 rounded-lg flex items-center justify-center shrink-0 shadow-sm ring-1 ring-border/50",
              colors.bg,
              colors.glow
            )}
          >
            {icon}
          </div>
        )}
      </div>
    </Card>
  );
});
