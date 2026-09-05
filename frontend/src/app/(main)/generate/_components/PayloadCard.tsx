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
    <Card className={cn("group/card flex flex-col overflow-hidden border-border/60 bg-gradient-to-b from-card to-card/80 p-0 shadow-sm backdrop-blur-sm transition-all duration-200 hover:shadow-md hover:shadow-primary/5 hover:border-primary/15 dark:from-card dark:to-card/60", className)}>
      <div className="flex items-center justify-between gap-2 border-b border-border/60 bg-gradient-to-r from-muted/40 via-muted/20 to-transparent px-3.5 py-2.5">
        <div className="flex min-w-0 items-center gap-x-2.5">
          <div className={cn("grid size-8 shrink-0 place-items-center rounded-lg shadow-sm ring-1 ring-border/40 transition-transform duration-200 group-hover/card:scale-105", tint)}>
            {icon}
          </div>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold tracking-tight text-foreground">{title}</div>
            {subtitle && <div className="truncate text-xs leading-4 text-muted-foreground">{subtitle}</div>}
          </div>
        </div>
        {badge}
      </div>
      <div className="flex-1 space-y-3 p-3.5">{children}</div>
      {footer && <div className="border-t border-border/60 bg-muted/20 px-3.5 py-3 backdrop-blur-sm">{footer}</div>}
    </Card>
  );
}

/** Consistent form label used across all payload panels. */
export function FieldLabel({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={cn("mb-1.5 block text-xs font-semibold tracking-wide text-muted-foreground/90", className)}>
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
    <div className={cn("mt-2 rounded-lg border border-border/50 bg-muted/20 p-2.5 transition-colors hover:bg-muted/30", className)}>
      <Collapsible open={open} onOpenChange={setOpen}>
        <CollapsibleTrigger className="group flex w-full items-center gap-1.5 text-xs font-medium text-muted-foreground transition-colors select-none hover:text-foreground">
          <div className="grid size-5 place-items-center rounded-md bg-background ring-1 ring-border/50 transition-colors group-hover:ring-primary/20">
            <ChevronDown className={cn("size-3 transition-transform duration-200", open && "rotate-180")} />
          </div>
          <span>{title}</span>
          <span className="ml-auto text-[10px] tracking-widest text-muted-foreground/60">{open ? "收起" : "展开"}</span>
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-2 space-y-2.5 border-t border-border/50 pt-2.5">{children}</CollapsibleContent>
      </Collapsible>
    </div>
  );
}
