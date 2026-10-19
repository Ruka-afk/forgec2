import { ROUTE_PAGE_IMPORTS } from "./route-imports";

/**
 * Pages operators open first, warmed once the browser goes idle. Deliberately
 * three: each entry is a full page chunk, and speculative bandwidth on a C2
 * console should stay a rounding error.
 */
const IDLE_PREFETCH_PATHS = ["/dashboard", "/beacons", "/listeners"];

/** Route ids whose import already started (hover fires repeatedly). */
const started = new Set<string>();

function saveDataEnabled(): boolean {
  const connection = (navigator as { connection?: { saveData?: boolean } }).connection;
  return connection?.saveData === true;
}

/**
 * Route id for a *top-level* nav href, or null when the path is not one of the
 * lazily-loaded main pages. Dynamic routes are deliberately excluded: warming
 * the list page for `/beacons/42/shell` would fetch the wrong chunk.
 */
export function routeIdFromPath(path: string): string | null {
  const id = /^\/([^/?#]+)\/?(?:[?#].*)?$/.exec(path)?.[1];
  return id && id in ROUTE_PAGE_IMPORTS ? id : null;
}

/**
 * Warm the chunk a navigation will need; call on hover/focus. Repeated calls
 * are free (the first import is reused) and a failed fetch is forgotten so the
 * next hover retries — the router's lazy() also retries on render, where the
 * page error boundary owns the failure UX.
 */
export function prefetchRoute(path: string): void {
  const id = routeIdFromPath(path);
  if (!id || started.has(id)) return;
  started.add(id);
  void ROUTE_PAGE_IMPORTS[id]().catch(() => {
    started.delete(id);
  });
}

/**
 * Speculatively warm the likely-next pages after first paint. Skipped entirely
 * on save-data connections; returns the cancel function for useEffect cleanup.
 */
export function startIdleRoutePrefetch(): () => void {
  if (saveDataEnabled()) return () => {};
  const run = () => {
    for (const path of IDLE_PREFETCH_PATHS) prefetchRoute(path);
  };
  if (typeof window.requestIdleCallback === "function") {
    const handle = window.requestIdleCallback(run, { timeout: 4000 });
    return () => window.cancelIdleCallback(handle);
  }
  const handle = window.setTimeout(run, 1500);
  return () => window.clearTimeout(handle);
}
