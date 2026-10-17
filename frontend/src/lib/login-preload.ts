/**
 * Warm the lazily-split login route chunk.
 *
 * The router loads `LoginPage` with `React.lazy`, so its chunk (and the zod
 * vendor chunk it drags in) is not part of first paint anymore. When a live
 * session expires, `handleUnauthorized` schedules a redirect to `/login`; this
 * helper starts the same dynamic import while the debounce timer runs, so the
 * sign-in form paints from cache instead of waiting for a fresh chunk fetch
 * after the browser navigates.
 *
 * The import failing is not an error here: the router fetches the chunk again
 * when the route renders, and that render has its own error boundary.
 */
export function preloadLoginPage(): Promise<unknown> {
  return import("@/pages/login/LoginPage").catch(() => undefined);
}
