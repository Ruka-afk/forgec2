"use client";

import type { ReactNode } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { useI18n } from "@/lib/i18n";
import { CheckCircle2, Loader2, XCircle } from "lucide-react";

/**
 * Every panel stores "" in state.result on success (the one-liner panel uses
 * the literal sentinel "success"), so any other non-empty string is a build
 * error. The old prefix check ("Error"/"ERROR") missed common failures like
 * "HTTP 500" or compiler output and mislabeled them as success.
 */
function isErrorString(result: ReactNode): boolean {
  if (typeof result !== "string") return false;
  const s = result.trim();
  return s !== "" && s !== "success" && s !== "OK";
}

/**
 * BuildResult — unified in-card build feedback:
 * busy → compiling strip with progress · result → success/error alert.
 */
export function BuildResult({ busy, result }: { busy?: boolean; result?: ReactNode }) {
  const { t } = useI18n();
  if (busy) {
    return (
      <div className="mt-3 flex items-center gap-2.5 rounded-lg border border-primary/20 bg-primary/5 px-3 py-2.5">
        <Spinner size="sm" />
        <div className="flex-1 space-y-1">
          <div className="text-xs font-medium text-foreground">{t("generate.panel.compiling")}</div>
          <div className="h-1 overflow-hidden rounded-full bg-primary/15">
            <div className="h-full w-1/2 animate-pulse rounded-full bg-primary/60" />
          </div>
        </div>
      </div>
    );
  }
  if (!result) return null;
  return (
    <Alert variant={isErrorString(result) ? "destructive" : "default"} className="mt-3">
      <AlertDescription>
        <pre className="font-mono whitespace-pre-wrap">{result}</pre>
      </AlertDescription>
    </Alert>
  );
}

/**
 * BuildStatusBadge — live state chip for the card header:
 * Ready · Compiling · Done · Failed.
 */
export function BuildStatusBadge({ busy, result }: { busy?: boolean; result?: ReactNode }) {
  const { t } = useI18n();
  if (busy) {
    return (
      <Badge variant="warning" className="gap-1 text-xs">
        <Loader2 className="size-3.5 animate-spin" /> {t("generate.badge_compiling")}
      </Badge>
    );
  }
  if (result) {
    return isErrorString(result) ? (
      <Badge variant="destructive" className="gap-1 text-xs">
        <XCircle className="size-3.5" /> {t("generate.badge_failed")}
      </Badge>
    ) : (
      <Badge variant="success" className="gap-1 text-xs">
        <CheckCircle2 className="size-3.5" /> {t("generate.badge_done")}
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="gap-1 text-xs text-muted-foreground">
      {t("generate.badge_ready")}
    </Badge>
  );
}
