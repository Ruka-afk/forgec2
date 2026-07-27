"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

export default function ErrorPage({ error, reset }: { error: Error; reset: () => void }) {
  const { t } = useI18n();
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center space-y-4 max-w-md px-4 animate-fade-slide-up">
        <div className="text-7xl font-bold text-destructive/30 dark:text-destructive/50 select-none">!</div>
        <h1 className="text-xl font-bold text-foreground">{t("error.title")}</h1>
        <p className="text-sm text-muted-foreground">{error.message || t("error.message")}</p>
        <div className="flex items-center justify-center gap-3">
          <Button onClick={reset}>
            {t("error.try_again")}
          </Button>
          <Button variant="outline" render={<Link href="/dashboard" />}>
            {t("error.dashboard")}
          </Button>
        </div>
      </div>
    </div>
  );
}
