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
    <section data-slot="workspace-shell" className={cn("isolate flex min-h-0 flex-1 flex-col overflow-hidden bg-background overscroll-none", className)}>
      {header && <div className="shrink-0 border-b border-border/75 bg-card px-4 py-3 sm:px-5">{header}</div>}
      {toolbar && <div className="sticky top-0 z-20 shrink-0 border-b border-border/70 bg-card/95 px-3 py-2 backdrop-blur sm:px-4">{toolbar}</div>}
      <div className="flex min-h-0 flex-1">
        {sidebar && <aside className="hidden w-64 shrink-0 overscroll-contain overflow-y-auto border-r border-border/75 bg-card scrollbar-thin lg:block">{sidebar}</aside>}
        <div className="content-auto min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain [scrollbar-gutter:stable]">{children}</div>
      </div>
      {statusbar && <div className="shrink-0 border-t border-border/75 bg-card px-3 py-1.5 pb-[calc(0.375rem+env(safe-area-inset-bottom))]">{statusbar}</div>}
    </section>
  );
}
