import type { Plugin } from "vite";

/**
 * Static route map for the Vite export.
 *
 * The Next.js "app dir" had 70 page.tsx files. Vite produces a single
 * index.html shell; this plugin re-emits it as <route>.html plus
 * <route>/index.html for every route so the Go spaFS contract keeps
 * working unchanged: a directory path (e.g. /dashboard) serves the
 * sibling <name>.html, a dynamic segment (e.g. /agents/<id>) falls
 * through to the root index.html exactly like the Next export did.
 *
 * Kept in lockstep with the router table (src/router.tsx) by
 * src/static-export-routes.test.ts: a retired route left here ships dead
 * HTML, and a new page missing here never gets its own file.
 */
export const STATIC_ROUTES: string[] = [
  "login",
  "forbidden",
  "agents",
  "ai",
  "asset-organization",
  "attack",
  "audit",
  "automation",
  "autotag",
  "bloodhound",
  "bof",
  "campaign",
  "chain",
  "chat",
  "circuit-breaker",
  "cloud",
  "credentials",
  "dashboard",
  "dns",
  "domain-fronting",
  "generate",
  "groups",
  "infrastructure",
  "integrations",
  "lateral",
  "listeners",
  "loot",
  "macros",
  "ntlm",
  "opsec",
  "phishing",
  "pivoting",
  "plugins",
  "privesc",
  "report",
  "roles",
  "scanner",
  "scripting",
  "settings",
  "tags",
  "timeline",
  "tokens",
  "toolkit",
  "topology",
  "traffic",
  "users",
  "agents/_",
  "agents/_/config",
  "agents/_/files",
  "agents/_/persistence",
  "agents/_/remote-desktop",
  "agents/_/screen",
  "agents/_/shell",
  "agents/_/token",
  "agents/_/traffic",
  "listeners/_",
];

export function staticExportPlugin(): Plugin {
  return {
    name: "forgec2-static-export",
    apply: "build",
    enforce: "post",
    generateBundle(_options, bundle) {
      const html = bundle["index.html"];
      if (!html || html.type !== "asset") return;
      const source =
        typeof html.source === "string" ? html.source : Buffer.from(html.source).toString("utf8");
      // Vite sees the lazy router imports from the application entry and
      // emits modulepreload tags for every route. That defeats route-level
      // code splitting: a dashboard visit downloads Agent, AI, credentials,
      // and every other page before the operator opens it. Keep only shared
      // vendor preloads; route chunks are fetched by React.lazy on navigation.
      const shell = source.replace(/<link rel="modulepreload"[^>]*>/g, (tag) =>
        /href="\/assets\/vendor-/.test(tag) ? tag : "",
      );
      html.source = shell;
      this.emitFile({ type: "asset", fileName: "404.html", source: shell });
      for (const route of STATIC_ROUTES) {
        this.emitFile({ type: "asset", fileName: `${route}.html`, source: shell });
        this.emitFile({ type: "asset", fileName: `${route}/index.html`, source: shell });
      }
    },
  };
}
