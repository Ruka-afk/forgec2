"use client";

/**
 * Unified frontend logger.
 *
 * - debug/info are dev-only; warn/error always emit.
 * - All structured args are redacted of known sensitive fields (tokens,
 *   cookies, authentication material, secrets) before being logged.
 * - Never pass raw request bodies, task parameters or auth headers here —
 *   even redaction is a last line of defense, not a license to log them.
 */

export type LogLevel = "debug" | "info" | "warn" | "error";

const LEVEL_ORDER: Record<LogLevel, number> = { debug: 0, info: 1, warn: 2, error: 3 };

/** dev-only sink for debug/info. */
function devOnly(level: LogLevel): boolean {
  const isDev = process.env.NODE_ENV !== "production";
  return isDev || LEVEL_ORDER[level] >= LEVEL_ORDER.warn;
}

const SENSITIVE_KEY_RE =
  /^(token|access_token|refresh_token|session|session_id|sessionid|cookie|cookies|csrf|csrf_token|x-csrf-token|authorization|auth|cookieheader|password|passwd|secret|api[_-]?key|apikey|private[_-]?key|client[_-]?secret|bearer|jwt)$/i;

const MAX_DEPTH = 4;

function redactValue(value: unknown, depth: number, keyHint?: string): unknown {
  if (keyHint && SENSITIVE_KEY_RE.test(keyHint)) return "[redacted]";
  if (depth > MAX_DEPTH) return value;
  if (Array.isArray(value)) return value.map((v) => redactValue(v, depth + 1));
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value)) out[k] = redactValue(v, depth + 1, k);
    return out;
  }
  return value;
}

/**
 * Deep-redact an argument: drops sensitive keys and their values at any
 * depth. Caller-supplied context objects are copied, so logging mutates
 * nothing.
 */
export function redact(value: unknown): unknown {
  return redactValue(value, 0);
}

export interface Logger {
  debug(fmt: string, ...args: unknown[]): void;
  info(fmt: string, ...args: unknown[]): void;
  warn(fmt: string, ...args: unknown[]): void;
  error(fmt: string, ...args: unknown[]): void;
  /** Bind a scope prefix, e.g. `logger.withScope("api")`. */
  withScope(scope: string): Logger;
}

function emit(level: LogLevel, scope: string | undefined, fmt: string, args: unknown[]): void {
  if (!devOnly(level)) return;
  const prefix = scope ? `[${scope}] ${fmt}` : fmt;
  const redactedArgs = args.map(redact);
  const consoleFn = level === "debug" ? console.debug : level === "info" ? console.info : level === "warn" ? console.warn : console.error;
  if (redactedArgs.length > 0) {
    consoleFn(prefix, ...redactedArgs);
  } else {
    consoleFn(prefix);
  }
}

function scopedLogger(scope: string | undefined): Logger {
  return {
    debug: (fmt, ...args) => emit("debug", scope, fmt, args),
    info: (fmt, ...args) => emit("info", scope, fmt, args),
    warn: (fmt, ...args) => emit("warn", scope, fmt, args),
    error: (fmt, ...args) => emit("error", scope, fmt, args),
    withScope: (next) => scopedLogger(scope ? `${scope}:${next}` : next),
  };
}

export const logger: Logger = scopedLogger(undefined);