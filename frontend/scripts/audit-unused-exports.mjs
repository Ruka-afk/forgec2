import ts from "typescript";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(fileURLToPath(new URL("..", import.meta.url)));
const srcDir = path.join(root, "src");

const configPath = path.join(root, "tsconfig.json");
const config = ts.readConfigFile(configPath, ts.sys.readFile);
const parsed = ts.parseJsonConfigFileContent(config.config, ts.sys, root);
const program = ts.createProgram(parsed.fileNames, parsed.options);

const norm = (p) => p.replace(/\\/g, "/");
// Inspect imports from every project TypeScript file (including vite.config.ts),
// while limiting the final unused-export report to application source files.
const projectFiles = program
  .getSourceFiles()
  .filter((f) => norm(f.fileName).startsWith(`${norm(root)}/`) && !f.isDeclarationFile)
  .map((f) => ({ path: norm(f.fileName), sf: f }));
const sourceFiles = projectFiles.filter(({ path: fp }) => fp.includes("/src/"));

const existsFile = (p) => program.getSourceFile(p) !== undefined;
// Vite alias map (mirrors vite.config.ts) so next/* shim imports resolve.
const NEXT_ALIASES = {
  "next/link": path.join(srcDir, "lib/next/link.tsx"),
  "next/navigation": path.join(srcDir, "lib/next/navigation.ts"),
  "next/dynamic": path.join(srcDir, "lib/next/dynamic.tsx"),
};
function resolveImport(fromFile, spec) {
  if (NEXT_ALIASES[spec]) return norm(NEXT_ALIASES[spec]);
  let p;
  if (spec.startsWith("@/")) p = path.join(srcDir, spec.slice(2));
  else p = path.resolve(path.dirname(fromFile), spec);
  p = norm(path.normalize(p));
  const candidates = [p, `${p}.tsx`, `${p}.ts`, `${p}.mts`];
  for (const c of candidates) if (existsFile(c)) return c;
  for (const c of candidates) {
    for (const suff of ["/index.tsx", "/index.ts"]) {
      if (existsFile(c + suff)) return c + suff;
    }
  }
  return null;
}

const namedExports = new Map(); // path -> Set(name)
const defaultExportByFile = new Set(); // path
const reexportFrom = new Map(); // path -> Set(modulePath)  (export * )
const namedReexportFrom = new Map(); // path -> Map(modulePath, Set(name))
const importsByFile = new Map(); // path -> { specs: Map(name, modulePath[]), namespaces: Set(modulePath), reexports: [{name?, module}] }

function modFlags(stmt) {
  // Only safe on declaration/statement nodes; binding elements inside the
  // recursion lack parent pointers (no type checker built), so swallow those.
  try {
    return ts.getCombinedModifierFlags(stmt);
  } catch {
    return 0;
  }
}

for (const { path: fp, sf } of projectFiles) {
  const ne = new Set();
  namedExports.set(fp, ne);
  reexportFrom.set(fp, new Set());
  namedReexportFrom.set(fp, new Map());
  importsByFile.set(fp, { specs: new Map(), namespaces: new Set(), reexports: [], dynamicImports: new Set() });

  const visit = (node) => {
    const rec = importsByFile.get(fp);
    if (ts.isCallExpression(node) && ts.isImportKeyword(node.expression) && node.arguments.length > 0) {
      const arg = node.arguments[0];
      if (ts.isStringLiteralLike(arg)) {
        const resolved = resolveImport(fp, arg.text);
        if (resolved && resolved !== fp) {
          rec.dynamicImports.add(resolved);
        }
      }
    }
    if (ts.isImportDeclaration(node)) {
      const m = node.moduleSpecifier?.text;
      if (!m) return;
      const resolved = resolveImport(fp, m);
      if (node.importClause) {
        if (node.importClause.name) {
          // default import -> counts toward default existence
          rec.specs.set(`__default__`, [...(rec.specs.get(`__default__`) || []), resolved ?? m]);
        }
        const nc = node.importClause.namedBindings;
        if (nc) {
          if (ts.isNamespaceImport(nc)) {
            // namespace import covers every export of the module
            if (resolved) rec.namespaces.add(resolved);
          } else if (ts.isNamedImports(nc)) {
            for (const el of nc.elements) {
              const orig = el.propertyName ? el.propertyName.text : el.name.text;
              rec.specs.set(orig, [...(rec.specs.get(orig) || []), resolved ?? m]);
            }
          }
        }
      }
      return;
    }
    if (ts.isExportDeclaration(node) && node.moduleSpecifier) {
      const m = node.moduleSpecifier.text;
      const resolved = resolveImport(fp, m);
      if (node.exportClause && ts.isNamedExports(node.exportClause)) {
        // `export { a as b } from "m"` — usage of a in m, plus re-export bookkeeping
        for (const el of node.exportClause.elements) {
          const orig = el.propertyName ? el.propertyName.text : el.name.text;
          rec.reexports.push({ name: orig, module: resolved ?? m });
        }
        let map = namedReexportFrom.get(fp);
        let set = map.get(m);
        if (!set) {
          set = new Set();
          map.set(m, set);
        }
        for (const el of node.exportClause.elements) {
          set.add(el.propertyName ? el.propertyName.text : el.name.text);
        }
      } else {
        // `export * from "m"` — all of m's exports are re-exported
        reexportFrom.get(fp).add(m);
        if (resolved) {
          namedExports.get(fp)?.add("*");
          rec.reexports.push({ name: undefined, module: resolved });
        }
      }
      return;
    }
    if (ts.isExportAssignment(node)) {
      // `export default X` — X is NOT importable by name as a named export.
      defaultExportByFile.add(fp);
      return;
    }
    const flags = modFlags(node);
    if (flags & ts.ModifierFlags.Default) {
      // default-exported function/class names are NOT importable by name —
      // do not register them as named exports (avoids false positives).
      defaultExportByFile.add(fp);
    }
    if (flags & ts.ModifierFlags.Export && !(flags & ts.ModifierFlags.Default)) {
      if (ts.isVariableStatement(node)) {
        for (const decl of node.declarationList.declarations) {
          if (ts.isIdentifier(decl.name)) namedExports.get(fp)?.add(decl.name.text);
        }
      } else if (
        (ts.isFunctionDeclaration(node) || ts.isClassDeclaration(node) || ts.isInterfaceDeclaration(node) ||
          ts.isTypeAliasDeclaration(node) || ts.isEnumDeclaration(node) || ts.isModuleDeclaration(node)) &&
        node.name
      ) {
        namedExports.get(fp)?.add(node.name.text);
      }
    }
    ts.forEachChild(node, visit);
  };
  ts.forEachChild(sf, visit);
}

// expand barrels: for each file, materialize what names flow through `export * from` / `export { } from`
function expandedNames(fp, seen = new Set()) {
  if (seen.has(fp)) return new Set();
  seen.add(fp);
  const own = new Set(namedExports.get(fp) || []);
  if (own.has("*")) own.delete("*");
  for (const m of reexportFrom.get(fp) || []) {
    const resolved = resolveImport(fp, m);
    if (resolved) for (const n of expandedNames(resolved, seen)) own.add(n);
  }
  for (const [m, names] of namedReexportFrom.get(fp) || []) {
    const resolved = resolveImport(fp, m);
    if (resolved) {
      const modNames = expandedNames(resolved, new Set());
      for (const n of names) if (modNames.has(n) || namedExports.get(resolved)?.has(n)) own.add(n);
    }
  }
  return own;
}

// count usage: name N exported from file M is used if some other file imports N
// from a path resolving directly to M, OR imports any name from a barrel that
// re-exports M's N, OR namespace-imports M / a barrel of M.
const used = new Map(); // path -> Set(name)
for (const { path: fp } of projectFiles) {
  used.set(fp, new Set());
  if (defaultExportByFile.has(fp)) used.get(fp).add("__default__");
}
const nameToFiles = new Map(); // name -> Set(filePath)
for (const { path: fp } of projectFiles) for (const n of namedExports.get(fp) || []) {
  if (!nameToFiles.has(n)) nameToFiles.set(n, new Set());
  nameToFiles.get(n).add(fp);
}

for (const { path: fp } of projectFiles) {
  const rec = importsByFile.get(fp);
  for (const [name, modules] of rec.specs) {
    for (const m of modules) {
      if (m === fp) {
        // self-import: not external usage
        continue;
      }
      if (name === "__default__") {
        if (defaultExportByFile.has(m)) used.get(m).add("__default__");
        continue;
      }
      const files = nameToFiles.get(name);
      if (!files) continue;
      if (files.has(m)) {
        used.get(m).add(name);
        continue;
      }
      // barrel? if the resolved import path re-exports this name, mark every
      // declaring file as used (conservative — same-name collisions are safe).
      if (expandedNames(m).has(name)) {
        for (const f of files) used.get(f).add(name);
      }
    }
  }
  for (const m of rec.namespaces) {
    // namespace import of a module references ALL its exports
    if (m === fp) continue;
    used.set(m, new Set([...(used.get(m) || []), ...(namedExports.get(m) || [])]));
  }
  for (const m of rec.dynamicImports) {
    // dynamic import of a module (lazy()/then()) references its exports
    if (m === fp) continue;
    used.set(m, new Set([...(used.get(m) || []), ...(namedExports.get(m) || [])]));
  }
  for (const { name, module } of rec.reexports) {
    if (module === fp) continue;
    if (name === undefined) {
      // export * from module — every export is re-exported (used)
      used.set(module, new Set([...(used.get(module) || []), ...(namedExports.get(module) || [])]));
      continue;
    }
    const files = nameToFiles.get(name);
    if (!files) continue;
    if (files.has(module)) {
      used.get(module).add(name);
      continue;
    }
    if (expandedNames(module).has(name)) {
      for (const f of files) used.get(f).add(name);
    }
  }
}

// report
const report = [];
for (const { path: fp } of sourceFiles) {
  const isBarrel = (reexportFrom.get(fp)?.size ?? 0) > 0 || (namedReexportFrom.get(fp)?.size ?? 0) > 0;
  if (isBarrel) continue; // assume barrels are consumed via their names above
  const unused = [...(namedExports.get(fp) || [])].filter((n) => !used.get(fp)?.has(n));
  if (unused.length === 0) continue;
  report.push({ fp: path.relative(srcDir, fp), unused });
}

report.sort((a, b) => b.unused.length - a.unused.length);
for (const { fp, unused } of report) {
  console.log(`${fp}:`);
  for (const n of unused) console.log(`    - ${n}`);
}
console.log(`\n${report.length} files with potentially unused named exports.`);
