import { describe, expect, it } from "vitest";
import { router } from "./router";
import { STATIC_ROUTES } from "../vite-plugins/staticExport";

// The export must mirror the router table exactly. A retired route left in
// STATIC_ROUTES ships dead HTML (the smoke suite used to probe several of
// them), and a page missing from it falls through to the root shell instead of
// its own file. Both directions are caught here rather than in production.
function collectPaths(routes: unknown[], out: string[]): string[] {
  for (const raw of routes) {
    const route = raw as { path?: string; children?: unknown[] };
    if (route.path) out.push(route.path);
    if (route.children) collectPaths(route.children, out);
  }
  return out;
}

/** "/agents/:id/files" -> "agents/_/files"; the export replaces dynamic
 *  segments with an underscore placeholder. */
function toStaticRoute(path: string): string | null {
  if (path === "/" || path === "*") return null;
  return path.replace(/^\//, "").replace(/:id/g, "_");
}

describe("static export route table", () => {
  it("emits exactly the router's addressable routes", () => {
    const expected = collectPaths(router.routes as never, [])
      .map(toStaticRoute)
      .filter((route): route is string => route !== null)
      .sort();
    expect([...STATIC_ROUTES].sort()).toEqual(expected);
  });
});
