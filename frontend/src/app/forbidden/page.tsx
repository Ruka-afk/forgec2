"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import { ShieldAlert } from "lucide-react";

export default function Forbidden() {
  const { t } = useI18n();
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center space-y-4 animate-fade-slide-up">
        <div className="w-16 h-16 mx-auto rounded-2xl bg-destructive/10 flex items-center justify-center">
          <ShieldAlert className="w-8 h-8 text-destructive" aria-hidden="true" />
        </div>
        <h1 className="text-xl font-bold text-foreground">{t("forbidden.title")}</h1>
        <p className="text-sm text-muted-foreground max-w-sm">{t("forbidden.message")}</p>
        <Button render={<Link href="/dashboard" />}>
          {t("forbidden.back")}
        </Button>
      </div>
    </div>
  );
}