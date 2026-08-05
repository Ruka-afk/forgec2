/** Pure login helpers — unit-tested without React. */

export interface LoginErrorBody {
  error?: string;
  message?: string;
  require_totp?: boolean;
  require_2fa?: boolean;
}

/**
 * Whether a fetch Response from POST /login indicates successful authentication.
 * Success: 302/303 redirect, or opaque redirect (status 0) under redirect:"manual".
 * Never treat generic 2xx as success (avoids swallowing 2FA challenges / error HTML).
 */
export function isLoginSuccessResponse(res: Pick<Response, "status" | "type" | "ok">): boolean {
  if (res.status === 302 || res.status === 303 || res.status === 307) return true;
  // Chromium: redirect:manual → type opaqueredirect, status 0
  if (res.status === 0 && (res.type === "opaqueredirect" || res.type === "opaque")) return true;
  return false;
}

/** Parse JSON login error body; returns null if not JSON-shaped. */
export function parseLoginErrorBody(data: unknown): string | null {
  if (!data || typeof data !== "object") return null;
  const o = data as LoginErrorBody;
  if (o.require_totp || o.require_2fa) {
    return o.error || o.message || "Two-factor authentication required";
  }
  if (typeof o.error === "string" && o.error) return o.error;
  if (typeof o.message === "string" && o.message) return o.message;
  return null;
}

/**
 * Safe post-login redirect target.
 * Rejects open redirects (//evil, protocol-relative, absolute URLs).
 */
export function safeNextPath(next: string | null | undefined, fallback = "/dashboard"): string {
  if (!next || typeof next !== "string") return fallback;
  const n = next.trim();
  if (!n.startsWith("/")) return fallback;
  if (n.startsWith("//") || n.startsWith("/\\")) return fallback;
  if (n.includes("://")) return fallback;
  if (n === "/login" || n.startsWith("/login?")) return fallback;
  // block encoded tricks
  try {
    const decoded = decodeURIComponent(n);
    if (decoded.startsWith("//") || decoded.includes("://")) return fallback;
  } catch {
    return fallback;
  }
  return n;
}
