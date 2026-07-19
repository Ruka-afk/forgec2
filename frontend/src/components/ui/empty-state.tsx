"use client";

import { Inbox } from "lucide-react";
import { Button } from "@/components/ui/button";

export function EmptyState({ icon, title, message, action }: {
  icon?: React.ComponentType<{ className?: string }>;
  title: string;
  message?: string;
  action?: { label: string; onClick: () => void };
}) {
  const IconComponent = icon || Inbox;
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center animate-fade-in">
      <div className="w-14 h-14 rounded-xl bg-muted flex items-center justify-center mb-4">
        <IconComponent className="w-7 h-7 text-muted-foreground/70" />
      </div>
      <p className="text-sm font-semibold text-foreground mb-1">{title}</p>
      {message && <p className="text-xs text-muted-foreground mb-4 max-w-xs leading-relaxed">{message}</p>}
      {action && (
        <Button onClick={action.onClick} size="sm">
          {action.label}
        </Button>
      )}
    </div>
  );
}
