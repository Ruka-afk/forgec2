"use client";

import { memo } from "react";
import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";
import { hueStyles, resolveHue, type Hue } from "@/lib/ui/statusStyles";

type IconBadgeColor = Hue | "red" | "blue" | "purple" | "green";
type IconBadgeSize = "xs" | "sm" | "md" | "lg" | "xl" | "2xl";

// Monotonic ramp (smaller → larger) with a consistent radius per tier.
const ICON_BADGE_SIZES: Record<IconBadgeSize, { container: string; icon: string }> = {
  xs: { container: "w-6 h-6 rounded-lg", icon: "w-3 h-3" },
  sm: { container: "w-8 h-8 rounded-lg", icon: "w-4 h-4" },
  md: { container: "w-9 h-9 rounded-xl", icon: "w-4 h-4" },
  lg: { container: "w-10 h-10 rounded-xl", icon: "w-5 h-5" },
  xl: { container: "w-12 h-12 rounded-xl", icon: "w-6 h-6" },
  "2xl": { container: "w-14 h-14 rounded-xl", icon: "w-7 h-7" },
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
  const colors = hueStyles[resolveHue(color)];
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
