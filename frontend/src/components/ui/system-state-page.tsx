"use client";

import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export function SystemStatePage({
  code,
  icon,
  title,
  message,
  action,
  tone = "primary",
}: {
  code?: string;
  icon?: ReactNode;
  title: ReactNode;
  message?: ReactNode;
  action?: ReactNode;
  tone?: "primary" | "destructive";
}) {
  return (
    <main className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background px-5 py-12">
      <div aria-hidden="true" className="absolute inset-0 opacity-[0.2] [background-image:linear-gradient(var(--border)_1px,transparent_1px),linear-gradient(90deg,var(--border)_1px,transparent_1px)] [background-size:40px_40px] [mask-image:radial-gradient(circle_at_center,black,transparent_72%)]" />
      <Card className="relative w-full max-w-lg items-center px-6 py-10 text-center shadow-lg sm:px-10">
        {code && <div className="mono-eyebrow text-primary">{code}</div>}
        {icon && <div className={cn("icon-well size-14", tone === "destructive" ? "bg-destructive/10 text-destructive" : "bg-primary/10 text-primary")}>{icon}</div>}
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">{title}</h1>
          {message && <p className="mx-auto mt-2 max-w-sm text-sm leading-6 text-muted-foreground">{message}</p>}
        </div>
        {action && <div className="mt-1">{action}</div>}
      </Card>
    </main>
  );
}
