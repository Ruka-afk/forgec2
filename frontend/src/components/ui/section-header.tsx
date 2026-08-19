"use client";

import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

interface SectionHeaderProps {
  title: ReactNode;
  description?: ReactNode;
  /** Right-aligned controls (buttons, links). */
  action?: ReactNode;
  className?: string;
}

/**
 * Standalone in-page section title with the canonical section size
 * (--fs-section). Use inside cards/page sections instead of hand-rolled
 * `text-lg font-semibold` headings so titles stay uniform across pages.
 */
export function SectionHeader({ title, description, action, className }: SectionHeaderProps) {
  return (
    <div className={cn("flex items-start justify-between gap-3", className)}>
      <div className="min-w-0">
        <h2 className="text-(--fs-section) font-semibold text-foreground leading-tight">{title}</h2>
        {description && <p className="text-sm text-muted-foreground mt-0.5">{description}</p>}
      </div>
      {action && <div className="flex items-center gap-2 shrink-0">{action}</div>}
    </div>
  );
}
