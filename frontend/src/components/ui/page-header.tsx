"use client";

import { cn } from "@/lib/utils";

export function PageHeader({ title, subtitle, icon, children, className }: {
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  icon?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col sm:flex-row sm:items-center justify-between mb-6 sm:mb-8 gap-3", className)}>
      <div className="flex items-center gap-x-3.5">
        {icon && <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center shrink-0">{icon}</div>}
        <div>
          <h1 className="text-[1.375rem] font-semibold tracking-tight text-foreground leading-tight">{title}</h1>
          {subtitle && <p className="text-[13px] text-muted-foreground mt-0.5">{subtitle}</p>}
        </div>
      </div>
      {children && <div className="flex items-center gap-2 shrink-0">{children}</div>}
    </div>
  );
}
