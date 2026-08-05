"use client";

import { cn } from "@/lib/utils";

/**
 * F1 blueprint — data toolbar row that hosts search, filters and bulk
 * actions with a consistent height / rhythm across all list pages.
 */
export function DataToolbar({
  left,
  right,
  className,
}: {
  left?: React.ReactNode;
  right?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-wrap items-center gap-2", className)}>
      <div className="flex flex-1 min-w-0 items-center gap-2">{left}</div>
      <div className="flex shrink-0 items-center gap-2">{right}</div>
    </div>
  );
}

/** Simple copy of the toolbar UI when placeholders are loading. */
export function DataToolbarSkeleton({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center gap-2", className)} aria-hidden="true">
      <div className="h-9 w-48 rounded-lg bg-muted/60 animate-pulse" />
      <div className="h-9 w-16 rounded-lg bg-muted/60 animate-pulse" />
      <div className="ml-auto h-9 w-24 rounded-lg bg-muted/60 animate-pulse" />
    </div>
  );
}