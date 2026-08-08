"use client";

import { cn } from "@/lib/utils";

export function FieldError({
  id,
  children,
  className,
}: {
  id?: string;
  children: React.ReactNode;
  className?: string;
}) {
  if (!children) return null;
  return (
    <p id={id} role="alert" className={cn("text-xs text-destructive mt-1", className)}>
      {children}
    </p>
  );
}
