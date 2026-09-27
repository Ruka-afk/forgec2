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
    // "tasks" is a tab of /timeline now, not a top-level route.
    for (const section of ["agents", "timeline", "loot", "settings", "generate"]) {
      expect(paths.filter((p) => p === `/${section}`)).toHaveLength(1);
    }
  });

  // The former top-level stubs rendered nothing and bounced the operator to a
  // tab elsewhere, so they were removed rather than kept as redirect routes.
  // This locks that in: reintroducing one would silently re-add a dead URL.
  it("does not register the removed redirect-stub sections", () => {
    const paths = collectPaths(router.routes as never, []);
    for (const gone of [
      "/builds",
      "/packer",
      "/profiles",
      "/stager",
      "/tasks",
      "/files",
      "/notifications",
    ]) {
      expect(paths).not.toContain(gone);
    }
    // ...and the content they pointed at is still served.
    for (const host of ["/generate", "/timeline"]) {
      expect(paths).toContain(host);
    }
  });
});
