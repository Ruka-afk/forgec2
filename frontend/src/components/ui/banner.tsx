"use client";

import { memo, type ReactNode } from "react";
import { cn } from "@/lib/utils";

export type BannerTone = "warning" | "destructive" | "info" | "success" | "muted";

const BANNER_SURFACE: Record<BannerTone, string> = {
  warning: "border-warning/40 bg-warning/10 dark:bg-warning/20 text-warning",
  destructive: "border-destructive/40 bg-destructive/15 text-destructive",
  info: "border-info/25 bg-info/8 dark:bg-info/10 text-info",
  success: "border-success/40 bg-success/10 text-success",
  muted: "border-border bg-muted/40 text-muted-foreground",
};

/** Shared banner surface classes (rounded border + tone tint). Reuse directly. */
export function bannerSurface(tone: BannerTone, className?: string): string {
  return cn("rounded-xl border overflow-hidden", BANNER_SURFACE[tone], className);
}

interface BannerProps {
  tone?: BannerTone;
  icon?: ReactNode;
  children: ReactNode;
  action?: ReactNode;
  className?: string;
  /** Render as a fixed floating toast (top-center). */
  floating?: boolean;
  /** Use an assertive alert role instead of polite status. */
  alert?: boolean;
}

export const Banner = memo(function Banner({
  tone = "warning",
  icon,
  children,
  action,
  className,
  floating = false,
  alert = false,
}: BannerProps) {
  return (
    <div className={cn(floating && "fixed top-16 left-1/2 z-50 -translate-x-1/2")}>
      <div
        className={cn(bannerSurface(tone), "flex items-center gap-3 px-4 py-2.5", className)}
        role={alert ? "alert" : "status"}
        aria-live={alert ? "assertive" : "polite"}
      >
        {icon && <span className="shrink-0">{icon}</span>}
        <div className="flex-1 text-sm font-medium">{children}</div>
        {action && <div className="shrink-0">{action}</div>}
      </div>
    </div>
  );
});
