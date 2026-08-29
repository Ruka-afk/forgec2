"use client";

import { memo } from "react";
import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";
import { hueStyles, resolveHue, type Hue } from "@/lib/ui/statusStyles";

type IconBadgeColor = Hue | "red" | "blue" | "purple" | "green";
type IconBadgeSize = "xs" | "sm" | "md" | "lg" | "xl" | "2xl";

// Monotonic ramp (smaller → larger) with a consistent radius per tier.
const ICON_BADGE_SIZES: Record<IconBadgeSize, { container: string; icon: string }> = {
  xs: { container: "size-6 rounded-lg", icon: "size-3" },
  sm: { container: "size-8 rounded-lg", icon: "size-4" },
  md: { container: "size-9 rounded-lg", icon: "size-4" },
  lg: { container: "size-10 rounded-lg", icon: "size-5" },
  xl: { container: "size-12 rounded-lg", icon: "size-6" },
  "2xl": { container: "size-14 rounded-lg", icon: "size-7" },
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
