"use client";

import { CircleAlert } from "lucide-react";
import { cn } from "@/lib/utils";

export function ErrorState({ title, message, icon, action, className }: {
  title?: string;
  message?: React.ReactNode;
  icon?: React.ComponentType<{ className?: string }>;
  action?: React.ReactNode;
  className?: string;
}) {
  const IconComponent = icon || CircleAlert;
  return (
    <div className={cn("flex items-start gap-3 rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive animate-fade-in", className)}>
      <IconComponent className="w-4 h-4 mt-0.5 shrink-0" aria-hidden="true" />
      <div className="flex-1 min-w-0">
        {title && <p className="font-medium mb-0.5">{title}</p>}
        {message && <div className="leading-relaxed">{message}</div>}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}
