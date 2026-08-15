/**
 * Rewrite `from "@/components/UI"` to per-file `@/components/ui/*` imports.
 */
import fs from "node:fs";
import path from "node:path";

const ROOT = path.resolve(import.meta.dirname, "..", "src");

const DEST = {
  Spinner: "@/components/ui/spinner",
  PageSpinner: "@/components/ui/spinner",
  PageHeader: "@/components/ui/page-header",
  EmptyState: "@/components/ui/empty-state",
  StatusIndicator: "@/components/ui/status-indicator",
  StatusBadge: "@/components/ui/status-indicator",
  Pagination: "@/components/ui/pagination",
  ConfirmModal: "@/components/ui/confirm-modal",
  CopyButton: "@/components/ui/copy-button",
  MdContent: "@/components/ui/md-content",
  AvatarFallback: "@/components/ui/avatar",
  Breadcrumb: "@/components/ui/breadcrumb",
  IconBadge: "@/components/ui/icon-badge",
  FieldError: "@/components/ui/field-error",
  StatCard: "@/components/ui/animated-stat-card",
  DataState: "@/components/ui/data-state",
  DataSpinner: "@/components/ui/data-state",
  DataError: "@/components/ui/data-state",
  ChartCard: "@/components/ChartCard",
  Separator: "@/components/ui/separator",
};

function walk(dir, out = []) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(p, out);
    else if (/\.(tsx|ts)$/.test(ent.name)) out.push(p);
  }
  return out;
}

const importRe = /^import\s+\{([^}]+)\}\s+from\s+["']@\/components\/UI["'];?\s*$/gm;

let changed = 0;
for (const file of walk(ROOT)) {
  if (file.endsWith(`${path.sep}UI.tsx`)) continue;
  const src = fs.readFileSync(file, "utf8").replace(/^\uFEFF/, "");
  if (!src.includes("@/components/UI")) continue;

  const next = src.replace(importRe, (_m, names) => {
    const specs = names.split(",").map((s) => s.trim()).filter(Boolean);
    const byDest = new Map();
    for (const spec of specs) {
      const ident = spec.split(/\s+as\s+/)[0].trim();
      const dest = DEST[ident];
      if (!dest) {
        throw new Error(`${file}: unknown UI export "${ident}"`);
      }
      if (!byDest.has(dest)) byDest.set(dest, []);
      byDest.get(dest).push(spec);
    }
    return [...byDest.entries()]
      .map(([dest, list]) => `import { ${list.join(", ")} } from "${dest}";`)
      .join("\n");
  });

  if (next === src) {
    console.warn("unhandled UI import:", path.relative(ROOT, file));
    continue;
  }
  fs.writeFileSync(file, next);
  changed++;
}

console.log(`rewrote ${changed} files`);
