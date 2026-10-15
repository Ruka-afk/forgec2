#!/usr/bin/env node
// normalize-webdist.mjs — rewrite CRLF line endings to LF in the embedded
// frontend bundle (internal/webdist/dist).
//
// Why this exists: `npm run build` on Windows emits CRLF inside the generated
// HTML/CSS, because vite's static export copies the line endings of
// frontend/index.html, and a core.autocrlf=true checkout made that template
// CRLF. Linux emits LF. The committed dist therefore has to be LF or CI's
// byte-compare against its own Linux build fails on 142 .html files.
//
// Verified: normalizing the Windows build output reproduces the Linux artifact
// byte for byte. For example internal/webdist/dist/css/xterm.css is 7397 B from
// a Windows build and 7112 B from a Linux build; after normalization the two
// are identical.
//
// Binary assets (woff2 fonts, images) are never touched.
//
// Usage: node scripts/normalize-webdist.mjs [dir]
// Exit codes: 0 = done, 1 = could not normalize
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, extname } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const distDir = process.argv[2] ?? join(root, "internal", "webdist", "dist");

// Only extensions that are unambiguously text. Anything else (woff2, png, …)
// is copied through untouched — rewriting bytes in a font would corrupt it.
const TEXT_EXT = new Set([
  ".js",
  ".mjs",
  ".css",
  ".html",
  ".json",
  ".svg",
  ".txt",
  ".xml",
  ".map",
  ".webmanifest",
]);

let files = 0;
let rewritten = 0;
let bytesSaved = 0;
try {
  const stack = [distDir];
  while (stack.length) {
    const cur = stack.pop();
    for (const entry of readdirSync(cur, { withFileTypes: true })) {
      const abs = join(cur, entry.name);
      if (entry.isDirectory()) {
        stack.push(abs);
        continue;
      }
      if (!TEXT_EXT.has(extname(entry.name).toLowerCase())) continue;
      files += 1;
      const buf = readFileSync(abs);
      // Cheap pre-check: only decode when a CR is actually present.
      if (!buf.includes(13)) continue;
      const lf = Buffer.from(buf.toString("utf8").replaceAll("\r\n", "\n"), "utf8");
      if (lf.equals(buf)) continue;
      writeFileSync(abs, lf);
      rewritten += 1;
      bytesSaved += buf.length - lf.length;
    }
  }
} catch (err) {
  console.error("normalize-webdist: failed:", err.message);
  process.exit(1);
}

if (rewritten === 0) {
  console.log(`normalize-webdist: ${files} text file(s) scanned, already LF.`);
} else {
  console.log(
    `normalize-webdist: rewrote ${rewritten}/${files} text file(s) CRLF -> LF (${bytesSaved} bytes saved).`,
  );
}