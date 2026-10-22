import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function PageToolbar({
  children,
  primary,
  sticky = false,
  className,
}: {
  children?: ReactNode;
  primary?: ReactNode;
  /** Pin the filter row under the top bar while a list page scrolls. */
  sticky?: boolean;
  className?: string;
}) {
  return (
    <div
      data-slot="page-toolbar"
      className={cn(
        "flex flex-col gap-3 rounded-xl border border-border/70 bg-card/90 px-3 py-2.5 shadow-none transition-[border-color,box-shadow] duration-150 sm:flex-row sm:items-center sm:justify-between sm:px-3.5",
        sticky && "sticky top-0 z-20 bg-card",
        className,
      )}
    >
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">{children}</div>
      {primary && <div className="flex w-full shrink-0 flex-wrap items-center justify-end gap-2 sm:w-auto">{primary}</div>}
    </div>
  );
}
