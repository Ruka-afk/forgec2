#!/usr/bin/env node
// Keep the English and Chinese READMEs structurally in sync.
//
// The two files are translated by hand, so a full text diff is useless. What
// must stay identical is the skeleton: heading levels, code fences, fact
// markers, and the set of repository files each one links to. When one side
// gains a section or a link and the other does not, this check fails.
//
// Usage: node scripts/check-readme-parity.mjs

import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const PAIR = { en: "README.md", zh: "README.zh.md" };

const read = (file) => readFileSync(join(repoRoot, file), "utf8");
const lines = (text) => text.split("\n");

function headings(text) {
  return lines(text)
    .map((line) => line.match(/^(#{1,6})\s+\S/))
    .filter(Boolean)
    .map((match) => match[1].length);
}

function fences(text) {
  return lines(text).filter((line) => line.trimStart().startsWith("```")).length;
}

function factKeys(text) {
  return [...text.matchAll(/<!--fact:([a-z]+)-->/g)].map((match) => match[1]).sort();
}

function relativeLinks(text) {
  const out = new Set();
  for (const match of text.matchAll(/\]\(([^)\s]+)\)/g)) {
    const target = match[1];
    if (target.startsWith("http") || target.startsWith("#") || target.startsWith("mailto:")) continue;
    const path = target.split("#")[0];
    // The language switcher is the one link that must differ between the pair.
    out.add(path === "README.md" || path === "README.zh.md" ? "README" : path);
  }
  return [...out].sort();
}

function images(text) {
  return [...text.matchAll(/!\[[^\]]*\]\(([^)\s]+)\)/g)].map((match) => match[1]).sort();
}

const docs = { en: read(PAIR.en), zh: read(PAIR.zh) };
const problems = [];

const same = (label, a, b) => {
  const left = JSON.stringify(a);
  const right = JSON.stringify(b);
  if (left !== right) problems.push(`${label}\n  ${PAIR.en}: ${left}\n  ${PAIR.zh}: ${right}`);
};

same("heading structure differs", headings(docs.en), headings(docs.zh));
same("code fence count differs", fences(docs.en), fences(docs.zh));
same("fact markers differ", factKeys(docs.en), factKeys(docs.zh));
same("relative link targets differ", relativeLinks(docs.en), relativeLinks(docs.zh));
same("image targets differ", images(docs.en), images(docs.zh));

if (!docs.en.includes("(README.zh.md)")) problems.push(`${PAIR.en} does not link to ${PAIR.zh}`);
if (!docs.zh.includes("(README.md)")) problems.push(`${PAIR.zh} does not link back to ${PAIR.en}`);

if (problems.length > 0) {
  console.error(`README parity check failed (${problems.length}):\n`);
  for (const problem of problems) console.error(`- ${problem}`);
  process.exit(1);
}

console.log(`ok: ${PAIR.en} and ${PAIR.zh} share the same skeleton`);
