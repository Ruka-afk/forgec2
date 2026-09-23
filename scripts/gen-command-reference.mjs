// gen-command-reference.mjs — regenerate docs/COMMAND_REFERENCE.md from the
// single source of truth in pkg/protocol/taskspec_data.go.
//
// Usage (repo root):
//   node scripts/gen-command-reference.mjs
//   node scripts/gen-command-reference.mjs --check   # fail if stale (CI)
//
// Parsing approach: extract each {Type: …, …} composite literal inside the
// init() registry block with a balanced-brace walker (no full Go parser).

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const dataPath = path.join(repoRoot, "pkg", "protocol", "taskspec_data.go");
const constsPath = path.join(repoRoot, "pkg", "protocol", "tasks.go");
const tasktypesPath = path.join(repoRoot, "internal", "server", "tasktypes.go");
const outPath = path.join(repoRoot, "docs", "COMMAND_REFERENCE.md");

const CATEGORY_ORDER = [
  "execution",
  "discovery",
  "collection",
  "credential-access",
  "defense-evasion",
  "lateral-movement",
  "privesc",
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
  persistence: "Persistence",
  c2: "C2 / Session",
  impact: "Impact",
  other: "Other",
};

function readUtf8(p) {
  return fs.readFileSync(p, "utf8");
}

/** Extract the body of init() between `[]TaskSpec{` and matching close. */
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

/** Split top-level `{...}` entries (depth-1 literals) from the block body. */
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
  // Field: value "..."  or  Field: "..."  or Field: true/false
  const re = new RegExp(`${field}:\\s*(?:"((?:[^"\\\\]|\\\\.)*)"|(true|false))`);
  const m = entry.match(re);
  if (!m) return m && m[2] === "true" ? true : m ? m[1] : undefined;
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
  // Matches: Aliases: []string{"a", "b"}  (composite literal, not a bare [] slice)
  const re = new RegExp(`${field}:\\s*\\[\\]\\w+\\{([^}]*)\\}`);
  const m = entry.match(re);
  if (!m || !m[1].trim()) return [];
  return [...m[1].matchAll(/"((?:[^"\\]|\\.)*)"/g)].map((x) => x[1]);
}

function parseParams(entry) {
  const m = entry.match(/Parameters:\s*\[\]TaskParam\s*\{/);
  if (!m) return [];
  let i = m.index + m[0].length;
  let depth = 1;
  const begin = i;
  while (i < entry.length && depth > 0) {
    const c = entry[i];
    if (c === "{") depth++;
    else if (c === "}") {
      depth--;
      if (depth === 0) {
        const body = entry.slice(begin, i);
        return splitTopLevelEntries(body).map((p) => ({
          name: parseStringField(p, "Name") ?? "",
          type: parseStringField(p, "Type") ?? "string",
          required: parseBoolField(p, "Required"),
          default: parseStringField(p, "Default"),
          description: parseStringField(p, "Description") ?? "",
        }));
      }
    }
    i++;
  }
  return [];
}

function parseSpecs(block) {
  return splitTopLevelEntries(block).map((entry) => {
    const typeRef = (entry.match(/Type:\s*(TaskType\w+)/) || [])[1] || "";
    return {
      typeRef,
      type: "", // filled after const map
      name: parseStringField(entry, "Name") ?? typeRef,
      description: parseStringField(entry, "Description") ?? "",
      category: parseStringField(entry, "Category") || "other",
      requiresShell: parseBoolField(entry, "RequiresShell"),
      requiresElev: parseBoolField(entry, "RequiresElev"),
      requiresApproval: parseBoolField(entry, "RequiresApproval"),
      aliases: parseStringSliceField(entry, "Aliases"),
      help: parseStringField(entry, "Help") ?? "",
      parameters: parseParams(entry),
    };
  });
}

/** Map TaskTypeXxx const name → string value from tasks.go. */
function parseConstMap(src) {
  const map = new Map();
  const re = /(?:^\t|\s)(TaskType\w+)\s*=\s*"([^"]+)"/gm;
  let m;
  while ((m = re.exec(src))) map.set(m[1], m[2]);
  return map;
}

function mdEscape(s) {
  return String(s ?? "").replace(/\|/g, "\\|");
}

function render(specs, constCount, missingConsts) {
  const byCat = new Map();
  for (const s of specs) {
    const cat = CATEGORY_ORDER.includes(s.category) ? s.category : "other";
    if (!byCat.has(cat)) byCat.set(cat, []);
    byCat.get(cat).push(s);
  }
  for (const list of byCat.values()) {
    list.sort((a, b) => a.type.localeCompare(b.type));
  }

  const totalParams = specs.reduce((n, s) => n + s.parameters.length, 0);
  const approvalCount = specs.filter((s) => s.requiresApproval).length;
  const aliasCount = specs.reduce((n, s) => n + s.aliases.length, 0);

  const lines = [];
  lines.push("# ForgeC2 Command Reference");
  lines.push("");
  lines.push("> **Generated** from `pkg/protocol/taskspec_data.go` by");
  lines.push("> `node scripts/gen-command-reference.mjs`. Do not edit by hand.");
  lines.push("");
  lines.push(`- **Task types:** ${specs.length}`);
  lines.push(`- **Constants in tasks.go:** ${constCount}`);
  lines.push(`- **Aliases:** ${aliasCount}`);
  lines.push(`- **Approval-gated:** ${approvalCount}`);
  lines.push(`- **Parameters (total):** ${totalParams}`);
  if (missingConsts.length) {
    lines.push(
      `- **Constants without specs:** ${missingConsts.join(", ")} ` +
        `(intentional: RESULT-type only, not dispatchable — see pkg/protocol/tasks.go)`,
    );
  }
  lines.push("");
  lines.push("## Categories");
  lines.push("");
  for (const cat of CATEGORY_ORDER) {
    const list = byCat.get(cat) || [];
    if (!list.length) continue;
    lines.push(
      `- [${CATEGORY_TITLES[cat] || cat}](#${(CATEGORY_TITLES[cat] || cat)
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")}) (${list.length})`,
    );
  }
  lines.push("");

  for (const cat of CATEGORY_ORDER) {
    const list = byCat.get(cat) || [];
    if (!list.length) continue;
    lines.push(`## ${CATEGORY_TITLES[cat] || cat}`);
    lines.push("");
    lines.push("| Type | Name | Approval | Aliases | Parameters | Description |");
    lines.push("|------|------|----------|---------|------------|-------------|");
    for (const s of list) {
      const params =
        s.parameters.length === 0
          ? "—"
          : s.parameters
              .map(
                (p) =>
                  `${p.name}:${p.type}${p.required ? "*" : ""}${
                    p.default ? `=${p.default}` : ""
                  }`,
              )
              .join(", ");
      const aliases = s.aliases.length ? s.aliases.map((a) => `\`${a}\``).join(", ") : "—";
      const flags = [];
      if (s.requiresShell) flags.push("shell");
      if (s.requiresElev) flags.push("elev");
      if (s.requiresApproval) flags.push("approval");
      const approval = s.requiresApproval ? "**yes**" : flags.length ? flags.join("/") : "no";
      lines.push(
        `| \`${mdEscape(s.type)}\` | ${mdEscape(s.name)} | ${approval} | ${aliases} | ${mdEscape(params)} | ${mdEscape(s.description)} |`,
      );
    }
    lines.push("");
  }

  lines.push("---");
  lines.push("");
  lines.push(
    "Regenerate after editing `pkg/protocol/taskspec_data.go` or the",
  );
  lines.push(
    "`dangerousTaskTypes` map in `internal/server/tasktypes.go`.",
  );
  lines.push("");
  lines.push(
    "```bash\nnode scripts/gen-command-reference.mjs\nnode scripts/gen-command-reference.mjs --check  # CI freshness gate\n```",
  );
  lines.push("");
  return lines.join("\n");
}

function main() {
  const check = process.argv.includes("--check");
  const dataSrc = readUtf8(dataPath);
  const constSrc = readUtf8(constsPath);
  const constMap = parseConstMap(constSrc);
  const block = extractSpecBlock(dataSrc);
  const specs = parseSpecs(block);

  // Merge server-side dangerousTaskTypes (two-man rule) into RequiresApproval.
  // The generated docs should reflect effective approval, not only the flag
  // stamped on the TaskSpec literal.
  const dangerous = new Set();
  if (fs.existsSync(tasktypesPath)) {
    const tt = readUtf8(tasktypesPath);
    const mapStart = tt.indexOf("dangerousTaskTypes = map[string]bool{");
    if (mapStart >= 0) {
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
    }
  }

  let missingConsts = [];
  for (const s of specs) {
    const v = constMap.get(s.typeRef);
    if (v) s.type = v;
    else {
      s.type = s.typeRef
        .replace(/^TaskType/, "")
        .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
        .toLowerCase();
    }
    if (dangerous.has(s.type)) s.requiresApproval = true;
  }

  // Constants present but with no spec entry
  const specRefs = new Set(specs.map((s) => s.typeRef));
  for (const [ref, val] of constMap) {
    if (!specRefs.has(ref)) missingConsts.push(val);
  }

  const md = render(specs, constMap.size, missingConsts);

  if (check) {
    const existing = fs.existsSync(outPath) ? fs.readFileSync(outPath, "utf8") : "";
    if (existing !== md) {
      console.error("COMMAND_REFERENCE.md is stale — run: node scripts/gen-command-reference.mjs");
      process.exit(1);
    }
    console.log("COMMAND_REFERENCE.md is up to date");
    return;
  }

  fs.writeFileSync(outPath, md, "utf8");
  console.log(
    `wrote ${path.relative(process.cwd(), outPath)}: ${specs.length} specs, ${missingConsts.length} unspec'd constants`,
  );
}

main();
