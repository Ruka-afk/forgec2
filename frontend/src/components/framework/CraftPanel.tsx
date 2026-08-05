"use client";

import { cn } from "@/lib/utils";

/**
 * F3 blueprint — persistent side panel that reflects the current craft
 * (build / stager / profile / report). Sticky-friendly; parent supplies the
 * live-summary content.
 */
export function CraftPanel({
  title,
  badge,
  children,
  footer,
  className,
  bodyClassName,
}: {
  title: React.ReactNode;
  badge?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <aside className={cn("flex max-h-full flex-col overflow-hidden rounded-2xl border border-border bg-card shadow-1", className)}>
      <div className="flex items-center justify-between gap-2 border-b border-border/60 px-5 py-3.5">
        <h2 className="mono-eyebrow text-muted-foreground">{title}</h2>
        {badge}
      </div>
      <div className={cn("min-h-0 flex-1 overflow-y-auto p-5", bodyClassName)}>{children}</div>
      {footer && (
        <div className="border-t border-border/60 bg-muted/30 px-5 py-3">{footer}</div>
      )}
    </aside>
  );
}