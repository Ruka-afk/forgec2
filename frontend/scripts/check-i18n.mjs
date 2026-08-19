// check-i18n.mjs
// Verifies every t("x.y") call used in the frontend has a matching key
// defined in BOTH en.ts and zh.ts under src/lib/i18n/.
// Exits non-zero (fails the build / git hook) when a key is missing.

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { DYNAMIC_PREFIXES } from "./i18n-dynamic.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(__dirname, "..", "src");
const I18N_DIR = path.join(SRC, "lib", "i18n");
const EN_FILE = path.join(I18N_DIR, "en.ts");
const ZH_FILE = path.join(I18N_DIR, "zh.ts");

const KEY_RE = /"([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)":/g;
// Also matches optional calls t?.("key") — those bypassed the checker and
// silently rendered raw keys in the UI (time.ago.* etc.).
const USE_DOUBLE = /\bt\?*\.?\(\s*"([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)"\s*[),]/g;
const USE_TICK = /\bt\?*\.?\(\s*`([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)`\s*[),]/g;
// Stale-closure-safe calls via a ref, e.g. tRef.current("shell.banner").
// (ShellTerminal keeps t in a ref so the xterm init/error paths can translate
// without re-subscribing; the plain t() regex above cannot see those.)
const USE_REF_DOUBLE = /\bt[A-Za-z]*\.current\(\s*"([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)"\s*[),]/g;
const USE_REF_TICK = /\bt[A-Za-z]*\.current\(\s*`([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)`\s*[),]/g;
// Keys referenced indirectly as data (labelKey/descKey/…: "x.y") are used
// but would otherwise look dead. Fold them into the used set.
const USE_KEYFIELD = /(?:labelKey|descKey|titleKey|subtitleKey|valueKey|inputLabel|btnKey|i18nKey)\s*[:=]\s*"([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)"/g;
// useErrorToast(err, "fallback.i18n_key") — the second useErrorToast argument
// is a key resolved with t() inside the hook and must be counted as used.
const USE_TOAST_FALLBACK = /\btoastError\(\s*[^,)]+,\s*"([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)"\s*\)/g;

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === "node_modules" || entry.name === ".next") continue;
      walk(full, out);
    } else if (/\.(tsx?|ts)$/.test(entry.name)) {
      out.push(full);
    }
  }
  return out;
}

function keysFromFile(content) {
  const keys = new Set();
  let m;
  KEY_RE.lastIndex = 0;
  while ((m = KEY_RE.exec(content)) !== null) keys.add(m[1]);
  return keys;
}

function main() {
  if (!fs.existsSync(EN_FILE) || !fs.existsSync(ZH_FILE)) {
    console.error("i18n locale files not found at", I18N_DIR);
    process.exit(1);
  }
  const enKeys = keysFromFile(fs.readFileSync(EN_FILE, "utf8"));
  const zhKeys = keysFromFile(fs.readFileSync(ZH_FILE, "utf8"));

  const used = new Set();
  for (const file of walk(SRC)) {
    let src;
    try {
      src = fs.readFileSync(file, "utf8");
    } catch {
      continue;
    }
    for (const re of [USE_DOUBLE, USE_TICK, USE_KEYFIELD, USE_TOAST_FALLBACK, USE_REF_DOUBLE, USE_REF_TICK]) {
      re.lastIndex = 0;
      let m;
      while ((m = re.exec(src)) !== null) used.add(m[1]);
    }
  }

  const missingEn = [];
  const missingZh = [];
  for (const k of used) {
    if (!enKeys.has(k)) missingEn.push(k);
    if (!zhKeys.has(k)) missingZh.push(k);
  }

  // Dead keys: defined in BOTH locale blocks but never referenced via t("x.y").
  // Reported as warnings only. Keys built dynamically (e.g. the sidebar's
  // `t(navKey)`) are false positives, so they're excluded from the dead
  // list. The dynamic families live in scripts/i18n-dynamic.mjs (single source).
  const isDynamic = (k) => DYNAMIC_PREFIXES.some((p) => k.startsWith(p));
  const deadEn = [...enKeys].filter((k) => !used.has(k) && !isDynamic(k));
  const deadZh = [...zhKeys].filter((k) => !used.has(k) && !isDynamic(k));
  const dead = [...new Set([...deadEn, ...deadZh])].sort();

  const problems = new Set([...missingEn, ...missingZh]);
  if (problems.size === 0) {
    console.log(`i18n OK — ${used.size} used keys present in both en & zh blocks.`);
    if (dead.length > 0) {
      console.log(`\ni18n WARN — ${dead.length} defined key(s) appear unused (candidates for pruning):`);
      for (const k of dead) console.log(`  ~ ${k}`);
    }
    process.exit(0);
  }

  console.error("i18n FAILED — the following t() keys are missing from a locale block:");
  for (const k of [...problems].sort()) {
    const flags = [];
    if (missingEn.includes(k)) flags.push("missing en");
    if (missingZh.includes(k)) flags.push("missing zh");
    console.error(`  - ${k}  [${flags.join(", ")}]`);
  }
  console.error(`\nFix: add "${[...problems][0]}" to the missing locale file(s) in src/lib/i18n/`);
  process.exit(1);
}

main();
