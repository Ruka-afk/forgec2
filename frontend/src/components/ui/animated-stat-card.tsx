"use client";

import { memo, useEffect, useRef, useState } from "react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { hueStyles, resolveHue, type Hue } from "@/lib/ui/statusStyles";

export const StatCard = memo(function StatCard({
  label,
  value,
  color = "indigo",
  sub,
  subColor,
  icon,
  dot,
  dotTone,
  iconSide = "right",
  className,
  style,
}: {
  label: string;
  value: number | string;
  color?: Hue | string;
  sub?: string;
  subColor?: string;
  icon?: React.ReactNode;
  dot?: boolean;
  dotTone?: string;
  iconSide?: "left" | "right";
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

  const colors = hueStyles[resolveHue(color)];
  const dotClasses =
    dotTone === "ok" ? "bg-success" : dotTone === "warn" ? "bg-warning" : dotTone === "crit" ? "bg-destructive" : "bg-info";

  return (
    <Card
      style={style}
      className={cn(
        "p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30",
        className
      )}
    >
      <div className={cn("flex items-start justify-between", iconSide === "left" && "flex-row-reverse")}>
        <div className="flex-1 min-w-0">
          <p className="mono-eyebrow text-muted-foreground/70">{label}</p>
          <p className="mt-1.5 text-(--fs-stat-value) font-bold leading-none text-foreground font-mono font-variant-numeric tabular-nums" aria-live="polite">{displayValue}</p>
          <div className={cn("flex items-center gap-1.5 mt-2 min-h-4", iconSide === "left" && "justify-start")}>
            {dot && (
              <span className={cn("w-1.5 h-1.5 rounded-full", dotClasses, "animate-pulse")} />
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
