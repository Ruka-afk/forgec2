#!/usr/bin/env node
// check-permissions.mjs — verify src/lib/permission-keys.ts is freshly
// generated from internal/db/models.go (i.e. `npm run gen:permissions` has
// been run after any backend permission change). Mirrors check-openapi-types:
// regeneration is fast enough to run in-process instead of comparing hashes.
//
// Usage: node scripts/check-permissions.mjs   (from frontend/)
// Exit codes: 0 = fresh, 1 = stale (run `npm run gen:permissions`)
import { existsSync, readFileSync } from "node:fs";
import { filePath, generate } from "./gen-permissions.mjs";

const committedFile = filePath();
const fresh = generate();

if (!existsSync(committedFile) || readFileSync(committedFile, "utf8") !== fresh) {
  console.error("FAIL: src/lib/permission-keys.ts is stale vs internal/db/models.go.");
  console.error("Run from frontend/:  npm run gen:permissions");
  process.exit(1);
}
console.log("OK: src/lib/permission-keys.ts matches internal/db/models.go");