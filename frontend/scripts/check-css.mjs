import { readFileSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const cssPath = resolve(__dirname, "../src/app/globals.css");
const css = readFileSync(cssPath, "utf-8");

const forbidden = [
  /@tailwind\s+(base|components|utilities)/,
];

const required = [
  /@import\s+["']tailwindcss["']/,
];

const errors = [];
for (const pattern of forbidden) {
  const match = css.match(pattern);
  if (match) {
    errors.push(`  FOUND: ${match[0]}  →  Must use PostCSS @import "tailwindcss", not @tailwind directives.`);
  }
}

for (const pattern of required) {
  if (!pattern.test(css)) {
    errors.push(`  MISSING: ${pattern}  →  Must use PostCSS Tailwind via @import "tailwindcss".`);
  }
}

if (errors.length > 0) {
  console.error("\n❌ globals.css architecture mismatch:\n");
  errors.forEach((e) => console.error(e));
  console.error("\nSee frontend/AGENTS.md — this project uses Tailwind CSS via PostCSS (shadcn/ui).\n");
  process.exit(1);
} else {
  console.log("✅ globals.css OK — PostCSS Tailwind architecture verified.");
}
