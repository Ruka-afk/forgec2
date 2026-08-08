#!/usr/bin/env node
// check-webdist.mjs — verify internal/webdist/dist is byte-identical to the
// latest frontend static export (frontend/out). Run after `npm run build`.
//
// Usage: node scripts/check-webdist.mjs
// Exit codes: 0 = fresh, 1 = stale/drift (an embed refresh is required)
import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const outDir = join(root, "frontend", "out");
const distDir = join(root, "internal", "webdist", "dist");

function sha256(file) {
  return createHash("sha256").update(readFileSync(file)).digest("hex");
}

function walk(dir) {
  const map = new Map();
  const stack = [dir];
  while (stack.length) {
    const cur = stack.pop();
    for (const entry of readdirSync(cur, { withFileTypes: true })) {
      const abs = join(cur, entry.name);
      if (entry.isDirectory()) {
        stack.push(abs);
      } else {
        const rel = relative(dir, abs).split(sep).join("/");
        map.set(rel, { size: statSync(abs).size, hash: sha256(abs) });
      }
    }
  }
  return map;
}

let out;
let dist;
try {
  out = walk(outDir);
  dist = walk(distDir);
} catch (err) {
  console.error("check-webdist: cannot walk export dirs:", err.message);
  process.exit(1);
}

const stale = [];
for (const [rel, meta] of out) {
  const d = dist.get(rel);
  if (!d) {
    stale.push(`missing: ${rel}`);
  } else if (d.size !== meta.size || d.hash !== meta.hash) {
    stale.push(`changed: ${rel}`);
  }
}
for (const rel of dist.keys()) {
  if (!out.has(rel)) stale.push(`orphan: ${rel}`);
}

if (stale.length) {
  console.error(`FAIL: internal/webdist/dist is stale (${stale.length} diff(s)).`);
  console.error("Run from repo root:  powershell -File scripts/build-embedded.ps1  (or refresh dist only)");
  for (const line of stale.slice(0, 40)) console.error(`  - ${line}`);
  if (stale.length > 40) console.error(`  ... and ${stale.length - 40} more`);
  process.exit(1);
}
console.log(`OK: internal/webdist/dist matches frontend/out (${out.size} files)`);