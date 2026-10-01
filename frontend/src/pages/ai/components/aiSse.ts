/** SSE connect timeout for AI event streams. Fetch has no connect phase, so
 * a hung TCP/TLS handshake must be bounded separately from the idle watchdog. */
export const AI_SSE_CONNECT_TIMEOUT_MS = 30_000;

/** Creates a per-attempt controller while preserving the caller's cancel and
 * unmount signal for the lifetime of the stream. */
export function fetchAISSE(input: string, init: RequestInit, parent: AbortSignal): Promise<Response> {
  const attempt = new AbortController();
  const onParentAbort = () => attempt.abort(parent.reason);
  if (parent.aborted) {
    attempt.abort(parent.reason);
  } else {
    parent.addEventListener("abort", onParentAbort);
  }
  const timer = window.setTimeout(
    () => attempt.abort(new DOMException("SSE connect timeout", "TimeoutError")),
    AI_SSE_CONNECT_TIMEOUT_MS,
  );
  return fetch(input, { ...init, signal: attempt.signal }).finally(() => {
    window.clearTimeout(timer);
    parent.removeEventListener("abort", onParentAbort);
  });
}
