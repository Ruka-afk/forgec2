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
 *
 * The pre-U7 names (agents, timeline, generate, privesc, chain,
 * asset-organization and the /agents/:id family) stay listed on purpose: the
 * router keeps them as compatibility redirects (U7 w2), and shipping their
 * shell is what lets a bookmarked URL — or the legacy server-side page
 * handlers — hand the SPA control before it rewrites to the new route.
 */
export const STATIC_ROUTES: string[] = [
  "login",
  "forbidden",
  "agents",
  "ai",
  "asset-management",
  "asset-organization",
  "attack",
  "audit",
  "automation",
  "autotag",
  "beacons",
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
  "events",
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
  "payload-generator",
  "phishing",
  "pivoting",
  "plugins",
  "privesc",
  "privilege-escalation",
  "proxy-chains",
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
  "beacons/_",
  "beacons/_/config",
  "beacons/_/files",
  "beacons/_/persistence",
  "beacons/_/remote-desktop",
  "beacons/_/screen",
  "beacons/_/shell",
  "beacons/_/token",
  "beacons/_/traffic",
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
      // and every other page before the operator opens it. Keep only the
      // entry's static import closure: those are the chunks first paint needs
      // anyway (vendor-*, plus since Vite 8 the shared `utils` chunk that
      // carries the English dictionary). Route chunks stay out.
      type ChunkLike = { type: "chunk"; fileName: string; imports: string[] };
      const critical = new Set<string>();
      const visit = (chunk: ChunkLike) => {
        if (critical.has(chunk.fileName)) return;
        critical.add(chunk.fileName);
        for (const dep of chunk.imports) {
          const next = bundle[dep];
          if (next && next.type === "chunk") visit(next);
        }
      };
      const entry = Object.values(bundle).find(
        (item) => item.type === "chunk" && item.isEntry,
      );
      if (entry && entry.type === "chunk") visit(entry);
      const shell = source.replace(/<link rel="modulepreload"[^>]*>/g, (tag) => {
        const href = tag.match(/href="\/assets\/([^"]+)"/)?.[1];
        if (!href) return "";
        const keep =
          critical.size > 0 ? critical.has(`assets/${href}`) : href.startsWith("vendor-");
        return keep ? tag : "";
      });
      html.source = shell;
      this.emitFile({ type: "asset", fileName: "404.html", source: shell });
      for (const route of STATIC_ROUTES) {
        this.emitFile({ type: "asset", fileName: `${route}.html`, source: shell });
        this.emitFile({ type: "asset", fileName: `${route}/index.html`, source: shell });
      }
    },
  };
}
