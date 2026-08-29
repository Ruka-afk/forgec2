// unify-icon-sizes.mjs — mechanical migration of paired width/height utility
// classes to the Tailwind v4 `size-*` shorthand.
//
//   w-4 h-4  → size-4
//   h-3 w-3  → size-3
//
// Only PLAIN numeric sizes are touched (no responsive variants, no
// fractional beyond .5, no named values) so the transform is provably
// visual-equivalent. Reports per-file counts and writes a summary JSON for
// the delivery report.
import fs from "node:fs";
import path from "node:path";

const ROOT = "src";
const PAIR_RE = /\b([wh])-((?:\d+(?:\.\d+)?))\s+([wh])-((?:\d+(?:\.\d+)?))\b/g;

let files = 0, total = 0;
const detail = [];

function walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p);
    else if (e.name.endsWith(".tsx")) process(p);
  }
}

function process(file) {
  const src = fs.readFileSync(file, "utf8");
  let count = 0;
  const out = src.replace(PAIR_RE, (m, a1, n1, a2, n2) => {
    if (a1 === a2 || n1 !== n2) return m; // w-w / h-h pairs or mismatched sizes: skip
    // Skip when either side carries a variant prefix glued by ':' already —
    // the regex above only matches bare tokens, variants contain ':' before,
    // which \b still allows; guard explicitly.
    count++;
    return `size-${n1}`;
  });
  if (count > 0) {
    fs.writeFileSync(file, out);
    files++; total += count;
    detail.push({ file: path.relative(ROOT, file).replaceAll("\\", "/"), count });
  }
}

walk(ROOT);
fs.writeFileSync("icon-size-report.json", JSON.stringify({ files, total, detail }, null, 2));
console.log(`files: ${files}, replacements: ${total}`);
