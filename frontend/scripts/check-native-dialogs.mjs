/**
 * Fail if frontend source reintroduces blocking browser dialogs.
 *
 * The app has its own confirmation primitive (useConfirm -> ConfirmModal) and
 * toast notifications. window.alert / window.confirm / window.prompt render
 * outside the app shell: they ignore the theme, block the event loop, cannot
 * be localized or styled, and bypass the a11y focus handling the modal provides.
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.join(__dirname, "..", "src");

const FORBIDDEN = [
  { re: /\bwindow\s*\.\s*alert\s*\(/g, name: "window.alert" },
  { re: /\bwindow\s*\.\s*confirm\s*\(/g, name: "window.confirm" },
  { re: /\bwindow\s*\.\s*prompt\s*\(/g, name: "window.prompt" },
  { re: /(?<![.\w$])alert\s*\(/g, name: "alert" },
  { re: /(?<![.\w$])prompt\s*\(/g, name: "prompt" },
];

const IGNORE_DIRS = new Set(["node_modules", "dist", "__snapshots__"]);

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (IGNORE_DIRS.has(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else if (/\.(tsx?|jsx?)$/.test(entry.name) && !/\.test\.[jt]sx?$/.test(entry.name)) {
      out.push(full);
    }
  }
  return out;
}

const violations = [];
for (const file of walk(SRC)) {
  const lines = fs.readFileSync(file, "utf8").split(/\r?\n/);
  lines.forEach((line, i) => {
    const code = line.split("//")[0];
    for (const { re, name } of FORBIDDEN) {
      re.lastIndex = 0;
      if (re.test(code)) {
        violations.push(`${path.relative(SRC, file)}:${i + 1}  ${name}`);
      }
    }
  });
}

if (violations.length > 0) {
  console.error("native-dialogs FAILED — blocking browser dialogs are not allowed:");
  for (const v of violations) console.error(`  - ${v}`);
  console.error("\nUse useConfirm() from @/lib/hooks/useConfirm, or toast.* from sonner.");
  process.exit(1);
}

console.log("native-dialogs OK — no window.alert/confirm/prompt in src.");
