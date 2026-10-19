import { getRateLimitRetryAfter } from "@/lib/api";

/**
 * Error normalization: collapses every failure surface (fetch network errors,
 * ApiError statuses, timeouts, aborts) into one small discriminated shape the
 * UI can render without per-page decoding.
 *
 * `i18nKey` is set when a locale-aware message exists; UI code should resolve
 * it through `t(i18nKey, params)` (see useErrorToast). `message` is a stable
 * English fallback for non-localizable raw errors (e.g. backend-provided
 * validation text) and for headless call sites.
 */

type ErrorKind =
  | "network"
  | "timeout"
  | "session"
  | "forbidden"
  | "rate_limited"
  | "server"
  | "validation"
  | "aborted"
  | "unknown";

export interface NormalizedError {
  kind: ErrorKind;
  status?: number;
  retryable: boolean;
  /** Locale-aware message key, resolved via t(key, params). */
  i18nKey?: string;
  /** Params for the i18nKey template. */
  params?: Record<string, number | string>;
  /** English fallback message (also the raw backend error text). */
  message: string;
}

/** Structural check for ApiError-like objects without importing api.ts. */
function isApiErrorLike(err: unknown): err is { status: number; message: string } {
  if (typeof err !== "object" || err === null) return false;
  const maybe = err as { status?: unknown; message?: unknown };
  return typeof maybe.status === "number" && typeof maybe.message === "string";
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException ? err.name === "AbortError" : err instanceof Error && err.name === "AbortError";
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export function normalizeError(err: unknown): NormalizedError {
  if (isAbortError(err)) {
    return { kind: "aborted", retryable: false, message: "Aborted" };
  }

  if (isApiErrorLike(err)) {
    if (err.status === 401) {
      // Session expiry is surfaced by handleUnauthorized (redirect to /login);
      // the message is informational only.
      return { kind: "session", status: 401, retryable: false, i18nKey: "common.session_expired", message: err.message };
    }
    if (err.status === 403) {
      return { kind: "forbidden", status: 403, retryable: false, i18nKey: "common.forbidden", message: err.message };
    }
    if (err.status === 429) {
      const seconds = getRateLimitRetryAfter();
      return {
        kind: "rate_limited",
        status: 429,
        retryable: true,
        i18nKey: "common.rate_limited",
        params: { seconds },
        message: err.message,
      };
    }
    if (err.status >= 500) {
      return { kind: "server", status: err.status, retryable: true, i18nKey: "common.server_error", message: err.message };
    }
    // Other 4xx: the backend error body (outcome-specific) is the message.
    return { kind: "validation", status: err.status, retryable: false, message: err.message };
  }

  const raw = messageOf(err);

  if (err instanceof TypeError) {
    // fetch rejects with TypeError on connection failure in every major browser.
    return { kind: "network", retryable: true, i18nKey: "common.network_error", message: raw };
  }

  if (/timed out|timeout/i.test(raw)) {
    return { kind: "timeout", retryable: true, i18nKey: "common.timeout_error", message: raw };
  }

  if (/networkerror|failed to fetch|load failed|socket hang up|connect/i.test(raw)) {
    return { kind: "network", retryable: true, i18nKey: "common.network_error", message: raw };
  }

  return { kind: "unknown", retryable: false, message: raw };
}

export type ErrorTextFn = (key: string, params?: Record<string, string | number>) => string;

/**
 * Resolve the operator-facing text for a normalized error: a mapped locale key
 * first, backend/validation text otherwise, and `fallback` only for a
 * message-less failure. Shared by `useErrorToast` (toasts) and inline error
 * surfaces via `errorText`, so the two rules cannot drift. The fallback is a
 * thunk so a call site never translates a key it will not show.
 */
export function resolveErrorText(t: ErrorTextFn, normalized: NormalizedError, fallback: () => string): string {
  if (normalized.i18nKey) {
    return t(normalized.i18nKey, normalized.params as Record<string, string | number> | undefined);
  }
  return normalized.message.trim() || fallback();
}

/** Normalize + resolve in one step for inline error surfaces (text only, no toast). */
export function errorText(t: ErrorTextFn, err: unknown, fallback: () => string): string {
  return resolveErrorText(t, normalizeError(err), fallback);
}