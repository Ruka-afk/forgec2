"use client";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { AlertTriangle } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function AgentDetailError({ error, reset }: { error: Error; reset: () => void }) {
  const { t } = useI18n();
  return (
    <div className="flex items-center justify-center py-20">
      <Card className="max-w-md w-full p-8 text-center">
        <div className="flex justify-center mb-4">
          <div className="w-12 h-12 rounded-xl bg-destructive/10 flex items-center justify-center">
            <AlertTriangle className="w-6 h-6 text-destructive" />
          </div>
        </div>
        <h2 className="text-lg font-semibold text-foreground mb-2">{t("common.error")}</h2>
        <p className="text-sm text-muted-foreground mb-6">{error.message}</p>
        <Button onClick={reset} className="text-sm">{t("common.try_again")}</Button>
      </Card>
    </div>
  );
}
