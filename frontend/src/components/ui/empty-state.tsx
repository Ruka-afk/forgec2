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
    <div role="status" className={cn("flex flex-col items-center justify-center py-12 sm:py-16 md:py-20 text-center animate-fade-in", className)}>
      <div className="w-16 h-16 rounded-2xl bg-muted flex items-center justify-center mb-5" aria-hidden="true">
        <IconComponent className="w-8 h-8 text-muted-foreground/60" aria-hidden="true" />
      </div>
      <p className="text-(--font-size-body-sm) font-semibold text-foreground mb-1.5">{title}</p>
      {message && <p className="text-[12.5px] text-muted-foreground mb-5 max-w-xs leading-relaxed">{message}</p>}
      {action && <div>{action}</div>}
    </div>
  );
}
