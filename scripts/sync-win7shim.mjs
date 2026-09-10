// sync-win7shim.mjs — mirror parent-module sources for legacy (Win7) builds.
//
// The Win7 agent build compiles with the go1.20 toolchain, which refuses
// packages from the go-1.25 main module. The build therefore resolves
// internal/crypto + pkg/{protocol,encoding} from a stub module whose
// SOURCES MUST BE AVAILABLE INSIDE THE SERVER BINARY — i.e. embedded.
// go:embed cannot reference parent directories, so this script mirrors the
// canonical sources into internal/payload/win7shim/ (embedded via win7shimFS).
//
// Usage:
//   node scripts/sync-win7shim.mjs          # refresh the mirror
//   node scripts/sync-win7shim.mjs --check  # CI freshness gate (exit 1 on drift)
import { copyFileSync, mkdirSync, readdirSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const dest = join(root, "internal", "payload", "win7shim");
const sources = [
  ["internal", "crypto", "crypto"],
  ["pkg", "protocol", "protocol"],
  ["pkg", "encoding", "encoding"],
];

function mirrorTo(target) {
  rmSync(target, { recursive: true, force: true });
  let files = 0;
  for (const [top, srcDir, name] of sources) {
    const src = join(root, top, srcDir);
    const out = join(target, name);
    mkdirSync(out, { recursive: true });
    for (const file of readdirSync(src)) {
      if (!file.endsWith(".go") || file.endsWith("_test.go")) continue;
      copyFileSync(join(src, file), join(out, file));
      files++;
    }
  }
  return files;
}

function snapshot(dir) {
  // Map of relpath -> content for comparison.
  const map = new Map();
  const walk = (cur, rel) => {
    for (const entry of readdirSync(cur, { withFileTypes: true })) {
      if (entry.isDirectory()) walk(join(cur, entry.name), `${rel}${entry.name}/`);
      else map.set(`${rel}${entry.name}`, readFileSync(join(cur, entry.name), "utf8"));
    }
  };
  walk(dir, "");
  return map;
}

const check = process.argv.includes("--check");
if (check) {
  const { mkdtempSync } = await import("node:fs");
  const tmp = mkdtempSync(join(tmpdir(), "win7shim-"));
  try {
    mirrorTo(tmp);
    const fresh = snapshot(tmp);
    const committed = snapshot(dest);
    const drift = [];
    for (const [rel, content] of fresh) {
      if (committed.get(rel) !== content) drift.push(`M ${rel}`);
    }
    for (const rel of committed.keys()) {
      if (!fresh.has(rel)) drift.push(`D ${rel}`);
    }
    if (drift.length > 0) {
      console.error("win7shim mirror is stale: run `node scripts/sync-win7shim.mjs` and commit the result.");
      for (const d of drift) console.error(`  ${d}`);
      process.exit(1);
    }
    console.log(`win7shim fresh (${fresh.size} files)`);
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
} else {
  const files = mirrorTo(dest);
  console.log(`win7shim synced (${files} files, 3 packages)`);
}
