"use client";

import { cn } from "@/lib/utils";

/**
 * F6 blueprint — grouped settings/management section card (title + optional
 * icon + description + actions, body, optional footer).
 */
export function ConfigSection({
  title,
  description,
  icon,
  actions,
  children,
  footer,
  className,
  bodyClassName,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  icon?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  footer?: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <section className={cn("overflow-hidden rounded-2xl border border-border bg-card shadow-1", className)}>
      <header className="flex items-start justify-between gap-3 border-b border-border/60 px-5 py-4">
        <div className="flex items-start gap-3">
          {icon && (
            <div className="mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary ring-1 ring-primary/10">
              {icon}
            </div>
          )}
          <div>
            <h2 className="text-sm font-semibold leading-snug text-foreground">{title}</h2>
            {description && <p className="mt-0.5 text-xs leading-relaxed text-muted-foreground/80">{description}</p>}
          </div>
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
      </header>
      {children && <div className={cn("px-5 py-4", bodyClassName)}>{children}</div>}
      {footer && (
        <div className="flex items-center justify-between gap-3 border-t border-border/60 bg-muted/20 px-5 py-3">
          {footer}
        </div>
      )}
    </section>
  );
}