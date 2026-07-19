"use client";

import { useEffect } from "react";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

export default function RecordingError({ error, reset }: { error: Error; reset: () => void }) {
  useEffect(() => { console.error(error); }, [error]);
  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 flex flex-col items-center justify-center py-20 text-center animate-fade-slide-up">
      <AlertTriangle className="w-10 h-10 text-destructive mb-4" />
      <h2 className="text-xl font-semibold text-foreground mb-2">Something went wrong</h2>
      <p className="text-sm text-muted-foreground mb-6">{error.message}</p>
      <Button onClick={reset}>Try Again</Button>
    </div>
  );
}
