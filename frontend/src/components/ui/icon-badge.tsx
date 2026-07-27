"use client";

import { memo } from "react";
import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";

type IconBadgeColor = "primary" | "emerald" | "amber" | "red" | "blue" | "purple" | "cyan" | "rose";
type IconBadgeSize = "sm" | "md" | "lg";

const ICON_BADGE_COLORS: Record<IconBadgeColor, { text: string; bg: string }> = {
  primary: { text: "text-primary", bg: "bg-primary/10" },
  emerald: { text: "text-emerald-600 dark:text-emerald-400", bg: "bg-emerald-500/10" },
  amber:   { text: "text-amber-600 dark:text-amber-400", bg: "bg-amber-500/10" },
  red:     { text: "text-destructive", bg: "bg-destructive/10" },
  blue:    { text: "text-blue-600 dark:text-blue-400", bg: "bg-blue-500/10" },
  purple:  { text: "text-purple-600 dark:text-purple-400", bg: "bg-purple-500/10" },
  cyan:    { text: "text-cyan-600 dark:text-cyan-400", bg: "bg-cyan-500/10" },
  rose:    { text: "text-rose-600 dark:text-rose-400", bg: "bg-rose-500/10" },
};

const ICON_BADGE_SIZES: Record<IconBadgeSize, { container: string; icon: string }> = {
  sm: { container: "w-6 h-6 rounded-lg", icon: "w-3 h-3" },
  md: { container: "w-9 h-9 rounded-lg", icon: "w-4 h-4" },
  lg: { container: "w-12 h-12 rounded-xl", icon: "w-5 h-5" },
};

interface IconBadgeProps {
  icon: LucideIcon;
  color?: IconBadgeColor;
  size?: IconBadgeSize;
  className?: string;
  iconClassName?: string;
}

export const IconBadge = memo(function IconBadge({
  icon: Icon,
  color = "primary",
  size = "md",
  className,
  iconClassName,
}: IconBadgeProps) {
  const colors = ICON_BADGE_COLORS[color] || ICON_BADGE_COLORS.primary;
  const sizes = ICON_BADGE_SIZES[size] || ICON_BADGE_SIZES.md;

  return (
    <div
      className={cn(
        "flex items-center justify-center shrink-0 shadow-sm",
        sizes.container,
        colors.bg,
        className
      )}
    >
      <Icon className={cn(sizes.icon, colors.text, iconClassName)} />
    </div>
  );
});
