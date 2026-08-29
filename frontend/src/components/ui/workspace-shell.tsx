"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function WorkspaceShell({
  header,
  toolbar,
  sidebar,
  statusbar,
  children,
  className,
}: {
  header?: ReactNode;
  toolbar?: ReactNode;
  sidebar?: ReactNode;
  statusbar?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section data-slot="workspace-shell" className={cn("flex min-h-0 flex-1 flex-col overflow-hidden bg-background", className)}>
      {header && <div className="shrink-0 border-b border-border/75 bg-card px-4 py-3 sm:px-5">{header}</div>}
      {toolbar && <div className="shrink-0 border-b border-border/70 bg-card/95 px-3 py-2 sm:px-4">{toolbar}</div>}
      <div className="flex min-h-0 flex-1">
        {sidebar && <aside className="hidden w-64 shrink-0 overflow-y-auto border-r border-border/75 bg-muted/35 lg:block">{sidebar}</aside>}
        <div className="min-h-0 min-w-0 flex-1 overflow-auto">{children}</div>
      </div>
      {statusbar && <div className="shrink-0 border-t border-border/75 bg-card px-3 py-1.5">{statusbar}</div>}
    </section>
  );
}
