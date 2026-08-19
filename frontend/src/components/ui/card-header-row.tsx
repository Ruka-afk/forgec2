"use client";

import { cn } from "@/lib/utils";
import { IconBadge } from "@/components/ui/icon-badge";
import { hueStyles, type Hue } from "@/lib/ui/statusStyles";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

const ACCENT_BORDER: Record<Hue, string> = {
  primary: "border-primary/20",
  success: "border-success/20",
  warning: "border-warning/20",
  info: "border-info/20",
  destructive: "border-destructive/20",
  muted: "border-border",
  emerald: "border-chart-1/20",
  cyan: "border-chart-2/20",
  indigo: "border-chart-3/20",
  amber: "border-chart-4/20",
  rose: "border-chart-5/20",
  violet: "border-chart-6/20",
};

interface CardHeaderRowProps {
  icon?: LucideIcon;
  /** Accent hue for the tinted header band + icon badge. */
  tone?: Hue;
  title: ReactNode;
  description?: ReactNode;
  /** Right-aligned controls (buttons, triggers). */
  action?: ReactNode;
  /** Tinted header band (`bg-{tone}/10` + matching border) vs plain `border-b`. */
  accent?: boolean;
  className?: string;
}

/**
 * Unified card title bar used inside `overflow-hidden` cards.
 * Replaces the hand-rolled `bg-{tone}/10 border-b px-6 py-4` header band
 * duplicated (with drifting icon-box sizes) across settings sections,
 * profiles, infrastructure and toolkit cards.
 */
export function CardHeaderRow({
  icon: Icon,
  tone = "primary",
  title,
  description,
  action,
  accent = true,
  className,
}: CardHeaderRowProps) {
  return (
    <div
      className={cn(
        "px-4 sm:px-6 py-3.5 border-b flex items-center gap-3",
        accent
          ? tone === "muted"
            ? "bg-secondary/60 border-border"
            : cn(hueStyles[tone].bg, ACCENT_BORDER[tone])
          : "border-border",
        className,
      )}
    >
      {Icon && <IconBadge icon={Icon} color={tone} size="lg" />}
      <div className="flex-1 min-w-0">
        <h2 className="text-(--fs-section) font-semibold text-foreground leading-tight truncate">{title}</h2>
        {description && <p className="text-xs text-muted-foreground mt-0.5 truncate">{description}</p>}
      </div>
      {action && <div className="flex items-center gap-2 shrink-0">{action}</div>}
    </div>
  );
}
