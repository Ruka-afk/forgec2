"use client";

import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center space-y-4 animate-fade-slide-up">
        <div className="text-7xl font-bold bg-gradient-to-br from-muted-foreground/30 to-muted-foreground/60 bg-clip-text text-transparent select-none">404</div>
        <h1 className="text-xl font-bold text-foreground">Page not found</h1>
        <p className="text-sm text-muted-foreground">The page you&apos;re looking for doesn&apos;t exist.</p>
        <Link href="/dashboard" className={buttonVariants()}>
          Back to Dashboard
        </Link>
      </div>
    </div>
  );
}
