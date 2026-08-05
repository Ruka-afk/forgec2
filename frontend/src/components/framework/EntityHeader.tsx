"use client";

import { cn } from "@/lib/utils";

/**
 * F2 blueprint — shared detail-page header: entity identity + status/meta
 * chips on the left, actions on the right, and an optional slot (usually a
 * tab bar) docked to the bottom edge of the card.
 */
export function EntityHeader({
  eyebrow,
  title,
  subtitle,
  identity,
  meta,
  actions,
  children,
  className,
}: {
  eyebrow?: React.ReactNode;
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  identity?: React.ReactNode;
  meta?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("overflow-hidden rounded-2xl border border-border bg-card shadow-1", className)}>
      <div className="flex flex-col gap-3 px-5 py-4 lg:flex-row lg:items-center lg:gap-5">
        {identity && <div className="shrink-0">{identity}</div>}
        <div className="min-w-0 flex-1">
          {eyebrow && <div className="mono-eyebrow text-muted-foreground/60">{eyebrow}</div>}
          <h1 className="truncate text-lg font-semibold leading-tight tracking-tight text-foreground">{title}</h1>
          {subtitle && <p className="mt-0.5 text-(--fs-compact) text-muted-foreground/80">{subtitle}</p>}
          {meta && <div className="mt-2 flex flex-wrap items-center gap-1.5">{meta}</div>}
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
      </div>
      {children && <div className="border-t border-border/60">{children}</div>}
    </div>
  );
}