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
      <div className="mt-3 flex items-center gap-3 rounded-xl border border-primary/20 bg-gradient-to-r from-primary/10 via-primary/5 to-transparent px-3.5 py-3 shadow-sm">
        <Spinner size="sm" />
        <div className="flex-1 space-y-1.5">
          <div className="text-xs font-semibold text-foreground">{t("generate.panel.compiling")}</div>
          <div className="h-1 overflow-hidden rounded-full bg-primary/15">
            <div className="h-full w-1/3 animate-[shimmer_1.2s_ease-in-out_infinite] rounded-full bg-gradient-to-r from-primary/40 via-primary to-primary/40" />
          </div>
        </div>
      </div>
    );
  }
  if (!result) return null;
  return (
    <Alert variant={isErrorString(result) ? "destructive" : "default"} className="mt-3 rounded-xl shadow-sm">
      <AlertDescription>
        <pre className="max-h-48 overflow-y-auto font-mono text-xs leading-5 whitespace-pre-wrap [scrollbar-width:thin]">{result}</pre>
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
      <Badge variant="warning" className="gap-1 rounded-full px-2.5 py-1 text-xs shadow-sm">
        <Loader2 className="size-3.5 animate-spin" /> {t("generate.badge_compiling")}
      </Badge>
    );
  }
  if (result) {
    return isErrorString(result) ? (
      <Badge variant="destructive" className="gap-1 rounded-full px-2.5 py-1 text-xs shadow-sm">
        <XCircle className="size-3.5" /> {t("generate.badge_failed")}
      </Badge>
    ) : (
      <Badge variant="success" className="gap-1 rounded-full px-2.5 py-1 text-xs shadow-sm">
        <CheckCircle2 className="size-3.5" /> {t("generate.badge_done")}
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="gap-1 rounded-full px-2.5 py-1 text-xs text-muted-foreground">
      <span className="size-1.5 rounded-full bg-muted-foreground/40" aria-hidden="true" />
      {t("generate.badge_ready")}
    </Badge>
  );
}
