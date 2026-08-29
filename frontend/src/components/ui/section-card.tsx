"use client";

import Link from "next/link";
import { ChevronDown } from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

interface SectionCardProps {
  title: ReactNode;
  icon?: ReactNode;
  /** Optional numeric badge shown next to the title (e.g. item count). */
  count?: number;
  /** Small muted line rendered under the title. */
  description?: ReactNode;
  /** Right-aligned controls (buttons, triggers). */
  action?: ReactNode;
  /** Renders a "view all" link on the right. */
  href?: string;
  linkLabel?: string;
  /** Apply the command-center left accent bar to the header. */
  headerAccent?: boolean;
  /** Make the header a collapse trigger. */
  collapsible?: boolean;
  defaultOpen?: boolean;
  /** Controlled open state: when defined, Base UI ignores defaultOpen —
   * required for parent-driven toggles (e.g. a "Load" action button that
   * must open the panel programmatically). */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  className?: string;
  children: ReactNode;
}

/**
 * Unified panel/section container used across the dashboard and agent detail
 * page. Replaces the hand-rolled `Card + CardHeader` boilerplate that was
 * duplicated (with drifting styling) across ~8 call sites.
 */
export function SectionCard({
  title,
  icon,
  count,
  description,
  action,
  href,
  linkLabel,
  headerAccent = true,
  collapsible = false,
  defaultOpen = true,
  open,
  onOpenChange,
  className,
  children,
}: SectionCardProps) {
  const header = (
    <CardHeader
      className={cn(
        "flex flex-row items-center justify-between gap-2 border-b border-border px-4 py-2.5",
        headerAccent && "panel-header-accent",
      )}
    >
      <div className="flex min-w-0 items-center gap-2">
        <h3 className="flex min-w-0 items-center gap-2 text-(--fs-section) font-semibold text-foreground">
          {icon}
          <span className="truncate">{title}</span>
          {typeof count === "number" && (
            <Badge variant={count > 0 ? "secondary" : "outline"} className="font-mono">
              {count}
            </Badge>
          )}
        </h3>
        {description && (
          <p className="truncate text-(--fs-micro-sm) text-muted-foreground">{description}</p>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {action}
        {collapsible && <ChevronDown className="size-2.5 text-muted-foreground/100" />}
        {href && linkLabel && (
          <Link href={href} className="text-xs text-primary hover:underline">
            {linkLabel}
          </Link>
        )}
      </div>
    </CardHeader>
  );

  if (collapsible) {
    // Pass `open` only when the caller controls it: Base UI flips to
    // controlled mode the moment the prop is defined (even undefined via
    // spread), so conditionally include it.
    const collapsibleProps = open !== undefined ? { open, onOpenChange } : { defaultOpen, onOpenChange };
    return (
      <Card className={cn("overflow-hidden", className)}>
        <Collapsible {...collapsibleProps}>
          <CollapsibleTrigger className="w-full text-left">{header}</CollapsibleTrigger>
          <CollapsibleContent>{children}</CollapsibleContent>
        </Collapsible>
      </Card>
    );
  }

  return (
    <Card className={cn("overflow-hidden", className)}>
      {header}
      {children}
    </Card>
  );
}
