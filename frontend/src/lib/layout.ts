import { NAV_LAYOUT_BY_HREF } from "./navigation";

/** Workspace routes that should fill the main column (no content max-width / padding). */
const FLUSH_AGENT_SUFFIX = /\/(files|shell|screen|remote-desktop)$/;

export function isFlushPath(pathname: string | null | undefined): boolean {
  if (!pathname) return false;
  if (NAV_LAYOUT_BY_HREF[pathname] === "workspace") return true;
  return /^\/agents\/[^/]+/.test(pathname) && FLUSH_AGENT_SUFFIX.test(pathname);
}

/** Hide the breadcrumb strip when it would be empty or redundant. */
export function showBreadcrumbBar(pathname: string | null | undefined, focusMode: boolean): boolean {
  if (focusMode || !pathname) return false;
  if (pathname === "/dashboard" || pathname === "/") return false;
  if (isFlushPath(pathname)) return false;
  return pathname.split("/").filter(Boolean).length > 0;
}
