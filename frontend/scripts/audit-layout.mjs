#!/usr/bin/env node
// Layout audit — measures the operator console's layout contract against a
// running server (U9 program). The harness is deliberately read-only: it
// logs in, visits the operator routes at four viewports and reports the
// geometry that the layout batches are supposed to guarantee.
//
//   FORGEC2_USER=labtest FORGEC2_PASS=... node scripts/audit-layout.mjs
//   ... --json audit.json            # machine-readable report
//   ... --routes /listeners,/beacons  # narrow the route set
//
// Exit code 0 when every measured page is inside its contract, 1 otherwise
// (a page overflows horizontally, or a filter row scrolls away while the
// page still has content below it).
import { chromium } from "playwright";
import { writeFileSync } from "node:fs";

const DEFAULT_ROUTES = [
  "/dashboard", "/beacons", "/listeners", "/events", "/loot",
  "/credentials", "/plugins", "/audit", "/tokens", "/settings",
];

const VIEWPORTS = [
  { name: "390", width: 390, height: 844 },
  { name: "768", width: 768, height: 1024 },
  { name: "1024", width: 1024, height: 768 },
  { name: "1440", width: 1440, height: 900 },
];

function argValue(flag) {
  const i = process.argv.indexOf(flag);
  return i >= 0 ? process.argv[i + 1] : undefined;
}

const base = process.env.FORGEC2_BASE || "http://127.0.0.1:8000";
const user = process.env.FORGEC2_USER;
const pass = process.env.FORGEC2_PASS;
const jsonOut = argValue("--json");
const routeArg = argValue("--routes");
const routes = routeArg
  ? routeArg.split(",").map((r) => (r.startsWith("/") ? r : `/${r}`))
  : DEFAULT_ROUTES;

if (!user || !pass) {
  console.error("audit-layout: set FORGEC2_USER and FORGEC2_PASS (a console account with read access).");
  process.exit(2);
}

/** Measures the layout contract of the page currently in the DOM. */
const measure = () => {
  const doc = document.documentElement;
  const pc = document.querySelector('[data-slot="page-container"]');
  const main = document.querySelector("main");
  const toolbar = document.querySelector('[data-slot="page-toolbar"]');
  const pcs = pc ? getComputedStyle(pc) : null;
  const body = pc && pc.children.length > 1 ? pc.children[pc.children.length - 1] : null;
  const kids = body
    ? Array.from(body.children).filter((c) => c.getBoundingClientRect().height > 0)
    : [];
  const gaps = [];
  for (let i = 1; i < kids.length; i++) {
    gaps.push(Math.round(kids[i].getBoundingClientRect().top - kids[i - 1].getBoundingClientRect().bottom));
  }
  const tables = Array.from(document.querySelectorAll('[data-slot="table-container"]')).map((t) => {
    const cells = Array.from(t.querySelectorAll('[data-slot="table-cell"], [data-slot="table-head"]'));
    return {
      overflowX: t.scrollWidth - t.clientWidth,
      // A table that scrolls horizontally is acceptable when it pins a
      // column (the operator's actions stay reachable either way). The
      // header counts: a pinned head implies a pinned column, and the head
      // renders even when the table is empty.
      pinned: cells.some((c) => getComputedStyle(c).position === "sticky"),
    };
  });
  return {
    pageOverflowX: doc.scrollWidth - doc.clientWidth,
    contentWidth: pc ? Math.round(pc.getBoundingClientRect().width) : null,
    rowGap: pcs ? pcs.rowGap : null,
    blockGaps: gaps,
    toolbarTop: toolbar ? Math.round(toolbar.getBoundingClientRect().top) : null,
    toolbarSticky: toolbar ? getComputedStyle(toolbar).position : null,
    tableOverflowX: tables.reduce((sum, t) => sum + Math.max(0, t.overflowX), 0),
    tableCount: tables.length,
    tablePinned: tables.every((t) => t.overflowX <= 0 || t.pinned),
    scrollHeight: main ? main.scrollHeight : null,
    clientHeight: main ? main.clientHeight : null,
  };
};

const browser = await chromium.launch();
const report = [];
let violations = 0;

for (const vp of VIEWPORTS) {
  const ctx = await browser.newContext({ viewport: { width: vp.width, height: vp.height } });
  const page = await ctx.newPage();
  await page.goto(`${base}/login`, { waitUntil: "domcontentloaded" });
  await page.fill('input[autocomplete="username"]', user);
  await page.fill('input[autocomplete="current-password"]', pass);
  await page.click('button[type="submit"]');
  await page.waitForTimeout(2500);

  for (const route of routes) {
    await page.goto(base + route, { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(1400);
    const before = await page.evaluate(measure);

    // Scroll the page and re-measure: a sticky filter row must stay pinned.
    let after = null;
    if (before.toolbarTop != null) {
      await page.evaluate(() => {
        const main = document.querySelector("main");
        if (main) main.scrollTop = 800;
      });
      await page.waitForTimeout(350);
      after = await page.evaluate(() => {
        const t = document.querySelector('[data-slot="page-toolbar"]');
        const r = t?.getBoundingClientRect();
        return { top: r ? Math.round(r.top) : null, position: t ? getComputedStyle(t).position : null };
      });
    }

    const row = {
      route,
      viewport: vp.name,
      ...before,
      toolbarTopAfterScroll: after?.top ?? null,
      toolbarPosition: after?.position ?? before.toolbarSticky,
    };
    const problems = [];
    if (row.pageOverflowX > 1) problems.push(`page overflows by ${row.pageOverflowX}px`);
    if (row.tableOverflowX > 0 && !row.tablePinned) {
      problems.push(`table overflows by ${row.tableOverflowX}px without a pinned action column`);
    }
    // A row that started pinned and is no longer pinned has a sticky bug.
    if (after && before.toolbarTop != null && before.toolbarTop < 120 && after.top != null && after.top > 120) {
      problems.push(`filter row unpinned (${before.toolbarTop} → ${after.top})`);
    }
    if (problems.length) violations++;
    report.push({ ...row, problems });
    console.log(
      `${problems.length ? "FAIL" : " ok "} ${vp.name.padStart(4)} ${route.padEnd(14)} ` +
      `w=${String(row.contentWidth).padStart(4)} gap=${row.rowGap} ` +
      `pageOv=${row.pageOverflowX} tblOv=${row.tableOverflowX}/${row.tableCount}${row.tablePinned ? "" : " (unpinned)"} ` +
      `row=${row.toolbarTop ?? "-"}/${row.toolbarTopAfterScroll ?? "-"}` +
      (problems.length ? `  ← ${problems.join("; ")}` : ""),
    );
  }
  await ctx.close();
}

await browser.close();
if (jsonOut) writeFileSync(jsonOut, JSON.stringify(report, null, 2));
console.log(violations === 0 ? "LAYOUT CONTRACT OK" : `${violations} viewport/route combinations violate the contract`);
process.exit(violations === 0 ? 0 : 1);
