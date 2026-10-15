#!/usr/bin/env node
// check-coverage.mjs — fail when frontend unit-test coverage drops below a
// floor, so untested code cannot quietly accumulate.
//
// Ratcheting floor, mirroring the Go side (ci.yml "Coverage floor"): raise it
// whenever tests are added, and lower it only as part of that same change —
// never to make a regression pass.
//
// The figure is statements over every file under src/ that matches the
// coverage include list, whether or not any test imports it. That is the only
// figure that reacts to a large new component shipping with no test at all.
//
// It reads low (14.2%) because most uncovered code is not a missed branch
// inside a tested file — it is page components no test ever imports. A "loaded
// files only" view looks far healthier (~77%) purely because importing a
// module executes its top level; json-summary does not distinguish the two, so
// this script does not report that number rather than mis-state it. The floor
// does not fix the gap, it stops it from widening.
//
// Usage: node scripts/check-coverage.mjs
// Requires: coverage/coverage-summary.json (produced by `npm run test:coverage`)
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

// Pinned at the measured level: 14.2% statements (8061/56767) at the time of
// the last raise. A new uncovered page or helper drops the figure below the
// floor, and the check fails with the exact delta.
const FLOOR = Number(process.env.COVERAGE_FLOOR ?? 14.2);

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const summaryPath = join(root, "coverage", "coverage-summary.json");

if (!existsSync(summaryPath)) {
  console.error("check-coverage: coverage/coverage-summary.json is missing.");
  console.error("Run `npm run test:coverage` first.");
  process.exit(1);
}

let summary;
try {
  summary = JSON.parse(readFileSync(summaryPath, "utf8"));
} catch (err) {
  console.error("check-coverage: cannot parse coverage summary:", err.message);
  process.exit(1);
}

const total = summary.total?.statements;
if (!total || typeof total.covered !== "number" || typeof total.total !== "number") {
  console.error("check-coverage: coverage summary has no usable total.statements block.");
  process.exit(1);
}

const pct = (total.covered / total.total) * 100;
const shown = pct.toFixed(2);

if (pct + 1e-9 < FLOOR) {
  console.error(`FAIL: frontend statement coverage is ${shown}% — below the ${FLOOR}% floor.`);
  console.error(
    `  ${total.covered}/${total.total} statements across src/ are exercised by unit tests.`,
  );
  console.error("Add tests, or raise the floor only as part of that same change.");
  process.exit(1);
}

console.log(
  `OK: frontend coverage ${shown}% (${total.covered}/${total.total} statements in src/) ` +
    `at or above the ${FLOOR}% floor`,
);
