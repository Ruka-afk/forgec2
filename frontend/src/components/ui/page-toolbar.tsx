"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function PageToolbar({
  children,
  primary,
  className,
}: {
  children?: ReactNode;
  primary?: ReactNode;
  className?: string;
}) {
  return (
    <div
      data-slot="page-toolbar"
      className={cn(
        "flex flex-col gap-3 rounded-xl border border-border/80 bg-card px-3 py-3 shadow-sm sm:flex-row sm:items-center sm:justify-between sm:px-4",
        className,
      )}
    >
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">{children}</div>
      {primary && <div className="flex w-full shrink-0 flex-wrap items-center justify-end gap-2 sm:w-auto">{primary}</div>}
    </div>
  );
}
