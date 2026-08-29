"use client";

import { Inbox } from "lucide-react";
import { cn } from "@/lib/utils";

export function EmptyState({ icon, title, message, action, className, compact = false }: {
  icon?: React.ComponentType<{ className?: string }>;
  title: string;
  message?: string;
  action?: React.ReactNode;
  className?: string;
  /** compact: table-cell / panel-friendly variant — small icon chip, tight padding. */
  compact?: boolean;
}) {
  const IconComponent = icon || Inbox;
  if (compact) {
    return (
      <div className={cn("flex flex-col items-center justify-center gap-1.5 py-8 text-center", className)}>
        <IconComponent className="size-5 text-muted-foreground/70" aria-hidden="true" />
        <p className="text-(--fs-compact) font-medium text-muted-foreground">{title}</p>
        {message && <p className="text-xs text-muted-foreground/80 max-w-xs">{message}</p>}
        {action && <div className="mt-1">{action}</div>}
      </div>
    );
  }
  return (
    <div className={cn("flex flex-col items-center justify-center py-12 sm:py-16 md:py-20 text-center animate-fade-in", className)}>
      <div className="size-14 rounded-lg bg-muted/80 ring-1 ring-border/50 shadow-sm flex items-center justify-center mb-5" aria-hidden="true">
        <IconComponent className="size-8 text-muted-foreground/85" aria-hidden="true" />
      </div>
      <p className="text-sm font-semibold text-foreground mb-1.5">{title}</p>
      {message && <p className="text-(--fs-compact) text-muted-foreground mb-5 max-w-xs leading-relaxed">{message}</p>}
      {action && <div>{action}</div>}
    </div>
  );
}
