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
 */
export const STATIC_ROUTES: string[] = [
  "login",
  "agents",
  "ai",
  "attack",
  "audit",
  "automation",
  "autotag",
  "bloodhound",
  "bof",
  "builds",
  "campaign",
  "chain",
  "chat",
  "chrome",
  "circuit-breaker",
  "cloud",
  "command_templates",
  "container",
  "credentials",
  "dashboard",
  "dns",
  "docs",
  "domain-fronting",
  "files",
  "generate",
  "groups",
  "infrastructure",
  "integrations",
  "lateral",
  "listeners",
  "loot",
  "notifications",
  "ntlm",
  "opsec",
  "packer",
  "password-spray",
  "phishing",
  "pivoting",
  "plugins",
  "privesc",
  "profiles",
  "report",
  "roles",
  "scanner",
  "scheduler",
  "screenshots",
  "scripting",
  "search",
  "settings",
  "stager",
  "tags",
  "tasks",
  "timeline",
  "tokens",
  "toolkit",
  "topology",
  "traffic",
  "users",
  "workflows",
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
      this.emitFile({ type: "asset", fileName: "404.html", source });
      for (const route of STATIC_ROUTES) {
        this.emitFile({ type: "asset", fileName: `${route}.html`, source });
        this.emitFile({ type: "asset", fileName: `${route}/index.html`, source });
      }
    },
  };
}
