"use client";

import { Inbox } from "lucide-react";
import { cn } from "@/lib/utils";

export function EmptyState({ icon, title, message, action, className }: {
  icon?: React.ComponentType<{ className?: string }>;
  title: string;
  message?: string;
  action?: React.ReactNode;
  className?: string;
}) {
  const IconComponent = icon || Inbox;
  return (
    <div className={cn("flex flex-col items-center justify-center py-12 sm:py-16 md:py-20 text-center animate-fade-in", className)}>
      <div className="w-16 h-16 rounded-xl bg-muted flex items-center justify-center mb-5" aria-hidden="true">
        <IconComponent className="w-8 h-8 text-muted-foreground/60" aria-hidden="true" />
      </div>
      <p className="text-(--fs-body-sm) font-semibold text-foreground mb-1.5">{title}</p>
      {message && <p className="text-(--fs-compact) text-muted-foreground mb-5 max-w-xs leading-relaxed">{message}</p>}
      {action && <div>{action}</div>}
    </div>
  );
}
