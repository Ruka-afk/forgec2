// migrate-taskspec.mjs — one-shot mechanical translocation of the server's
// TaskTypeInfo registry into pkg/protocol TaskSpec registrations.
//
// Reads internal/server/tasktypes.go, extracts the registeredTaskTypes init
// block, rewrites it as []TaskSpec literals inside a generated
// pkg/protocol/taskspec_data.go, then emits minimal specs for the constants
// that had no metadata. Also rewrites the server file to derive its view
// from protocol.AllSpecs().
import fs from "node:fs";

const serverPath = "internal/server/tasktypes.go";
const protoPath = "pkg/protocol/tasks.go";
const dataOutPath = "pkg/protocol/taskspec_data.go";

const serverSrc = fs.readFileSync(serverPath, "utf8");
const protoSrc = fs.readFileSync(protoPath, "utf8");

// ── extract the registry block ──────────────────────────────────────────
const startMarker = "registeredTaskTypes = []TaskTypeInfo{";
const startIdx = serverSrc.indexOf(startMarker);
if (startIdx < 0) throw new Error("registry start marker not found");
const bodyStart = startIdx + startMarker.length;
// find the matching "\n\t}" that closes the composite literal
const endIdx = serverSrc.indexOf("\n\t}", bodyStart);
if (endIdx < 0) throw new Error("registry end marker not found");
const block = serverSrc.slice(bodyStart, endIdx);

// transform entries for package-protocol context
let transformed = block
  .replaceAll("protocol.", "")
  .replaceAll("[]TaskTypeParam{", "[]TaskParam{")
  .trim();

// collect which types got specs
const specTypes = [...transformed.matchAll(/Type:\s*TaskType(\w+)/g)].map(
  (m) => `TaskType${m[1]}`,
);

// ── all constants from AllTaskTypes ─────────────────────────────────────
const allBlock = protoSrc.slice(
  protoSrc.indexOf("func AllTaskTypes()"),
  protoSrc.indexOf("}", protoSrc.indexOf("return []string{")),
);
const allConsts = [...allBlock.matchAll(/^\t\t(TaskType\w+),$/gm)].map((m) => m[1]);
if (allConsts.length === 0) throw new Error("no AllTaskTypes constants parsed");

const missing = allConsts.filter((c) => !specTypes.includes(c));

// humanize CamelCase constant suffix: Kerberoast -> "Kerberoast"
const humanize = (c) =>
  c.replace(/^TaskType/, "")
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/_/g, " ");

const minimalSpecs = missing
  .map(
    (c) => `\t\t{Type: ${c}, Name: ${JSON.stringify(humanize(c))}, Category: "other"},`,
  )
  .join("\n");

// ── emit taskspec_data.go ───────────────────────────────────────────────
const out = `package protocol

// Code generated from internal/server/tasktypes.go by migrate-taskspec.mjs.
// The declarations below are the SINGLE SOURCE OF TRUTH for task metadata:
// the teamserver derives its metadata API and the agent derives aliases/help
// from this registry. Edit here, not in the consumers.

func init() {
\tfor _, s := range []TaskSpec{
${transformed}
${minimalSpecs}
\t} {
\t\tRegisterTaskSpec(s)
\t}
}
`;
fs.writeFileSync(dataOutPath, out);

console.log(`specs translocated: ${specTypes.length}`);
console.log(`minimal specs added: ${missing.length}`);
console.log(`missing list: ${missing.join(", ")}`);
