import { describe, it, expect } from "vitest";
import { normalizeError } from "./errors";

function apiError(status: number, message: string): Error & { status: number } {
  const e = new Error(message) as Error & { status: number };
  e.status = status;
  return e;
}

describe("normalizeError", () => {
  it("maps aborts to non-retryable aborted", () => {
    const e = new DOMException("Aborted", "AbortError");
    const n = normalizeError(e);
    expect(n.kind).toBe("aborted");
    expect(n.retryable).toBe(false);
    expect(n.i18nKey).toBeUndefined();
  });

  it("maps 401 to session expiry", () => {
    const n = normalizeError(apiError(401, "session invalid"));
    expect(n.kind).toBe("session");
    expect(n.i18nKey).toBe("common.session_expired");
    expect(n.retryable).toBe(false);
  });

  it("maps 403 to forbidden", () => {
    const n = normalizeError(apiError(403, "no permission"));
    expect(n.kind).toBe("forbidden");
    expect(n.i18nKey).toBe("common.forbidden");
  });

  it("maps 429 to rate limited with retry seconds", () => {
    const n = normalizeError(apiError(429, "slow down"));
    expect(n.kind).toBe("rate_limited");
    expect(n.i18nKey).toBe("common.rate_limited");
    expect(typeof n.params?.seconds).toBe("number");
    expect(n.retryable).toBe(true);
  });

  it("maps 5xx to server error", () => {
    const n = normalizeError(apiError(500, "boom"));
    expect(n.kind).toBe("server");
    expect(n.i18nKey).toBe("common.server_error");
    expect(n.retryable).toBe(true);
  });

  it("keeps the backend message for other 4xx", () => {
    const n = normalizeError(apiError(400, "invalid host"));
    expect(n.kind).toBe("validation");
    expect(n.message).toBe("invalid host");
    expect(n.i18nKey).toBeUndefined();
  });

  it("maps TypeError to network", () => {
    const n = normalizeError(new TypeError("Failed to fetch"));
    expect(n.kind).toBe("network");
    expect(n.i18nKey).toBe("common.network_error");
    expect(n.retryable).toBe(true);
  });

  it("maps timeout text to timeout", () => {
    const n = normalizeError(new Error("Request timed out after 30000ms"));
    expect(n.kind).toBe("timeout");
    expect(n.i18nKey).toBe("common.timeout_error");
    expect(n.retryable).toBe(true);
  });

  it("fallback keeps raw message as unknown", () => {
    const n = normalizeError("weird payload");
    expect(n.kind).toBe("unknown");
    expect(n.message).toBe("weird payload");
  });
});