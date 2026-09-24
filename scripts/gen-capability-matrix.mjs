// gen-capability-matrix.mjs — inject the TaskSpec-derived task inventory into
// docs/CAPABILITY_MATRIX.md between HTML markers, and stamp the status-line
// version from the root VERSION file. Hand-written sections (transports,
// quality notes, product modules) stay outside the markers.
//
// Usage (repo root):
//   node scripts/gen-capability-matrix.mjs
//   node scripts/gen-capability-matrix.mjs --check   # fail if stale (CI)
//
// Parsing reuses the same balanced-brace approach as gen-command-reference.mjs.

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const dataPath = path.join(repoRoot, "pkg", "protocol", "taskspec_data.go");
const constsPath = path.join(repoRoot, "pkg", "protocol", "tasks.go");
const tasktypesPath = path.join(repoRoot, "internal", "server", "tasktypes.go");
const outPath = path.join(repoRoot, "docs", "CAPABILITY_MATRIX.md");
const versionPath = path.join(repoRoot, "VERSION");

const BEGIN = "<!-- BEGIN GENERATED TASK INVENTORY -->";
const END = "<!-- END GENERATED TASK INVENTORY -->";
const VERSION_LINE_RE =
  /^> Status of implant tasks \/ transports as of \*\*v[0-9]+\.[0-9]+\.[0-9]+\*\*\.\s*$/m;

function readVersion() {
  if (!fs.existsSync(versionPath)) return null;
  const v = readUtf8(versionPath).trim();
  return /^\d+\.\d+\.\d+$/.test(v) ? v : null;
}

const CATEGORY_ORDER = [
  "execution",
  "discovery",
  "collection",
  "credential-access",
  "defense-evasion",
  "lateral-movement",
  "privesc",
  "privilege-escalation",
  "persistence",
  "c2",
  "impact",
  "other",
];

const CATEGORY_TITLES = {
  execution: "Execution",
  discovery: "Discovery",
  collection: "Collection",
  "credential-access": "Credential Access",
  "defense-evasion": "Defense Evasion",
  "lateral-movement": "Lateral Movement",
  privesc: "Privilege Escalation",
  "privilege-escalation": "Privilege Escalation",
  persistence: "Persistence",
  c2: "C2 / Session",
  impact: "Impact",
  other: "Other",
};

function readUtf8(p) {
  return fs.readFileSync(p, "utf8");
}

function extractSpecBlock(src) {
  const marker = "[]TaskSpec{";
  const start = src.indexOf(marker);
  if (start < 0) throw new Error("[]TaskSpec{ not found in taskspec_data.go");
  let i = start + marker.length;
  let depth = 1;
  const begin = i;
  while (i < src.length && depth > 0) {
    const c = src[i];
    if (c === "{") depth++;
    else if (c === "}") {
      depth--;
      if (depth === 0) return src.slice(begin, i);
    }
    i++;
  }
  throw new Error("unbalanced TaskSpec block");
}

function splitTopLevelEntries(block) {
  const entries = [];
  let depth = 0;
  let begin = -1;
  let inStr = false;
  let strCh = "";
  for (let i = 0; i < block.length; i++) {
    const c = block[i];
    if (inStr) {
      if (c === "\\") i++;
      else if (c === strCh) inStr = false;
      continue;
    }
    if (c === '"' || c === "`") {
      inStr = true;
      strCh = c;
      continue;
    }
    if (c === "{") {
      if (depth === 0) begin = i;
      depth++;
    } else if (c === "}") {
      depth--;
      if (depth === 0 && begin >= 0) {
        entries.push(block.slice(begin, i + 1));
        begin = -1;
      }
    }
  }
  return entries;
}

function parseStringField(entry, field) {
  const re = new RegExp(`${field}:\\s*(?:"((?:[^"\\\\]|\\\\.)*)"|(true|false))`);
  const m = entry.match(re);
  if (!m) return undefined;
  if (m[2] === "true") return true;
  if (m[2] === "false") return false;
  return m[1].replace(/\\"/g, '"').replace(/\\\\/g, "\\");
}

function parseBoolField(entry, field) {
  const re = new RegExp(`${field}:\\s*(true|false)`);
  const m = entry.match(re);
  return m ? m[1] === "true" : false;
}

function parseStringSliceField(entry, field) {
  const re = new RegExp(`${field}:\\s*\\[\\]\\w+\\{([^}]*)\\}`);
  const m = entry.match(re);
  if (!m || !m[1].trim()) return [];
  return [...m[1].matchAll(/"((?:[^"\\]|\\.)*)"/g)].map((x) => x[1]);
}

function parseConstMap(src) {
  const map = new Map();
  const re = /(?:^\t|\s)(TaskType\w+)\s*=\s*"([^"]+)"/gm;
  let m;
  while ((m = re.exec(src))) map.set(m[1], m[2]);
  return map;
}

function parseSpecs(block, constMap) {
  return splitTopLevelEntries(block).map((entry) => {
    const typeRef = (entry.match(/Type:\s*(TaskType\w+)/) || [])[1] || "";
    const type = constMap.get(typeRef) || "";
    return {
      type,
      name: parseStringField(entry, "Name") ?? typeRef,
      description: parseStringField(entry, "Description") ?? "",
      category: parseStringField(entry, "Category") || "other",
      requiresShell: parseBoolField(entry, "RequiresShell"),
      requiresElev: parseBoolField(entry, "RequiresElev"),
      requiresApproval: parseBoolField(entry, "RequiresApproval"),
      aliases: parseStringSliceField(entry, "Aliases"),
    };
  }).filter((s) => s.type);
}

function loadDangerous(constMap) {
  const dangerous = new Set();
  if (!fs.existsSync(tasktypesPath)) return dangerous;
  const tt = readUtf8(tasktypesPath);
  const mapStart = tt.indexOf("dangerousTaskTypes = map[string]bool{");
  if (mapStart < 0) return dangerous;
  let i = mapStart + "dangerousTaskTypes = map[string]bool{".length;
  let depth = 1;
  const begin = i;
  while (i < tt.length && depth > 0) {
    const c = tt[i];
    if (c === "{") depth++;
    else if (c === "}") {
      depth--;
      if (depth === 0) {
        const body = tt.slice(begin, i);
        for (const ref of body.matchAll(/protocol\.(TaskType\w+)/g)) {
          const val = constMap.get(ref[1]);
          if (val) dangerous.add(val);
        }
        break;
      }
    }
    i++;
  }
  return dangerous;
}

function mdEscape(s) {
  return String(s ?? "").replace(/\|/g, "\\|");
}

function renderInventory(specs, dangerous) {
  const byCat = new Map();
  for (const s of specs) {
    const cat = CATEGORY_ORDER.includes(s.category) ? s.category : "other";
    if (!byCat.has(cat)) byCat.set(cat, []);
    byCat.get(cat).push(s);
  }
  for (const list of byCat.values()) {
    list.sort((a, b) => a.type.localeCompare(b.type));
  }

  const approvalCount = specs.filter(
    (s) => s.requiresApproval || dangerous.has(s.type),
  ).length;
  const aliasCount = specs.reduce((n, s) => n + s.aliases.length, 0);

  const lines = [];
  lines.push(BEGIN);
  lines.push("");
  lines.push(
    `> **Generated** from \`pkg/protocol/taskspec_data.go\` by \`node scripts/gen-capability-matrix.mjs\`. Do not edit inside these markers.`,
  );
  lines.push("");
  lines.push(
    `- **Dispatchable task types:** ${specs.length} · **aliases:** ${aliasCount} · **approval-gated:** ${approvalCount}`,
  );
  lines.push("");
  lines.push("| Category | Types |");
  lines.push("|----------|-------|");
  for (const cat of CATEGORY_ORDER) {
    const list = byCat.get(cat) || [];
    if (!list.length) continue;
    const types = list.map((s) => `\`${mdEscape(s.type)}\``).join(", ");
    lines.push(`| ${CATEGORY_TITLES[cat] || cat} (${list.length}) | ${types} |`);
  }
  lines.push("");
  lines.push(
    "Full per-command parameters, aliases and help: [`COMMAND_REFERENCE.md`](COMMAND_REFERENCE.md).",
  );
  lines.push("");
  lines.push(END);
  return lines.join("\n");
}

function stampVersion(doc, version) {
  if (!version) return doc;
  const line = `> Status of implant tasks / transports as of **v${version}**.`;
  if (VERSION_LINE_RE.test(doc)) {
    return doc.replace(VERSION_LINE_RE, line);
  }
  // Insert after the H1 title if the status line is missing.
  return doc.replace(/^(# .*\n)/m, `$1\n${line}  \n`);
}

function applySection(doc, section) {
  const bi = doc.indexOf(BEGIN);
  const ei = doc.indexOf(END);
  if (bi >= 0 && ei > bi) {
    return doc.slice(0, bi) + section + doc.slice(ei + END.length);
  }
  // First run: append the section before "## Regenerating" if present.
  const anchor = doc.indexOf("## Regenerating this matrix");
  const block = `\n## Task inventory (generated)\n\n${section}\n`;
  if (anchor >= 0) {
    return doc.slice(0, anchor) + block + doc.slice(anchor);
  }
  return doc.trimEnd() + "\n" + block;
}

function main() {
  const check = process.argv.includes("--check");
  const constMap = parseConstMap(readUtf8(constsPath));
  const block = extractSpecBlock(readUtf8(dataPath));
  const specs = parseSpecs(block, constMap);
  const dangerous = loadDangerous(constMap);
  const section = renderInventory(specs, dangerous);

  const existing = fs.existsSync(outPath) ? readUtf8(outPath) : "";
  let next = applySection(existing, section);
  next = stampVersion(next, readVersion());

  if (check) {
    if (existing !== next) {
      console.error(
        "CAPABILITY_MATRIX.md task inventory is stale — run: node scripts/gen-capability-matrix.mjs",
      );
      process.exit(1);
    }
    console.log("CAPABILITY_MATRIX.md task inventory is up to date");
    return;
  }

  fs.writeFileSync(outPath, next, "utf8");
  console.log(
    `wrote ${path.relative(process.cwd(), outPath)} inventory: ${specs.length} specs` +
      (readVersion() ? ` version: v${readVersion()}` : ""),
  );
}

main();
