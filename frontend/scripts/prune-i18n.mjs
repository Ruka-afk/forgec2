// prune-i18n.mjs
// Removes dead i18n keys (defined but unused) from en.ts and zh.ts.
// Run after check-i18n.mjs to keep locale files clean.
import fs from "node:fs";
import path from "node:path";
import { execSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { DYNAMIC_PREFIXES } from "./i18n-dynamic.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(__dirname, "..", "src");
const I18N_DIR = path.join(SRC, "lib", "i18n");
const EN_FILE = path.join(I18N_DIR, "en.ts");
const ZH_FILE = path.join(I18N_DIR, "zh.ts");

const KEY_RE = /"([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)":/g;
const USE_DOUBLE = /t\(\s*"([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)"\s*[),]/g;
const USE_TICK = /t\(\s*`([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)`\s*[),]/g;
const USE_KEYFIELD = /(?:labelKey|descKey|titleKey|subtitleKey|valueKey|inputLabel|btnKey)\s*[:=]\s*"([A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+)"/g;

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

function collectKeys(content) {
  const keys = new Set();
  let m;
  KEY_RE.lastIndex = 0;
  while ((m = KEY_RE.exec(content)) !== null) keys.add(m[1]);
  return keys;
}

function collectUsedKeys() {
  const used = new Set();
  for (const file of walk(SRC)) {
    let src;
    try { src = fs.readFileSync(file, "utf8"); } catch { continue; }
    for (const re of [USE_DOUBLE, USE_TICK, USE_KEYFIELD]) {
      re.lastIndex = 0;
      let m;
      while ((m = re.exec(src)) !== null) used.add(m[1]);
    }
  }
  return used;
}

function pruneFile(filePath, lang, usedKeys) {
  const content = fs.readFileSync(filePath, "utf8");
  const defined = collectKeys(content);

  const isDynamic = (k) => DYNAMIC_PREFIXES.some((p) => k.startsWith(p));
  const dead = [...defined].filter((k) => !usedKeys.has(k) && !isDynamic(k)).sort();

  if (dead.length === 0) {
    console.log(`  ${lang}.ts: no dead keys to remove`);
    return;
  }

  // Remove each dead key's line(s) from the file
  let result = content;
  let removed = 0;
  for (const key of dead) {
    // Match: optional leading whitespace, "key": value, possibly multi-line, trailing comma
    const escapedKey = key.replace(/\./g, "\\.");
    // Match from the key definition to the end of its value (non-greedy, stop at next key or closing brace)
    const lineRe = new RegExp(`^[ \\t]*"${escapedKey}":\\s*"[^"]*",?\\s*$`, "gm");
    result = result.replace(lineRe, () => {
      removed++;
      return "";
    });
  }

  // Clean up blank lines left behind
  result = result.replace(/\n{3,}/g, "\n\n");

  fs.writeFileSync(filePath, result, "utf8");
  console.log(`  ${lang}.ts: removed ${removed} dead key(s) from ${dead.length} candidates`);
}

function main() {
  if (!fs.existsSync(EN_FILE) || !fs.existsSync(ZH_FILE)) {
    console.error("i18n locale files not found at", I18N_DIR);
    process.exit(1);
  }

  console.log("Collecting used i18n keys from source...");
  const usedKeys = collectUsedKeys();
  console.log(`  Found ${usedKeys.size} used keys`);

  console.log("\nPruning en.ts...");
  pruneFile(EN_FILE, "en", usedKeys);

  console.log("\nPruning zh.ts...");
  pruneFile(ZH_FILE, "zh", usedKeys);

  console.log("\nRunning validation...");
  try {
    execSync("node scripts/check-i18n.mjs", { cwd: path.join(__dirname, ".."), stdio: "inherit" });
  } catch {
    console.error("\n⚠️  Validation found issues after pruning — review manually.");
    process.exit(1);
  }
  console.log("Done.");
}

main();
