/**
 * Fail if frontend source introduces list/fetch paths that bypass api-paths conventions.
 * - Forbidden bare list paths (must use /api/... helpers)
 * - Strict: static string paths must match ALLOW_PREFIX (or be dual-use / known)
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const srcRoot = path.join(__dirname, "..", "src");

const CALL_RE =
  /\b(?:api\.(?:get|post|postJson|put|putJson|del|download|downloadGet)|fetch)\(\s*[`'"](\/[^`'"]+)[`'"]/g;

/**
 * Bare list paths that MUST use /api/... (backend JSON list routes).
 * Do not include dual-use page routes like /notifications, /groups, /loot.
 */
const FORBIDDEN_LIST = [
  /^\/agents(\?|$)/, // list JSON is /api/agents
  /^\/listeners(\?|$)/, // list JSON is /api/listeners
  /^\/credentials(\?|$)/, // list JSON is /api/credentials
  /^\/audit\/logs/, // prefer /api/audit when available
];

const ALLOW_PREFIX = [
  "/api/",
  "/loot",
  "/agents/", // detail + commands
  "/notifications",
  "/groups",
  "/users",
  "/builds",
  "/login",
  "/logout",
  "/health",
  "/ready",
  "/ws",
  "/metrics",
  "/screenshots/",
  "/ai",
  "/chat",
  "/campaigns",
  "/bloodhound",
  "/chrome",
  "/cloud",
  "/circuit-breaker",
  "/integrations",
  "/redirectors",
  "/phishing",
  "/settings",
  "/settings/",
  "/mitre/",
  "/config/",
  "/collab/",
  "/chain",
  "/credentials",
  "/scheduled-reports",
  "/report",
  "/workflows",
  "/plugins",
  "/automation",
  "/profiles",
  "/generate/",
  // Lab / ops pages (dual-use or dedicated, not bare list under /agents)
  "/infra/",
  "/infrastructure/",
  "/tasks",
  "/opsec",
  "/socks/",
  "/rportfwd/",
  "/privesc",
  "/scanner",
  "/scheduler/",
  "/extc2/",
  "/tokens",
  "/toolkit/",
  "/traffic",
  "/stager",
  "/mesh/",
  "/packer/",
  "/payload/",
  "/dns/",
  "/ntlm/",
  "/cloud/",
  "/tags",
  "/roles",
  "/bof",
  "/chat/",
];

function walk(dir, out = []) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    if (ent.name === "node_modules" || ent.name.startsWith(".")) continue;
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(p, out);
    else if (/\.(ts|tsx)$/.test(ent.name) && !ent.name.endsWith(".test.ts") && !ent.name.endsWith(".test.tsx")) {
      out.push(p);
    }
  }
  return out;
}

const files = walk(srcRoot);
const violations = [];

for (const file of files) {
  // skip path definition itself
  if (file.replace(/\\/g, "/").endsWith("/lib/api-paths.ts")) continue;
  if (file.replace(/\\/g, "/").endsWith("/lib/api.test.ts")) continue;

  const text = fs.readFileSync(file, "utf8");
  let m;
  CALL_RE.lastIndex = 0;
  while ((m = CALL_RE.exec(text)) !== null) {
    const pth = m[1];
    // strip template tail noise like ${id} already in path
    if (pth.includes("${")) {
      // allow templated agent paths and /api paths
      if (pth.startsWith("/agents/") || pth.startsWith("/api/") || pth.startsWith("/campaigns/") || pth.startsWith("/workflows/") || pth.startsWith("/credentials/") || pth.startsWith("/users/") || pth.startsWith("/groups/") || pth.startsWith("/scheduled-reports/") || pth.startsWith("/listeners/") || pth.startsWith("/mitre/")) {
        continue;
      }
    }
    const bare = pth.split("?")[0];
    const forbidden = FORBIDDEN_LIST.some((re) => re.test(pth));
    if (forbidden) {
      violations.push({ file, path: pth, reason: "use /api/... list path from api-paths" });
      continue;
    }
    const allowed = ALLOW_PREFIX.some(
      (pre) => bare === pre || bare.startsWith(pre) || pth.startsWith(pre),
    );
    if (!allowed && bare.startsWith("/") && !bare.startsWith("//") && !pth.includes("${")) {
      violations.push({
        file,
        path: pth,
        reason: "unknown path prefix — add to api-paths or ALLOW_PREFIX",
      });
    }
  }
}

if (violations.length) {
  console.error("check-api-paths FAILED — path policy violations:\n");
  for (const v of violations) {
    const rel = path.relative(path.join(__dirname, ".."), v.file);
    console.error(`  ${rel}\n    ${v.path}  (${v.reason})`);
  }
  console.error("\nFix: import { paths } from \"@/lib/api-paths\" and use paths.* helpers.");
  process.exit(1);
}

console.log("check-api-paths OK — no forbidden or unknown bare paths.");
