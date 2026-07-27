"use client";

import { cn } from "@/lib/utils";

interface PageSectionProps {
  title?: string;
  description?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  headerClassName?: string;
  contentClassName?: string;
}

export function PageSection({
  title,
  description,
  action,
  children,
  className,
  headerClassName,
  contentClassName,
}: PageSectionProps) {
  return (
    <section className={cn("space-y-5", className)}>
      {(title || description || action) && (
        <div className={cn("flex items-start justify-between gap-4", headerClassName)}>
          <div className="flex-1 min-w-0">
            {title && <h3 className="text-sm font-semibold text-foreground tracking-tight">{title}</h3>}
            {description && <p className="text-[12.5px] text-muted-foreground mt-0.5">{description}</p>}
          </div>
          {action && <div className="flex items-center gap-2 shrink-0">{action}</div>}
        </div>
      )}
      <div className={cn(contentClassName)}>{children}</div>
    </section>
  );
}
