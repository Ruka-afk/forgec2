"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";

/**
 * F6 blueprint — grouped settings/management section card (title + optional
 * icon + description + actions, body, optional footer).
 */
export function ConfigSection({
  title,
  description,
  icon,
  actions,
  children,
  footer,
  className,
  bodyClassName,
}: {
  title: ReactNode;
  description?: ReactNode;
  icon?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <Card className={cn("shadow-1", className)}>
      <CardHeader className="border-b">
        <div className="flex items-start gap-3">
          {icon && (
            <div className="mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary ring-1 ring-primary/10">
              {icon}
            </div>
          )}
          <div>
            <CardTitle className="text-sm">{title}</CardTitle>
            {description && <CardDescription className="mt-0.5 text-xs leading-relaxed">{description}</CardDescription>}
          </div>
        </div>
        {actions && <CardAction>{actions}</CardAction>}
      </CardHeader>
      {children && <CardContent className={cn("py-4", bodyClassName)}>{children}</CardContent>}
      {footer && <CardFooter className="justify-between">{footer}</CardFooter>}
    </Card>
  );
}