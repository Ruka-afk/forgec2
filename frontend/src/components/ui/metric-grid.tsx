"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

const columns = {
  2: "grid-cols-1 sm:grid-cols-2",
  3: "grid-cols-1 sm:grid-cols-2 xl:grid-cols-3",
  4: "grid-cols-2 lg:grid-cols-4",
  5: "grid-cols-2 sm:grid-cols-3 xl:grid-cols-5",
  6: "grid-cols-2 sm:grid-cols-3 xl:grid-cols-6",
} as const;

export function MetricGrid({ children, count = 4, className }: { children: ReactNode; count?: keyof typeof columns; className?: string }) {
  return <div data-slot="metric-grid" className={cn("grid gap-3 sm:gap-4", columns[count], className)}>{children}</div>;
}
