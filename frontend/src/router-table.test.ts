import { describe, it, expect } from "vitest";
import { router } from "./router";

// The route table is the single source of truth for paths: a duplicated path
// would silently shadow one page behind another, so fail loudly instead.
function collectPaths(routes: { path?: string; children?: unknown[] }[], out: string[]): string[] {
  for (const r of routes) {
    if (r.path) out.push(r.path);
    const kids = (r as { children?: { path?: string; children?: unknown[] }[] }).children;
    if (kids) collectPaths(kids, out);
  }
  return out;
}

describe("router table", () => {
  it("has no duplicate paths", () => {
    const paths = collectPaths(router.routes as never, []);
    const dupes = paths.filter((p, i) => paths.indexOf(p) !== i);
    expect(dupes).toEqual([]);
  });

  it("registers every top-level section exactly once", () => {
    const paths = collectPaths(router.routes as never, []);
    for (const section of ["agents", "tasks", "loot", "settings", "generate"]) {
      expect(paths.filter((p) => p === `/${section}`)).toHaveLength(1);
    }
  });
});
