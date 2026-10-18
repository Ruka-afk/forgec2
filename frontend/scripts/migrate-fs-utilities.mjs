/**
 * Migrate `text-(--fs-X)` -> `text-X`.
 *
 * `text-(--fs-compact)` compiled to `color: var(--fs-compact)` — a length in a
 * color slot, which the browser discards. Every such call site silently lost
 * its intended font size and inherited 14/15px instead. The size ramp now
 * lives in `@theme inline` as `--text-<name>`, so `text-compact` is a real
 * font-size utility.
 *
 * Longest token first so `text-(--fs-micro-sm)` is not truncated to
 * `text-(--fs-micro)` + "-sm)".
 */
import { readFileSync, writeFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const srcRoot = resolve(__dirname, "../src");

const TOKENS = ["micro-sm", "micro", "xs-sm", "compact", "page-title", "section", "stat-value", "prose"];

function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (/\.(ts|tsx)$/.test(full)) out.push(full);
  }
  return out;
}

const dryRun = process.argv.includes("--dry-run");
let files = 0;
let hits = 0;
for (const file of walk(srcRoot)) {
  let text = readFileSync(file, "utf-8");
  const before = text;
  for (const token of TOKENS) {
    const needle = `text-(--fs-${token})`;
    const replacement = `text-${token}`;
    let idx;
    let count = 0;
    while ((idx = text.indexOf(needle)) !== -1) {
      text = text.slice(0, idx) + replacement + text.slice(idx + needle.length);
      count++;
      if (count > 500) break;
    }
    hits += count;
  }
  if (text !== before) {
    files++;
    if (!dryRun) writeFileSync(file, text, "utf-8");
    console.log(`${dryRun ? "would fix" : "fixed"} ${file.slice(srcRoot.length + 1)}`);
  }
}
console.log(`\n${dryRun ? "[dry-run] " : ""}${hits} call sites in ${files} files`);
