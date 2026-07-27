"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

export default function NotFound() {
  const { t } = useI18n();
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center space-y-4 animate-fade-slide-up">
        <div className="text-7xl font-bold bg-gradient-to-br from-muted-foreground/30 to-muted-foreground/60 bg-clip-text text-transparent select-none">404</div>
        <h1 className="text-xl font-bold text-foreground">{t("notfound.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("notfound.message")}</p>
        <Button render={<Link href="/dashboard" />}>
          {t("notfound.back")}
        </Button>
      </div>
    </div>
  );
}
