#!/usr/bin/env node
// check-aria.mjs — accessible-name coverage for icon-only triggers.
//
// Icon-only buttons jog memory: when the children are just an <svg>, the only
// name a screen reader (or the a11y tree, or a future test) can use is
// aria-label / aria-labelledby. `title` alone is a weak fallback (tooltip +
// inconsistent SR support) and is reported separately. This scan walks every
// JSX opening tag for <Button ... size="icon...">-style triggers, parses the
// tag the way JSX does (brace/quote aware, multi-line), and flags the ones
// with no accessible-name attribute.
//
// Prototype from the (2026-10) UI/UX audit: report-only, exits 0 unless
// --strict is passed (U2 fixes the findings and promotes it into
// `npm run check`).
//
// Usage:
//   node scripts/check-aria.mjs            # report, always exit 0
//   node scripts/check-aria.mjs --strict   # exit 1 when a name is missing
//   node scripts/check-aria.mjs --list     # print every finding, not just counts
//
// Known limits (prototype honesty):
//   - Only <Button> and raw <button> tags with a literal `size="icon…"` are
//     scanned. A size passed through a variable (`size={size}`) or a
//     non-Button trigger (bare <a>, custom chip) is not covered yet.
//   - `aria-labelledby` is accepted without checking the id target exists.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const SRC = join(root, "src");

/* ── file walk ──────────────────────────────────────────────────────────── */

function* walk(dir) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) yield* walk(full);
    else yield full;
  }
}

const isProductionTsx = (file) =>
  /\.tsx?$/.test(file) && !/\.test\.tsx?$/.test(file) && !/\.d\.ts$/.test(file);

/* ── JSX opening-tag parser ─────────────────────────────────────────────── */

/**
 * Given source text and the index of "<", return the full opening tag
 * (through its closing ">") — brace- and quote-aware, multi-line safe.
 * Returns null when the tag never closes (broken file).
 */
function readOpeningTag(src, start) {
  let depth = 0;
  let quote = null;
  for (let i = start; i < src.length; i++) {
    const ch = src[i];
    if (quote) {
      if (ch === "\\") i += 1;
      else if (ch === quote) quote = null;
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") {
      quote = ch;
      continue;
    }
    if (ch === "{" || ch === "(" || ch === "[") depth += 1;
    else if (ch === "}" || ch === ")" || ch === "]") depth -= 1;
    else if (ch === ">" && depth === 0) return src.slice(start, i + 1);
  }
  return null;
}

const TAG_RE = /<(?:Button|button)\b/g;

/**
 * Only tags whose own attributes carry a literal icon size are "icon-only by
 * declaration". (A <Button size="sm"> with an icon child plus text is fine;
 * an icon-only one must use size="icon…" by the style conventions.)
 */
function scanFile(file) {
  const src = readFileSync(file, "utf8");
  const findings = [];
  let scanned = 0;
  let srOnlyNamed = 0;
  let m;
  TAG_RE.lastIndex = 0;
  while ((m = TAG_RE.exec(src)) !== null) {
    const tag = readOpeningTag(src, m.index);
    if (!tag) break;
    const tagEnd = m.index + tag.length;
    TAG_RE.lastIndex = tagEnd;
    if (!/size="icon/.test(tag)) continue;
    scanned += 1;
    if (/\baria-label(?:ledby)?=/.test(tag)) continue;
    // Pattern used by the Dialog/Sheet close buttons: the accessible name is
    // an sr-only text child (possibly inside a render-prop wrapper), not an
    // attribute. Treat the icon + sr-only pair as named.
    const children = src.slice(tagEnd, tagEnd + 400);
    if (/sr-only/.test(children)) {
      srOnlyNamed += 1;
      continue;
    }
    const line = src.slice(0, m.index).split("\n").length;
    const weakTitle = /\btitle=/.test(tag);
    const compact = tag.replace(/\s+/g, " ").slice(0, 140);
    findings.push({ file, line, weakTitle, snippet: compact });
  }
  return { scanned, srOnlyNamed, findings };
}

/* ── main ───────────────────────────────────────────────────────────────── */

function main() {
  const args = process.argv.slice(2);
  const strict = args.includes("--strict");
  const listAll = args.includes("--list");

  const results = [];
  let totalScanned = 0;
  let totalSrOnly = 0;
  for (const file of walk(SRC).filter(isProductionTsx)) {
    const { scanned, srOnlyNamed, findings } = scanFile(file);
    totalScanned += scanned;
    totalSrOnly += srOnlyNamed;
    for (const f of findings) results.push(f);
  }

  const weak = results.filter((f) => f.weakTitle);
  const missing = results.filter((f) => !f.weakTitle);

  console.log(`icon-sized triggers scanned: ${totalScanned}`);
  console.log(`named by an sr-only text child: ${totalSrOnly}`);
  console.log(`missing accessible name (no aria-label/labelledby): ${missing.length}`);
  console.log(`title-only (weak fallback, a11y tree may have no name): ${weak.length}`);

  const show = (items) => {
    for (const f of items) {
      console.log(`  ${relative(root, f.file)}:${f.line}`);
      if (listAll) console.log(`      ${f.snippet}`);
    }
  };
  if (missing.length) {
    console.log("\nmissing name:");
    show(missing);
  }
  if (weak.length) {
    console.log("\ntitle-only:");
    show(weak);
  }
  if (!listAll && (missing.length || weak.length)) {
    console.log("\n(pass --list for every snippet)");
  }

  if (strict && missing.length) {
    console.error(`\nFAIL: ${missing.length} icon-only trigger(s) without a label.`);
    process.exitCode = 1;
  }
}

main();
