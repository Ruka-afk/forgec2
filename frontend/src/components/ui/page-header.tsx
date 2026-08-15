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
    <div className={cn("flex flex-col sm:flex-row sm:items-start justify-between mb-6 sm:mb-8 gap-3", className)}>
      <div className="flex items-start gap-x-3.5 min-w-0">
        {icon && <div className="w-10 h-10 rounded-xl bg-primary/10 ring-1 ring-primary/10 flex items-center justify-center shrink-0 shadow-sm">{icon}</div>}
        <div className="min-w-0">
          {eyebrow !== undefined && eyebrow !== null && (
            <div className="mono-eyebrow text-muted-foreground/70 mb-1">{eyebrow}</div>
          )}
          <h1 className="text-(--fs-page-title) font-semibold tracking-tight text-foreground leading-tight truncate">{title}</h1>
          <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 mt-0.5">
            {subtitle && <p className="text-(--fs-compact) text-muted-foreground">{subtitle}</p>}
            {meta && <span className="text-(--fs-compact) text-muted-foreground/50">·</span>}
            {meta && <span className="mono-cell text-(--fs-compact) text-muted-foreground/70">{meta}</span>}
          </div>
        </div>
      </div>
      {children && <div className="flex items-center gap-2 shrink-0">{children}</div>}
    </div>
  );
}
