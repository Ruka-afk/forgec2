"use client";

import { useCallback } from "react";
import { toast } from "sonner";
import { normalizeError, type NormalizedError } from "@/lib/errors";
import { useI18n } from "@/lib/i18n";

/**
 * i18n-aware error toast: normalizes the error, resolves the locale message
 * and shows a sonner toast. Replaces scattered `toast.error(err.message)`
 * call sites. `fallbackKey` is used only for errors with no mapped key.
 */
export function useErrorToast() {
  const { t } = useI18n();

  const toastError = useCallback(
    (err: unknown, fallbackKey = "common.error"): NormalizedError => {
      const normalized = normalizeError(err);
      if (normalized.kind === "aborted") return normalized;
      const msg = normalized.i18nKey
        ? t(normalized.i18nKey, normalized.params as Record<string, number | string> | undefined)
        : normalized.message.trim() || t(fallbackKey);
      toast.error(msg);
      return normalized;
    },
    [t],
  );

  return { toastError };
}