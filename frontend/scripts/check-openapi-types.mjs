#!/usr/bin/env node
// check-openapi-types.mjs — verify src/lib/api-schema.d.ts is freshly generated
// from ../api/openapi.yaml (i.e. `npm run gen:openapi` has been run after any
// spec change). Mirrors check-webdist: regeneration here is fast enough to run
// in-process instead of comparing hashes.
//
// Usage: node scripts/check-openapi-types.mjs   (from frontend/)
// Exit codes: 0 = fresh, 1 = stale (run `npm run gen:openapi`)
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, readFileSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const frontendDir = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const committedFile = join(frontendDir, "src", "lib", "api-schema.d.ts");

if (!existsSync(committedFile)) {
  console.error("FAIL: src/lib/api-schema.d.ts does not exist.");
  console.error("Run:  npm run gen:openapi");
  process.exit(1);
}

const tmp = mkdtempSync(join(tmpdir(), "openapi-types-"));
const genFile = join(tmp, "api-schema.d.ts");
try {
  execFileSync(
    process.execPath,
    [
      join(frontendDir, "node_modules", "openapi-typescript", "bin", "cli.js"),
      join(frontendDir, "..", "api", "openapi.yaml"),
      "-o",
      genFile,
    ],
    { stdio: "pipe", cwd: frontendDir }
  );
} catch (err) {
  console.error("FAIL: could not regenerate types from api/openapi.yaml:", err.message);
  rmSync(tmp, { recursive: true, force: true });
  process.exit(1);
}

const sha = (f) => createHash("sha256").update(readFileSync(f)).digest("hex");
const committedHash = sha(committedFile);
const freshHash = sha(genFile);
rmSync(tmp, { recursive: true, force: true });

if (committedHash !== freshHash) {
  console.error("FAIL: src/lib/api-schema.d.ts is stale vs api/openapi.yaml.");
  console.error("Run from frontend/:  npm run gen:openapi   (then re-run this check)");
  process.exit(1);
}
console.log("OK: src/lib/api-schema.d.ts matches api/openapi.yaml");