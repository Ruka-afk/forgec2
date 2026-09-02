"use client";

import { cn } from "@/lib/utils";

export function PageHeader({ title, subtitle, eyebrow, icon, meta, children, className }: {
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  eyebrow?: React.ReactNode;
  icon?: React.ReactNode;
  meta?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn(
      "relative flex flex-col gap-4 border-b border-border/75 pb-4 sm:flex-row sm:items-end sm:justify-between sm:pb-5",
      className,
    )}>
      <div className="flex min-w-0 items-center gap-x-3.5">
        {icon && <div className="icon-well size-10 border border-primary/15 bg-primary/8 text-primary">{icon}</div>}
        <div className="min-w-0">
          {eyebrow !== undefined && eyebrow !== null && (
            <div className="mono-eyebrow text-muted-foreground/100 mb-1">{eyebrow}</div>
          )}
          <h1 className="break-words text-(--fs-page-title) font-semibold leading-tight tracking-tight text-foreground sm:truncate">{title}</h1>
          <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 mt-0.5">
            {subtitle && <p className="text-(--fs-compact) text-muted-foreground">{subtitle}</p>}
            {meta && <span className="text-(--fs-compact) text-muted-foreground/100">·</span>}
            {meta && <span className="mono-cell text-(--fs-compact) text-muted-foreground/100">{meta}</span>}
          </div>
        </div>
      </div>
      {children && <div className="flex w-full shrink-0 flex-wrap items-center gap-2 sm:w-auto sm:justify-end sm:pb-0.5">{children}</div>}
    </div>
  );
}
