#!/usr/bin/env node
// Keep the volatile numbers in README.md / README.zh.md in sync with the code.
//
// Sources of truth:
//   VERSION                            -> archive names (forgec2-vX.Y.Z-*)
//   pkg/protocol/taskspec_data.go      -> TaskSpec entries
//   plugins/**/manifest.yaml           -> bundled plugins
//   frontend/src/lib/route-imports.ts  -> routed console pages
//   frontend/package.json engines.node -> minimum Node version
//   go.mod                             -> minimum Go version
//
// Usage:
//   node scripts/gen-readme-facts.mjs          # rewrite both READMEs
//   node scripts/gen-readme-facts.mjs --check  # exit 1 when they are stale
//
// The markers are HTML comments, so GitHub renders the numbers without them:
//   <!--fact:tasks-->222<!--/fact-->

import { readFileSync, writeFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const checkOnly = process.argv.includes("--check");

const read = (relative) => readFileSync(join(repoRoot, relative), "utf8");

function countPluginManifests(dir) {
  let total = 0;
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) {
      total += countPluginManifests(full);
    } else if (entry === "manifest.yaml") {
      total += 1;
    }
  }
  return total;
}

function collectFacts() {
  const taskspec = read("pkg/protocol/taskspec_data.go");
  const routes = read("frontend/src/lib/route-imports.ts");
  const pkg = JSON.parse(read("frontend/package.json"));
  const goMod = read("go.mod");
  const nodeEngine = String(pkg.engines?.node ?? "").replace(/^[>=^\s]+/, "");
  const goVersion = goMod.match(/^go\s+(\d+\.\d+)/m)?.[1];
  if (!goVersion) throw new Error("go.mod: cannot read the go directive");
  if (!nodeEngine) throw new Error("frontend/package.json: engines.node is missing");

  return {
    version: read("VERSION").trim(),
    tasks: (taskspec.match(/\{Type:\s*TaskType/g) ?? []).length,
    plugins: countPluginManifests(join(repoRoot, "plugins")),
    routes: (routes.match(/=>\s*import\(/g) ?? []).length,
    node: nodeEngine,
    go: goVersion,
  };
}

function applyFacts(text, facts) {
  let next = text;
  for (const key of ["tasks", "plugins", "routes", "node", "go"]) {
    const marker = new RegExp(`<!--fact:${key}-->[\\s\\S]*?<!--/fact-->`, "g");
    next = next.replace(marker, `<!--fact:${key}-->${facts[key]}<!--/fact-->`);
  }
  next = next.replace(/forgec2-v\d+\.\d+\.\d+-/g, `forgec2-v${facts.version}-`);
  next = next.replace(/go-\d+\.\d+-00ADD8/g, `go-${facts.go}-00ADD8`);
  return next;
}

const facts = collectFacts();
const targets = ["README.md", "README.zh.md"];
const stale = [];

for (const file of targets) {
  const before = read(file);
  const after = applyFacts(before, facts);
  if (before === after) {
    console.log(`ok ${file}`);
    continue;
  }
  stale.push(file);
  if (checkOnly) {
    const changed = before
      .split("\n")
      .map((line, index) => [line, after.split("\n")[index]])
      .filter(([a, b]) => a !== b)
      .slice(0, 8);
    console.error(`stale ${file}`);
    for (const [a, b] of changed) {
      console.error(`  - ${a.trim()}`);
      console.error(`  + ${b.trim()}`);
    }
  } else {
    writeFileSync(join(repoRoot, file), after, "utf8");
    console.log(`updated ${file}`);
  }
}

const summary =
  `version=${facts.version} tasks=${facts.tasks} plugins=${facts.plugins} ` +
  `routes=${facts.routes} node=${facts.node} go=${facts.go}`;
console.log(summary);

if (checkOnly && stale.length > 0) {
  console.error(`\n${stale.length} README file(s) out of date — run: node scripts/gen-readme-facts.mjs`);
  process.exit(1);
}
