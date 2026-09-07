"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Card, CardAction, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";

/**
 * F3 blueprint — persistent side panel that reflects the current craft
 * (build / stager / profile / report). Sticky-friendly; parent supplies the
 * live-summary content.
 */
export function CraftPanel({
  title,
  badge,
  children,
  footer,
  className,
  bodyClassName,
}: {
  title: ReactNode;
  badge?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <Card className={cn("flex max-h-full flex-col overflow-hidden shadow-1", className)}>
      <CardHeader className="border-b py-3">
        <CardTitle className="mono-eyebrow text-muted-foreground font-normal">{title}</CardTitle>
        {badge && <CardAction>{badge}</CardAction>}
      </CardHeader>
      <CardContent className={cn("min-h-0 flex-1 p-0", bodyClassName)}>
        <ScrollArea className="h-full">
          <div className="p-3.5">{children}</div>
        </ScrollArea>
      </CardContent>
      {footer && <CardFooter className="px-3.5 py-3">{footer}</CardFooter>}
    </Card>
  );
}