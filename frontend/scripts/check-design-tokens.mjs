import { readFileSync } from "fs";
import { readdirSync, statSync } from "fs";
import { resolve, dirname, join } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const srcRoot = resolve(__dirname, "../src");
const cssPath = resolve(srcRoot, "app/globals.css");

const HEX_RE = /#[0-9a-fA-F]{3,8}\b/g;

// Files where raw hex is legitimate: CSS-var fallbacks (xterm theme),
// vis.js graph config, user-facing color pickers, and PNG chart export
// (html-to-image needs explicit pixel colors; CSS vars do not paint).
const HEX_EXEMPT = new Set([
  "components/ShellTerminal.tsx",
  "components/TopologyGraph.tsx",
  "app/(main)/groups/page.tsx",
  "app/(main)/tags/page.tsx",
  "lib/chartExport.ts",
]);

const errors = [];
const css = readFileSync(cssPath, "utf-8");

function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      walk(full, out);
    } else {
      out.push(full);
    }
  }
  return out;
}

// --- Rule 1: Card must stay rounded-lg (radius contract) ---
const cardSrc = readFileSync(resolve(srcRoot, "components/ui/card.tsx"), "utf-8");
if (/rounded-(2xl|3xl)/.test(cardSrc)) {
  errors.push("Card radius contract violated: components/ui/card.tsx must NOT use rounded-2xl/rounded-3xl (Card surface is rounded-lg).");
}

// --- Rule 2: every named shadow utility used in src must resolve to a --shadow-* token ---
const files = walk(srcRoot);
const usedShadows = new Set();
for (const f of files) {
  if (!/\.(ts|tsx)$/.test(f)) continue;
  const content = readFileSync(f, "utf-8");
  for (const m of content.matchAll(/(?<!drop-)shadow-(2xl|xl|lg|md|sm|base|none)(?![a-z0-9-])/g)) {
    usedShadows.add(m[1]);
  }
  // Arbitrary shadows are raw elevations — must go through a token.
  if (content.includes("shadow-[")) {
    errors.push(`${rel(f)}: arbitrary shadow class shadow-[...] — define a --shadow-* token instead.`);
  }
}
const defined = new Set([...css.matchAll(/--shadow-([a-z0-9]+)\s*:/g)].map((m) => m[1]));
for (const s of [...usedShadows]) {
  if (!defined.has(s)) {
    errors.push(`shadow-${s} used in src but globals.css has no --shadow-${s} token.`);
  }
}

// --- Rule 3: raw hex colors are banned outside exempt files ---
for (const f of files) {
  if (!/\.(ts|tsx)$/.test(f)) continue;
  const relPath = rel(f);
  if (HEX_EXEMPT.has(relPath)) continue;
  const lines = readFileSync(f, "utf-8").split("\n");
  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(HEX_RE);
    if (m) {
      errors.push(`${relPath}:${i + 1}: raw hex ${m[0]} — use a design token (--primary, --card, --chart-*, ...) or add the file to HEX_EXEMPT with a reason.`);
    }
  }
}

function rel(f) {
  return f.slice(srcRoot.length + 1).replace(/\\/g, "/");
}

if (errors.length > 0) {
  console.error("\n❌ design-token contract violations:\n");
  errors.forEach((e) => console.error(`  ${e}`));
  console.error("\nDesign tokens live in src/app/globals.css (--elevation-*, --shadow-*, --fs-*, ...). Elevations, radii and colors must go through tokens, not raw values.\n");
  process.exit(1);
} else {
  console.log("✅ design tokens OK — no raw hex, tokenized shadows, Card radius contract held.");
}
