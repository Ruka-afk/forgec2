"use client";

import { cn } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/ui/spinner";
import { EmptyState } from "@/components/ui/empty-state";
import { Button } from "@/components/ui/button";
import { AlertCircle, RefreshCw } from "lucide-react";

interface DataStateProps {
  loading?: boolean;
  error?: string | null;
  empty?: boolean;
  emptyIcon?: React.ComponentType<{ className?: string }>;
  emptyTitle?: string;
  emptyMessage?: string;
  emptyAction?: React.ReactNode;
  onRetry?: () => void;
  onDismiss?: () => void;
  loadingSkeleton?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function DataState({
  loading = false,
  error = null,
  empty = false,
  emptyIcon,
  emptyTitle,
  emptyMessage,
  emptyAction,
  onRetry,
  onDismiss,
  loadingSkeleton,
  children,
  className,
}: DataStateProps) {
  const { t } = useI18n();
  if (loading) {
    return loadingSkeleton ? <>{loadingSkeleton}</> : <DataSpinner className={className} />;
  }

  if (error) {
    return (
      <DataError
        message={error}
        onRetry={onRetry}
        onDismiss={onDismiss}
        className={className}
      />
    );
  }

  if (empty) {
    return (
      <EmptyState
        icon={emptyIcon}
        title={emptyTitle || t("common.no_data")}
        message={emptyMessage}
        action={emptyAction}
      />
    );
  }

  return <>{children}</>;
}

interface DataSpinnerProps {
  message?: string;
  className?: string;
}

export function DataSpinner({ message, className }: DataSpinnerProps) {
  return (
    <div className={cn("flex flex-col items-center justify-center py-12 sm:py-16 md:py-20 animate-fade-in", className)}>
      <Spinner size="md" />
      {message && <p className="text-xs text-muted-foreground mt-3">{message}</p>}
    </div>
  );
}

interface DataErrorProps {
  message?: string;
  onRetry?: () => void;
  onDismiss?: () => void;
  className?: string;
}

export function DataError({ message, onRetry, onDismiss, className }: DataErrorProps) {
  const { t } = useI18n();
  return (
    <div
      role="alert"
      className={cn(
        "flex flex-col items-center justify-center py-16 text-center animate-fade-in",
        className
      )}
    >
      <div className="size-14 rounded-lg bg-destructive/10 flex items-center justify-center mb-4">
        <AlertCircle className="size-7 text-destructive" aria-hidden="true" />
      </div>
      <p className="text-sm font-semibold text-foreground mb-1">{message || t("common.error")}</p>
      <p className="text-xs text-muted-foreground mb-4 max-w-xs">{t("common.error_hint")}</p>
      <div className="flex items-center gap-2">
        {onRetry && (
          <Button onClick={onRetry} size="sm" variant="outline" className="min-h-11 px-4 sm:min-h-7 sm:px-2.5">
            <RefreshCw className="size-3 mr-1.5" aria-hidden="true" />
            {t("common.try_again")}
          </Button>
        )}
        {onDismiss && (
          <Button onClick={onDismiss} size="sm" variant="ghost" className="min-h-11 px-4 sm:min-h-7 sm:px-2.5">
            {t("common.dismiss")}
          </Button>
        )}
      </div>
    </div>
  );
}
