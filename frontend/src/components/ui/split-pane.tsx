"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function SplitPane({
  children,
  aside,
  side = "right",
  className,
}: {
  children: ReactNode;
  aside: ReactNode;
  side?: "left" | "right";
  className?: string;
}) {
  return (
    <div className={cn("grid min-h-0 flex-1 gap-4 lg:grid-cols-[18rem_minmax(0,1fr)] xl:gap-5", side === "right" && "lg:grid-cols-[minmax(0,1fr)_18rem]", className)}>
      {side === "left" && <aside className="min-h-0 min-w-0">{aside}</aside>}
      <div className="min-h-0 min-w-0">{children}</div>
      {side === "right" && <aside className="min-h-0 min-w-0">{aside}</aside>}
    </div>
  );
}
