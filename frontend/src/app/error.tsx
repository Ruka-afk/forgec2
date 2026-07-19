"use client";

import Link from "next/link";
import { Button, buttonVariants } from "@/components/ui/button";

export default function ErrorPage({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center space-y-4 max-w-md px-4 animate-fade-slide-up">
        <div className="text-7xl font-bold text-destructive/30 dark:text-destructive/50 select-none">!</div>
        <h1 className="text-xl font-bold text-foreground">Something went wrong</h1>
        <p className="text-sm text-muted-foreground">{error.message || "An unexpected error occurred."}</p>
        <div className="flex items-center justify-center gap-3">
          <Button onClick={reset}>
            Try again
          </Button>
          <Link href="/dashboard" className={buttonVariants({ variant: "outline" })}>
            Dashboard
          </Link>
        </div>
      </div>
    </div>
  );
}
