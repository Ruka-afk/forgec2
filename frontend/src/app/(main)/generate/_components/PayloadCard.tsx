"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";
import { ChevronDown } from "lucide-react";

/**
 * PayloadCard — unified card anatomy for the payload workshop:
 * header (icon chip + title/subtitle + status badge) · body · footer slot.
 */
export function PayloadCard({
  icon,
  tint = "bg-primary/10 text-primary",
  title,
  subtitle,
  badge,
  footer,
  children,
  className,
}: {
  icon: ReactNode;
  tint?: string;
  title: string;
  subtitle?: string;
  badge?: ReactNode;
  footer?: ReactNode;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <Card className={cn("flex flex-col p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow", className)}>
      <div className="mb-4 flex items-start justify-between gap-3 border-b border-border pb-4">
        <div className="flex min-w-0 items-center gap-x-3">
          <div className={cn("grid h-10 w-10 shrink-0 place-items-center rounded-lg ring-1 ring-border/50", tint)}>
            {icon}
          </div>
          <div className="min-w-0">
            <div className="truncate text-base font-semibold text-foreground">{title}</div>
            {subtitle && <div className="truncate text-xs text-muted-foreground">{subtitle}</div>}
          </div>
        </div>
        {badge}
      </div>
      <div className="flex-1 space-y-3">{children}</div>
      {footer && <div className="mt-4 border-t border-border pt-3">{footer}</div>}
    </Card>
  );
}

/** Consistent form label used across all payload panels. */
export function FieldLabel({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={cn("mb-1.5 block text-xs font-semibold text-muted-foreground", className)}>
      {children}
    </span>
  );
}

/** Collapsible advanced-options disclosure replacing native <details>. */
export function AdvancedSection({
  title,
  children,
  defaultOpen = false,
  className,
}: {
  title: string;
  children: ReactNode;
  defaultOpen?: boolean;
  className?: string;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className={cn("mt-2 border-t border-border pt-2", className)}>
      <Collapsible open={open} onOpenChange={setOpen}>
        <CollapsibleTrigger className="group flex w-full items-center gap-1.5 text-xs text-muted-foreground transition-colors select-none hover:text-primary">
          <ChevronDown className={cn("h-4 w-4 transition-transform duration-200", open && "rotate-180")} />
          <span className="font-medium">{title}</span>
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-2 space-y-2">{children}</CollapsibleContent>
      </Collapsible>
    </div>
  );
}
