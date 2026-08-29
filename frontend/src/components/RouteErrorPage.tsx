"use client";

import { useEffect } from "react";
import { useI18n } from "@/lib/i18n";
import { logger } from "@/lib/logger";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

export default function RouteErrorPage({
  error,
  reset,
}: {
  error: Error;
  reset: () => void;
}) {
  const { t } = useI18n();

  useEffect(() => {
    if (process.env.NODE_ENV === "development") {
      logger.error("route error", error);
    }
  }, [error]);

  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="flex justify-center mb-4">
        <div className="flex size-12 items-center justify-center rounded-lg bg-destructive/10">
          <AlertTriangle className="size-6 text-destructive" />
        </div>
      </div>
      <h2 className="text-lg font-semibold text-foreground mb-2">{t("common.error")}</h2>
      <p className="text-sm text-muted-foreground mb-6">{error.message}</p>
      <Button onClick={reset} className="text-sm">{t("common.try_again")}</Button>
    </div>
  );
}
