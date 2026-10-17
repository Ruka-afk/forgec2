#!/usr/bin/env node
// check-contrast.mjs — WCAG contrast audit over the design tokens.
//
// Parses the token blocks in src/styles/globals.css (:root + .dark), composes
// every pair the UI actually renders (solid fills, 10/15/20% tinted surfaces
// over the page background, chart marks, rings) and checks the WCAG 2.x
// contrast ratio: 4.5:1 for normal text, 3:1 for large text / non-text UI.
//
// This began as the (2026-10) UI/UX audit prototype and stays report-only:
// it exits 0 unless --strict is passed, so it can run beside the existing
// gates while findings are being fixed (U1 promotes it into `npm run check`).
//
// Usage:
//   node scripts/check-contrast.mjs            # report, always exit 0
//   node scripts/check-contrast.mjs --strict   # exit 1 on text failures
//   node scripts/check-contrast.mjs --theme=dark
//   node scripts/check-contrast.mjs --selftest # verify the color math
//
// Limitations (deliberate, prototype honesty):
//   - Alpha compositing runs in gamma-encoded sRGB space, matching how
//     browsers blend `bg-x/NN` over a surface (cross-checked against the
//     audit's canvas/pixel harness on the built app).
//   - Only token-level pairs are checked. A one-off class combination
//     (`bg-warning text-warning-foreground` is listed; a bespoke
//     `hover:bg-x/37` is not) needs the runtime sampler or eyeballs.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

/* ── color math ─────────────────────────────────────────────────────────── */

function oklchToLinear(l, c, hDeg) {
  const h = (hDeg * Math.PI) / 180;
  const a = c * Math.cos(h);
  const b = c * Math.sin(h);
  const l_ = l + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = l - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = l - 0.0894841775 * a - 1.291485548 * b;
  const L = l_ * l_ * l_;
  const M = m_ * m_ * m_;
  const S = s_ * s_ * s_;
  return [
    4.0767416621 * L - 3.3077115913 * M + 0.2309699292 * S,
    -1.2684380046 * L + 2.6097574011 * M - 0.3413193965 * S,
    -0.0041960863 * L - 0.7034186147 * M + 1.707614701 * S,
  ];
}

const clamp01 = (x) => Math.min(1, Math.max(0, x));

function luminance(linear) {
  const [r, g, b] = linear.map(clamp01);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function hexToLinear(hex) {
  const h = hex.replace("#", "");
  const full = h.length === 3 ? h.split("").map((ch) => ch + ch).join("") : h;
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16) / 255);
  const toLinear = (v) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
  return [toLinear(r), toLinear(g), toLinear(b)];
}

/** Parse one CSS color value → { linear, alpha } (null when unsupported). */
function parseColor(value) {
  const v = value.trim();
  const oklch = v.match(
    /^oklch\(\s*([\d.]+%?)\s+([\d.]+%?)\s+([\d.]+)(?:deg|rad|grad|turn)?\s*(?:\/\s*([\d.]+%?)\s*)?\)$/i,
  );
  if (oklch) {
    const num = (s, scale) => (s.endsWith("%") ? (parseFloat(s) / 100) * scale : parseFloat(s));
    const alpha = oklch[4] === undefined ? 1 : Math.min(1, num(oklch[4], 1));
    return { linear: oklchToLinear(num(oklch[1], 1), num(oklch[2], 0.4), parseFloat(oklch[3])), alpha };
  }
  const hex = v.match(/^#[0-9a-f]{3,8}$/i);
  if (hex && (v.length === 4 || v.length === 7)) {
    return { linear: hexToLinear(v), alpha: 1 };
  }
  return null;
}

/** Deterministic composite over an opaque backdrop. Alpha blending happens in
 *  gamma-encoded sRGB space, matching how browsers composite `bg-x/NN` over a
 *  surface (verified against the audit's canvas/pixel harness). */
function linearToSrgb(c) {
  return c <= 0.0031308 ? c * 12.92 : 1.055 * c ** (1 / 2.4) - 0.055;
}

function srgbToLinear(v) {
  return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
}

function over(fg, bg) {
  if (fg.alpha >= 1) return fg.linear;
  return fg.linear.map((c, i) =>
    srgbToLinear(linearToSrgb(c) * fg.alpha + linearToSrgb(bg.linear[i]) * (1 - fg.alpha)),
  );
}

function contrast(fg, bg) {
  const l1 = luminance(over(fg, bg));
  const l2 = luminance(bg.linear);
  const [hi, lo] = l1 >= l2 ? [l1, l2] : [l2, l1];
  return (hi + 0.05) / (lo + 0.05);
}

/* ── token extraction ───────────────────────────────────────────────────── */

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const CSS_PATH = join(root, "src", "styles", "globals.css");

function stripComments(css) {
  return css.replace(/\/\*[\s\S]*?\*\//g, "");
}

/** Extract custom props from every selector block whose text matches `sel`. */
function collectTokens(css, sel) {
  const props = {};
  const blockRe = /([^{}]+)\{([^{}]*)\}/g;
  let m;
  while ((m = blockRe.exec(css)) !== null) {
    if (!sel.test(m[1].trim())) continue;
    const declRe = /--([a-z0-9-]+)\s*:\s*([^;]+);/gi;
    let d;
    while ((d = declRe.exec(m[2])) !== null) {
      props[d[1]] = d[2].trim();
    }
  }
  return props;
}

function loadThemes() {
  const css = stripComments(readFileSync(CSS_PATH, "utf8"));
  const lightRaw = collectTokens(css, /^:root$/);
  const darkRaw = { ...lightRaw, ...collectTokens(css, /^\.dark$/) };
  const build = (raw) => {
    const map = new Map();
    for (const [name, value] of Object.entries(raw)) {
      const parsed = parseColor(value);
      if (parsed) map.set(name, parsed);
    }
    return map;
  };
  return { light: build(lightRaw), dark: build(darkRaw) };
}

/* ── pair matrix ────────────────────────────────────────────────────────── */

// kind: "text" → 4.5:1 required; "nontext" → 3:1 required;
//       "info"  → printed with its ratio, never fails.
function buildPairs(t) {
  const pairs = [];
  const add = (kind, fgName, bgName, label = `${fgName} on ${bgName}`) => {
    const fg = t.get(fgName);
    const bg = t.get(bgName);
    if (fg && bg) pairs.push({ kind, label, fg, bg, fgName, bgName });
  };
  const addTint = (kind, fgName, tintName, pct, overName) => {
    const fg = t.get(fgName);
    const tint = t.get(tintName);
    const overBg = t.get(overName);
    if (!fg || !tint || !overBg) return;
    const bg = { linear: over({ linear: tint.linear, alpha: pct / 100 }, overBg), alpha: 1 };
    pairs.push({
      kind,
      label: `${fgName} on ${tintName}/${pct}% over ${overName}`,
      fg,
      bg,
      fgName,
      bgName: `${tintName}@${pct}%/${overName}`,
    });
  };

  // Page text on every surface it really sits on.
  for (const surface of ["background", "card", "popover", "surface-1", "surface-2", "surface-3", "surface-4", "sidebar"]) {
    add("text", "foreground", surface);
    add("text", "muted-foreground", surface);
    add("text", "card-foreground", surface);
  }
  add("text", "foreground", "muted");
  add("text", "muted-foreground", "muted");
  add("text", "foreground", "secondary");
  add("text", "muted-foreground", "secondary");
  add("text", "foreground", "accent");
  add("text", "muted-foreground", "accent");
  add("text", "muted-foreground", "table-header");
  add("text", "muted-foreground", "table-stripe");
  add("text", "secondary-foreground", "secondary");
  add("text", "accent-foreground", "accent");
  add("text", "sidebar-foreground", "sidebar");
  add("text", "sidebar-accent-foreground", "sidebar-accent");
  add("text", "primary-foreground", "primary");
  add("text", "popover-foreground", "popover");

  // Status tokens (three roles; see docs/frontend-style-conventions.md):
  //   a) `text-X-ink` — text-safe status color on neutral surfaces and on the
  //      tinted chips (`bg-X/8..20`) badges/banners/method pills render;
  //   b) `text-X-foreground` — ink on solid `bg-X` / `bg-X-solid` fills;
  //   c) bare `text-X` — legacy pattern, kept tracked until the U1b page
  //      migration moves every call site to the ink tokens.
  const statuses = ["warning", "info", "success", "destructive"];
  for (const s of statuses) {
    add("text", `${s}-ink`, "background");
    add("text", `${s}-ink`, "card");
    add("text", `${s}-ink`, "sidebar");
    for (const pct of [8, 10, 12, 15, 20]) {
      addTint("text", `${s}-ink`, s, pct, "background");
      addTint("text", `${s}-ink`, s, pct, "card");
      addTint("text", `${s}-ink`, s, pct, "sidebar");
    }
    add("text", s, "background");
    add("text", s, "card");
    for (const pct of [8, 10, 12, 15, 20]) {
      addTint("text", s, s, pct, "background");
      addTint("text", s, s, pct, "card");
    }
    if (s !== "destructive") {
      for (const pct of [10, 15, 20]) {
        addTint("text", `${s}-foreground`, s, pct, "background");
        addTint("text", `${s}-foreground`, s, pct, "card");
      }
    }
  }
  // Ink on solid fills (chips, banners, solid buttons).
  add("text", "success-foreground", "success", "success-foreground on success (solid)");
  add("text", "info-foreground", "info", "info-foreground on info (solid)");
  add("text", "warning-foreground", "warning-solid", "warning-foreground on warning-solid (solid)");
  add("text", "destructive-foreground", "destructive-solid", "destructive-foreground on destructive-solid (solid)");
  // primary as text: links, active nav (`bg-primary/12 text-primary`).
  add("text", "primary", "background");
  add("text", "primary", "card");
  for (const pct of [10, 12, 15, 20]) {
    addTint("text", "primary", "primary", pct, "background");
    addTint("text", "primary", "primary", pct, "card");
    addTint("text", "primary", "primary", pct, "sidebar");
  }

  add("text", "code-text", "code-bg");

  // Chart series must be distinguishable from the plate they are drawn on.
  for (let i = 1; i <= 6; i++) {
    add("nontext", `chart-${i}`, "card");
    add("nontext", `chart-${i}`, "background");
  }

  // Focus ring and input boundary (WCAG 1.4.11 candidates — informational).
  add("nontext", "ring", "background", "ring on background");
  add("nontext", "ring", "card", "ring on card");
  add("info", "input", "card", "input boundary on card");
  add("info", "border", "card", "border on card");
  add("info", "border", "background", "border on background");

  // Avatar glyphs on avatar fills.
  for (let i = 1; i <= 8; i++) {
    add("text", `avatar-${i}-fg`, `avatar-${i}`);
  }

  return pairs;
}

/* ── selftest: pin the math ─────────────────────────────────────────────── */

function selftest() {
  const white = parseColor("oklch(1 0 0)");
  const black = parseColor("oklch(0 0 0)");
  const checks = [
    ["white/black = 21", contrast(white, black), 21, 0.05],
    ["black/white = 21", contrast(black, white), 21, 0.05],
    ["#777/#fff ≈ 4.48", contrast(parseColor("#777"), parseColor("#ffffff")), 4.478, 0.01],
    ["#777/#000 ≈ 4.69", contrast(parseColor("#777"), parseColor("#000000")), 4.689, 0.01],
    [
      "50% white over black = sRGB 0.5 (linear 0.2140)",
      luminance(over({ linear: white.linear, alpha: 0.5 }, black)),
      0.214,
      0.01,
    ],
  ];
  let failed = 0;
  for (const [name, got, want, tol] of checks) {
    const ok = Math.abs(got - want) <= tol;
    if (!ok) failed += 1;
    console.log(`${ok ? "ok  " : "FAIL"} ${name} — got ${got.toFixed(3)}`);
  }
  process.exitCode = failed ? 1 : 0;
  console.log(failed ? `selftest: ${failed} failure(s)` : "selftest: all color-math checks pass");
}

/* ── main ───────────────────────────────────────────────────────────────── */

function fmt(ratio) {
  return ratio.toFixed(2).padStart(5);
}

function main() {
  const args = process.argv.slice(2);
  if (args.includes("--selftest")) return selftest();

  const strict = args.includes("--strict");
  const only = args.find((a) => a.startsWith("--theme="))?.split("=")[1];
  const themes = loadThemes();
  const names = only ? [only] : ["light", "dark"];

  let textFailures = 0;
  let nontextFailures = 0;
  let infoSubThree = 0;
  let total = 0;

  for (const theme of names) {
    const tokens = themes[theme];
    if (!tokens || tokens.size === 0) {
      console.error(`check-contrast: no tokens parsed for theme "${theme}"`);
      process.exitCode = 1;
      return;
    }
    console.log(`\n[${theme}] ${tokens.size} color tokens`);
    const pairs = buildPairs(tokens);
    for (const p of pairs) {
      total += 1;
      const ratio = contrast(p.fg, p.bg);
      const need = p.kind === "text" ? 4.5 : p.kind === "nontext" ? 3.0 : 0;
      const pass = need === 0 ? ratio >= 3.0 : ratio >= need;
      let tag;
      if (p.kind === "text" && !pass) {
        tag = "FAIL  ";
        textFailures += 1;
      } else if (p.kind === "nontext" && !pass) {
        tag = "nontext";
        nontextFailures += 1;
      } else if (p.kind === "info" && !pass) {
        tag = "info  ";
        infoSubThree += 1;
      } else {
        tag = "ok    ";
      }
      console.log(`  ${tag} ${fmt(ratio)}  ${p.kind === "text" ? "text " : p.kind}  ${p.label}`);
    }
  }

  console.log(`\nchecked ${total} pairs across ${names.join("+")}`);
  console.log(`text failures (below 4.5): ${textFailures}`);
  console.log(`non-text failures (below 3.0): ${nontextFailures} (ring/chart marks)`);
  console.log(`informational below 3.0 (input/border boundaries): ${infoSubThree}`);
  if (strict && textFailures > 0) {
    console.error(`\nFAIL: ${textFailures} text pair(s) below 4.5:1 — fix the tokens or the usage.`);
    process.exitCode = 1;
  }
}

main();
