#!/usr/bin/env node
// check-bundle.mjs — fail the check when the static client bundle grows
// beyond a sanity ceiling. Guards against accidental weight regressions
// (e.g. an un-split import of a large chart/graph lib).
//
// Baseline measured 2026-08-18: 4.58MB JS + 0.16MB CSS (4.74MB total).
// Threshold = 6.5MB (~37% headroom) for JS+CSS in out/_next/static.
import { readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const LIMIT_BYTES = 6.5 * 1024 * 1024;
const staticDir = join(fileURLToPath(new URL(".", import.meta.url)), "..", "out", "_next", "static");

let total = 0;
let files = 0;
try {
  const stack = [staticDir];
  while (stack.length) {
    const cur = stack.pop();
    for (const entry of readdirSync(cur, { withFileTypes: true })) {
      const abs = join(cur, entry.name);
      if (entry.isDirectory()) {
        stack.push(abs);
      } else if (entry.name.endsWith(".js") || entry.name.endsWith(".css")) {
        total += statSync(abs).size;
        files += 1;
      }
    }
  }
} catch (err) {
  console.error("check-bundle: cannot walk out/_next/static:", err.message);
  console.error("Run `npm run build` first.");
  process.exit(1);
}

const mb = (b) => `${(b / 1024 / 1024).toFixed(2)}MB`;
if (total > LIMIT_BYTES) {
  console.error(`FAIL: static bundle is ${mb(total)} (${files} files) — exceeds ${mb(LIMIT_BYTES)}.`);
  console.error("Split lazy imports or remove heavy deps before raising the threshold.");
  process.exit(1);
}
console.log(`OK: static bundle ${mb(total)} (${files} files) under ${mb(LIMIT_BYTES)}`);