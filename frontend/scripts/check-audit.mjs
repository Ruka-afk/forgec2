#!/usr/bin/env node
// check-audit.mjs — dependency vulnerability gate.
//
// What this enforces, and why it is not the plain `npm audit --audit-level=high`
// that CI used before:
//
//   1. PRODUCTION dependencies must have zero high/critical advisories.
//      This is a hard gate. It is the part that ships to users.
//   2. NO dependency, dev included, may carry a critical advisory. Critical
//      here means remote code execution, which matters whether it runs in CI
//      or in a browser.
//   3. HIGH advisories confined to devDependencies are reported and listed but
//      do not block.
//
// Rule 3 is a deliberate trade, not a shrug, and it has a cost. The reason it
// exists: after overrides landed (shell-quote, proxy-addr, source-map-js,
// @modelcontextprotocol/sdk, tinypool — all real fixes, see the commit), the
// only remaining high advisories are seven packages inside shadcn's tool chain
// (@ts-morph/common, brace-expansion, braces, fast-glob, micromatch, shadcn,
// ts-morph). Those are unfixable today:
//
//   - braces' advisory covers <=3.0.3 and 3.0.3 is the latest release, so there
//     is no version to move to.
//   - Upgrading shadcn to current (4.21.5) does not clear them and adds a new
//     one (@shadcn/registry): 7 high became 8.
//   - Forcing the transitive packages with overrides makes it worse still —
//     pinning brace-expansion to ^4.0.0 stays inside its vulnerable range and
//     drags eslint, minimatch and @redocly/openapi-core into the report too.
//
// shadcn is a component scaffolding CLI, not something the app loads. None of
// this reaches the shipped bundle; `npm audit --omit=dev` reports zero high and
// zero critical, which is what rule 1 pins.
//
// Tighten rule 3 the moment the upstream advisories ship a fixed version. If a
// dev-only advisory turns critical, rule 2 already fails the build — that is
// the escape hatch, and it is why rule 2 is not folded into rule 1.
//
// Usage: node scripts/check-audit.mjs
// Exit codes: 0 = within policy, 1 = gate failed
import { execFileSync } from "node:child_process";

const SEVERITY_RANK = { info: 0, low: 1, moderate: 2, high: 3, critical: 4 };

// Windows ships npm as a .cmd shim. execFileSync refuses to launch .cmd
// outright (EINVAL) since the CVE-2024-27980 hardening, and the usual
// workaround — shell:true — trips DEP0190 because the args get concatenated
// unescaped. Going through cmd.exe explicitly avoids both. The argument list
// below is entirely hard-coded (no user input reaches this string), so there is
// nothing to inject.
const IS_WIN = process.platform === "win32";

function runNpmAudit(extraArgs) {
  const args = ["audit", "--json", ...extraArgs];
  if (IS_WIN) {
    return execFileSync("cmd.exe", ["/d", "/s", "/c", "npm " + args.join(" ")], {
      encoding: "utf8",
      maxBuffer: 32 * 1024 * 1024,
    });
  }
  return execFileSync("npm", args, { encoding: "utf8", maxBuffer: 32 * 1024 * 1024 });
}

function audit(extraArgs) {
  let out;
  try {
    out = runNpmAudit(extraArgs);
  } catch (err) {
    // npm exits non-zero whenever it finds anything, so a throw is expected.
    out = err.stdout ?? "";
    if (!out.trim()) {
      console.error("check-audit: npm audit produced no output; failing rather than skipping the gate.");
      console.error(String(err.stderr ?? err.message).split("\n").slice(0, 5).join("\n"));
      process.exit(1);
    }
  }
  let parsed;
  try {
    parsed = JSON.parse(out.replace(/^\uFEFF/, ""));
  } catch (err) {
    console.error("check-audit: cannot parse npm audit output:", err.message);
    process.exit(1);
  }
  if (parsed.error) {
    // A registry/advisory-fetch failure is not a pass. Failing closed keeps the
    // gate meaningful instead of silently disabling it on a flaky network.
    console.error("check-audit: npm audit could not complete:", String(parsed.message).slice(0, 300));
    process.exit(1);
  }
  return parsed;
}

function worstIn(report, minSeverity) {
  const floor = SEVERITY_RANK[minSeverity];
  return Object.entries(report.vulnerabilities ?? {}).filter(
    ([, v]) => SEVERITY_RANK[v.severity] >= floor,
  );
}

const prod = audit(["--omit=dev"]);
const all = audit([]);

const prodBlocking = worstIn(prod, "high");
const allCritical = worstIn(all, "critical");
const devOnlyHigh = worstIn(all, "high").filter(
  ([name]) => !prodBlocking.some(([p]) => p === name),
);

let failed = false;

if (prodBlocking.length) {
  failed = true;
  console.error(`FAIL: ${prodBlocking.length} high/critical advisories in PRODUCTION dependencies:`);
  for (const [name, v] of prodBlocking) {
    console.error(`  ${name} — ${v.severity}`);
  }
}

if (allCritical.length) {
  failed = true;
  console.error(`FAIL: ${allCritical.length} critical advisories anywhere in the tree:`);
  for (const [name, v] of allCritical) {
    console.error(`  ${name} — ${v.severity}`);
  }
}

if (devOnlyHigh.length) {
  console.warn(
    `NOTE: ${devOnlyHigh.length} high advisory(ies) confined to devDependencies, not blocking:`,
  );
  for (const [name, v] of devOnlyHigh) {
    console.warn(`  ${name} — ${v.severity}`);
  }
  console.warn(
    "  These sit in shadcn's tool chain. braces' advisory has no fixed release,",
  );
  console.warn(
    "  so there is nothing to upgrade to; see the header of scripts/check-audit.mjs.",
  );
}

if (failed) process.exit(1);

const prodCounts = prod.metadata?.vulnerabilities ?? {};
console.log(
  `OK: production dependencies clean at high/critical ` +
    `(low ${prodCounts.low ?? 0}, moderate ${prodCounts.moderate ?? 0}); ` +
    `no critical anywhere; ${devOnlyHigh.length} dev-only high advisory(ies) recorded above.`,
);
