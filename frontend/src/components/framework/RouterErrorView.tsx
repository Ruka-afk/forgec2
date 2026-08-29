"use client";

import { useRouteError, Link } from "react-router-dom";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Bug } from "lucide-react";

/**
 * Route-level error view wired into EVERY createBrowserRouter node's
 * `errorElement`. Covers loader/render failures for the whole app; the
 * top-level ErrorBoundary (ClientProvider) still guards render crashes
 * outside of routing.
 */
export default function RouterErrorView() {
  const { t } = useI18n();
  const error = useRouteError();
  const message = error instanceof Error ? error.message : undefined;

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] p-8 text-center">
      <div className="mb-4 flex size-14 items-center justify-center rounded-lg bg-destructive/10">
        <Bug className="size-7 text-destructive" />
      </div>
      <h2 className="text-lg font-semibold text-foreground mb-2">{t("error.boundary_title")}</h2>
      <p className="text-sm text-muted-foreground mb-4 max-w-md break-words">
        {message || t("error.boundary_message")}
      </p>
      <div className="flex gap-2">
        <Button size="sm" variant="outline" onClick={() => window.location.reload()}>
          {t("error.try_again")}
        </Button>
        <Button size="sm" render={<Link to="/dashboard" />}>
          {t("notfound.back")}
        </Button>
      </div>
    </div>
  );
}